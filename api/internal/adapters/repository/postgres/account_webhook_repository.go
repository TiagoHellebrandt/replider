package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// AccountWebhookRepository persists tenant-managed outbound webhook
// subscriptions. The dispatcher reads from this on every event emit
// so the hot path is `ListActiveForEvent` — backed by the partial
// account+is_active index from migration 000050.
type AccountWebhookRepository struct {
	queries *Queries
}

func NewAccountWebhookRepository(db DBTX) *AccountWebhookRepository {
	return &AccountWebhookRepository{queries: New(db)}
}

func (r *AccountWebhookRepository) Create(ctx context.Context, w *domain.AccountWebhook) (*domain.AccountWebhook, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.CreateAccountWebhook(ctx, CreateAccountWebhookParams{
		ID:          w.ID,
		AccountID:   w.AccountID,
		Url:         w.URL,
		Secret:      w.Secret,
		Events:      w.Events,
		IsActive:    w.IsActive,
		Description: w.Description,
	})
	if err != nil {
		return nil, err
	}
	return mapAccountWebhook(row), nil
}

func (r *AccountWebhookRepository) GetByID(ctx context.Context, accountID, id string) (*domain.AccountWebhook, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAccountWebhook(ctx, GetAccountWebhookParams{
		ID:        id,
		AccountID: accountID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapAccountWebhook(row), nil
}

func (r *AccountWebhookRepository) ListByAccount(ctx context.Context, accountID string) ([]*domain.AccountWebhook, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAccountWebhooks(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AccountWebhook, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAccountWebhook(row))
	}
	return out, nil
}

func (r *AccountWebhookRepository) ListActiveForEvent(ctx context.Context, accountID, event string) ([]*domain.AccountWebhook, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListActiveWebhooksForEvent(ctx, ListActiveWebhooksForEventParams{
		AccountID: accountID,
		EventName: event,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AccountWebhook, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAccountWebhook(row))
	}
	return out, nil
}

func (r *AccountWebhookRepository) Update(ctx context.Context, w *domain.AccountWebhook) (*domain.AccountWebhook, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.UpdateAccountWebhook(ctx, UpdateAccountWebhookParams{
		ID:          w.ID,
		AccountID:   w.AccountID,
		Url:         w.URL,
		Events:      w.Events,
		IsActive:    w.IsActive,
		Description: w.Description,
	})
	if err != nil {
		return nil, err
	}
	return mapAccountWebhook(row), nil
}

func (r *AccountWebhookRepository) Delete(ctx context.Context, accountID, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteAccountWebhook(ctx, DeleteAccountWebhookParams{
		ID:        id,
		AccountID: accountID,
	})
}

func (r *AccountWebhookRepository) RecordSuccess(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.RecordWebhookSuccess(ctx, id)
}

func (r *AccountWebhookRepository) RecordFailure(ctx context.Context, id string, errMsg string) error {
	q := getQueries(ctx, r.queries)
	return q.RecordWebhookFailure(ctx, RecordWebhookFailureParams{
		ID:        id,
		LastError: &errMsg,
	})
}

func (r *AccountWebhookRepository) Disable(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DisableWebhook(ctx, id)
}

func mapAccountWebhook(row AccountWebhook) *domain.AccountWebhook {
	w := &domain.AccountWebhook{
		ID:           row.ID,
		AccountID:    row.AccountID,
		URL:          row.Url,
		Secret:       row.Secret,
		Events:       row.Events,
		IsActive:     row.IsActive,
		Description:  row.Description,
		LastError:    row.LastError,
		FailureCount: int(row.FailureCount),
	}
	if row.CreatedAt.Valid {
		w.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		w.UpdatedAt = row.UpdatedAt.Time
	}
	if row.LastFiredAt.Valid {
		t := row.LastFiredAt.Time
		w.LastFiredAt = &t
	}
	if row.LastSucceededAt.Valid {
		t := row.LastSucceededAt.Time
		w.LastSucceededAt = &t
	}
	return w
}
