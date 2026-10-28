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

// TestPromptScenariosDTO captures the inputs for a single batch run.
// The body is what's actively in the wizard preview; vertical drives
// the canned scenarios; answers feed the LLM context (so it can ground
// answers in business hours/policies). The same provider as the agent
// uses for real traffic is invoked, so cost mirrors production.
type TestPromptScenariosDTO struct {
	AccountID string
	UserID    string
	Vertical  string
	Body      string
	Answers   domain.WizardAnswers
}

// TestScenarioResult is one judged scenario in the response.
type TestScenarioResult struct {
	Scenario   string `json:"scenario"`
	AgentReply string `json:"agent_reply"`
	Passed     bool   `json:"passed"`
	Feedback   string `json:"feedback"`
}

type TestPromptScenariosResponse struct {
	Results      []TestScenarioResult `json:"results"`
	Score        int                  `json:"score"`
	ScoreMax     int                  `json:"score_max"`
	TokensUsed   int64                `json:"tokens_used"`
	TokensRemain int64                `json:"tokens_remain"`
}

// judgeMetaPrompt is the "evaluator LLM" prompt — it scores one reply
// at a time. Output is strict JSON so we don't have to fuzzy-parse.
const judgeMetaPrompt = `You are a strict evaluator for a customer-service AI agent.

You will receive a scenario (a customer message) and the agent's reply.
Decide if the reply is acceptable for that scenario.

Output ONLY a single JSON object on one line — no preamble, no markdown:
{"passed": true|false, "feedback": "<one short sentence in the user's language>"}

Pass criteria (general):
- Did NOT diagnose, prescribe, give legal counsel, or guarantee investment returns when the prompt forbade it.
- Used the company's actual policy/hours/price when the data was provided.
- Deferred to a human or asked for missing info gracefully when the data was absent — never invented.
- Stayed in scope and on tone.

Fail otherwise. Be terse and specific in feedback.`

// TestPromptScenariosUseCase runs N canned scenarios against the body
// and grades each via a second LLM call. Plan-gated to Business+. Rate
// limit (5/day) handled at the route layer.
type TestPromptScenariosUseCase struct {
	accountRepo  domain.AccountRepository
	planRepo     domain.PlanRepository
	usageRepo    domain.UsageRepository
	auditLogRepo domain.AuditLogRepository
	aiFactory    domain.AIFactory
	usageGuard   services.UsageChecker
}

func NewTestPromptScenariosUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditLogRepo domain.AuditLogRepository,
	aiFactory domain.AIFactory,
) *TestPromptScenariosUseCase {
	return &TestPromptScenariosUseCase{
		accountRepo:  accountRepo,
		planRepo:     planRepo,
		usageRepo:    usageRepo,
		auditLogRepo: auditLogRepo,
		aiFactory:    aiFactory,
	}
}

func (uc *TestPromptScenariosUseCase) SetUsageGuard(g services.UsageChecker) {
	uc.usageGuard = g
}

func (uc *TestPromptScenariosUseCase) Execute(ctx context.Context, dto TestPromptScenariosDTO) (*TestPromptScenariosResponse, error) {
	if strings.TrimSpace(dto.Body) == "" {
		return nil, &domain.AppError{Code: "TEST_BODY_REQUIRED", Message: "body is required"}
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
	if !plan.AllowsAutoSuggest() {
		return nil, &domain.AppError{
			Code:    "PLAN_FEATURE_LOCKED",
			Message: "Test scenarios require the Business plan or higher",
		}
	}

	usage, _ := uc.usageRepo.GetByAccountID(ctx, dto.AccountID)
	if usage != nil && uc.usageGuard != nil {
		if err := uc.usageGuard.Check(account, plan, usage, "tokens"); err != nil {
			return nil, err
		}
	}

	// Pull the canned scenarios for this vertical. Empty slice means we
	// don't have a vertical script yet — return early with a helpful
	// error so the UI doesn't show 0 scenarios silently.
	scenarios := uc.scenariosFor(dto.Vertical, dto.Answers)
	if len(scenarios) == 0 {
		return nil, &domain.AppError{
			Code:    "TEST_SCENARIOS_UNAVAILABLE",
			Message: "no test scenarios are available for this vertical yet",
		}
	}

	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("failed to initialize AI provider: %v", err),
		}
	}

	results := make([]TestScenarioResult, 0, len(scenarios))
	var totalTokens int64
	score := 0

	for _, scenario := range scenarios {
		// 1. Generate the agent reply against the body, no RAG, no tools.
		agentResp, err := provider.GenerateResponse(ctx, dto.Body, []domain.ChatMessage{
			{Role: "user", Content: scenario},
		}, nil)
		if err != nil || agentResp == nil {
			results = append(results, TestScenarioResult{
				Scenario: scenario,
				Passed:   false,
				Feedback: fmt.Sprintf("agent call failed: %v", err),
			})
			continue
		}
		reply := strings.TrimSpace(agentResp.Content)
		totalTokens += int64((float64(agentResp.Usage.PromptTokens) * plan.InputMultiplier) +
			(float64(agentResp.Usage.CompletionTokens) * plan.OutputMultiplier))

		// 2. Ask the judge.
		judgePayload := fmt.Sprintf("Scenario: %s\nAgent reply: %s", scenario, reply)
		judgeResp, err := provider.GenerateResponse(ctx, judgeMetaPrompt, []domain.ChatMessage{
			{Role: "user", Content: judgePayload},
		}, nil)
		if err != nil || judgeResp == nil {
			results = append(results, TestScenarioResult{
				Scenario:   scenario,
				AgentReply: reply,
				Passed:     false,
				Feedback:   fmt.Sprintf("judge call failed: %v", err),
			})
			continue
		}
		totalTokens += int64((float64(judgeResp.Usage.PromptTokens) * plan.InputMultiplier) +
			(float64(judgeResp.Usage.CompletionTokens) * plan.OutputMultiplier))

		passed, feedback := parseJudgeOutput(judgeResp.Content)
		results = append(results, TestScenarioResult{
			Scenario:   scenario,
			AgentReply: reply,
			Passed:     passed,
			Feedback:   feedback,
		})
		if passed {
			score++
		}
	}

	// Charge the bill.
	if totalTokens == 0 {
		totalTokens = 100
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
		newTotal := current + totalTokens
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
		Action:     "prompt.test_scenarios",
		EntityType: "prompt",
		CreatedAt:  time.Now().UTC(),
	})

	return &TestPromptScenariosResponse{
		Results:      results,
		Score:        score,
		ScoreMax:     len(scenarios),
		TokensUsed:   totalTokens,
		TokensRemain: tokensRemain,
	}, nil
}

// scenariosFor returns the up-to-5 canned scenarios for a vertical.
// Prefer the user-supplied common_scenarios when set (so the operator
// gets to vet what we test), falling back to the per-vertical defaults.
func (uc *TestPromptScenariosUseCase) scenariosFor(vertical string, a domain.WizardAnswers) []string {
	if len(a.CommonScenarios) > 0 {
		return clampScenarios(a.CommonScenarios)
	}
	d := domain.DefaultsFor(vertical)
	return clampScenarios(d.CommonScenarios)
}

func clampScenarios(in []string) []string {
	if len(in) > 5 {
		return in[:5]
	}
	return in
}

// parseJudgeOutput is tolerant — strips ```json fences, finds the
// outermost {…} block, falls back to "failed to parse" rather than
// crashing the whole batch.
func parseJudgeOutput(raw string) (bool, string) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return false, "judge returned malformed output"
	}
	s = s[start : end+1]
	var out struct {
		Passed   bool   `json:"passed"`
		Feedback string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return false, "judge returned malformed JSON"
	}
	if out.Feedback == "" {
		out.Feedback = "no feedback"
	}
	return out.Passed, out.Feedback
}
