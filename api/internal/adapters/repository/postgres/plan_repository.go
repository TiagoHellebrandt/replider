package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// --- PlanRepository ---

type PlanRepository struct {
	queries *Queries
}

func NewPlanRepository(db DBTX) *PlanRepository {
	return &PlanRepository{queries: New(db)}
}

func (r *PlanRepository) GetByID(ctx context.Context, id string) (*domain.Plan, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetPlanByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	inputMult, _ := row.InputMultiplier.Float64Value()
	outputMult, _ := row.OutputMultiplier.Float64Value()

	capMult, _ := row.OverageCapMultiplier.Float64Value()

	return &domain.Plan{
		ID:                             row.ID,
		Name:                           row.Name,
		StripePriceID:                  row.StripePriceID,
		StripePriceIDAnnual:            row.StripePriceIDAnnual,
		Currency:                       row.Currency,
		Price:                          int(row.Price),
		AIProvider:                     row.AiProvider,
		AIModel:                        row.AiModel,
		InputMultiplier:                inputMult.Float64,
		OutputMultiplier:               outputMult.Float64,
		MaxAgents:                      int(row.MaxAgents),
		MaxChannels:                    int(row.MaxChannels),
		MaxSeats:                       int(row.MaxSeats),
		MaxConversationsPerMonth:       int(row.MaxConversationsPerMonth),
		MaxAITokensPerMonth:            row.MaxAiTokensPerMonth,
		MaxDailyAITokens:               row.MaxDailyAiTokens,
		MaxStorageMB:                   int(row.MaxStorageMb),
		MaxBroadcastMessagesPerMonth:   int(row.MaxBroadcastMessagesPerMonth),
		MaxRecipientsPerCampaign:       int(row.MaxRecipientsPerCampaign),
		MaxLeadsPerMonth:               int(row.MaxLeadsPerMonth),
		MaxEnrichmentsPerMonth:         int(row.MaxEnrichmentsPerMonth),
		MaxAIDraftsPerMonth:            int(row.MaxAiDraftsPerMonth),
		MaxDocumentExtractionsPerMonth: int(row.MaxDocumentExtractionsPerMonth),
		MaxEmailMailboxes:              int(row.MaxEmailMailboxes),
		MaxFunnels:                     int(row.MaxFunnels),
		OverageConversationCents:       int(row.OverageConversationCents),
		OverageCapMultiplier:           capMult.Float64,
		IsListed:                       row.IsListed,
		CreatedAt:                      row.CreatedAt.Time,
		UpdatedAt:                      row.UpdatedAt.Time,
	}, nil
}

// GetByStripePriceID resolves the plan a Stripe price id belongs to,
// matching either the monthly or annual price column. Returns (nil, nil)
// when no plan maps to the price — the caller decides how loud to be.
// A price id matching MORE than one plan (misconfigured seed) is also
// (nil, nil) with an error log: refusing to guess routes the webhook to
// its plan_activation_failed alert instead of silently activating an
// arbitrary tier (US-023).
func (r *PlanRepository) GetByStripePriceID(ctx context.Context, priceID string) (*domain.Plan, error) {
	if priceID == "" {
		return nil, nil
	}
	q := getQueries(ctx, r.queries)
	rows, err := q.GetPlanByStripePriceID(ctx, &priceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) > 1 {
		slog.Error("stripe price id maps to more than one plan — refusing to guess, treating as unmapped",
			"price_id", priceID, "plan_ids", []string{rows[0].ID, rows[1].ID})
		return nil, nil
	}
	row := rows[0]

	inputMult, _ := row.InputMultiplier.Float64Value()
	outputMult, _ := row.OutputMultiplier.Float64Value()
	capMult, _ := row.OverageCapMultiplier.Float64Value()

	return &domain.Plan{
		ID:                             row.ID,
		Name:                           row.Name,
		StripePriceID:                  row.StripePriceID,
		StripePriceIDAnnual:            row.StripePriceIDAnnual,
		Currency:                       row.Currency,
		Price:                          int(row.Price),
		AIProvider:                     row.AiProvider,
		AIModel:                        row.AiModel,
		InputMultiplier:                inputMult.Float64,
		OutputMultiplier:               outputMult.Float64,
		MaxAgents:                      int(row.MaxAgents),
		MaxChannels:                    int(row.MaxChannels),
		MaxSeats:                       int(row.MaxSeats),
		MaxConversationsPerMonth:       int(row.MaxConversationsPerMonth),
		MaxAITokensPerMonth:            row.MaxAiTokensPerMonth,
		MaxDailyAITokens:               row.MaxDailyAiTokens,
		MaxStorageMB:                   int(row.MaxStorageMb),
		MaxBroadcastMessagesPerMonth:   int(row.MaxBroadcastMessagesPerMonth),
		MaxRecipientsPerCampaign:       int(row.MaxRecipientsPerCampaign),
		MaxLeadsPerMonth:               int(row.MaxLeadsPerMonth),
		MaxEnrichmentsPerMonth:         int(row.MaxEnrichmentsPerMonth),
		MaxAIDraftsPerMonth:            int(row.MaxAiDraftsPerMonth),
		MaxDocumentExtractionsPerMonth: int(row.MaxDocumentExtractionsPerMonth),
		MaxEmailMailboxes:              int(row.MaxEmailMailboxes),
		MaxFunnels:                     int(row.MaxFunnels),
		OverageConversationCents:       int(row.OverageConversationCents),
		OverageCapMultiplier:           capMult.Float64,
		IsListed:                       row.IsListed,
		CreatedAt:                      row.CreatedAt.Time,
		UpdatedAt:                      row.UpdatedAt.Time,
	}, nil
}

func (r *PlanRepository) ListAll(ctx context.Context) ([]*domain.Plan, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAllPlans(ctx)
	if err != nil {
		return nil, err
	}

	plans := make([]*domain.Plan, len(rows))
	for i, row := range rows {
		inputMult, _ := row.InputMultiplier.Float64Value()
		outputMult, _ := row.OutputMultiplier.Float64Value()
		capMult, _ := row.OverageCapMultiplier.Float64Value()
		plans[i] = &domain.Plan{
			ID:                             row.ID,
			Name:                           row.Name,
			StripePriceID:                  row.StripePriceID,
			Currency:                       row.Currency,
			Price:                          int(row.Price),
			AIProvider:                     row.AiProvider,
			AIModel:                        row.AiModel,
			InputMultiplier:                inputMult.Float64,
			OutputMultiplier:               outputMult.Float64,
			MaxAgents:                      int(row.MaxAgents),
			MaxChannels:                    int(row.MaxChannels),
			MaxSeats:                       int(row.MaxSeats),
			MaxConversationsPerMonth:       int(row.MaxConversationsPerMonth),
			MaxAITokensPerMonth:            row.MaxAiTokensPerMonth,
			MaxDailyAITokens:               row.MaxDailyAiTokens,
			MaxStorageMB:                   int(row.MaxStorageMb),
			MaxBroadcastMessagesPerMonth:   int(row.MaxBroadcastMessagesPerMonth),
			MaxRecipientsPerCampaign:       int(row.MaxRecipientsPerCampaign),
			MaxLeadsPerMonth:               int(row.MaxLeadsPerMonth),
			MaxEnrichmentsPerMonth:         int(row.MaxEnrichmentsPerMonth),
			MaxAIDraftsPerMonth:            int(row.MaxAiDraftsPerMonth),
			MaxDocumentExtractionsPerMonth: int(row.MaxDocumentExtractionsPerMonth),
			MaxEmailMailboxes:              int(row.MaxEmailMailboxes),
			MaxFunnels:                     int(row.MaxFunnels),
			OverageConversationCents:       int(row.OverageConversationCents),
			OverageCapMultiplier:           capMult.Float64,
			IsListed:                       row.IsListed,
			CreatedAt:                      row.CreatedAt.Time,
			UpdatedAt:                      row.UpdatedAt.Time,
		}
	}
	return plans, nil
}
