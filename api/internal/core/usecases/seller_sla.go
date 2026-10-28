package usecases

import (
	"context"
	"sort"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Narrow local interface (mock only what the use case touches).
type sellerSLARepo interface {
	ListSLAConversations(ctx context.Context, accountID string, from, to time.Time) ([]domain.SLAConversation, error)
	GetSLAState(ctx context.Context, sellerUserID, cycle string) (*domain.SellerSLAState, error)
}

// SellerSLAUseCase serves the seller panel's live SLA widget (US-012):
// the CURRENT cycle's first-response metric measured on demand with the
// same rules the payroll compute uses (US-009 — business-time window,
// unanswered counts once the window blows past the threshold), the
// ladder step that streak would land on, and the list of breached
// conversations so the seller is never punished by an invisible ruler.
// Read-only: it NEVER persists a seller_sla_state row — ComputeCycle is
// the only writer.
type SellerSLAUseCase struct {
	repo     sellerSLARepo
	settings domain.PlatformSettingRepository
	// platformAccountID is the metric source; the /seller routes 404
	// when it's unset, so this is a belt-and-braces guard.
	platformAccountID string
	// now is swappable in tests.
	now func() time.Time
}

func NewSellerSLAUseCase(repo sellerSLARepo, settings domain.PlatformSettingRepository, platformAccountID string) *SellerSLAUseCase {
	return &SellerSLAUseCase{repo: repo, settings: settings, platformAccountID: platformAccountID, now: time.Now}
}

// SLABreachedConversation is one conversation that blew the
// first-response SLA, linkable from the widget.
type SLABreachedConversation struct {
	ConversationID       string     `json:"conversation_id"`
	ContactName          string     `json:"contact_name"`
	CreatedAt            time.Time  `json:"created_at"`
	FirstHumanResponseAt *time.Time `json:"first_human_response_at,omitempty"`
	// BusinessMinutes the first human response took (or has been
	// pending, when unanswered) inside the business window.
	BusinessMinutes int64 `json:"business_minutes"`
}

// SellerSLAStatus is the GET /seller/sla payload: a live preview of the
// current (open) cycle — the settled states are written by the payroll
// compute at cycle close.
// Counting note (canal-interno US-009): TotalConversations and
// BreachedConversations count SLA UNITS — a customer account's internal
// threads collapse into one unit (see measureSLAUnits) — while Breached
// lists every individual blown thread, so its length can exceed
// BreachedConversations when one account has several late threads.
type SellerSLAStatus struct {
	Cycle                 string `json:"cycle"`
	TotalConversations    int    `json:"total_conversations"`
	BreachedConversations int    `json:"breached_conversations"`
	// WithinSLAPct = 100 × (total − breached) / total; 100 with no
	// conversations yet.
	WithinSLAPct float64 `json:"within_sla_pct"`
	// Breach: the cycle's breached % currently exceeds the configured
	// cut (strict >, same rule as the compute).
	Breach bool `json:"breach"`
	// ConsecutiveBreaches the current cycle would close with (previous
	// cycle's persisted streak + this one).
	ConsecutiveBreaches int `json:"consecutive_breaches"`
	// Action is the ladder step that streak maps to:
	// none/warning/reduced/flag_reallocation.
	Action string `json:"action"`
	// PreviousCycleAction is the last SETTLED ladder step (previous
	// cycle's persisted state; "none" when never computed).
	PreviousCycleAction string                    `json:"previous_cycle_action"`
	Settings            *SLASettings              `json:"settings"`
	Breached            []SLABreachedConversation `json:"breached"`
}

// Execute measures the calling seller's current cycle.
func (uc *SellerSLAUseCase) Execute(ctx context.Context, sellerUserID string) (*SellerSLAStatus, error) {
	nowT := uc.now().UTC()
	cycle := nowT.Format("2006-01")
	from, to, err := parseCycle(cycle)
	if err != nil {
		return nil, err
	}

	cfg, err := loadSLASettings(ctx, uc.settings)
	if err != nil {
		return nil, err
	}
	out := &SellerSLAStatus{
		Cycle:               cycle,
		WithinSLAPct:        100,
		Action:              domain.SLAActionNone,
		PreviousCycleAction: domain.SLAActionNone,
		Settings:            cfg,
		Breached:            []SLABreachedConversation{},
	}

	prevCycle := from.AddDate(0, -1, 0).Format("2006-01")
	prev, err := uc.repo.GetSLAState(ctx, sellerUserID, prevCycle)
	if err != nil {
		return nil, err
	}
	if prev != nil {
		out.PreviousCycleAction = prev.Action
	}

	if uc.platformAccountID == "" {
		return out, nil // no metric source — widget shows all-clear
	}

	convs, err := uc.repo.ListSLAConversations(ctx, uc.platformAccountID, from, to)
	if err != nil {
		return nil, err
	}
	// Open cycle: unanswered conversations are measured up to now, so a
	// fresh one isn't punished before its window runs out (same rule as
	// the compute's prévia).
	unansweredCutoff := nowT
	if to.Before(unansweredCutoff) {
		unansweredCutoff = to
	}
	for _, u := range measureSLAUnits(convs, cfg, unansweredCutoff) {
		if u.AssignedTo != sellerUserID {
			continue
		}
		out.TotalConversations++
		if !u.Breached {
			continue
		}
		out.BreachedConversations++
		out.Breached = append(out.Breached, u.BreachedThreads...)
	}
	sort.Slice(out.Breached, func(i, j int) bool {
		return out.Breached[i].CreatedAt.After(out.Breached[j].CreatedAt)
	})

	if out.TotalConversations > 0 {
		out.WithinSLAPct = float64(out.TotalConversations-out.BreachedConversations) * 100 / float64(out.TotalConversations)
		// Strict >: exactly at the cut is NOT a breach (same as compute).
		out.Breach = int64(out.BreachedConversations)*100 > int64(out.TotalConversations)*cfg.BreachPct
	}
	if out.Breach {
		out.ConsecutiveBreaches = 1
		if prev != nil && prev.Breach {
			out.ConsecutiveBreaches = prev.ConsecutiveBreaches + 1
		}
	}
	out.Action = slaActionForStreak(out.ConsecutiveBreaches)
	return out, nil
}
