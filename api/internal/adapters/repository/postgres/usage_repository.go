package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// dateToPtr converts a nullable pgtype.Date to *time.Time (nil when unset).
func dateToPtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

// --- UsageRepository ---

type UsageRepository struct {
	queries *Queries
	db      DBTX
}

func NewUsageRepository(db DBTX) *UsageRepository {
	return &UsageRepository{queries: New(db), db: db}
}

// getUsageSummaryByAccountID reads the precomputed latest-cycle row from the
// account_usage_summary materialized view (US-065). Column order MUST match
// the Scan target order in GetSummaryByAccountID.
const getUsageSummaryByAccountID = `
SELECT id, account_id, billing_cycle_start, conversations_count,
       ai_tokens_used, ai_input_tokens_used, ai_output_tokens_used,
       addon_tokens, daily_tokens, daily_tokens_date, storage_used_mb,
       broadcast_messages_used, warned_80_percent, warned_100_percent, updated_at
FROM account_usage_summary
WHERE account_id = $1`

// GetSummaryByAccountID returns the account's usage from the materialized view
// (refreshed every 5 min by UsageSummaryRefreshWorker). It backs the read-only
// dashboard endpoint; live enforcement uses GetByAccountID against the table.
// Returns (nil, nil) when no row exists so callers fall back to defaults.
func (r *UsageRepository) GetSummaryByAccountID(ctx context.Context, accountID string) (*domain.AccountUsage, error) {
	u := &domain.AccountUsage{}
	var billingCycleStart, dailyTokensDate pgtype.Date
	var updatedAt pgtype.Timestamptz
	var broadcastUsed int32
	err := r.db.QueryRow(ctx, getUsageSummaryByAccountID, accountID).Scan(
		&u.ID,
		&u.AccountID,
		&billingCycleStart,
		&u.ConversationsCount,
		&u.AITokensUsed,
		&u.AIInputTokensUsed,
		&u.AIOutputTokensUsed,
		&u.AddonTokens,
		&u.DailyTokens,
		&dailyTokensDate,
		&u.StorageUsedMB,
		&broadcastUsed,
		&u.Warned80Percent,
		&u.Warned100Percent,
		&updatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.BillingCycleStart = billingCycleStart.Time
	u.DailyTokensDate = dateToPtr(dailyTokensDate)
	u.BroadcastMessagesUsed = &broadcastUsed
	u.UpdatedAt = updatedAt.Time
	return u, nil
}

// RefreshUsageSummary recomputes the account_usage_summary materialized view.
// CONCURRENTLY keeps readers on the previous snapshot during the refresh and
// requires the unique index created alongside the view; it must run outside a
// transaction, so this method runs directly on the pool (not via getQueries).
func (r *UsageRepository) RefreshUsageSummary(ctx context.Context) error {
	_, err := r.db.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY account_usage_summary")
	return err
}

func (r *UsageRepository) Create(ctx context.Context, usage *domain.AccountUsage) error {
	q := getQueries(ctx, r.queries)
	return q.CreateUsage(ctx, CreateUsageParams{
		ID:                 usage.ID,
		AccountID:          usage.AccountID,
		BillingCycleStart:  pgtype.Date{Time: usage.BillingCycleStart, Valid: true},
		ConversationsCount: usage.ConversationsCount,
		AiTokensUsed:       usage.AITokensUsed,
		StorageUsedMb:      usage.StorageUsedMB,
		UpdatedAt:          pgtype.Timestamptz{Time: usage.UpdatedAt, Valid: true},
	})
}

func (r *UsageRepository) GetByAccountID(ctx context.Context, accountID string) (*domain.AccountUsage, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetUsageByAccountID(ctx, accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	usage := &domain.AccountUsage{
		ID:                        row.ID,
		AccountID:                 row.AccountID,
		BillingCycleStart:         row.BillingCycleStart.Time,
		ConversationsCount:        row.ConversationsCount,
		AITokensUsed:              row.AiTokensUsed,
		AIInputTokensUsed:         row.AiInputTokensUsed,
		AIOutputTokensUsed:        row.AiOutputTokensUsed,
		AddonTokens:               row.AddonTokens,
		DailyTokens:               row.DailyTokens,
		DailyTokensDate:           dateToPtr(row.DailyTokensDate),
		StorageUsedMB:             row.StorageUsedMb,
		BroadcastMessagesUsed:     &row.BroadcastMessagesUsed,
		Warned80Percent:           row.Warned80Percent,
		Warned100Percent:          row.Warned100Percent,
		OverageConversationsCount: row.OverageConversationsCount,
		OverageCentsCharged:       row.OverageCentsCharged,
		UpdatedAt:                 row.UpdatedAt.Time,
	}
	if row.CapReachedAt.Valid {
		t := row.CapReachedAt.Time
		usage.CapReachedAt = &t
	}
	return usage, nil
}

func (r *UsageRepository) Update(ctx context.Context, usage *domain.AccountUsage) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateUsage(ctx, UpdateUsageParams{
		ID:                 usage.ID,
		ConversationsCount: usage.ConversationsCount,
		AiTokensUsed:       usage.AITokensUsed,
		StorageUsedMb:      usage.StorageUsedMB,
		UpdatedAt:          pgtype.Timestamptz{Time: usage.UpdatedAt, Valid: true},
	})
}

func (r *UsageRepository) AddUsage(ctx context.Context, accountID string, tokensToAdd int64, inputTokens int64, outputTokens int64, conversationsToAdd int32) error {
	q := getQueries(ctx, r.queries)

	return q.AddUsage(ctx, AddUsageParams{
		AccountID:          accountID,
		AiTokensUsed:       &tokensToAdd,
		AiInputTokensUsed:  &inputTokens,
		AiOutputTokensUsed: &outputTokens,
		ConversationsCount: &conversationsToAdd,
	})
}

func (r *UsageRepository) AddAddonTokens(ctx context.Context, accountID string, tokens int64) error {
	q := getQueries(ctx, r.queries)
	return q.AddAddonTokens(ctx, AddAddonTokensParams{
		AccountID:   accountID,
		AddonTokens: &tokens,
	})
}

// AddBroadcastMessages atomically increments broadcast_messages_used for the
// account's current usage row. Called at campaign creation to reserve budget
// pre-flight against the plan cap.
func (r *UsageRepository) AddBroadcastMessages(ctx context.Context, accountID string, count int32) error {
	q := getQueries(ctx, r.queries)
	return q.AddBroadcastMessages(ctx, AddBroadcastMessagesParams{
		AccountID:             accountID,
		BroadcastMessagesUsed: count,
	})
}

func (r *UsageRepository) ResetUsage(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ResetUsage(ctx, accountID)
}

func (r *UsageRepository) SetUsageWarned80(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetUsageWarned80(ctx, accountID)
}

func (r *UsageRepository) SetUsageWarned100(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetUsageWarned100(ctx, accountID)
}

func (r *UsageRepository) ListOverageCandidates(ctx context.Context) ([]domain.OverageCandidate, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListOverageCandidates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.OverageCandidate, 0, len(rows))
	for _, row := range rows {
		customerID := ""
		if row.StripeCustomerID != nil {
			customerID = *row.StripeCustomerID
		}
		out = append(out, domain.OverageCandidate{
			AccountID:                row.AccountID,
			StripeCustomerID:         customerID,
			Currency:                 row.Currency,
			PlanPriceCents:           int(row.PlanPriceCents),
			MaxConversationsPerMonth: int(row.MaxConversationsPerMonth),
			OverageConversationCents: int(row.OverageConversationCents),
			OverageCapMultiplier:     row.OverageCapMultiplier,
			ConversationsCount:       row.ConversationsCount,
			OverageCentsCharged:      row.OverageCentsCharged,
			CapReached:               row.CapReached,
		})
	}
	return out, nil
}

func (r *UsageRepository) GetOverageCandidate(ctx context.Context, accountID string) (*domain.OverageCandidate, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetOverageCandidate(ctx, accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	customerID := ""
	if row.StripeCustomerID != nil {
		customerID = *row.StripeCustomerID
	}
	return &domain.OverageCandidate{
		AccountID:                row.AccountID,
		StripeCustomerID:         customerID,
		Currency:                 row.Currency,
		PlanPriceCents:           int(row.PlanPriceCents),
		MaxConversationsPerMonth: int(row.MaxConversationsPerMonth),
		OverageConversationCents: int(row.OverageConversationCents),
		OverageCapMultiplier:     row.OverageCapMultiplier,
		ConversationsCount:       row.ConversationsCount,
		OverageCentsCharged:      row.OverageCentsCharged,
		CapReached:               row.CapReached,
	}, nil
}

func (r *UsageRepository) RecordOverageCharge(ctx context.Context, accountID string, overageConvs, totalCentsCharged int32, capReached bool) error {
	q := getQueries(ctx, r.queries)
	return q.RecordOverageCharge(ctx, RecordOverageChargeParams{
		AccountID:                 accountID,
		OverageConversationsCount: overageConvs,
		OverageCentsCharged:       totalCentsCharged,
		CapReached:                capReached,
	})
}
