package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// CampaignRepository persists campaigns and their recipients.
type CampaignRepository struct {
	queries *Queries
}

func NewCampaignRepository(db DBTX) *CampaignRepository {
	return &CampaignRepository{queries: New(db)}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func varsToBytes(v []string) []byte {
	if len(v) == 0 {
		return []byte("[]")
	}
	b, _ := json.Marshal(v)
	return b
}

func bytesToVars(b []byte) []string {
	var out []string
	if len(b) == 0 {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return out
}

func toPtrTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// ── Campaign mapping ───────────────────────────────────────────────────────

func toDomainCampaign(row Campaign) *domain.Campaign {
	return &domain.Campaign{
		ID:              row.ID,
		AccountID:       row.AccountID,
		ChannelID:       row.ChannelID,
		Name:            row.Name,
		TemplateName:    row.TemplateName,
		TemplateLang:    row.TemplateLang,
		TemplateVars:    bytesToVars(row.TemplateVars),
		Status:          domain.CampaignStatus(row.Status),
		ScheduledAt:     toPtrTime(row.ScheduledAt),
		StartedAt:       toPtrTime(row.StartedAt),
		FinishedAt:      toPtrTime(row.FinishedAt),
		TotalRecipients: int(row.TotalRecipients),
		SentCount:       int(row.SentCount),
		DeliveredCount:  int(row.DeliveredCount),
		ReadCount:       int(row.ReadCount),
		FailedCount:     int(row.FailedCount),
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

func toDomainRecipient(row CampaignRecipient) *domain.CampaignRecipient {
	return &domain.CampaignRecipient{
		ID:            row.ID,
		CampaignID:    row.CampaignID,
		Recipient:     row.Recipient,
		Vars:          bytesToVars(row.Vars),
		Status:        domain.RecipientStatus(row.Status),
		ProviderMsgID: row.ProviderMsgID,
		ErrorMessage:  row.ErrorMessage,
		SentAt:        toPtrTime(row.SentAt),
		DeliveredAt:   toPtrTime(row.DeliveredAt),
		ReadAt:        toPtrTime(row.ReadAt),
		FailedAt:      toPtrTime(row.FailedAt),
		CreatedAt:     row.CreatedAt.Time,
	}
}

// ── Repository methods ─────────────────────────────────────────────────────

func (r *CampaignRepository) Create(ctx context.Context, c *domain.Campaign) error {
	q := getQueries(ctx, r.queries)
	var scheduledAt pgtype.Timestamptz
	if c.ScheduledAt != nil {
		scheduledAt = pgtype.Timestamptz{Time: *c.ScheduledAt, Valid: true}
	}
	return q.CreateCampaign(ctx, CreateCampaignParams{
		ID:              c.ID,
		AccountID:       c.AccountID,
		ChannelID:       c.ChannelID,
		Name:            c.Name,
		TemplateName:    c.TemplateName,
		TemplateLang:    c.TemplateLang,
		TemplateVars:    varsToBytes(c.TemplateVars),
		Status:          string(c.Status),
		ScheduledAt:     scheduledAt,
		TotalRecipients: int32(c.TotalRecipients),
		CreatedBy:       c.CreatedBy,
	})
}

func (r *CampaignRepository) GetByID(ctx context.Context, id, accountID string) (*domain.Campaign, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetCampaignByID(ctx, GetCampaignByIDParams{ID: id, AccountID: accountID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainCampaign(row), nil
}

func (r *CampaignRepository) ListByAccount(ctx context.Context, accountID string, limit, offset int32) ([]*domain.Campaign, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListCampaignsByAccount(ctx, ListCampaignsByAccountParams{
		AccountID: accountID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Campaign, len(rows))
	for i, row := range rows {
		out[i] = toDomainCampaign(row)
	}
	return out, nil
}

func (r *CampaignRepository) UpdateStatus(ctx context.Context, id string, status domain.CampaignStatus) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateCampaignStatus(ctx, UpdateCampaignStatusParams{ID: id, Status: string(status)})
}

func (r *CampaignRepository) IncrementCounter(ctx context.Context, id string, status domain.RecipientStatus) error {
	q := getQueries(ctx, r.queries)
	return q.IncrementCampaignCounter(ctx, IncrementCampaignCounterParams{ID: id, Column2: string(status)})
}

func (r *CampaignRepository) Delete(ctx context.Context, id, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteCampaign(ctx, DeleteCampaignParams{ID: id, AccountID: accountID})
}

func (r *CampaignRepository) ListScheduledDue(ctx context.Context, limit int32) ([]*domain.Campaign, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListScheduledCampaignsDue(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Campaign, len(rows))
	for i, row := range rows {
		out[i] = toDomainCampaign(row)
	}
	return out, nil
}

func (r *CampaignRepository) CreateRecipient(ctx context.Context, rec *domain.CampaignRecipient) error {
	q := getQueries(ctx, r.queries)
	return q.CreateRecipient(ctx, CreateRecipientParams{
		ID:         rec.ID,
		CampaignID: rec.CampaignID,
		Recipient:  rec.Recipient,
		Vars:       varsToBytes(rec.Vars),
	})
}

func (r *CampaignRepository) ListRecipients(ctx context.Context, campaignID string, limit, offset int32) ([]*domain.CampaignRecipient, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListRecipientsByCampaign(ctx, ListRecipientsByCampaignParams{
		CampaignID: campaignID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.CampaignRecipient, len(rows))
	for i, row := range rows {
		out[i] = toDomainRecipient(row)
	}
	return out, nil
}

func (r *CampaignRepository) ListPendingRecipients(ctx context.Context, campaignID string, limit int32) ([]*domain.CampaignRecipient, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListPendingRecipients(ctx, ListPendingRecipientsParams{
		CampaignID: campaignID,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.CampaignRecipient, len(rows))
	for i, row := range rows {
		out[i] = toDomainRecipient(row)
	}
	return out, nil
}

func (r *CampaignRepository) MarkRecipientSent(ctx context.Context, recipientID, providerMsgID string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkRecipientSent(ctx, MarkRecipientSentParams{ID: recipientID, ProviderMsgID: &providerMsgID})
}

func (r *CampaignRepository) MarkRecipientFailed(ctx context.Context, recipientID, errorMessage string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkRecipientFailed(ctx, MarkRecipientFailedParams{ID: recipientID, ErrorMessage: &errorMessage})
}

func (r *CampaignRepository) UpdateRecipientStatusByProviderID(ctx context.Context, providerMsgID string, status domain.RecipientStatus) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateRecipientStatusByProviderID(ctx, UpdateRecipientStatusByProviderIDParams{
		ProviderMsgID: &providerMsgID,
		Status:        string(status),
	})
}

func (r *CampaignRepository) GetCampaignByProviderMsgID(ctx context.Context, providerMsgID string) (*domain.Campaign, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetCampaignByRecipientProviderID(ctx, &providerMsgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainCampaign(row), nil
}
