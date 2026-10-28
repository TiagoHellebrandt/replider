package usecases

import (
	"context"
	"log/slog"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Finding the customers, without being asked.
//
// The product's promise is that it goes looking. Before this, the CNES park —
// tens of thousands of establishments with the exact equipment an account
// sells — only turned into leads when somebody opened the hunt screen and
// typed a word. An account could hold ninety products, a whole state's worth
// of matching hospitals in the database, and one lead.
//
// So the catalogue asks the question by itself, nightly: what does this
// account sell, who in its territory has that equipment, and which of them is
// not in the funnel yet.
//
// It stops at the lead. Writing the approach costs a model call on the
// customer's own key, and spending that on ground nobody has looked at is the
// difference between a tool that works for you and one that bills you while
// you sleep — so drafting stays a thing a person asks for.

// autoHuntCaps bound one night's work.
const (
	// New leads per account per night. A first run against a whole state
	// could mint hundreds; a funnel nobody can work through in a week is a
	// funnel that gets abandoned. The rest are still in the Radar, and
	// tomorrow's run takes the next batch.
	maxNewLeadsPerRun = 15
	// Establishments read per page, most-equipment first. Wider than the
	// lead cap so a page whose hospitals are already customers still leaves
	// room to fill the night.
	maxHuntCandidates = 60
	// Pages one night may walk.
	//
	// This is the difference between a sweep and a doorstep. There was no
	// paging at all: every night read the SAME first page, so the hunt's
	// reach was one page for the life of the account. Production on
	// 2026-09-17 — 60 hospitals in the funnel, 2.818 matching establishments
	// in the territory, nine consecutive nights of `found=52 created=0`.
	//
	// Five pages is 300 establishments a night, which walks a state the size
	// of Mato Grosso in ten nights and still spends at most maxNewLeadsPerRun
	// on the funnel.
	maxHuntPagesPerRun = 5
)

// untouchedLister answers "who in this territory holds what I sell and has
// never been approached", biggest park first.
//
// The SAME answer the home screen's tile is built from, on purpose. This
// worker used to compute its own: it asked the CNES dictionary about the six
// LONGEST words in the catalogue, and on a real one the longest words are
// "antitransbordamento", "multiparamétricos", "bilirrubinômetro" — none of
// which the registry knows. So it asked six questions nothing could answer and
// reported, truthfully and uselessly, that it had found nothing, while
// "incubadora" and "ventilador" sat further down the list unasked.
type untouchedLister interface {
	ListPage(ctx context.Context, accountID string, offset, limit int) (UntouchedPage, error)
}

// leadEnsurer turns one CNES establishment into a lead, without drafting.
type leadEnsurer interface {
	EnsureLead(ctx context.Context, accountID, cnes string) (*domain.Lead, bool, error)
}

// huntCandidateLister names the accounts worth hunting for.
type huntCandidateLister interface {
	AccountsForAutoHunt(ctx context.Context) ([]domain.SignalWatchCandidate, error)
}

// huntCursorStore remembers where a territory was left off. Optional: with
// no store every night restarts at the top, which is the old behaviour and
// the old ceiling.
type huntCursorStore interface {
	HuntCursor(ctx context.Context, accountID string) (int, error)
	SetHuntCursor(ctx context.Context, accountID string, offset int) error
}

type AutoHuntUseCase struct {
	candidates huntCandidateLister
	targets    untouchedLister
	leads      leadEnsurer
	cursors    huntCursorStore
}

// SetCursorStore wires the per-account reading position.
func (uc *AutoHuntUseCase) SetCursorStore(s huntCursorStore) { uc.cursors = s }

func NewAutoHuntUseCase(
	candidates huntCandidateLister,
	targets untouchedLister,
	leads leadEnsurer,
) *AutoHuntUseCase {
	return &AutoHuntUseCase{candidates: candidates, targets: targets, leads: leads}
}

// AutoHuntResult is what one night produced, per account.
type AutoHuntResult struct {
	AccountID string
	Found     int // establishments the territory offered
	Created   int // leads that did not exist before
}

// Run hunts for every eligible account. One account's failure never stops the
// others: this is a nightly sweep, not a transaction.
func (uc *AutoHuntUseCase) Run(ctx context.Context) []AutoHuntResult {
	if uc.candidates == nil || uc.targets == nil || uc.leads == nil {
		return nil
	}
	accounts, err := uc.candidates.AccountsForAutoHunt(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "auto hunt: could not list accounts", "error", err)
		return nil
	}
	out := make([]AutoHuntResult, 0, len(accounts))
	for _, acc := range accounts {
		res, err := uc.RunForAccount(ctx, acc.AccountID)
		if err != nil {
			slog.WarnContext(ctx, "auto hunt: account failed",
				"account_id", acc.AccountID, "error", err)
			continue
		}
		out = append(out, res)
	}
	return out
}

// RunForAccount turns this account's untouched territory into leads.
//
// It WALKS the territory: from wherever the last night stopped, page by page,
// until the night's lead cap is full or the page budget runs out. Reaching the
// end of the ranking wraps the cursor back to the top — the park is
// republished monthly and a hospital a name collision swallowed deserves
// another pass, so the sweep is a cycle, not a one-way scan that ends in
// silence.
func (uc *AutoHuntUseCase) RunForAccount(ctx context.Context, accountID string) (AutoHuntResult, error) {
	res := AutoHuntResult{AccountID: accountID}
	if uc.targets == nil || uc.leads == nil {
		return res, nil
	}

	offset := uc.cursorFor(ctx, accountID)
	scanned, wrapped := 0, false
	for page := 0; page < maxHuntPagesPerRun && res.Created < maxNewLeadsPerRun; page++ {
		got, err := uc.targets.ListPage(ctx, accountID, offset, maxHuntCandidates)
		if err != nil {
			// The cursor is left where it was: a page that failed to read
			// has not been worked, and skipping it would lose that ground
			// until the next full lap.
			return res, err
		}
		// END OF THE RANKING IS `Scanned == 0`, NOT AN EMPTY WORK LIST.
		//
		// A page can hold sixty hospitals and offer zero of them — that is
		// what the FIRST page looks like the moment this ships, since those
		// sixty are precisely the ground the old sweep already took. Treating
		// it as the end would stop the walk on page one and reproduce the
		// exact ceiling paging exists to remove.
		if got.Scanned == 0 {
			// Past the end. The cursor restarts at the top — but only when
			// this run has read nothing yet, because wrapping after reading
			// is walking the same ground twice in one night. A run that
			// reaches the end mid-way stops and leaves the cursor there;
			// tomorrow's first page is the empty one, and tomorrow is when
			// it wraps.
			if scanned > 0 || wrapped || offset == 0 {
				break
			}
			wrapped = true
			offset = 0
			continue
		}
		scanned += got.Scanned
		res.Found += len(got.Holders)
		offset += got.Scanned
		uc.workPage(ctx, accountID, got.Holders, &res)
	}
	uc.saveCursor(ctx, accountID, offset)
	return res, nil
}

// workPage turns one page of untouched establishments into leads, stopping at
// the night's cap.
func (uc *AutoHuntUseCase) workPage(
	ctx context.Context, accountID string, holders []domain.CNESEquipmentHolder, res *AutoHuntResult,
) {
	for _, holder := range holders {
		if res.Created >= maxNewLeadsPerRun {
			return
		}
		cnes := strings.TrimSpace(holder.CNES)
		if cnes == "" {
			continue
		}
		_, existed, err := uc.leads.EnsureLead(ctx, accountID, cnes)
		if err != nil {
			slog.WarnContext(ctx, "auto hunt: could not create lead",
				"account_id", accountID, "cnes", cnes, "error", err)
			continue
		}
		if !existed {
			res.Created++
		}
	}
}

// cursorFor reads where to start. A store that cannot answer starts at the
// top: re-reading ground is a wasted page, losing the account's turn is a
// night with no leads.
func (uc *AutoHuntUseCase) cursorFor(ctx context.Context, accountID string) int {
	if uc.cursors == nil {
		return 0
	}
	offset, err := uc.cursors.HuntCursor(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "auto hunt: could not read the territory cursor",
			"account_id", accountID, "error", err)
		return 0
	}
	if offset < 0 {
		return 0
	}
	return offset
}

func (uc *AutoHuntUseCase) saveCursor(ctx context.Context, accountID string, offset int) {
	if uc.cursors == nil {
		return
	}
	if err := uc.cursors.SetHuntCursor(ctx, accountID, offset); err != nil {
		slog.WarnContext(ctx, "auto hunt: could not save the territory cursor",
			"account_id", accountID, "error", err)
	}
}
