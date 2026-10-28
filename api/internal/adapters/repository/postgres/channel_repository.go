package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

// --- ChannelRepository ---

// ChannelRepository persists channels. Sensitive config values
// (domain.SensitiveConfigKey) are encrypted at rest with the shared
// AES-256-GCM cipher (see channel_config_crypto.go); everything above
// this layer reads and writes plaintext. cipher is nil-able: writes of
// sensitive values then fail closed (CHANNEL_CIPHER_UNCONFIGURED),
// reads of already-encrypted values are logged and treated as absent.
type ChannelRepository struct {
	queries *Queries
	cipher  *secrets.AESGCMCipher
}

func NewChannelRepository(db DBTX, cipher *secrets.AESGCMCipher) *ChannelRepository {
	return &ChannelRepository{queries: New(db), cipher: cipher}
}

func (r *ChannelRepository) GetByWebhookToken(ctx context.Context, token string) (*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetChannelByWebhookToken(ctx, &token)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapChannel(row), nil
}

func (r *ChannelRepository) GetByPhoneNumberID(ctx context.Context, phoneNumberID string) (*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetChannelByPhoneNumberID(ctx, phoneNumberID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapChannel(row), nil
}

func (r *ChannelRepository) GetByWABAID(ctx context.Context, wabaID string) (*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetChannelByWABAID(ctx, wabaID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapChannel(row), nil
}

// ListByWABAID returns every active WhatsApp channel under the given
// Meta WABA. Used by the webhook fan-out so events scoped to a WABA
// (template approvals, account review, quality updates) reach every
// tenant sharing the WABA — the single-row GetByWABAID version
// silently leaked events from one account to another.
func (r *ChannelRepository) ListByWABAID(ctx context.Context, wabaID string) ([]*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListChannelsByWABAID(ctx, wabaID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Channel, 0, len(rows))
	for _, row := range rows {
		out = append(out, r.mapChannel(row))
	}
	return out, nil
}

// UpdateCredentialField writes a single credential key via jsonb_set so
// other keys keep their value. Required so the WhatsApp token refresh
// worker (which rotates access_token every ~50 days) doesn't undo the
// admin handler's registration_status / subscribe_status writes that
// happened between the worker's read and write.
func (r *ChannelRepository) UpdateCredentialField(ctx context.Context, channelID, key, value string) error {
	if value != "" && domain.SensitiveConfigKey(key) {
		sealed, err := encryptCredentialValue(r.cipher, value)
		if err != nil {
			return err
		}
		value = sealed
	}
	q := getQueries(ctx, r.queries)
	return q.UpdateChannelCredentialString(ctx, UpdateChannelCredentialStringParams{
		Key:   key,
		Value: value,
		ID:    channelID,
	})
}

// MergeCredentials writes multiple credential keys in one statement
// via JSONB `||`. Empty-string values delete the key (mapped to JSON
// null then jsonb_strip_nulls equivalent — but `||` keeps nulls; we
// instead just emit them as JSON null and let the consumer treat
// missing == null == ""). Used by the WhatsApp handlers that touch
// 5+ status fields per call so concurrent writers don't clobber.
func (r *ChannelRepository) MergeCredentials(ctx context.Context, channelID string, fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	// Copy before sealing so the caller's map never sees ciphertext.
	toStore := make(map[string]string, len(fields))
	for key, value := range fields {
		if value != "" && domain.SensitiveConfigKey(key) {
			sealed, err := encryptCredentialValue(r.cipher, value)
			if err != nil {
				return err
			}
			value = sealed
		}
		toStore[key] = value
	}
	patch, err := json.Marshal(toStore)
	if err != nil {
		return err
	}
	q := getQueries(ctx, r.queries)
	return q.MergeChannelCredentials(ctx, MergeChannelCredentialsParams{
		Patch: patch,
		ID:    channelID,
	})
}

// mapChannel converts a DB row to the domain shape, opening any
// at-rest-encrypted credential values so callers only ever see
// plaintext config.
func (r *ChannelRepository) mapChannel(row Channel) *domain.Channel {
	row.Credentials = decryptSensitiveConfig(r.cipher, row.Credentials, row.ID)
	return mapChannelToDomain(row)
}

func mapChannelToDomain(row Channel) *domain.Channel {
	ch := &domain.Channel{
		ID:        row.ID,
		AccountID: row.AccountID,
		AgentID:   row.AgentID,
		Type:      row.Provider,
		Name:      row.Name,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
	if row.Credentials != nil {
		ch.Config = row.Credentials
	}
	if row.IsActive != nil {
		ch.IsActive = *row.IsActive
	}
	if row.WebhookToken != nil {
		ch.WebhookToken = *row.WebhookToken
	}
	return ch
}

func (r *ChannelRepository) Create(ctx context.Context, ch *domain.Channel) error {
	q := getQueries(ctx, r.queries)
	token := ch.WebhookToken
	creds, err := encryptSensitiveConfig(r.cipher, toBytes(ch.Config))
	if err != nil {
		return err
	}
	_, err = q.CreateChannel(ctx, CreateChannelParams{
		ID:           ch.ID,
		AccountID:    ch.AccountID,
		AgentID:      ch.AgentID,
		Provider:     ch.Type,
		Name:         ch.Name,
		Credentials:  creds,
		WebhookToken: &token,
	})
	return mapChannelCreateErr(err)
}

// mapChannelCreateErr translates known DB constraint violations into
// typed domain errors so handlers can return user-actionable 4xx
// instead of leaking pgconn details. Today we only catch the WhatsApp
// phone-number partial unique index from migration 000048; add more
// branches as new constraints land.
func mapChannelCreateErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
		return err
	}
	if strings.Contains(pgErr.ConstraintName, "whatsapp_phone_number_id") {
		return domain.ErrWAPhoneInUse
	}
	return err
}

func (r *ChannelRepository) GetByID(ctx context.Context, id string) (*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetChannelByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapChannel(row), nil
}

func (r *ChannelRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListChannelsByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Channel, 0, len(rows))
	for _, row := range rows {
		result = append(result, r.mapChannel(row))
	}
	return result, nil
}

func (r *ChannelRepository) ListByAgentID(ctx context.Context, agentID string) ([]*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListChannelsByAgentID(ctx, &agentID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Channel, 0, len(rows))
	for _, row := range rows {
		result = append(result, r.mapChannel(row))
	}
	return result, nil
}

func (r *ChannelRepository) ListAllByProvider(ctx context.Context, provider string) ([]*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAllChannelsByProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Channel, 0, len(rows))
	for _, row := range rows {
		result = append(result, r.mapChannel(row))
	}
	return result, nil
}

func (r *ChannelRepository) Update(ctx context.Context, ch *domain.Channel) error {
	q := getQueries(ctx, r.queries)
	creds, err := encryptSensitiveConfig(r.cipher, toBytes(ch.Config))
	if err != nil {
		return err
	}
	row, err := q.UpdateChannel(ctx, UpdateChannelParams{
		ID:          ch.ID,
		Name:        ch.Name,
		Credentials: creds,
		AgentID:     ch.AgentID,
	})
	if err != nil {
		return err
	}
	updated := r.mapChannel(row)
	*ch = *updated
	return nil
}

func (r *ChannelRepository) Delete(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteChannel(ctx, id)
}

func (r *ChannelRepository) Toggle(ctx context.Context, id string, isActive bool) (*domain.Channel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.ToggleChannel(ctx, ToggleChannelParams{ID: id, IsActive: &isActive})
	if err != nil {
		return nil, err
	}
	return r.mapChannel(row), nil
}
