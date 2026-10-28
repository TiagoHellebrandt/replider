package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// refineMetaPrompt instructs the model to regenerate ONE section of an
// existing system prompt while leaving the rest byte-identical. When
// section == "ALL", the LLM may rewrite anywhere — but still bounded by
// the same length cap and section-order rules from the synthesis pass.
const refineMetaPrompt = `You are refining a production system prompt that was previously synthesized.

You will receive:
- the current full body
- the wizard answers used to build it
- a target section name (e.g. ROLE / TONE / SCOPE / STRATEGY / EXAMPLES / EDGE-CASES) or ALL
- a delta instruction (e.g. "make more formal", "shorter", "more empathetic")

Output ONLY the new full body — no preamble, no explanation, no markdown headings, no fenced code blocks.

Rules:
- If the target section is named, regenerate THAT section only and copy every other section character-by-character from the input.
- If the target is ALL, apply the delta to every section but keep section names + order identical.
- Preserve the section names as plain uppercase lines exactly as written in the input.
- Maximum 700 words total. Imperative tone. No placeholders.
- Never reveal you are an AI. Never invent business facts.
- Reuse the user's wording when possible.`

// RefinePromptDTO carries the inputs for a single refinement turn. The
// useCase mutates nothing — it returns the new body so the caller can
// decide whether to overwrite the wizard's stashed body or discard.
type RefinePromptDTO struct {
	AccountID        string
	UserID           string
	CurrentBody      string
	Answers          domain.WizardAnswers
	Section          string // ROLE | TONE | SCOPE | STRATEGY | EXAMPLES | EDGE-CASES | ALL
	DeltaInstruction string
}

type RefinePromptResponse struct {
	Body         string `json:"body"`
	TokensUsed   int64  `json:"tokens_used"`
	TokensRemain int64  `json:"tokens_remain"`
}

// Allowed refinement sections — anything else returns an error so the
// LLM can't be tricked into rewriting unrelated content.
var validRefineSections = map[string]struct{}{
	"IDENTITY":   {},
	"PERSONA":    {},
	"KNOWLEDGE":  {},
	"TONE":       {},
	"SCOPE":      {},
	"STRATEGY":   {},
	"ESCALATION": {},
	"EXAMPLES":   {},
	"EDGE-CASES": {},
	"ALL":        {},
}

// RefinePromptUseCase wraps a single LLM call that rewrites a slice of
// the synthesized body. Plan-gated to Pro+ (same as A/B). Token-metered
// against the same usage bucket as synthesis. Rate-limited at the
// route layer — see prompt_routes.go.
type RefinePromptUseCase struct {
	accountRepo  domain.AccountRepository
	planRepo     domain.PlanRepository
	usageRepo    domain.UsageRepository
	auditLogRepo domain.AuditLogRepository
	aiFactory    domain.AIFactory
	usageGuard   services.UsageChecker
}

func NewRefinePromptUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditLogRepo domain.AuditLogRepository,
	aiFactory domain.AIFactory,
) *RefinePromptUseCase {
	return &RefinePromptUseCase{
		accountRepo:  accountRepo,
		planRepo:     planRepo,
		usageRepo:    usageRepo,
		auditLogRepo: auditLogRepo,
		aiFactory:    aiFactory,
	}
}

func (uc *RefinePromptUseCase) SetUsageGuard(g services.UsageChecker) {
	uc.usageGuard = g
}

func (uc *RefinePromptUseCase) Execute(ctx context.Context, dto RefinePromptDTO) (*RefinePromptResponse, error) {
	if strings.TrimSpace(dto.CurrentBody) == "" {
		return nil, &domain.AppError{Code: "REFINE_BODY_REQUIRED", Message: "current_body is required"}
	}
	section := strings.ToUpper(strings.TrimSpace(dto.Section))
	if section == "" {
		section = "ALL"
	}
	if _, ok := validRefineSections[section]; !ok {
		return nil, &domain.AppError{Code: "REFINE_SECTION_INVALID", Message: "section must be one of: IDENTITY/PERSONA/KNOWLEDGE/TONE/SCOPE/STRATEGY/ESCALATION/EXAMPLES/EDGE-CASES/ALL"}
	}
	delta := strings.TrimSpace(dto.DeltaInstruction)
	if delta == "" {
		return nil, &domain.AppError{Code: "REFINE_DELTA_REQUIRED", Message: "delta_instruction is required"}
	}

	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}

	plan := domain.TrialPlan()
	if account.PlanID != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	if !plan.AllowsExperiments() {
		return nil, &domain.AppError{
			Code:    "PLAN_FEATURE_LOCKED",
			Message: "Refinement requires the Pro plan or higher",
		}
	}

	usage, _ := uc.usageRepo.GetByAccountID(ctx, dto.AccountID)
	if usage != nil && uc.usageGuard != nil {
		if err := uc.usageGuard.Check(account, plan, usage, "tokens"); err != nil {
			return nil, err
		}
	}

	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("failed to initialize AI provider: %v", err),
		}
	}

	userPayload := buildRefinePayload(dto.CurrentBody, &dto.Answers, section, delta)
	resp, err := provider.GenerateResponse(ctx, refineMetaPrompt, []domain.ChatMessage{
		{Role: "user", Content: userPayload},
	}, nil)
	if err != nil || resp == nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("AI provider failed to refine prompt: %v", err),
		}
	}

	body := strings.TrimSpace(resp.Content)
	if body == "" {
		return nil, &domain.AppError{Code: "REFINE_EMPTY", Message: "refiner returned an empty body"}
	}
	if len(body) > domain.MaxPromptBodyBytes {
		body = body[:domain.MaxPromptBodyBytes]
	}

	tokensCost := int64((float64(resp.Usage.PromptTokens) * plan.InputMultiplier) +
		(float64(resp.Usage.CompletionTokens) * plan.OutputMultiplier))
	if tokensCost == 0 {
		tokensCost = 100
	}

	var tokensRemain int64
	if usage != nil {
		current := int64(0)
		if usage.AITokensUsed != nil {
			current = *usage.AITokensUsed
		}
		addon := int64(0)
		if usage.AddonTokens != nil {
			addon = *usage.AddonTokens
		}
		newTotal := current + tokensCost
		usage.AITokensUsed = &newTotal
		usage.UpdatedAt = time.Now().UTC()
		_ = uc.usageRepo.Update(ctx, usage)
		tokensRemain = (plan.MaxAITokensPerMonth + addon) - newTotal
		if tokensRemain < 0 {
			tokensRemain = 0
		}
	}

	_ = uc.auditLogRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  dto.AccountID,
		Action:     "prompt.refine",
		EntityType: "prompt",
		CreatedAt:  time.Now().UTC(),
	})

	return &RefinePromptResponse{
		Body:         body,
		TokensUsed:   tokensCost,
		TokensRemain: tokensRemain,
	}, nil
}

func buildRefinePayload(body string, a *domain.WizardAnswers, section, delta string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Target section: %s\n", section)
	fmt.Fprintf(&b, "Delta instruction: %s\n\n", delta)
	b.WriteString("Current body (regenerate as instructed):\n---\n")
	b.WriteString(body)
	b.WriteString("\n---\n\n")
	b.WriteString("Wizard answers (for context — do not echo unless relevant):\n")
	b.WriteString(buildSynthesisUserPayload(a))
	return b.String()
}
