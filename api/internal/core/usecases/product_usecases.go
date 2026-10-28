package usecases

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ProductUseCase covers the account-scoped CRUD for catalog products (US-004).
// Ownership is enforced on every read/mutation by comparing the resource's
// account_id against the caller's account_id.
type ProductUseCase struct {
	repo domain.ProductRepository
}

func NewProductUseCase(repo domain.ProductRepository) *ProductUseCase {
	return &ProductUseCase{repo: repo}
}

type CreateProductDTO struct {
	AccountID   string
	Name        string
	Description string
	PriceCents  int64
	Currency    string
	Attributes  json.RawMessage
	Active      *bool
	// SourceID / SourcePage carry the brochure link through creation. They are
	// on the DTO because the import creates products THROUGH this use case:
	// leaving them off silently dropped the link on the one path that has one,
	// which is the whole feature.
	SourceID   string
	SourcePage int
}

func normalizeAttributes(attrs json.RawMessage) json.RawMessage {
	if len(attrs) == 0 {
		return json.RawMessage("{}")
	}
	return attrs
}

func (uc *ProductUseCase) Create(ctx context.Context, dto CreateProductDTO) (*domain.Product, error) {
	currency := strings.TrimSpace(dto.Currency)
	if currency == "" {
		currency = "BRL"
	}
	active := true
	if dto.Active != nil {
		active = *dto.Active
	}

	now := time.Now()
	p := &domain.Product{
		ID:          ulid.Make().String(),
		AccountID:   dto.AccountID,
		Name:        dto.Name,
		Description: dto.Description,
		PriceCents:  dto.PriceCents,
		Currency:    currency,
		Attributes:  normalizeAttributes(dto.Attributes),
		Active:      active,
		SourceID:    strings.TrimSpace(dto.SourceID),
		SourcePage:  dto.SourcePage,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *ProductUseCase) List(ctx context.Context, accountID string) ([]*domain.Product, error) {
	return uc.repo.ListByAccountID(ctx, accountID)
}

func (uc *ProductUseCase) Get(ctx context.Context, accountID, id string) (*domain.Product, error) {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, domain.ErrProductNotFound
	}
	if p.AccountID != accountID {
		return nil, domain.ErrUnauthorized
	}
	return p, nil
}

type UpdateProductDTO struct {
	AccountID   string
	ID          string
	Name        string
	Description string
	PriceCents  int64
	Currency    string
	Attributes  json.RawMessage
	Active      *bool
}

func (uc *ProductUseCase) Update(ctx context.Context, dto UpdateProductDTO) (*domain.Product, error) {
	p, err := uc.repo.GetByID(ctx, dto.ID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, domain.ErrProductNotFound
	}
	if p.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}

	currency := strings.TrimSpace(dto.Currency)
	if currency == "" {
		currency = p.Currency
	}

	p.Name = dto.Name
	p.Description = dto.Description
	p.PriceCents = dto.PriceCents
	p.Currency = currency
	// An absent attributes field PRESERVES what is stored. The product form
	// never sends attributes, and replacing them with "{}" on every rename
	// silently erased the sku/brand/model a PDF import extracted — the one
	// copy of those facts. Sending a non-empty object still replaces.
	if attrs := dto.Attributes; len(attrs) > 0 && string(attrs) != "{}" &&
		string(attrs) != "null" {
		p.Attributes = attrs
	}
	if dto.Active != nil {
		p.Active = *dto.Active
	}
	p.UpdatedAt = time.Now()

	if err := uc.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *ProductUseCase) Delete(ctx context.Context, accountID, id string) error {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return domain.ErrProductNotFound
	}
	if p.AccountID != accountID {
		return domain.ErrUnauthorized
	}
	return uc.repo.Delete(ctx, id)
}
