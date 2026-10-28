package domain

import (
	"context"
	"time"
)

// ProductSource is the brochure a product was read from.
//
// The PDF was already archived on every import and immediately orphaned: a
// random object name, no row, no way back. This record is what makes the copy
// a source — the file a person can open at the page the model read, and the
// raw material anything later (images, a re-read with a better prompt) needs.
type ProductSource struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	// Filename is the name the person recognises ("Incubadora Line Duo G.pdf").
	// ObjectPath is our plumbing and never reaches a screen.
	Filename   string `json:"filename"`
	ObjectPath string `json:"object_path"`
	// ContentHash (sha256, hex) makes re-importing the same brochure reuse
	// this row instead of minting a rival origin for the same pages.
	ContentHash string    `json:"content_hash"`
	Pages       int       `json:"pages"`
	ByteSize    int64     `json:"byte_size"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProductSourceImage is one picture pulled out of a brochure. It belongs to
// the PAGE it was printed on rather than to a product: one page can describe
// several products, and the association a human would make is "the pictures on
// the page this came from".
type ProductSourceImage struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	SourceID    string    `json:"source_id"`
	Page        int       `json:"page"`
	ObjectPath  string    `json:"object_path"`
	ContentType string    `json:"content_type"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	ByteSize    int64     `json:"byte_size"`
	ContentHash string    `json:"content_hash"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProductSourceRepository stores brochure records. Upsert returns the id of
// the row for these bytes, whether it created it or found it.
type ProductSourceRepository interface {
	Upsert(ctx context.Context, src *ProductSource) (string, error)
	GetByID(ctx context.Context, id, accountID string) (*ProductSource, error)
	// AddImage stores one picture. Repeated artwork is silently one row.
	AddImage(ctx context.Context, img *ProductSourceImage) error
	// ListImages returns a source's pictures. A nil page means the whole
	// brochure — a product with no known page still deserves the catalogue's
	// pictures rather than none.
	ListImages(ctx context.Context, sourceID, accountID string, page *int) ([]*ProductSourceImage, error)
}
