-- name: InsertCoupon :exec
INSERT INTO coupons (
    id, code, stripe_coupon_id, stripe_promo_id, percent_off,
    max_redemptions, expires_at, created_by, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
);

-- name: GetCouponByCode :one
SELECT * FROM coupons WHERE code = $1 LIMIT 1;

-- name: GetCouponByID :one
SELECT * FROM coupons WHERE id = $1 LIMIT 1;

-- name: ListCoupons :many
SELECT * FROM coupons ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: SetCouponActive :exec
UPDATE coupons SET is_active = $2, updated_at = NOW() WHERE id = $1;

-- name: BumpCouponRedeemed :exec
UPDATE coupons
SET redeemed_count = $2, updated_at = NOW()
WHERE id = $1;
