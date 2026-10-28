package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Opening the brochure a product came from.
//
// The archive existed for months and was unreachable — no record, random
// object name. With the record in place, this is the door: given a product,
// hand back its source file and the page the model read it from, so a person
// checking "cabe na minha UTI?" reads the manufacturer's own sheet instead of
// trusting a 126-character summary.

var ErrProductSourceNotFound = &domain.AppError{
	Code:    "PRODUCT_SOURCE_NOT_FOUND",
	Message: "este produto não veio de um folheto arquivado (import por CSV ou cadastro manual)",
}

// productByIDReader is the slice of the product repository this needs. Narrow
// on purpose: the wide domain.ProductRepository has a dozen implementations in
// tests, and none of them should have to grow a method for this.
type productByIDReader interface {
	GetByID(ctx context.Context, id string) (*domain.Product, error)
}

// productSourceSigner is the slice of the storage adapter this needs.
type productSourceSigner interface {
	GenerateReadSignedURL(ctx context.Context, objectPath string, expiry time.Duration) (string, error)
}

type ProductSourceLinkUseCase struct {
	products productByIDReader
	sources  domain.ProductSourceRepository
	signer   productSourceSigner
}

func NewProductSourceLinkUseCase(
	products productByIDReader,
	sources domain.ProductSourceRepository,
	signer productSourceSigner,
) *ProductSourceLinkUseCase {
	return &ProductSourceLinkUseCase{products: products, sources: sources, signer: signer}
}

// ProductImage is one picture from the brochure, ready to render or attach.
type ProductImage struct {
	ID     string `json:"id"`
	Page   int    `json:"page"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
}

// Images returns the pictures printed on the page this product was read from.
//
// Scoped to the page on purpose: a forty-page catalogue's worth of artwork
// under one product is a scroll, not an answer to "manda uma foto disso".
// When the page is unknown (0) the whole brochure is offered instead of
// nothing — a picture in the wrong order still beats an empty gallery.
func (uc *ProductSourceLinkUseCase) Images(ctx context.Context, accountID, productID string) ([]ProductImage, error) {
	if uc.products == nil || uc.sources == nil || uc.signer == nil {
		return nil, ErrProductSourceNotFound
	}
	product, err := uc.products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product == nil || product.AccountID != accountID {
		return nil, domain.ErrProductNotFound
	}
	if product.SourceID == "" {
		return nil, ErrProductSourceNotFound
	}
	var page *int
	if product.SourcePage > 0 {
		p := product.SourcePage
		page = &p
	}
	rows, err := uc.sources.ListImages(ctx, product.SourceID, accountID, page)
	if err != nil {
		return nil, err
	}
	// A page with no pictures of its own falls back to the brochure. Asked
	// only when the narrow answer was empty, so the common case stays cheap.
	if len(rows) == 0 && page != nil {
		if rows, err = uc.sources.ListImages(ctx, product.SourceID, accountID, nil); err != nil {
			return nil, err
		}
	}
	out := make([]ProductImage, 0, len(rows))
	for _, row := range rows {
		url, err := uc.signer.GenerateReadSignedURL(ctx, row.ObjectPath, sourceLinkTTL)
		if err != nil {
			// One unsignable object must not empty the gallery.
			slog.WarnContext(ctx, "product images: could not sign object",
				"account_id", accountID, "path", row.ObjectPath, "error", err)
			continue
		}
		out = append(out, ProductImage{
			ID: row.ID, Page: row.Page, Width: row.Width, Height: row.Height, URL: url,
		})
	}
	return out, nil
}

// ProductSourceLink is what the screen needs: the name a person recognises,
// the page to jump to, and a URL that works for a while.
type ProductSourceLink struct {
	Filename string `json:"filename"`
	Page     int    `json:"page"`
	Pages    int    `json:"pages"`
	URL      string `json:"url"`
}

// sourceLinkTTL is short on purpose: the link is for opening a file now, not
// for pasting into a message. Long-lived object URLs are how a private bucket
// stops being private.
const sourceLinkTTL = 30 * time.Minute

func (uc *ProductSourceLinkUseCase) Execute(ctx context.Context, accountID, productID string) (*ProductSourceLink, error) {
	if uc.products == nil || uc.sources == nil || uc.signer == nil {
		return nil, ErrProductSourceNotFound
	}
	product, err := uc.products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	// Ownership before existence: a product of another account answers the
	// same "not found" as one that never existed.
	if product == nil || product.AccountID != accountID {
		return nil, domain.ErrProductNotFound
	}
	if product.SourceID == "" {
		return nil, ErrProductSourceNotFound
	}
	src, err := uc.sources.GetByID(ctx, product.SourceID, accountID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, ErrProductSourceNotFound
	}
	url, err := uc.signer.GenerateReadSignedURL(ctx, src.ObjectPath, sourceLinkTTL)
	if err != nil {
		return nil, err
	}
	return &ProductSourceLink{
		Filename: src.Filename,
		Page:     product.SourcePage,
		Pages:    src.Pages,
		URL:      url,
	}, nil
}
