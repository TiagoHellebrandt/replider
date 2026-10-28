-- Coupons / invite links for the owner panel. The discount itself
-- lives in Stripe (we POST to /v1/coupons + /v1/promotion_codes on
-- creation), so this table is just the local registry: it pairs the
-- public-facing code with the Stripe IDs and stores the audit
-- fields the owner panel needs to render its list.
--
--   code              Public-facing slug (uppercase, [A-Z0-9-_]). The
--                     register page reads it from `?coupon=` and the
--                     checkout flow forwards the linked stripe_coupon_id
--                     to Stripe Checkout's `discounts` field.
--   stripe_coupon_id  ID of the underlying Stripe Coupon (durable
--                     across promotion_code rotations).
--   stripe_promo_id   ID of the Stripe Promotion Code that owns the
--                     human-readable string. Nullable so we can keep
--                     a coupon active in our DB even if Stripe
--                     archives the promo (e.g. expired).
--   percent_off       Whole-number percent discount (1..100). We don't
--                     model amount_off today — Brazilian self-serve
--                     plans are flat BRL prices and the owner panel UX
--                     is much cleaner with a single dimension.
--   max_redemptions   NULL = unlimited. Mirrors Stripe semantics.
--   redeemed_count    Local cache of /v1/promotion_codes.times_redeemed
--                     so the owner panel can show usage without an
--                     API roundtrip per row. Refreshed on the next
--                     /v1/owner/coupons GET.
--   expires_at        Hard-stop date. Independent of Stripe's
--                     `expires_at` so we can deactivate locally
--                     without touching the upstream coupon.
--   created_by        Operator who created the coupon (audit trail —
--                     paired with audit_logs.metadata.coupon_id).
CREATE TABLE coupons (
    id                CHAR(26) PRIMARY KEY,
    code              VARCHAR(64) UNIQUE NOT NULL,
    stripe_coupon_id  VARCHAR(255) NOT NULL,
    stripe_promo_id   VARCHAR(255),
    percent_off       INTEGER NOT NULL CHECK (percent_off > 0 AND percent_off <= 100),
    max_redemptions   INTEGER,
    redeemed_count    INTEGER NOT NULL DEFAULT 0,
    expires_at        TIMESTAMPTZ,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        VARCHAR(255) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_coupons_active_code ON coupons(code) WHERE is_active = TRUE;
