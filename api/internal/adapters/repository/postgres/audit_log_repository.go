package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- AuditLogRepository ---

type AuditLogRepository struct {
	queries *Queries
}

func NewAuditLogRepository(db DBTX) *AuditLogRepository {
	return &AuditLogRepository{queries: New(db)}
}

func (r *AuditLogRepository) Create(ctx context.Context, logObj *domain.AuditLog) error {
	// Stamp the request origin (IP + User-Agent) from the context unless
	// the caller set it explicitly, so every admin mutation records
	// "from where" without each write site threading the gin request
	// down (US-073).
	logObj.FillMetaFromContext(ctx)
	q := getQueries(ctx, r.queries)
	return q.CreateAuditLog(ctx, CreateAuditLogParams{
		ID:         logObj.ID,
		AccountID:  logObj.AccountID,
		ActorID:    logObj.ActorID,
		Action:     logObj.Action,
		EntityType: logObj.EntityType,
		EntityID:   logObj.EntityID,
		Payload:    logObj.Payload,
		Ip:         logObj.IP,
		UserAgent:  logObj.UserAgent,
		CreatedAt:  pgtype.Timestamptz{Time: logObj.CreatedAt, Valid: true},
	})
}

func (r *AuditLogRepository) ListByAccount(ctx context.Context, accountID string, limit, offset int32) ([]*domain.AuditLog, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAuditLogsByAccount(ctx, ListAuditLogsByAccountParams{
		AccountID: accountID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AuditLog, len(rows))
	for i, row := range rows {
		out[i] = &domain.AuditLog{
			ID:         row.ID,
			AccountID:  row.AccountID,
			ActorID:    row.ActorID,
			Action:     row.Action,
			EntityType: row.EntityType,
			EntityID:   row.EntityID,
			Payload:    row.Payload,
			IP:         row.Ip,
			UserAgent:  row.UserAgent,
			CreatedAt:  row.CreatedAt.Time,
		}
	}
	return out, nil
}

func (r *AuditLogRepository) CountByAccount(ctx context.Context, accountID string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.CountAuditLogsByAccount(ctx, accountID)
}

func (r *AuditLogRepository) ListFiltered(ctx context.Context, f domain.AuditLogFilter) ([]*domain.AuditLog, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAuditLogsFiltered(ctx, ListAuditLogsFilteredParams{
		AccountID:   f.AccountID,
		Action:      f.Action,
		EntityType:  f.EntityType,
		ActorID:     f.ActorID,
		FromTs:      timeToTimestamptz(f.From),
		ToTs:        timeToTimestamptz(f.To),
		LimitCount:  f.Limit,
		OffsetCount: f.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AuditLog, len(rows))
	for i, row := range rows {
		out[i] = &domain.AuditLog{
			ID:         row.ID,
			AccountID:  row.AccountID,
			ActorID:    row.ActorID,
			Action:     row.Action,
			EntityType: row.EntityType,
			EntityID:   row.EntityID,
			Payload:    row.Payload,
			IP:         row.Ip,
			UserAgent:  row.UserAgent,
			CreatedAt:  row.CreatedAt.Time,
		}
	}
	return out, nil
}

func (r *AuditLogRepository) CountFiltered(ctx context.Context, f domain.AuditLogFilter) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.CountAuditLogsFiltered(ctx, CountAuditLogsFilteredParams{
		AccountID:  f.AccountID,
		Action:     f.Action,
		EntityType: f.EntityType,
		ActorID:    f.ActorID,
		FromTs:     timeToTimestamptz(f.From),
		ToTs:       timeToTimestamptz(f.To),
	})
}
