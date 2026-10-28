package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// suggesterMetaPrompt asks the LLM to compare two prompt variants
// against their measured outcomes and propose a v3 body. The strict
// JSON output contract is the single hardest constraint here — without
// it parsing falls over and the user sees a "try again" CTA.
const suggesterMetaPrompt = `You analyze A/B test results for customer-service AI agents.

You receive: variant A body, variant B body, the measured deltas (CSAT, AI-resolved %, tokens, conversation count) and 3-10 sample conversations per variant (best- and worst-rated).

Output STRICTLY valid JSON, no preamble or trailing text, matching this shape:
{
  "rationale": "One sentence explaining why one variant out-performed (or why neither did).",
  "suggested_body": "A single new system prompt (max 350 words) that merges what worked across both variants.",
  "next_hypotheses": ["First testable hypothesis", "Second", "Third"]
}

Rules for suggested_body:
- imperative, unambiguous, in the user's language;
- no markdown headings, no fenced code, no placeholders like {{x}};
- never reveal the model is an AI;
- preserve any guardrails present in BOTH variants.

Rules for next_hypotheses: 3 bullets, each a single concrete change to test next (e.g. "Open with the customer's first name when the channel provides it").`

// SuggestImprovementsDTO is the input.
type SuggestImprovementsDTO struct {
	AccountID    string
	UserID       string
	ExperimentID string
}

type SuggestImprovementsUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	usageRepo   domain.UsageRepository
	auditRepo   domain.AuditLogRepository
	expRepo     domain.PromptExperimentRepository
	msgRepo     domain.MessageRepository
	aiFactory   domain.AIFactory
	maxBest     int
	maxWorst    int
	maxMsgs     int
}

func NewSuggestImprovementsUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditRepo domain.AuditLogRepository,
	expRepo domain.PromptExperimentRepository,
	msgRepo domain.MessageRepository,
	aiFactory domain.AIFactory,
) *SuggestImprovementsUseCase {
	return &SuggestImprovementsUseCase{
		accountRepo: accountRepo,
		planRepo:    planRepo,
		usageRepo:   usageRepo,
		auditRepo:   auditRepo,
		expRepo:     expRepo,
		msgRepo:     msgRepo,
		aiFactory:   aiFactory,
		maxBest:     5,
		maxWorst:    5,
		maxMsgs:     20,
	}
}

func (uc *SuggestImprovementsUseCase) Execute(ctx context.Context, dto SuggestImprovementsDTO) (*domain.ImprovementSuggestion, error) {
	plan, account, err := uc.resolvePlan(ctx, dto.AccountID)
	if err != nil {
		return nil, err
	}
	if !plan.AllowsAutoSuggest() {
		return nil, domain.ErrPlanFeatureLocked
	}

	exp, err := uc.expRepo.GetByID(ctx, dto.ExperimentID)
	if err != nil || exp == nil {
		return nil, domain.ErrExperimentNotFound
	}
	if exp.AccountID != dto.AccountID {
		return nil, domain.ErrExperimentNotFound
	}

	mA, err := uc.expRepo.MetricsForVariant(ctx, exp.ID, domain.VariantA)
	if err != nil {
		return nil, err
	}
	mB, err := uc.expRepo.MetricsForVariant(ctx, exp.ID, domain.VariantB)
	if err != nil {
		return nil, err
	}

	bestA, worstA, err := uc.expRepo.SampleConversations(ctx, exp.ID, domain.VariantA, uc.maxBest, uc.maxWorst)
	if err != nil {
		return nil, err
	}
	bestB, worstB, err := uc.expRepo.SampleConversations(ctx, exp.ID, domain.VariantB, uc.maxBest, uc.maxWorst)
	if err != nil {
		return nil, err
	}

	payload := uc.buildPayload(ctx, exp, mA, mB, bestA, worstA, bestB, worstB)

	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		return nil, &domain.AppError{Code: domain.ErrAIProviderUnavailable.Code, Message: err.Error()}
	}
	resp, err := provider.GenerateResponse(ctx, suggesterMetaPrompt, []domain.ChatMessage{
		{Role: "user", Content: payload},
	}, nil)
	if err != nil || resp == nil {
		return nil, &domain.AppError{Code: domain.ErrAIProviderUnavailable.Code, Message: fmt.Sprintf("suggester call failed: %v", err)}
	}

	parsed, err := parseSuggestion(resp.Content)
	if err != nil {
		return nil, err
	}

	tokensCost := int64(float64(resp.Usage.PromptTokens)*plan.InputMultiplier +
		float64(resp.Usage.CompletionTokens)*plan.OutputMultiplier)
	if tokensCost == 0 {
		tokensCost = 200
	}
	parsed.TokensUsed = tokensCost

	usage, _ := uc.usageRepo.GetByAccountID(ctx, dto.AccountID)
	if usage != nil {
		current := int64(0)
		if usage.AITokensUsed != nil {
			current = *usage.AITokensUsed
		}
		newTotal := current + tokensCost
		usage.AITokensUsed = &newTotal
		usage.UpdatedAt = time.Now().UTC()
		_ = uc.usageRepo.Update(ctx, usage)
		addon := int64(0)
		if usage.AddonTokens != nil {
			addon = *usage.AddonTokens
		}
		if rem := (plan.MaxAITokensPerMonth + addon) - newTotal; rem >= 0 {
			parsed.TokensRemain = rem
		}
	}

	_ = uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  dto.AccountID,
		Action:     "experiment.suggest_improvement",
		EntityType: "experiment",
		EntityID:   exp.ID,
		CreatedAt:  time.Now().UTC(),
	})

	return parsed, nil
}

func (uc *SuggestImprovementsUseCase) resolvePlan(ctx context.Context, accountID string) (*domain.Plan, *domain.Account, error) {
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || acc == nil {
		return nil, nil, domain.ErrAccountNotFound
	}
	if acc.PlanID == nil {
		return domain.TrialPlan(), acc, nil
	}
	p, err := uc.planRepo.GetByID(ctx, *acc.PlanID)
	if err != nil || p == nil {
		return domain.TrialPlan(), acc, nil
	}
	return p, acc, nil
}

func (uc *SuggestImprovementsUseCase) buildPayload(
	ctx context.Context,
	exp *domain.PromptExperiment,
	a, b *domain.VariantMetrics,
	bestA, worstA, bestB, worstB []domain.ExperimentSample,
) string {
	var s strings.Builder
	fmt.Fprintf(&s, "Experiment: %s\n\n", exp.Name)
	s.WriteString("VARIANT A BODY:\n")
	s.WriteString(exp.VariantABody)
	s.WriteString("\n\nVARIANT B BODY:\n")
	s.WriteString(exp.VariantBBody)
	s.WriteString("\n\nMETRICS DELTA (A vs B):\n")
	fmt.Fprintf(&s, "- conversations: %d / %d\n", a.ConversationCount, b.ConversationCount)
	fmt.Fprintf(&s, "- ai_resolved:   %d / %d\n", a.AIResolvedCount, b.AIResolvedCount)
	fmt.Fprintf(&s, "- handoff:       %d / %d\n", a.HandoffCount, b.HandoffCount)
	fmt.Fprintf(&s, "- csat avg:      %s / %s (responses %d / %d)\n",
		fmtFloat(a.CSATAverage), fmtFloat(b.CSATAverage), a.CSATResponses, b.CSATResponses)
	fmt.Fprintf(&s, "- tokens total:  %d / %d\n", a.TokensTotal, b.TokensTotal)

	uc.appendSamples(ctx, &s, "VARIANT A — BEST CONVERSATIONS", bestA)
	uc.appendSamples(ctx, &s, "VARIANT A — WORST CONVERSATIONS", worstA)
	uc.appendSamples(ctx, &s, "VARIANT B — BEST CONVERSATIONS", bestB)
	uc.appendSamples(ctx, &s, "VARIANT B — WORST CONVERSATIONS", worstB)

	return s.String()
}

func (uc *SuggestImprovementsUseCase) appendSamples(ctx context.Context, s *strings.Builder, header string, samples []domain.ExperimentSample) {
	if len(samples) == 0 {
		return
	}
	fmt.Fprintf(s, "\n%s:\n", header)
	for _, sample := range samples {
		rating := "no rating"
		if sample.Rating != nil {
			rating = fmt.Sprintf("CSAT %d/5", *sample.Rating)
		}
		fmt.Fprintf(s, "- conversation %s (%s, resolved=%v)\n", sample.ConversationID, rating, sample.Resolved)
		msgs, err := uc.msgRepo.ListByConversationID(ctx, sample.ConversationID, int32(uc.maxMsgs), 0) //nolint:gosec // G115: maxMsgs is a small bounded config int
		if err != nil {
			continue
		}
		// Messages come newest-first — reverse for chronological order
		// in the LLM prompt.
		for i := len(msgs) - 1; i >= 0; i-- {
			m := msgs[i]
			if m.Content == nil || strings.TrimSpace(*m.Content) == "" {
				continue
			}
			role := "customer"
			if m.Direction == "outbound" {
				role = "agent"
			}
			content := *m.Content
			if len(content) > 200 {
				content = content[:200] + "…"
			}
			fmt.Fprintf(s, "  %s: %s\n", role, content)
		}
	}
}

func fmtFloat(f *float64) string {
	if f == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f", *f)
}

// parseSuggestion extracts the JSON object from the LLM output.
// Some providers emit ```json fences even when told not to; strip them
// defensively before unmarshalling. A trailing comma or stray backtick
// surfaces as ErrSuggestionParseFailed so the UI shows a retry CTA
// instead of a half-rendered card.
func parseSuggestion(raw string) (*domain.ImprovementSuggestion, error) {
	body := strings.TrimSpace(raw)
	body = strings.TrimPrefix(body, "```json")
	body = strings.TrimPrefix(body, "```")
	body = strings.TrimSuffix(body, "```")
	body = strings.TrimSpace(body)

	// Find the first `{` and last `}` so we tolerate models that put
	// "Here is the JSON:" before the object.
	open := strings.Index(body, "{")
	close := strings.LastIndex(body, "}")
	if open < 0 || close <= open {
		return nil, domain.ErrSuggestionParseFailed
	}
	body = body[open : close+1]

	var out domain.ImprovementSuggestion
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, domain.ErrSuggestionParseFailed
	}
	if strings.TrimSpace(out.SuggestedBody) == "" || strings.TrimSpace(out.Rationale) == "" {
		return nil, domain.ErrSuggestionParseFailed
	}
	if len(out.SuggestedBody) > domain.MaxPromptBodyBytes {
		out.SuggestedBody = out.SuggestedBody[:domain.MaxPromptBodyBytes]
	}
	return &out, nil
}
