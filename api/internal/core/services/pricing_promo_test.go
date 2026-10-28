package services

import (
	"testing"
	"time"
)

// TestGeminiPromoPricingExpiry is a dated tripwire: it PASSES until
// geminiPromoEndsAt (2027-01-01) and FAILS from that day on, until somebody
// bumps the gemini-3.7-flash row in modelRates to the post-promo price AND
// moves geminiPromoEndsAt forward (or removes it). The provider_costs table
// flips on its own; this static fallback does not, and a silently stale
// fallback under-bills every account by half.
func TestGeminiPromoPricingExpiry(t *testing.T) {
	now := time.Now().UTC()
	rate := modelRates["gemini-3.7-flash"]

	if now.Before(geminiPromoEndsAt) {
		if rate != geminiPromoRate {
			t.Fatalf("gemini-3.7-flash rate = %+v, expected the introductory %+v until %s",
				rate, geminiPromoRate, geminiPromoEndsAt.Format("2006-01-02"))
		}
		return
	}

	// The promo is over. The table must no longer carry the promo rate.
	if rate == geminiPromoRate {
		t.Fatalf("gemini-3.7-flash introductory pricing ended on %s: bump modelRates[\"gemini-3.7-flash\"] "+
			"in internal/core/services/pricing.go to the post-promo price ($1.50/$7.50 per 1M as announced — "+
			"confirm on the vendor's price page) and then move geminiPromoEndsAt to the next scheduled change "+
			"or delete this test",
			geminiPromoEndsAt.Format("2006-01-02"))
	}
	// Bumped but the constant was left behind: the test would keep running
	// this branch forever. Make that loud too.
	t.Fatalf("modelRates was bumped past the promo but geminiPromoEndsAt (%s) still points at the old date — "+
		"move it forward with the next known price change or delete this test",
		geminiPromoEndsAt.Format("2006-01-02"))
}
