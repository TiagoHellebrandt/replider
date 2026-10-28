// seed_stripe_prices creates Stripe Products + monthly recurring Prices
// for every plan that's missing a stripe_price_id, then updates the
// plans row with the new Stripe ID. It also creates a yearly recurring
// Price (annualDiscount off 12× the monthly rate, US-169) and persists
// it to stripe_price_id_annual. Idempotent: re-running only touches
// plans whose respective column is still empty, so you can add a fifth
// plan next month — or backfill annual prices on an existing DB — and
// rerun without duplicating.
//
// Usage (from api/):
//
//	make seed-stripe                    # uses .env via godotenv
//	STRIPE_SECRET_KEY=... go run ./cmd/seed_stripe_prices
//
// Pre-reqs:
//
//   - STRIPE_SECRET_KEY set (sk_test_... for dev, sk_live_... for prod).
//     The script refuses to run in live mode unless STRIPE_SEED_LIVE=1
//     is also set — cheap guardrail against fat-fingering a prod seed.
//   - DB env vars (DB_HOST/DB_PORT/...) pointing at the target database.
//
// Prices are configured inline in the `planSeeds` slice below. Keeping
// them in code (vs. JSON config) makes "which plan costs what" a
// code-review concern rather than a drift-prone ops detail.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/product"
)

// planSeed ties a row in the plans table (by id) to the Stripe pricing
// details we want to create. priceCents is BRL cents; the Stripe Price
// is created as a recurring monthly charge.
type planSeed struct {
	planID      string
	productName string
	priceCents  int64
}

// planSeeds mirrors the 2026-04 six-tier grid in web/src/i18n/ui.ts.
// Partner is intentionally omitted — it's commercial handshake-only
// and never has a Stripe Price (plans.stripe_price_id stays NULL).
//
// Keep this list in sync with migration 000033. If you change a plan's
// price in the landing copy, rerun this script against a fresh DB
// (it skips plans whose stripe_price_id is already populated, so
// production is safe).
//
// Plan IDs are the ULIDs from migrations 000001 + 000033.
var planSeeds = []planSeed{
	{planID: "01PLAN_SOLO_00000000000", productName: "Replider Solo", priceCents: 19900},     // R$ 199/mo
	{planID: "01PLAN_BRONZE_000000000", productName: "Replider Starter", priceCents: 39900},  // R$ 399/mo
	{planID: "01PLAN_SILVER_000000000", productName: "Replider Pro", priceCents: 89900},      // R$ 899/mo
	{planID: "01PLAN_GOLD_0000000000", productName: "Replider Business", priceCents: 199000}, // R$ 1.990/mo
	{planID: "01PLAN_DIAMOND_00000000", productName: "Replider Scale", priceCents: 399000},   // R$ 3.990/mo
}

// annualDiscount is the fraction off 12× the monthly rate when billed
// yearly (US-169). 0.20 == "Save 20%", matching the landing-page copy
// (web/src/data/pricing-plans.ts ANNUAL_DISCOUNT).
const annualDiscount = 0.20

// annualPriceCents returns the yearly charge for a monthly rate: 12
// months at a 20% discount, rounded to the nearest centavo.
func annualPriceCents(monthlyCents int64) int64 {
	yearly := float64(monthlyCents) * 12 * (1 - annualDiscount)
	return int64(yearly + 0.5)
}

func main() {
	// Load .env if present — optional so CI/prod can use real env vars.
	_ = godotenv.Load()

	secret := os.Getenv("STRIPE_SECRET_KEY")
	if secret == "" {
		log.Fatal("STRIPE_SECRET_KEY not set")
	}
	if strings.HasPrefix(secret, "sk_live_") && os.Getenv("STRIPE_SEED_LIVE") != "1" {
		log.Fatal("refusing to run against live Stripe key without STRIPE_SEED_LIVE=1")
	}
	stripe.Key = secret

	ctx := context.Background()
	pool, err := openDB(ctx)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	for _, s := range planSeeds {
		if err := seedOne(ctx, pool, s); err != nil {
			log.Fatalf("seed %s: %v", s.planID, err)
		}
	}
	slog.Info("done — every plan has a stripe_price_id")
}

// seedOne fetches the plan row and backfills whichever Stripe Price ids
// are still empty: the monthly stripe_price_id and the yearly
// stripe_price_id_annual (US-169). Both prices attach to one Product per
// plan. A plan with both columns populated is skipped entirely; a plan
// with only the monthly price (e.g. a DB seeded before US-169) gets just
// its annual price backfilled, reusing the monthly price's Product.
func seedOne(ctx context.Context, pool *pgxpool.Pool, s planSeed) error {
	var monthlyID, annualID *string
	err := pool.QueryRow(ctx,
		`SELECT stripe_price_id, stripe_price_id_annual FROM plans WHERE id = $1`, s.planID,
	).Scan(&monthlyID, &annualID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("plan not found in DB, skipping", "plan_id", s.planID)
			return nil
		}
		return fmt.Errorf("select plan: %w", err)
	}

	hasMonthly := monthlyID != nil && *monthlyID != ""
	hasAnnual := annualID != nil && *annualID != ""
	if hasMonthly && hasAnnual {
		slog.Info("plan already seeded (monthly + annual), skipping", "plan_id", s.planID)
		return nil
	}

	// Resolve the Product these prices attach to. Reuse the existing
	// monthly price's Product when present (annual-only backfill);
	// otherwise create a fresh Product.
	productID := ""
	if hasMonthly {
		pr, gerr := price.Get(*monthlyID, nil)
		if gerr != nil {
			return fmt.Errorf("get monthly price %s: %w", *monthlyID, gerr)
		}
		if pr.Product != nil {
			productID = pr.Product.ID
		}
	}
	if productID == "" {
		prod, perr := product.New(&stripe.ProductParams{
			Name: stripe.String(s.productName),
			Metadata: map[string]string{
				"plan_id":   s.planID,
				"seeded_by": "replider/seed_stripe_prices",
			},
		})
		if perr != nil {
			return fmt.Errorf("create product: %w", perr)
		}
		productID = prod.ID
	}

	// Monthly recurring BRL Price.
	if !hasMonthly {
		pr, perr := price.New(&stripe.PriceParams{
			Product:    stripe.String(productID),
			Currency:   stripe.String("brl"),
			UnitAmount: stripe.Int64(s.priceCents),
			Recurring: &stripe.PriceRecurringParams{
				Interval: stripe.String("month"),
			},
			Metadata: map[string]string{"plan_id": s.planID},
		})
		if perr != nil {
			return fmt.Errorf("create monthly price: %w", perr)
		}
		if _, eerr := pool.Exec(ctx,
			`UPDATE plans SET stripe_price_id = $1, updated_at = NOW() WHERE id = $2`,
			pr.ID, s.planID,
		); eerr != nil {
			return fmt.Errorf("update plan monthly: %w", eerr)
		}
		slog.Info("seeded monthly", "plan_id", s.planID, "product_id", productID, "price_id", pr.ID, "amount_cents", s.priceCents)
	}

	// Yearly recurring BRL Price (20% off 12× monthly).
	if !hasAnnual {
		annualCents := annualPriceCents(s.priceCents)
		pr, perr := price.New(&stripe.PriceParams{
			Product:    stripe.String(productID),
			Currency:   stripe.String("brl"),
			UnitAmount: stripe.Int64(annualCents),
			Recurring: &stripe.PriceRecurringParams{
				Interval: stripe.String("year"),
			},
			Metadata: map[string]string{"plan_id": s.planID, "interval": "annual"},
		})
		if perr != nil {
			return fmt.Errorf("create annual price: %w", perr)
		}
		if _, eerr := pool.Exec(ctx,
			`UPDATE plans SET stripe_price_id_annual = $1, updated_at = NOW() WHERE id = $2`,
			pr.ID, s.planID,
		); eerr != nil {
			return fmt.Errorf("update plan annual: %w", eerr)
		}
		slog.Info("seeded annual", "plan_id", s.planID, "product_id", productID, "price_id", pr.ID, "amount_cents", annualCents)
	}

	return nil
}

func openDB(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getenvOr("DB_USER", "replider"),
		getenvOr("DB_PASSWORD", "secret"),
		getenvOr("DB_HOST", "localhost"),
		getenvOr("DB_PORT", "5432"),
		getenvOr("DB_NAME", "replider_db"),
		getenvOr("DB_SSLMODE", "disable"),
	)
	return pgxpool.New(ctx, dsn)
}

func getenvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
