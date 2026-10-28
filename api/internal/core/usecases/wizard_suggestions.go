package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// WizardSuggestionsResponse is the lean payload the wizard banners
// consume — a handful of bullets the user can one-click into ScopeIn,
// plus an optional company name we sniffed from the RAG content.
type WizardSuggestionsResponse struct {
	ScopeIn      []string `json:"scope_in"`
	CompanyName  string   `json:"company_name,omitempty"`
	TokensUsed   int64    `json:"tokens_used"`
	TokensRemain int64    `json:"tokens_remain"`
}

// suggestionsMetaPrompt asks the model to extract intent from the
// account's already-loaded knowledge. Lean output keeps the call cheap.
const suggestionsMetaPrompt = `You analyze a sample of a business' knowledge base to suggest wizard answers.

Output ONLY a single JSON object on one line — no preamble, no markdown:
{"scope_in": ["…", "…"], "company_name": "<best guess or empty>"}

Rules:
- scope_in: up to 5 short bullets describing topics the agent SHOULD answer (verb-led, e.g. "schedule consultations", "explain pricing"). Reuse the source's wording when possible.
- company_name: only if a clearly named company appears literally in the source; otherwise empty string.
- Never invent topics not grounded in the sample. Empty arrays are valid.`

type WizardSuggestionsDTO struct {
	AccountID string
	UserID    string
	AgentID   string
}

type WizardSuggestionsUseCase struct {
	accountRepo   domain.AccountRepository
	planRepo      domain.PlanRepository
	usageRepo     domain.UsageRepository
	auditLogRepo  domain.AuditLogRepository
	agentRepo     domain.AgentRepository
	knowledgeRepo domain.KnowledgeRepository
	aiFactory     domain.AIFactory
	usageGuard    services.UsageChecker
}

func NewWizardSuggestionsUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditLogRepo domain.AuditLogRepository,
	agentRepo domain.AgentRepository,
	knowledgeRepo domain.KnowledgeRepository,
	aiFactory domain.AIFactory,
) *WizardSuggestionsUseCase {
	return &WizardSuggestionsUseCase{
		accountRepo:   accountRepo,
		planRepo:      planRepo,
		usageRepo:     usageRepo,
		auditLogRepo:  auditLogRepo,
		agentRepo:     agentRepo,
		knowledgeRepo: knowledgeRepo,
		aiFactory:     aiFactory,
	}
}

func (uc *WizardSuggestionsUseCase) SetUsageGuard(g services.UsageChecker) {
	uc.usageGuard = g
}

func (uc *WizardSuggestionsUseCase) Execute(ctx context.Context, dto WizardSuggestionsDTO) (*WizardSuggestionsResponse, error) {
	// Validate agent ownership.
	agent, err := uc.agentRepo.GetByID(ctx, dto.AgentID)
	if err != nil || agent == nil {
		return nil, domain.ErrAgentNotFound
	}
	if agent.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}

	// Pull a small sample of chunks from the agent's sources. We grab the
	// first N chunks per source until we hit our overall sample cap; no
	// embedding search needed — we just want a representative slice.
	sources, err := uc.knowledgeRepo.ListSources(ctx, dto.AgentID)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return &WizardSuggestionsResponse{ScopeIn: []string{}}, nil
	}

	const sampleCap = 20
	const charCap = 3000
	collected := make([]string, 0, sampleCap)
	totalChars := 0
	for _, s := range sources {
		if len(collected) >= sampleCap || totalChars >= charCap {
			break
		}
		chunks, err := uc.knowledgeRepo.ListChunksBySource(ctx, s.ID)
		if err != nil {
			continue
		}
		for _, ch := range chunks {
			c := strings.TrimSpace(ch.Content)
			if c == "" {
				continue
			}
			if len(c) > 400 {
				c = c[:400]
			}
			collected = append(collected, c)
			totalChars += len(c)
			if len(collected) >= sampleCap || totalChars >= charCap {
				break
			}
		}
	}
	if len(collected) == 0 {
		return &WizardSuggestionsResponse{ScopeIn: []string{}}, nil
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

	payload := "Knowledge sample (concatenated chunks):\n---\n" + strings.Join(collected, "\n---\n")
	resp, err := provider.GenerateResponse(ctx, suggestionsMetaPrompt, []domain.ChatMessage{
		{Role: "user", Content: payload},
	}, nil)
	if err != nil || resp == nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("AI provider failed to compute suggestions: %v", err),
		}
	}

	scopeIn, companyName := parseWizardSuggestions(resp.Content)

	tokensCost := int64((float64(resp.Usage.PromptTokens) * plan.InputMultiplier) +
		(float64(resp.Usage.CompletionTokens) * plan.OutputMultiplier))
	if tokensCost == 0 {
		tokensCost = 50
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
		Action:     "prompt.wizard_suggestions",
		EntityType: "agent",
		CreatedAt:  time.Now().UTC(),
	})

	return &WizardSuggestionsResponse{
		ScopeIn:      scopeIn,
		CompanyName:  companyName,
		TokensUsed:   tokensCost,
		TokensRemain: tokensRemain,
	}, nil
}

// parseWizardSuggestions is tolerant of code fences and preamble —
// same shape as parseSuggestion in suggest_improvements.go and
// parseJudgeOutput in test_prompt_scenarios.go.
func parseWizardSuggestions(raw string) ([]string, string) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, ""
	}
	s = s[start : end+1]
	var out struct {
		ScopeIn     []string `json:"scope_in"`
		CompanyName string   `json:"company_name"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, ""
	}
	if len(out.ScopeIn) > 5 {
		out.ScopeIn = out.ScopeIn[:5]
	}
	return out.ScopeIn, out.CompanyName
}
