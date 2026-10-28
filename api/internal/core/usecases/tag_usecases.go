package usecases

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

type TagUseCase struct {
	repo     domain.TagRepository
	convRepo domain.ConversationRepository
}

func NewTagUseCase(repo domain.TagRepository, convRepo domain.ConversationRepository) *TagUseCase {
	return &TagUseCase{repo: repo, convRepo: convRepo}
}

type CreateTagDTO struct {
	AccountID string
	Name      string
	Color     string
}

func (uc *TagUseCase) Create(ctx context.Context, dto CreateTagDTO) (*domain.Tag, error) {
	color := dto.Color
	if color == "" {
		color = "#6366f1"
	}

	tag := &domain.Tag{
		ID:        ulid.Make().String(),
		AccountID: dto.AccountID,
		Name:      dto.Name,
		Color:     color,
		CreatedAt: time.Now(),
	}

	if err := uc.repo.Create(ctx, tag); err != nil {
		return nil, err
	}

	return tag, nil
}

func (uc *TagUseCase) List(ctx context.Context, accountID string) ([]*domain.Tag, error) {
	return uc.repo.ListByAccountID(ctx, accountID)
}

type UpdateTagDTO struct {
	AccountID string
	ID        string
	Name      string
	Color     string
}

func (uc *TagUseCase) Update(ctx context.Context, dto UpdateTagDTO) (*domain.Tag, error) {
	tag, err := uc.repo.GetByID(ctx, dto.ID)
	if err != nil || tag == nil {
		return nil, domain.ErrTagNotFound
	}
	if tag.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}

	tag.Name = dto.Name
	if dto.Color != "" {
		tag.Color = dto.Color
	}

	if err := uc.repo.Update(ctx, tag); err != nil {
		return nil, err
	}

	return tag, nil
}

func (uc *TagUseCase) Delete(ctx context.Context, accountID, id string) error {
	tag, err := uc.repo.GetByID(ctx, id)
	if err != nil || tag == nil {
		return domain.ErrTagNotFound
	}
	if tag.AccountID != accountID {
		return domain.ErrUnauthorized
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *TagUseCase) AddToConversation(ctx context.Context, accountID, conversationID, tagID string) error {
	// Verify conversation belongs to account
	conv, err := uc.convRepo.GetByID(ctx, conversationID, accountID)
	if err != nil || conv == nil {
		return domain.ErrConversationNotFound
	}
	// Verify tag belongs to account
	tag, err := uc.repo.GetByID(ctx, tagID)
	if err != nil || tag == nil {
		return domain.ErrTagNotFound
	}
	if tag.AccountID != accountID {
		return domain.ErrUnauthorized
	}
	return uc.repo.AddToConversation(ctx, conversationID, tagID)
}

func (uc *TagUseCase) RemoveFromConversation(ctx context.Context, accountID, conversationID, tagID string) error {
	conv, err := uc.convRepo.GetByID(ctx, conversationID, accountID)
	if err != nil || conv == nil {
		return domain.ErrConversationNotFound
	}
	return uc.repo.RemoveFromConversation(ctx, conversationID, tagID)
}

func (uc *TagUseCase) ListByConversation(ctx context.Context, accountID, conversationID string) ([]*domain.Tag, error) {
	conv, err := uc.convRepo.GetByID(ctx, conversationID, accountID)
	if err != nil || conv == nil {
		return nil, domain.ErrConversationNotFound
	}
	return uc.repo.ListByConversation(ctx, accountID, conversationID)
}
