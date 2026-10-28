package usecases

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// PromptUseCase exposes the CRUD operations on the "Meus Prompts"
// library. Synthesis (LLM-driven) and apply-to-agent (transactional,
// versioned) live in their own files due to dependency complexity.
type PromptUseCase struct {
	repo domain.PromptRepository
}

func NewPromptUseCase(repo domain.PromptRepository) *PromptUseCase {
	return &PromptUseCase{repo: repo}
}

type CreatePromptDTO struct {
	AccountID     string
	UserID        string
	Name          string
	Description   *string
	Body          string
	Source        string // wizard|manual|duplicated|template; defaults to "manual"
	WizardAnswers *domain.WizardAnswers
}

func (uc *PromptUseCase) Create(ctx context.Context, dto CreatePromptDTO) (*domain.Prompt, error) {
	if err := validatePromptName(dto.Name); err != nil {
		return nil, err
	}
	if err := validatePromptBody(dto.Body); err != nil {
		return nil, err
	}
	source := dto.Source
	if source == "" {
		source = domain.PromptSourceManual
	}

	p := &domain.Prompt{
		ID:            ulid.Make().String(),
		AccountID:     dto.AccountID,
		Name:          strings.TrimSpace(dto.Name),
		Description:   trimPtr(dto.Description),
		Body:          dto.Body,
		Source:        source,
		WizardAnswers: dto.WizardAnswers,
		CreatedBy:     ptrIfNotEmpty(dto.UserID),
	}
	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *PromptUseCase) Get(ctx context.Context, accountID, id string) (*domain.Prompt, error) {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil || p == nil {
		return nil, domain.ErrPromptNotFound
	}
	if p.AccountID != accountID {
		return nil, domain.ErrPromptNotFound
	}
	return p, nil
}

func (uc *PromptUseCase) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.Prompt, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return uc.repo.ListByAccountID(ctx, accountID, limit, offset)
}

type UpdatePromptDTO struct {
	AccountID     string
	ID            string
	Name          string
	Description   *string
	Body          string
	WizardAnswers *domain.WizardAnswers
}

func (uc *PromptUseCase) Update(ctx context.Context, dto UpdatePromptDTO) (*domain.Prompt, error) {
	p, err := uc.Get(ctx, dto.AccountID, dto.ID)
	if err != nil {
		return nil, err
	}
	if err := validatePromptName(dto.Name); err != nil {
		return nil, err
	}
	if err := validatePromptBody(dto.Body); err != nil {
		return nil, err
	}
	p.Name = strings.TrimSpace(dto.Name)
	p.Description = trimPtr(dto.Description)
	p.Body = dto.Body
	if dto.WizardAnswers != nil {
		p.WizardAnswers = dto.WizardAnswers
	}
	if err := uc.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *PromptUseCase) Delete(ctx context.Context, accountID, id string) error {
	if _, err := uc.Get(ctx, accountID, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

// Duplicate copies a prompt under a new name ("Copy of X") so the user
// can iterate without losing the original. WizardAnswers carry over so
// the duplicate can be re-synthesized later from the same intent.
func (uc *PromptUseCase) Duplicate(ctx context.Context, accountID, userID, id string) (*domain.Prompt, error) {
	src, err := uc.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	dup := &domain.Prompt{
		ID:            ulid.Make().String(),
		AccountID:     accountID,
		Name:          duplicateName(src.Name),
		Description:   src.Description,
		Body:          src.Body,
		Source:        domain.PromptSourceDuplicated,
		WizardAnswers: src.WizardAnswers,
		CreatedBy:     ptrIfNotEmpty(userID),
	}
	if err := uc.repo.Create(ctx, dup); err != nil {
		return nil, err
	}
	return dup, nil
}

func validatePromptName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return &domain.AppError{Code: "PROMPT_NAME_REQUIRED", Message: "prompt name is required"}
	}
	if len(n) > 120 {
		return &domain.AppError{Code: "PROMPT_NAME_TOO_LONG", Message: "prompt name must be 120 characters or fewer"}
	}
	return nil
}

func validatePromptBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return &domain.AppError{Code: "PROMPT_BODY_REQUIRED", Message: "prompt body is required"}
	}
	if len(body) > domain.MaxPromptBodyBytes {
		return domain.ErrPromptBodyTooLarge
	}
	return nil
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func duplicateName(orig string) string {
	const prefix = "Copy of "
	if strings.HasPrefix(orig, prefix) {
		return orig
	}
	return prefix + orig
}
