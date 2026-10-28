package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// SalesFunnelRepository implements domain.SalesFunnelRepository over the
// sqlc-generated Queries (US-006). Wraps the sales_funnels.sql queries created
// in US-002.
type SalesFunnelRepository struct {
	queries *Queries
}

func NewSalesFunnelRepository(db DBTX) *SalesFunnelRepository {
	return &SalesFunnelRepository{queries: New(db)}
}

func toDomainSalesFunnel(row SalesFunnel) *domain.SalesFunnel {
	schema := json.RawMessage(row.MetadataSchema)
	if len(schema) == 0 {
		schema = json.RawMessage("[]")
	}
	return &domain.SalesFunnel{
		ID:                        row.ID,
		AccountID:                 row.AccountID,
		Name:                      row.Name,
		ProductID:                 row.ProductID,
		AgentID:                   row.AgentID,
		ChannelID:                 row.ChannelID,
		PersonaPrompt:             row.PersonaPrompt,
		MetadataSchema:            schema,
		WebhookURL:                row.WebhookUrl,
		WebhookSecret:             row.WebhookSecret,
		TriggerOnRequiredComplete: row.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        row.TriggerOnBuyIntent,
		Active:                    row.Active,
		IsDefault:                 row.IsDefault,
		CreatedAt:                 row.CreatedAt.Time,
		UpdatedAt:                 row.UpdatedAt.Time,
	}
}

func (r *SalesFunnelRepository) Create(ctx context.Context, funnel *domain.SalesFunnel) error {
	q := getQueries(ctx, r.queries)
	return q.CreateSalesFunnel(ctx, CreateSalesFunnelParams{
		ID:                        funnel.ID,
		AccountID:                 funnel.AccountID,
		Name:                      funnel.Name,
		ProductID:                 funnel.ProductID,
		AgentID:                   funnel.AgentID,
		ChannelID:                 funnel.ChannelID,
		PersonaPrompt:             funnel.PersonaPrompt,
		MetadataSchema:            toBytes(funnel.MetadataSchema),
		WebhookUrl:                funnel.WebhookURL,
		WebhookSecret:             funnel.WebhookSecret,
		TriggerOnRequiredComplete: funnel.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        funnel.TriggerOnBuyIntent,
		Active:                    funnel.Active,
		IsDefault:                 funnel.IsDefault,
		CreatedAt:                 pgtype.Timestamptz{Time: funnel.CreatedAt, Valid: true},
		UpdatedAt:                 pgtype.Timestamptz{Time: funnel.UpdatedAt, Valid: true},
	})
}

func (r *SalesFunnelRepository) GetByID(ctx context.Context, id string) (*domain.SalesFunnel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetSalesFunnelByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainSalesFunnel(row), nil
}

func (r *SalesFunnelRepository) GetByChannelID(ctx context.Context, channelID string) (*domain.SalesFunnel, error) {
	if channelID == "" {
		return nil, nil
	}
	q := getQueries(ctx, r.queries)
	row, err := q.GetSalesFunnelByChannelID(ctx, &channelID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainSalesFunnel(row), nil
}

func (r *SalesFunnelRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.SalesFunnel, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListSalesFunnelsByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	funnels := make([]*domain.SalesFunnel, 0, len(rows))
	for _, row := range rows {
		funnels = append(funnels, toDomainSalesFunnel(row))
	}
	return funnels, nil
}

func (r *SalesFunnelRepository) Update(ctx context.Context, funnel *domain.SalesFunnel) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateSalesFunnel(ctx, UpdateSalesFunnelParams{
		ID:                        funnel.ID,
		Name:                      funnel.Name,
		ProductID:                 funnel.ProductID,
		AgentID:                   funnel.AgentID,
		ChannelID:                 funnel.ChannelID,
		PersonaPrompt:             funnel.PersonaPrompt,
		MetadataSchema:            toBytes(funnel.MetadataSchema),
		WebhookUrl:                funnel.WebhookURL,
		WebhookSecret:             funnel.WebhookSecret,
		TriggerOnRequiredComplete: funnel.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        funnel.TriggerOnBuyIntent,
		Active:                    funnel.Active,
	})
}

func (r *SalesFunnelRepository) Delete(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteSalesFunnel(ctx, id)
}

// EnsureDefault returns the account's default funnel, creating it on first
// call. Safe under concurrent callers: the create is guarded by the partial
// unique index, so a race leaves one row and one loser, and the loser reads
// the winner's row back instead of surfacing the violation.
func (r *SalesFunnelRepository) EnsureDefault(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetDefaultSalesFunnel(ctx, accountID)
	if err == nil {
		return toDomainSalesFunnel(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	created, err := q.EnsureDefaultSalesFunnel(ctx, EnsureDefaultSalesFunnelParams{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		Name:      domain.DefaultFunnelName,
	})
	if err == nil {
		return toDomainSalesFunnel(created), nil
	}
	var pgErr *pgconn.PgError
	raced := errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
	// ErrNoRows here means the NOT EXISTS guard saw a default appear between
	// our read and our insert — same outcome as losing the race.
	if !raced && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	row, err = q.GetDefaultSalesFunnel(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return toDomainSalesFunnel(row), nil
}
