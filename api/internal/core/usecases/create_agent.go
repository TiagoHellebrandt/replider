package usecases

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

type CreateAgentDTO struct {
	AccountID    string
	Name         string
	SystemPrompt *string
	Provider     string
	Model        string
	Timezone     string
	// Mission 7: optional server-side template. When non-empty, the
	// use case looks up domain.GetAgentTemplate(TemplateID), applies
	// Placeholders to its prompt, and overrides SystemPrompt+Name only
	// if the caller didn't set them explicitly. The knowledge seed
	// side-effect lives in the handler — the UC stays single-purpose
	// (create one agent) so tests don't need a knowledge mock just to
	// cover the agent path.
	TemplateID   string
	Placeholders map[string]string
}

type CreateAgentUseCase struct {
	agentRepo   domain.AgentRepository
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
}

func NewCreateAgentUseCase(
	agentRepo domain.AgentRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
) *CreateAgentUseCase {
	return &CreateAgentUseCase{
		agentRepo:   agentRepo,
		accountRepo: accountRepo,
		planRepo:    planRepo,
	}
}

func (uc *CreateAgentUseCase) Execute(ctx context.Context, dto CreateAgentDTO) (*domain.Agent, error) {
	// Enforce plan-defined agent count. Resolved via
	// account.EffectiveMaxAgents so per-account overrides apply and
	// contract/enterprise plans (MaxAgents == 0, "negotiate per deal")
	// resolve to UnlimitedLimit instead of the trial floor — otherwise
	// a contract account is capped at 1 agent. Accounts without a
	// resolved plan fall back to 1 (trial-safe default) so a
	// misconfigured plan_id can't unlock infinite agents.
	existingAgents, err := uc.agentRepo.ListByAccountID(ctx, dto.AccountID)
	if err != nil {
		return nil, err
	}
	maxAgents := 1
	if uc.accountRepo != nil && uc.planRepo != nil {
		if acc, err := uc.accountRepo.GetByID(ctx, dto.AccountID); err == nil && acc != nil && acc.PlanID != nil {
			if plan, err := uc.planRepo.GetByID(ctx, *acc.PlanID); err == nil && plan != nil {
				maxAgents = acc.EffectiveMaxAgents(plan)
			}
		}
	}
	if len(existingAgents) >= maxAgents {
		return nil, domain.NewPlanLimitError("agents", len(existingAgents), maxAgents)
	}

	provider := dto.Provider
	if provider == "" {
		provider = domain.AIProviderOpenAI
	}
	// The default model FOLLOWS the provider. As two independent constants
	// (openai / gpt-4o) an agent created as google with no model named
	// inherited an OpenAI model — which is how four of six production
	// agents ended up unable to answer anything.
	model := dto.Model
	if model == "" {
		model = domain.DefaultModelFor(provider)
	}
	if err := domain.ValidateModelForProvider(provider, model); err != nil {
		return nil, err
	}
	timezone := dto.Timezone
	if timezone == "" {
		timezone = "America/Sao_Paulo"
	}

	// Resolve template (if any) and fill in fields the caller left
	// empty. Caller-supplied values always win — the template is the
	// starter, not the authority, so "advanced" edits on the form
	// survive the round-trip.
	name := dto.Name
	systemPrompt := dto.SystemPrompt
	if dto.TemplateID != "" {
		tpl := domain.GetAgentTemplate(dto.TemplateID)
		if tpl == nil {
			return nil, &domain.AppError{Code: "TEMPLATE_NOT_FOUND", Message: "agent template not found"}
		}
		applied := tpl.ApplyPlaceholders(dto.Placeholders)
		if systemPrompt == nil || *systemPrompt == "" {
			systemPrompt = &applied
		}
		if name == "" {
			name = tpl.Name
		}
	}

	agent := &domain.Agent{
		ID:              ulid.Make().String(),
		AccountID:       dto.AccountID,
		Name:            name,
		SystemPrompt:    systemPrompt,
		Provider:        provider,
		Model:           model,
		Timezone:        timezone,
		HideAIIdentity:  true,
		SilentOnUnknown: true,
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := uc.agentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}

	return agent, nil
}
