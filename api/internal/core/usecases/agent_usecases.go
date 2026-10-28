package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type ListAgentsUseCase struct {
	agentRepo domain.AgentRepository
}

func NewListAgentsUseCase(agentRepo domain.AgentRepository) *ListAgentsUseCase {
	return &ListAgentsUseCase{agentRepo: agentRepo}
}

func (uc *ListAgentsUseCase) Execute(ctx context.Context, accountID string) ([]*domain.Agent, error) {
	return uc.agentRepo.ListByAccountID(ctx, accountID)
}

type UpdateAgentDTO struct {
	ID           string
	AccountID    string
	Name         string
	SystemPrompt *string
	Provider     string
	Model        string
	Timezone     string
	// BusinessHours* are nilable so a plain PATCH can keep the existing
	// values without forcing the client to round-trip them. When the
	// client wants to disable business hours entirely, they send Days=0
	// (the domain treats 0 as "no window, 24/7 bot").
	BusinessHoursStart *string
	BusinessHoursEnd   *string
	BusinessHoursDays  *int
	// Hands-off toggles — nil means "leave as-is", matching the PATCH
	// semantics of the other fields above.
	HideAIIdentity  *bool
	SilentOnUnknown *bool
}

type UpdateAgentUseCase struct {
	agentRepo domain.AgentRepository
}

func NewUpdateAgentUseCase(agentRepo domain.AgentRepository) *UpdateAgentUseCase {
	return &UpdateAgentUseCase{agentRepo: agentRepo}
}

func (uc *UpdateAgentUseCase) Execute(ctx context.Context, dto UpdateAgentDTO) (*domain.Agent, error) {
	agent, err := uc.agentRepo.GetByID(ctx, dto.ID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != dto.AccountID {
		return nil, domain.ErrAgentNotFound
	}

	agent.Name = dto.Name
	agent.SystemPrompt = dto.SystemPrompt
	if dto.Provider != "" {
		agent.Provider = dto.Provider
	}
	if dto.Model != "" {
		agent.Model = dto.Model
	}
	// Checked on the MERGED pair, not on the DTO: a PATCH that changes only
	// the provider has to be judged against the model already stored, or
	// the mismatch walks in one field at a time.
	if err := domain.ValidateModelForProvider(agent.Provider, agent.Model); err != nil {
		return nil, err
	}
	if dto.Timezone != "" {
		agent.Timezone = dto.Timezone
	}
	// Business hours are PATCH-semantic: only touched when the DTO
	// carries a non-nil value. Days=0 explicitly disables the window
	// per the domain contract.
	if dto.BusinessHoursStart != nil {
		agent.BusinessHoursStart = dto.BusinessHoursStart
	}
	if dto.BusinessHoursEnd != nil {
		agent.BusinessHoursEnd = dto.BusinessHoursEnd
	}
	if dto.BusinessHoursDays != nil {
		agent.BusinessHoursDays = *dto.BusinessHoursDays
	}
	if dto.HideAIIdentity != nil {
		agent.HideAIIdentity = *dto.HideAIIdentity
	}
	if dto.SilentOnUnknown != nil {
		agent.SilentOnUnknown = *dto.SilentOnUnknown
	}
	agent.UpdatedAt = time.Now()

	if err := uc.agentRepo.Update(ctx, agent); err != nil {
		return nil, err
	}

	return agent, nil
}

type ToggleAgentStatusUseCase struct {
	agentRepo domain.AgentRepository
}

func NewToggleAgentStatusUseCase(agentRepo domain.AgentRepository) *ToggleAgentStatusUseCase {
	return &ToggleAgentStatusUseCase{agentRepo: agentRepo}
}

func (uc *ToggleAgentStatusUseCase) Execute(ctx context.Context, accountID, agentID string, active bool) error {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.AccountID != accountID {
		return domain.ErrAgentNotFound
	}

	agent.IsActive = active
	agent.UpdatedAt = time.Now()

	return uc.agentRepo.Update(ctx, agent)
}

// --- DeleteAgentUseCase ---

type DeleteAgentUseCase struct {
	agentRepo domain.AgentRepository
}

func NewDeleteAgentUseCase(agentRepo domain.AgentRepository) *DeleteAgentUseCase {
	return &DeleteAgentUseCase{agentRepo: agentRepo}
}

func (uc *DeleteAgentUseCase) Execute(ctx context.Context, accountID, agentID string) error {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.AccountID != accountID {
		return domain.ErrAgentNotFound
	}

	return uc.agentRepo.Delete(ctx, agentID)
}
