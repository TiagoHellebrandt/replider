package usecases

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

type CannedResponseUseCase struct {
	repo domain.CannedResponseRepository
}

func NewCannedResponseUseCase(repo domain.CannedResponseRepository) *CannedResponseUseCase {
	return &CannedResponseUseCase{repo: repo}
}

// canManage enforces the US-018 visibility rules on mutations:
//   - personal rows: owner-only. Others (admins included) get NotFound,
//     never Forbidden — a foreign personal response must not leak its
//     existence.
//   - account rows: admin-only ("account pra todos, admin gerencia").
func canManage(cr *domain.CannedResponse, userID, role string) error {
	if cr.Visibility == domain.CannedVisibilityPersonal {
		if cr.OwnerUserID == nil || *cr.OwnerUserID != userID {
			return domain.ErrCannedResponseNotFound
		}
		return nil
	}
	if role != domain.RoleAdmin {
		return domain.ErrCannedResponseForbidden
	}
	return nil
}

type CreateCannedResponseDTO struct {
	AccountID string
	UserID    string
	Role      string
	Shortcut  string
	Title     string
	Content   string
	Category  string
	// Visibility defaults to 'account' when empty (legacy clients).
	Visibility string
}

func (uc *CannedResponseUseCase) Create(ctx context.Context, dto CreateCannedResponseDTO) (*domain.CannedResponse, error) {
	visibility := dto.Visibility
	if visibility == "" {
		visibility = domain.CannedVisibilityAccount
	}
	if !domain.IsValidCannedVisibility(visibility) {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "invalid visibility"}
	}
	if visibility == domain.CannedVisibilityAccount && dto.Role != domain.RoleAdmin {
		return nil, domain.ErrCannedResponseForbidden
	}

	// Uniqueness within the caller's visible set (account rows + own
	// personal rows) — the partial unique indexes are the race backstop.
	existing, _ := uc.repo.GetVisibleByShortcut(ctx, dto.AccountID, dto.UserID, dto.Shortcut)
	if existing != nil {
		return nil, domain.ErrCannedResponseShortcutExists
	}

	category := dto.Category
	if category == "" {
		category = "general"
	}

	cr := &domain.CannedResponse{
		ID:         ulid.Make().String(),
		AccountID:  dto.AccountID,
		Shortcut:   dto.Shortcut,
		Title:      dto.Title,
		Content:    dto.Content,
		Category:   category,
		Visibility: visibility,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if visibility == domain.CannedVisibilityPersonal {
		owner := dto.UserID
		cr.OwnerUserID = &owner
	}

	if err := uc.repo.Create(ctx, cr); err != nil {
		return nil, err
	}

	return cr, nil
}

func (uc *CannedResponseUseCase) List(ctx context.Context, accountID, userID string) ([]*domain.CannedResponse, error) {
	return uc.repo.ListVisible(ctx, accountID, userID)
}

type UpdateCannedResponseDTO struct {
	AccountID string
	UserID    string
	Role      string
	ID        string
	Shortcut  string
	Title     string
	Content   string
	Category  string
}

func (uc *CannedResponseUseCase) Update(ctx context.Context, dto UpdateCannedResponseDTO) (*domain.CannedResponse, error) {
	cr, err := uc.repo.GetByID(ctx, dto.ID)
	if err != nil || cr == nil {
		return nil, domain.ErrCannedResponseNotFound
	}
	if cr.AccountID != dto.AccountID {
		return nil, domain.ErrCannedResponseNotFound
	}
	if err := canManage(cr, dto.UserID, dto.Role); err != nil {
		return nil, err
	}

	// Check shortcut uniqueness if changed
	if dto.Shortcut != cr.Shortcut {
		existing, _ := uc.repo.GetVisibleByShortcut(ctx, dto.AccountID, dto.UserID, dto.Shortcut)
		if existing != nil && existing.ID != cr.ID {
			return nil, domain.ErrCannedResponseShortcutExists
		}
	}

	cr.Shortcut = dto.Shortcut
	cr.Title = dto.Title
	cr.Content = dto.Content
	cr.Category = dto.Category
	cr.UpdatedAt = time.Now()

	if err := uc.repo.Update(ctx, cr); err != nil {
		return nil, err
	}

	return cr, nil
}

func (uc *CannedResponseUseCase) Delete(ctx context.Context, accountID, userID, role, id string) error {
	cr, err := uc.repo.GetByID(ctx, id)
	if err != nil || cr == nil {
		return domain.ErrCannedResponseNotFound
	}
	if cr.AccountID != accountID {
		return domain.ErrCannedResponseNotFound
	}
	if err := canManage(cr, userID, role); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
