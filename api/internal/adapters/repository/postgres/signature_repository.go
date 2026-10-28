package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

// Electronic signature (§6.11, G7).
type SignatureRepository struct{ db DBTX }

func NewSignatureRepository(db DBTX) *SignatureRepository {
	return &SignatureRepository{db: db}
}

const signatureCols = `id, account_id, contract_id, lead_id, contact_id, provider,
	provider_doc_token, title, status, failure_reason, signers, object_path,
	signed_object_path, value_cents, currency, sale_id, sent_at, viewed_at,
	signed_at, refused_at, created_by_user_id, created_at, updated_at`

func scanSignature(row pgx.Row) (*domain.SignatureRequest, error) {
	r := &domain.SignatureRequest{}
	var contractID, leadID, contactID, saleID, createdBy *string
	var provider, status string
	var signers []byte

	err := row.Scan(&r.ID, &r.AccountID, &contractID, &leadID, &contactID, &provider,
		&r.ProviderDocToken, &r.Title, &status, &r.FailureReason, &signers,
		&r.ObjectPath, &r.SignedObjectPath, &r.ValueCents, &r.Currency, &saleID,
		&r.SentAt, &r.ViewedAt, &r.SignedAt, &r.RefusedAt, &createdBy,
		&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	r.Provider = domain.SignatureProvider(provider)
	r.Status = domain.SignatureStatus(status)
	if contractID != nil {
		r.ContractID = *contractID
	}
	if leadID != nil {
		r.LeadID = *leadID
	}
	if contactID != nil {
		r.ContactID = *contactID
	}
	if saleID != nil {
		r.SaleID = *saleID
	}
	if createdBy != nil {
		r.CreatedByUserID = *createdBy
	}
	// A malformed signer list reads as absent rather than failing the row: a
	// request that loads without its signing links is visible and refreshable;
	// one that will not load hides a signed document.
	_ = json.Unmarshal(signers, &r.Signers)
	return r, nil
}

func (r *SignatureRepository) Create(ctx context.Context, req *domain.SignatureRequest) error {
	signers, err := json.Marshal(orEmpty(req.Signers))
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO signature_requests (id, account_id, contract_id, lead_id,
		    contact_id, provider, provider_doc_token, title, status, failure_reason,
		    signers, object_path, signed_object_path, value_cents, currency,
		    sale_id, sent_at, viewed_at, signed_at, refused_at, created_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		req.ID, req.AccountID, nilIfEmpty(req.ContractID), nilIfEmpty(req.LeadID),
		nilIfEmpty(req.ContactID), string(req.Provider), req.ProviderDocToken,
		req.Title, string(req.Status), req.FailureReason, signers,
		req.ObjectPath, req.SignedObjectPath, req.ValueCents, req.Currency,
		nilIfEmpty(req.SaleID), req.SentAt, req.ViewedAt, req.SignedAt, req.RefusedAt,
		nilIfEmpty(req.CreatedByUserID))
	return err
}

// Update writes the tracked state.
//
// sale_id is deliberately ABSENT: it moves only through AttachSale, whose
// conditional WHERE is the guard against a repeated webhook registering two
// sales. A blanket UPDATE here would let a stale in-memory copy clear it.
func (r *SignatureRepository) Update(ctx context.Context, req *domain.SignatureRequest) error {
	signers, err := json.Marshal(orEmpty(req.Signers))
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE signature_requests
		    SET provider_doc_token = $3, title = $4, status = $5,
		        failure_reason = $6, signers = $7, object_path = $8,
		        signed_object_path = $9, value_cents = $10, currency = $11,
		        sent_at = $12, viewed_at = $13, signed_at = $14, refused_at = $15,
		        contact_id = $16, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`,
		req.ID, req.AccountID, req.ProviderDocToken, req.Title, string(req.Status),
		req.FailureReason, signers, req.ObjectPath, req.SignedObjectPath,
		req.ValueCents, req.Currency, req.SentAt, req.ViewedAt, req.SignedAt,
		req.RefusedAt, nilIfEmpty(req.ContactID))
	return err
}

func (r *SignatureRepository) Get(ctx context.Context, accountID, id string) (*domain.SignatureRequest, error) {
	return scanSignature(r.db.QueryRow(ctx,
		`SELECT `+signatureCols+` FROM signature_requests
		  WHERE id = $1 AND account_id = $2`, id, accountID))
}

func (r *SignatureRepository) GetByProviderToken(
	ctx context.Context, accountID, docToken string,
) (*domain.SignatureRequest, error) {
	if docToken == "" {
		return nil, nil
	}
	return scanSignature(r.db.QueryRow(ctx,
		`SELECT `+signatureCols+` FROM signature_requests
		  WHERE account_id = $1 AND provider_doc_token = $2`, accountID, docToken))
}

func (r *SignatureRepository) List(
	ctx context.Context, accountID string, limit, offset int,
) ([]*domain.SignatureRequest, error) {
	return r.query(ctx,
		`SELECT `+signatureCols+` FROM signature_requests
		  WHERE account_id = $1
		  ORDER BY created_at DESC
		  LIMIT $2 OFFSET $3`, accountID, limit, offset)
}

func (r *SignatureRepository) ListByContract(
	ctx context.Context, accountID, contractID string,
) ([]*domain.SignatureRequest, error) {
	return r.query(ctx,
		`SELECT `+signatureCols+` FROM signature_requests
		  WHERE account_id = $1 AND contract_id = $2
		  ORDER BY created_at DESC`, accountID, contractID)
}

// AttachSale claims or releases the sale slot.
//
// The conditional WHERE is the whole idempotency guarantee: two webhooks arriving
// together both read an empty sale_id, and only one UPDATE finds the row still
// empty. Passing an empty saleID releases the claim, which is what lets a failed
// registration be retried instead of being wedged forever.
func (r *SignatureRepository) AttachSale(
	ctx context.Context, accountID, id, saleID string,
) (bool, error) {
	if saleID == "" {
		tag, err := r.db.Exec(ctx,
			`UPDATE signature_requests SET sale_id = NULL, updated_at = NOW()
			  WHERE id = $1 AND account_id = $2`, id, accountID)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() > 0, nil
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE signature_requests SET sale_id = $3, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2
		    AND (sale_id IS NULL OR sale_id = 'pending')`,
		id, accountID, saleID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *SignatureRepository) query(
	ctx context.Context, sql string, args ...any,
) ([]*domain.SignatureRequest, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.SignatureRequest
	for rows.Next() {
		req, err := scanSignature(rows)
		if err != nil {
			return nil, err
		}
		if req != nil {
			out = append(out, req)
		}
	}
	return out, rows.Err()
}

var _ domain.SignatureRepository = (*SignatureRepository)(nil)

// ── The BYO credential ─────────────────────────────────────────────────────

// SignatureSettingsRepository holds the customer's own provider token.
//
// Encrypted at rest with the same AES-256-GCM primitive as the channel
// credentials, in its BYTE form because the column is bytea. A nil cipher fails
// the WRITE closed (never store a plaintext credential) and treats a stored value
// as absent on READ, so a key rotation cannot hard-fail every fetch.
type SignatureSettingsRepository struct {
	db     DBTX
	cipher *secrets.AESGCMCipher
}

func NewSignatureSettingsRepository(db DBTX, cipher *secrets.AESGCMCipher) *SignatureSettingsRepository {
	return &SignatureSettingsRepository{db: db, cipher: cipher}
}

func (r *SignatureSettingsRepository) Get(ctx context.Context, accountID string) (*domain.SignatureSettings, error) {
	return r.scan(r.db.QueryRow(ctx,
		`SELECT account_id, provider, api_token_encrypted, webhook_token, sandbox,
		        created_at, updated_at
		   FROM signature_settings WHERE account_id = $1`, accountID))
}

func (r *SignatureSettingsRepository) ByWebhookToken(
	ctx context.Context, token string,
) (*domain.SignatureSettings, error) {
	if token == "" {
		return nil, nil
	}
	return r.scan(r.db.QueryRow(ctx,
		`SELECT account_id, provider, api_token_encrypted, webhook_token, sandbox,
		        created_at, updated_at
		   FROM signature_settings WHERE webhook_token = $1`, token))
}

func (r *SignatureSettingsRepository) scan(row pgx.Row) (*domain.SignatureSettings, error) {
	s := &domain.SignatureSettings{}
	var provider string
	var encrypted []byte

	err := row.Scan(&s.AccountID, &provider, &encrypted, &s.WebhookToken,
		&s.Sandbox, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.Provider = domain.SignatureProvider(provider)

	if len(encrypted) > 0 {
		if r.cipher == nil {
			slog.Error("signature settings: no cipher configured — " +
				"the stored provider token cannot be read")
		} else if plain, err := r.cipher.OpenBytes(encrypted); err != nil {
			// A value we cannot open is treated as absent: the account reads as
			// "not configured" and can be re-pasted, instead of every request
			// failing after a key rotation.
			slog.Error("signature settings: stored token could not be decrypted",
				"account_id", s.AccountID, "error", err)
		} else {
			s.APIToken = plain
		}
	}
	s.HasToken = s.APIToken != ""
	return s, nil
}

func (r *SignatureSettingsRepository) Upsert(ctx context.Context, s *domain.SignatureSettings) error {
	var encrypted []byte
	if s.APIToken != "" {
		if r.cipher == nil {
			// Fail closed. Storing a provider credential in plaintext because the
			// deployment forgot a key is not a degradation anybody consented to.
			return &domain.AppError{
				Code:    "SIGNATURE_CIPHER_UNCONFIGURED",
				Message: "esta instalação não está configurada para guardar credenciais com segurança",
			}
		}
		sealed, err := r.cipher.SealBytes(s.APIToken)
		if err != nil {
			return err
		}
		encrypted = sealed
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO signature_settings (account_id, provider, api_token_encrypted,
		    webhook_token, sandbox)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (account_id) DO UPDATE
		    SET provider = EXCLUDED.provider,
		        -- An empty token KEEPS the stored one: the settings screen shows a
		        -- mask, and saving the sandbox checkbox must not wipe the credential.
		        api_token_encrypted = COALESCE(EXCLUDED.api_token_encrypted,
		                                       signature_settings.api_token_encrypted),
		        webhook_token = EXCLUDED.webhook_token,
		        sandbox = EXCLUDED.sandbox,
		        updated_at = NOW()`,
		s.AccountID, string(s.Provider), encrypted, s.WebhookToken, s.Sandbox)
	return err
}

var _ domain.SignatureSettingsRepository = (*SignatureSettingsRepository)(nil)
