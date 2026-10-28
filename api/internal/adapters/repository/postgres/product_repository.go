package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// ProductRepository implements domain.ProductRepository over the sqlc-generated
// Queries (US-004). Wraps the products.sql queries created in US-001.
type ProductRepository struct {
	queries *Queries
	// db backs the handful of statements not worth a sqlc round trip —
	// see SetCycles.
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{queries: New(db), db: db}
}

func toDomainProduct(row Product) *domain.Product {
	attrs := json.RawMessage(row.Attributes)
	if len(attrs) == 0 {
		attrs = json.RawMessage("{}")
	}
	return &domain.Product{
		ID:          row.ID,
		AccountID:   row.AccountID,
		Name:        row.Name,
		Description: row.Description,
		PriceCents:  row.PriceCents,
		Currency:    row.Currency,
		Attributes:  attrs,
		Active:      row.Active,
		// The repurchase cycles (§6.5 F3). Read here so a product's own
		// cycle can stand in when a sale did not record one — which is most
		// sales, because nobody types a cycle for something they sell every
		// week.
		ConsumptionCycleDays: int(row.ConsumptionCycleDays),
		ShelfLifeDays:        int(row.ShelfLifeDays),
		SourceID:             stringPtrValue(row.SourceID),
		SourcePage:           int(row.SourcePage),
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
}

// SetCycles writes the repurchase cycles.
//
// Its own statement rather than fields on UpdateProduct: the cycle is set
// from the portfolio screen and from an MCP tool, by somebody thinking about
// reorders, not while editing a price.
//
// Account-scoped, and a miss is an ERROR: the statement always filtered on
// the account, but a zero-row update returned nil, so set_product_cycle on
// another account's product answered ok:true while writing nothing.
func (r *ProductRepository) SetCycles(ctx context.Context, id, accountID string, consumptionDays, shelfLifeDays int) error {
	//nolint:gosec // G115: both bounded by the handler to [0, 3650].
	tag, err := r.db.Exec(ctx,
		`UPDATE products
		    SET consumption_cycle_days = $3, shelf_life_days = $4, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`,
		id, accountID, int32(consumptionDays), int32(shelfLifeDays))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func (r *ProductRepository) Create(ctx context.Context, product *domain.Product) error {
	q := getQueries(ctx, r.queries)
	return q.CreateProduct(ctx, CreateProductParams{
		ID:          product.ID,
		AccountID:   product.AccountID,
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
		Currency:    product.Currency,
		Attributes:  toBytes(product.Attributes),
		Active:      product.Active,
		SourceID:    nullableString(product.SourceID),
		SourcePage:  int32(product.SourcePage), //nolint:gosec // G115: a brochure page, bounded by the import's page cap
		CreatedAt:   pgtype.Timestamptz{Time: product.CreatedAt, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: product.UpdatedAt, Valid: true},
	})
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetProductByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainProduct(row), nil
}

func (r *ProductRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Product, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListProductsByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	products := make([]*domain.Product, 0, len(rows))
	for _, row := range rows {
		products = append(products, toDomainProduct(row))
	}
	return products, nil
}

func (r *ProductRepository) Update(ctx context.Context, product *domain.Product) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateProduct(ctx, UpdateProductParams{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
		Currency:    product.Currency,
		Attributes:  toBytes(product.Attributes),
		Active:      product.Active,
	})
}

func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteProduct(ctx, id)
}
