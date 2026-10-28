package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

type CouponRepository struct {
	queries *Queries
}

func NewCouponRepository(db DBTX) *CouponRepository {
	return &CouponRepository{queries: New(db)}
}

func mapCouponToDomain(row Coupon) *domain.Coupon {
	c := &domain.Coupon{
		ID:             row.ID,
		Code:           row.Code,
		StripeCouponID: row.StripeCouponID,
		StripePromoID:  row.StripePromoID,
		PercentOff:     int(row.PercentOff),
		RedeemedCount:  int(row.RedeemedCount),
		IsActive:       row.IsActive,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
	if row.MaxRedemptions != nil {
		v := int(*row.MaxRedemptions)
		c.MaxRedemptions = &v
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		c.ExpiresAt = &t
	}
	return c
}

func (r *CouponRepository) Insert(ctx context.Context, c *domain.Coupon) error {
	q := getQueries(ctx, r.queries)
	var maxRedemptions *int32
	if c.MaxRedemptions != nil {
		v := int32(*c.MaxRedemptions)
		maxRedemptions = &v
	}
	expires := pgtype.Timestamptz{}
	if c.ExpiresAt != nil {
		expires = pgtype.Timestamptz{Time: *c.ExpiresAt, Valid: true}
	}
	return q.InsertCoupon(ctx, InsertCouponParams{
		ID:             c.ID,
		Code:           c.Code,
		StripeCouponID: c.StripeCouponID,
		StripePromoID:  c.StripePromoID,
		PercentOff:     int32(c.PercentOff), //nolint:gosec // G115: percent discount is 0..100
		MaxRedemptions: maxRedemptions,
		ExpiresAt:      expires,
		CreatedBy:      c.CreatedBy,
	})
}

func (r *CouponRepository) GetByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetCouponByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapCouponToDomain(row), nil
}

func (r *CouponRepository) GetByID(ctx context.Context, id string) (*domain.Coupon, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetCouponByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapCouponToDomain(row), nil
}

func (r *CouponRepository) List(ctx context.Context, limit, offset int) ([]*domain.Coupon, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListCoupons(ctx, ListCouponsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Coupon, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCouponToDomain(row))
	}
	return out, nil
}

func (r *CouponRepository) SetActive(ctx context.Context, id string, active bool) error {
	q := getQueries(ctx, r.queries)
	return q.SetCouponActive(ctx, SetCouponActiveParams{ID: id, IsActive: active})
}

func (r *CouponRepository) BumpRedeemed(ctx context.Context, id string, count int) error {
	q := getQueries(ctx, r.queries)
	return q.BumpCouponRedeemed(ctx, BumpCouponRedeemedParams{
		ID:            id,
		RedeemedCount: int32(count),
	})
}
