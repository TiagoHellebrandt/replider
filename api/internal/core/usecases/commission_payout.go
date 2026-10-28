package usecases

import (
	"bytes"
	"context"
	"encoding/csv"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Owner payout endpoints (US-013, vendedores v3) live on the
// ComputeCommissionsUseCase — same repo, same audit trail, same wiring.
//
// The flow is deliberately manual: the owner reads each seller's total
// due (cycle pending + earlier-cycle adjustments), transfers via PIX
// outside the system, then marks the seller settled. Marking flips the
// pending rows to paid (idempotent — a second click matches nothing);
// entries a LATE recompute mints afterwards stay pending and surface as
// "ajuste" on the NEXT cycle's payout, settling with it.

// maxPayoutNoteLen caps the free-text PIX transaction note.
const maxPayoutNoteLen = 280

// PayoutCycleListing bundles one cycle's per-seller payout summaries
// with the cycle totals the owner screen shows.
type PayoutCycleListing struct {
	Cycle          string                       `json:"cycle"`
	Sellers        []domain.SellerPayoutSummary `json:"sellers"`
	TotalDueCents  int64                        `json:"total_due_cents"`
	TotalPaidCents int64                        `json:"total_paid_cents"`
}

// ListPayouts returns the cycle's payout summaries grouped per seller.
func (uc *ComputeCommissionsUseCase) ListPayouts(ctx context.Context, cycle string) (*PayoutCycleListing, error) {
	if _, _, err := parseCycle(cycle); err != nil {
		return nil, err
	}
	sellers, err := uc.repo.ListPayoutSummaries(ctx, cycle)
	if err != nil {
		return nil, err
	}
	out := &PayoutCycleListing{Cycle: cycle, Sellers: sellers}
	for _, s := range sellers {
		out.TotalDueCents += s.TotalDueCents
		out.TotalPaidCents += s.PaidCents
	}
	return out, nil
}

// MarkPayoutPaid settles a seller's pending entries through the cycle
// (earlier-cycle adjustments included) and returns the refreshed
// listing so the panel updates in one roundtrip. note is the optional
// PIX transaction reference stamped on every settled row.
//
// US-005 guard: only CLOSED cycles settle by default — paying the
// running month freezes a partial number and every later recompute
// becomes an adjustment. allowOpenCycle is the explicit "pagar ciclo em
// aberto" override; the audit trail records when it was used.
func (uc *ComputeCommissionsUseCase) MarkPayoutPaid(ctx context.Context, actorEmail, sellerUserID, cycle, note string, allowOpenCycle bool) (*PayoutCycleListing, error) {
	if _, _, err := parseCycle(cycle); err != nil {
		return nil, err
	}
	if sellerUserID == "" {
		return nil, errCommissionSellerNotFound()
	}
	if !allowOpenCycle && cycle >= uc.currentCycle() {
		return nil, &domain.AppError{
			Code:    "COMMISSION_CYCLE_OPEN",
			Message: "cycle is still open; settling a partial cycle requires allow_open_cycle",
		}
	}
	note = strings.TrimSpace(note)
	if len(note) > maxPayoutNoteLen {
		return nil, &domain.AppError{Code: "COMMISSION_INVALID_NOTE", Message: "note must be at most 280 characters"}
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	// US-006: the repo settles in a transaction under the cycles'
	// advisory locks and reports the total from the rows it actually
	// flipped — a concurrent recompute commits first or waits, so the
	// number below is never a stale screen snapshot.
	updated, totalCents, err := uc.repo.MarkPayoutPaid(ctx, sellerUserID, cycle, uc.now(), notePtr)
	if err != nil {
		return nil, err
	}

	uc.auditPayload(ctx, actorEmail, "commission.mark_paid", sellerUserID, map[string]any{
		"cycle":            cycle,
		"entries":          updated,
		"total_cents":      totalCents,
		"allow_open_cycle": allowOpenCycle,
	})
	slog.Info("owner marked seller payout paid",
		"actor", actorEmail, "seller_user_id", sellerUserID, "cycle", cycle,
		"entries", updated, "total_cents", totalCents, "allow_open_cycle", allowOpenCycle)
	return uc.ListPayouts(ctx, cycle)
}

// currentCycle is the running YYYY-MM in the platform's business
// timezone (the SLA window's America/Cuiaba) — cycles compare
// lexicographically, so cycle >= currentCycle() means "not closed yet".
func (uc *ComputeCommissionsUseCase) currentCycle() string {
	loc, err := time.LoadLocation(DefaultSLATimezone)
	if err != nil {
		loc = time.UTC
	}
	return uc.now().In(loc).Format("2006-01")
}

// ExportPayoutCSV renders the cycle's full ledger as CSV for
// accounting: one row per entry with the seller's PIX key joined.
func (uc *ComputeCommissionsUseCase) ExportPayoutCSV(ctx context.Context, cycle string) ([]byte, error) {
	if _, _, err := parseCycle(cycle); err != nil {
		return nil, err
	}
	entries, err := uc.repo.ListEntriesByCycle(ctx, cycle)
	if err != nil {
		return nil, err
	}
	summaries, err := uc.repo.ListPayoutSummaries(ctx, cycle)
	if err != nil {
		return nil, err
	}
	pixBySeller := map[string]string{}
	for _, s := range summaries {
		if s.PixKey != nil {
			pixBySeller[s.SellerUserID] = *s.PixKey
		}
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{
		"cycle", "seller_email", "seller_pix_key", "account_name", "reason",
		"base_net_profit_cents", "rate_applied", "amount_cents", "status", "paid_at", "note",
	}); err != nil {
		return nil, err
	}
	for _, e := range entries {
		paidAt, note := "", ""
		if e.PaidAt != nil {
			paidAt = e.PaidAt.UTC().Format(time.RFC3339)
		}
		if e.Note != nil {
			note = *e.Note
		}
		if err := w.Write([]string{
			e.Cycle, e.SellerEmail, pixBySeller[e.SellerUserID], e.AccountName, e.Reason,
			strconv.FormatInt(e.BaseNetProfitCents, 10),
			strconv.FormatFloat(e.RateApplied, 'f', -1, 64),
			strconv.FormatInt(e.AmountCents, 10),
			e.Status, paidAt, note,
		}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
