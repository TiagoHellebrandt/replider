package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ApplyPromptToAgentDTO accepts either a library entry (PromptID) or
// a free-text body. Mutually exclusive — when both are supplied
// PromptID wins.
type ApplyPromptToAgentDTO struct {
	AccountID string
	UserID    string
	AgentID   string
	PromptID  *string
	Body      *string
	Note      *string
}

type ApplyPromptToAgentUseCase struct {
	tx                domain.TransactionManager
	agentRepo         domain.AgentRepository
	promptRepo        domain.PromptRepository
	promptVersionRepo domain.PromptVersionRepository
	auditLogRepo      domain.AuditLogRepository
}

func NewApplyPromptToAgentUseCase(
	tx domain.TransactionManager,
	agentRepo domain.AgentRepository,
	promptRepo domain.PromptRepository,
	promptVersionRepo domain.PromptVersionRepository,
	auditLogRepo domain.AuditLogRepository,
) *ApplyPromptToAgentUseCase {
	return &ApplyPromptToAgentUseCase{
		tx:                tx,
		agentRepo:         agentRepo,
		promptRepo:        promptRepo,
		promptVersionRepo: promptVersionRepo,
		auditLogRepo:      auditLogRepo,
	}
}

func (uc *ApplyPromptToAgentUseCase) Execute(ctx context.Context, dto ApplyPromptToAgentDTO) (*domain.PromptVersion, error) {
	agent, err := uc.agentRepo.GetByID(ctx, dto.AgentID)
	if err != nil || agent == nil {
		return nil, domain.ErrAgentNotFound
	}
	if agent.AccountID != dto.AccountID {
		return nil, domain.ErrAgentNotFound
	}

	newBody, sourcePromptID, err := uc.resolveBody(ctx, dto)
	if err != nil {
		return nil, err
	}
	if err := validatePromptBody(newBody); err != nil {
		return nil, err
	}

	previousBody := ""
	if agent.SystemPrompt != nil {
		previousBody = *agent.SystemPrompt
	}

	version := &domain.PromptVersion{
		ID:        ulid.Make().String(),
		AgentID:   agent.ID,
		PromptID:  sourcePromptID,
		Body:      previousBody,
		AppliedBy: ptrIfNotEmpty(dto.UserID),
		Note:      dto.Note,
	}

	err = uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.promptVersionRepo.Create(ctx, version); err != nil {
			return err
		}
		agent.SystemPrompt = &newBody
		agent.UpdatedAt = time.Now().UTC()
		return uc.agentRepo.Update(ctx, agent)
	})
	if err != nil {
		return nil, err
	}

	_ = uc.auditLogRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  dto.AccountID,
		Action:     "agent.prompt_apply",
		EntityType: "agent",
		EntityID:   agent.ID,
		CreatedAt:  time.Now().UTC(),
	})

	return version, nil
}

func (uc *ApplyPromptToAgentUseCase) resolveBody(ctx context.Context, dto ApplyPromptToAgentDTO) (string, *string, error) {
	if dto.PromptID != nil && *dto.PromptID != "" {
		p, err := uc.promptRepo.GetByID(ctx, *dto.PromptID)
		if err != nil || p == nil {
			return "", nil, domain.ErrPromptNotFound
		}
		if p.AccountID != dto.AccountID {
			return "", nil, domain.ErrPromptNotFound
		}
		return p.Body, &p.ID, nil
	}
	if dto.Body == nil {
		return "", nil, &domain.AppError{Code: "PROMPT_APPLY_BODY_REQUIRED", Message: "either prompt_id or body must be provided"}
	}
	body := strings.TrimSpace(*dto.Body)
	return body, nil, nil
}

// ListVersionsByAgent powers the Versions tab. Pagination matches the
// rest of the API (max 200 per page).
func (uc *ApplyPromptToAgentUseCase) ListVersions(ctx context.Context, accountID, agentID string, limit, offset int) ([]*domain.PromptVersion, error) {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return nil, domain.ErrAgentNotFound
	}
	if agent.AccountID != accountID {
		return nil, domain.ErrAgentNotFound
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return uc.promptVersionRepo.ListByAgentID(ctx, agentID, limit, offset)
}

// Rollback reverts the agent's system_prompt to the snapshot captured
// in the given version. Always creates a new audit row (so the rollback
// itself is undoable).
func (uc *ApplyPromptToAgentUseCase) Rollback(ctx context.Context, accountID, userID, agentID, versionID string) (*domain.PromptVersion, error) {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return nil, domain.ErrAgentNotFound
	}
	if agent.AccountID != accountID {
		return nil, domain.ErrAgentNotFound
	}

	target, err := uc.promptVersionRepo.GetByID(ctx, versionID)
	if err != nil || target == nil {
		return nil, domain.ErrPromptVersionNotFound
	}
	if target.AgentID != agentID {
		return nil, domain.ErrPromptVersionNotFound
	}

	previousBody := ""
	if agent.SystemPrompt != nil {
		previousBody = *agent.SystemPrompt
	}
	note := "rollback to version " + versionID
	newAudit := &domain.PromptVersion{
		ID:        ulid.Make().String(),
		AgentID:   agentID,
		PromptID:  target.PromptID,
		Body:      previousBody,
		AppliedBy: ptrIfNotEmpty(userID),
		Note:      &note,
	}

	err = uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.promptVersionRepo.Create(ctx, newAudit); err != nil {
			return err
		}
		body := target.Body
		agent.SystemPrompt = &body
		agent.UpdatedAt = time.Now().UTC()
		return uc.agentRepo.Update(ctx, agent)
	})
	if err != nil {
		return nil, err
	}

	_ = uc.auditLogRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		Action:     "agent.prompt_rollback",
		EntityType: "agent",
		EntityID:   agentID,
		CreatedAt:  time.Now().UTC(),
	})

	return newAudit, nil
}
