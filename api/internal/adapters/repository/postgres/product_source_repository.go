package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// ProductSourceRepository stores the brochures products were read from.
type ProductSourceRepository struct {
	queries *Queries
}

func NewProductSourceRepository(db DBTX) *ProductSourceRepository {
	return &ProductSourceRepository{queries: New(db)}
}

// Upsert returns the id of the row for these bytes, creating it or finding the
// one already there. Keyed on (account, content hash) so the same brochure
// re-sent under a different filename does not mint a rival origin.
func (r *ProductSourceRepository) Upsert(ctx context.Context, src *domain.ProductSource) (string, error) {
	if src == nil {
		return "", errors.New("product source: nil")
	}
	id := src.ID
	if id == "" {
		id = ulid.Make().String()
	}
	q := getQueries(ctx, r.queries)
	return q.UpsertProductSource(ctx, UpsertProductSourceParams{
		ID:          id,
		AccountID:   src.AccountID,
		Filename:    src.Filename,
		ObjectPath:  src.ObjectPath,
		ContentHash: src.ContentHash,
		Pages:       int32(src.Pages), //nolint:gosec // G115: bounded by the import's page cap
		ByteSize:    src.ByteSize,
	})
}

func (r *ProductSourceRepository) GetByID(ctx context.Context, id, accountID string) (*domain.ProductSource, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetProductSource(ctx, GetProductSourceParams{ID: id, AccountID: accountID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.ProductSource{
		ID:          row.ID,
		AccountID:   row.AccountID,
		Filename:    row.Filename,
		ObjectPath:  row.ObjectPath,
		ContentHash: row.ContentHash,
		Pages:       int(row.Pages),
		ByteSize:    row.ByteSize,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *ProductSourceRepository) AddImage(ctx context.Context, img *domain.ProductSourceImage) error {
	if img == nil {
		return errors.New("product source image: nil")
	}
	id := img.ID
	if id == "" {
		id = ulid.Make().String()
	}
	q := getQueries(ctx, r.queries)
	return q.InsertProductSourceImage(ctx, InsertProductSourceImageParams{
		ID:          id,
		AccountID:   img.AccountID,
		SourceID:    img.SourceID,
		Page:        int32(img.Page), //nolint:gosec // G115: a page number, bounded by the import's page cap
		ObjectPath:  img.ObjectPath,
		ContentType: img.ContentType,
		Width:       int32(img.Width),  //nolint:gosec // G115: pixel dimensions of a brochure image
		Height:      int32(img.Height), //nolint:gosec // G115: pixel dimensions of a brochure image
		ByteSize:    img.ByteSize,
		ContentHash: img.ContentHash,
	})
}

func (r *ProductSourceRepository) ListImages(
	ctx context.Context, sourceID, accountID string, page *int,
) ([]*domain.ProductSourceImage, error) {
	arg := ListProductSourceImagesParams{SourceID: sourceID, AccountID: accountID}
	if page != nil {
		p32 := int32(*page) //nolint:gosec // G115: a page number, bounded by the import's page cap
		arg.Page = &p32
	}
	q := getQueries(ctx, r.queries)
	rows, err := q.ListProductSourceImages(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.ProductSourceImage, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.ProductSourceImage{
			ID:          row.ID,
			AccountID:   row.AccountID,
			SourceID:    row.SourceID,
			Page:        int(row.Page),
			ObjectPath:  row.ObjectPath,
			ContentType: row.ContentType,
			Width:       int(row.Width),
			Height:      int(row.Height),
			ByteSize:    row.ByteSize,
			ContentHash: row.ContentHash,
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return out, nil
}
