package domain

import (
	"context"
	"encoding/json"
	"time"
)

// Product is a catalog item belonging to an account (US-001). Sales funnels
// reference products so the AI knows what it is selling. `Attributes` is a
// free-form JSONB bag for vertical-specific fields (sku, tags, warranty, ...).
type Product struct {
	ID          string          `json:"id"`
	AccountID   string          `json:"account_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	PriceCents  int64           `json:"price_cents"`
	Currency    string          `json:"currency"`
	Attributes  json.RawMessage `json:"attributes"`

	// ConsumptionCycleDays is how long a customer typically takes to go
	// through one order. The fallback when a sale did not record its own
	// cycle — which is most sales, because nobody types a cycle for a
	// product they sell every week.
	ConsumptionCycleDays int `json:"consumption_cycle_days"`
	// ShelfLifeDays is a DIFFERENT number: gauze expires whether or not
	// anyone uses it, and a reorder offered after the expiry date is a
	// reorder for stock the customer throws away.
	ShelfLifeDays int  `json:"shelf_life_days"`
	Active        bool `json:"active"`

	// SourceID / SourcePage point back at the brochure this product was read
	// from. The PDF was already being archived on every import and then
	// orphaned — no record, random object name, unreachable forever. These
	// two fields are what turn that copy into a source a person can open.
	SourceID   string    `json:"source_id,omitempty"`
	SourcePage int       `json:"source_page,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProductRepository defines persistence operations for Products.
type ProductRepository interface {
	Create(ctx context.Context, product *Product) error
	GetByID(ctx context.Context, id string) (*Product, error)
	ListByAccountID(ctx context.Context, accountID string) ([]*Product, error)
	Update(ctx context.Context, product *Product) error
	Delete(ctx context.Context, id string) error
}

var ErrProductNotFound = &AppError{Code: "PRODUCT_NOT_FOUND", Message: "product not found"}
