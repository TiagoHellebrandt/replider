package domain

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Command centre (§6.13, I2).
//
// The first screen anyone opens, and it exists to answer one question: what
// needs me right now? Which means the list has to be RANKED, not merely
// complete — a "needs you" section of eleven equally-weighted rows is a section
// nobody reads.
//
// Two rules run through the file:
//
//  1. Every action is DERIVED from live state. Doing the work makes the row
//     vanish on its own; there is no "mark as done" that could disagree with
//     reality. The only stored thing is a SNOOZE (see PendingActionDismissal),
//     and a snooze always expires.
//  2. Every row names the CUSTOMER. "3 assinaturas pendentes" is a statistic;
//     "Clínica Bem Viver abriu e não assinou — R$ 100.000" is a phone call.

// PendingActionKind is what needs doing. A closed set: each kind is one query
// with one deep link, and an unknown kind on the client would render a row
// nobody can act on.
type PendingActionKind string

const (
	// PendingApproveDraft is an AI-written approach waiting for a human to
	// approve it. The queue that means "someone is waiting on us".
	PendingApproveDraft PendingActionKind = "approve_draft"
	// PendingUnownedLead is a live lead nobody is responsible for. Managers
	// only — a salesperson cannot assign leads, so it would be a nag they
	// cannot act on.
	PendingUnownedLead PendingActionKind = "unowned_lead"
	// PendingSignatureViewed is a document the customer OPENED and did not
	// sign. The highest-intent moment in the whole product: they looked.
	PendingSignatureViewed PendingActionKind = "signature_viewed"
	// PendingContractAlert is a contract deadline — a renewal that will fire
	// itself, an indexed month, an ata with balance left.
	PendingContractAlert PendingActionKind = "contract_alert"
	// PendingConfirmSale is an AI-extracted sale no human has confirmed.
	// Unconfirmed, it pays nobody a commission and enters no portfolio.
	PendingConfirmSale PendingActionKind = "confirm_sale"
	// PendingReorderDue is a customer past the cycle their last purchase
	// should have lasted.
	PendingReorderDue PendingActionKind = "reorder_due"
	// PendingWaitingReply is a customer whose last message has no human
	// answer yet (conversations.waiting_since). The oldest silence is the
	// most expensive one.
	PendingWaitingReply PendingActionKind = "waiting_reply"
	// PendingUntouchedTarget is an establishment in the account's territory
	// that holds the equipment this account sells and has never been
	// approached. The opportunity being LOST rather than the one in hand —
	// and named, because "480 hospitais" is a statistic while "Santa Casa de
	// Rondonópolis, 63 aparelhos, nunca abordada" is a phone call.
	PendingUntouchedTarget PendingActionKind = "untouched_target"
)

// ValidPendingActionKind guards a value arriving from a client (a dismissal
// names its kind).
func ValidPendingActionKind(v string) bool {
	switch PendingActionKind(v) {
	case PendingApproveDraft, PendingUnownedLead, PendingSignatureViewed,
		PendingContractAlert, PendingConfirmSale, PendingReorderDue,
		PendingWaitingReply, PendingUntouchedTarget:
		return true
	}
	return false
}

// PendingAction is one row of "Precisa de você".
type PendingAction struct {
	Kind PendingActionKind `json:"kind"`
	// TargetID is what the action is about (a lead, a signature request, an
	// alert). Also the dismissal key, so a snooze survives the row being
	// recomputed from scratch on the next load.
	TargetID string `json:"target_id"`

	// CustomerName is the point. Empty only when the underlying record
	// genuinely has no contact — then the client falls back to the detail.
	CustomerName string `json:"customer_name,omitempty"`
	// Detail is the specific fact, already assembled ("assinou dia 3, venceu
	// ontem"). Not a template for the client to fill: the server knows which
	// of six shapes this row is.
	Detail string `json:"detail,omitempty"`

	// MoneyAtStakeCents is what acting on it is worth. Zero means the action
	// carries no amount (an unowned lead, a draft) — NOT that it is worth
	// nothing, which is why ranking never divides by it.
	MoneyAtStakeCents int64  `json:"money_at_stake_cents"`
	Currency          string `json:"currency,omitempty"`

	// DueOn is the calendar deadline, for the actions that have one.
	DueOn *time.Time `json:"due_on,omitempty"`
	// SinceAt is when this started waiting. Drives the age ranking for the
	// moneyless actions and the "há 4 dias" the row shows.
	SinceAt *time.Time `json:"since_at,omitempty"`

	// DeepLink is where the work happens, built server-side so a new action
	// kind needs no client release to be actionable.
	DeepLink string `json:"deep_link"`
}

// Overdue reports a deadline already passed. Kept as a method rather than a
// stored flag so it can never be stale by a day.
func (a PendingAction) Overdue(now time.Time) bool {
	return a.DueOn != nil && a.DueOn.Before(now)
}

// pendingKindRank breaks ties between actions that are otherwise equal, so the
// list order is stable across two loads of the same data. A list that reshuffles
// looks made up.
func pendingKindRank(k PendingActionKind) int {
	switch k {
	case PendingSignatureViewed:
		return 0
	case PendingContractAlert:
		return 1
	case PendingConfirmSale:
		return 2
	case PendingApproveDraft:
		return 3
	// A customer already talking outranks every prospecting nag: the reply
	// window is the one that closes by itself.
	case PendingWaitingReply:
		return 4
	case PendingReorderDue:
		return 5
	case PendingUnownedLead:
		return 6
	// Coldest of all, deliberately last: a hospital that has never heard of
	// you is not waiting on an answer. It belongs on the screen because
	// nobody counts what they cannot see, but it must never outrank a
	// customer who is already talking.
	case PendingUntouchedTarget:
		return 7
	}
	return 8
}

// RankPendingActions orders the list by impact, in place.
//
// The ranking, in order of precedence:
//
//  1. An OVERDUE deadline first, regardless of amount. A renewal that fired
//     yesterday cannot be un-fired by a bigger opportunity elsewhere.
//  2. Then by money at stake, descending. This is the "impacto" the screen
//     promises: R$100k outranks R$900, and both outrank a row with no amount.
//  3. Then by KIND, using pendingKindRank. Among rows with no amount, what the
//     work IS matters more than how long it has sat: an offer ready to send and
//     a lead nobody owns are not two lengths of the same queue.
//  4. Then the OLDEST first, WITHIN a kind. Staleness is the only impact left
//     once two rows are the same kind of work.
//  5. Then target id: pure determinism, so the same data always renders the
//     same order.
//
// Steps 3 and 4 were the other way round until 2026-09-09, and the effect in
// production was the opposite of what this screen is for. Both "approve this
// offer" and "assign this lead" carry no amount, so age decided — and the two
// ages come from different columns (a draft's from `updated_at`, which a
// redraft moves forward; an unowned lead's from `created_at`, which never
// moves). Every night the engine rewrote the drafts, every rewrite made them
// younger, and the six rows at the top of "Precisa de você" were six
// administrative chores while 39 written offers waited below the fold.
//
// Deliberately NOT a single blended score. A weighted formula mixing reais and
// days would need a constant nobody can justify ("um dia de espera vale R$50?"),
// and the first time it ranked a R$40k renewal below a stale draft the whole
// list would lose its authority.
func RankPendingActions(actions []PendingAction, now time.Time) {
	sort.SliceStable(actions, func(i, j int) bool {
		a, b := actions[i], actions[j]

		if ao, bo := a.Overdue(now), b.Overdue(now); ao != bo {
			return ao
		}
		if a.MoneyAtStakeCents != b.MoneyAtStakeCents {
			return a.MoneyAtStakeCents > b.MoneyAtStakeCents
		}
		if ra, rb := pendingKindRank(a.Kind), pendingKindRank(b.Kind); ra != rb {
			return ra < rb
		}
		// Same kind, same amount: oldest first. A nil SinceAt sorts last —
		// an action with no age is not "infinitely old", it is simply
		// unmeasured.
		switch {
		case a.SinceAt != nil && b.SinceAt != nil && !a.SinceAt.Equal(*b.SinceAt):
			return a.SinceAt.Before(*b.SinceAt)
		case a.SinceAt != nil && b.SinceAt == nil:
			return true
		case a.SinceAt == nil && b.SinceAt != nil:
			return false
		}
		return a.TargetID < b.TargetID
	})
}

// CommandCenterTiles is the four-number strip.
//
// Every number is COUNTED over the whole set, never a page: the strip's job is
// to say whether somebody is on top of their work, and a tile that silently
// reports the first fifty rows is worse than no tile.
type CommandCenterTiles struct {
	// ActiveLeads excludes won/lost — the work actually in front of the user.
	ActiveLeads int64 `json:"active_leads"`
	// PendingApproval is the human-in-the-loop backlog.
	PendingApproval int64 `json:"pending_approval"`
	// Meetings booked in the period. Real now that the sales event log
	// records them (it was deliberately absent from the first command centre
	// while nothing measured it).
	Meetings int64 `json:"meetings"`
	// WonGrossCents is what closed in the period.
	WonGrossCents int64  `json:"won_gross_cents"`
	Currency      string `json:"currency,omitempty"`
	// Period names the window the last two tiles cover, because "ganhos"
	// with no period is a number nobody can check.
	PeriodLabel string `json:"period_label"`
	// UntouchedTargets is how many establishments in the account's territory
	// hold the equipment it sells and have never been approached.
	//
	// The size of what is being LOST, which no other tile shows: the other
	// four all count work already in hand, so an account with one lead and a
	// whole state in front of it looked identical to one that had worked the
	// state through.
	//
	// Deliberately NOT a money figure. CNES says how many machines an
	// establishment has, never what a deal is worth — inventing a ticket to
	// fill a currency column would be the product guessing.
	UntouchedTargets int64 `json:"untouched_targets"`
}

// CommandCenterStage is one bar of the mini-funnel.
type CommandCenterStage struct {
	Status LeadStatus `json:"status"`
	Label  string     `json:"label"`
	Count  int64      `json:"count"`
}

// MiniFunnelLadder is the compressed board for the home screen: the stages a
// human parks a deal in, in order.
//
// DERIVED from DefaultFunnelStages rather than written out again. The
// hand-written copy had five rungs against the board's eight, and the two it
// dropped — `prospected` and `enriched` — are exactly where prospecting parks
// its output. An account that had just imported two hundred clinics saw a home
// funnel of all zeros next to a tile reading "200 leads ativos". The old
// comment claimed the two lists agreed about stage NAMES; they did, while
// disagreeing about who gets counted.
//
// `enriching` and `pending_approval` have no rung of their own because they
// are not stages: the first lasts seconds, and the second already has its own
// tile at the top of the screen. They are still COUNTED, folded onto the
// column the board shows them in — see BuildMiniFunnel. `lost` stays out
// because this is a ladder of progress.
func MiniFunnelLadder() []struct {
	Status LeadStatus
	Label  string
} {
	out := make([]struct {
		Status LeadStatus
		Label  string
	}, 0, len(DefaultFunnelStages()))
	for _, stage := range DefaultFunnelStages() {
		if stage.Status == LeadStatusLost {
			continue
		}
		// "Esperando você" is a board column but not a home rung: it already
		// has its own TILE above this funnel, and the same number twice on one
		// screen reads as two different numbers.
		if stage.Status == LeadStatusPendingApproval {
			continue
		}
		out = append(out, struct {
			Status LeadStatus
			Label  string
		}{stage.Status, stage.Name})
	}
	return out
}

// BuildMiniFunnel projects the full per-status counts onto the compressed
// ladder.
//
// The projection runs through ProjectStatusCounts, the same fold the board
// uses, because two screens disagreeing about where a lead is means neither
// can be trusted. Reading the raw statuses left 41 of an account's 60 leads
// — every written offer it had — off the home funnel entirely.
func BuildMiniFunnel(raw map[LeadStatus]int64) []CommandCenterStage {
	byStatus := ProjectStatusCounts(raw)
	ladder := MiniFunnelLadder()
	out := make([]CommandCenterStage, 0, len(ladder))
	for _, rung := range ladder {
		out = append(out, CommandCenterStage{
			Status: rung.Status,
			Label:  rung.Label,
			Count:  byStatus[rung.Status],
		})
	}
	return out
}

// CommandCenter is the whole home payload — one round trip, like the dashboard
// BFF, because a first screen that fires six requests is a first screen that
// renders in pieces.
type CommandCenter struct {
	Tiles   CommandCenterTiles   `json:"tiles"`
	Funnel  []CommandCenterStage `json:"funnel"`
	Actions []PendingAction      `json:"actions"`

	// Suggestion is at most ONE copilot nudge. Singular by design: a screen
	// with three suggestions is a screen where all three get ignored.
	Suggestion *CommandCenterSuggestion `json:"suggestion,omitempty"`

	// Briefing is what the overnight run produced (K2), when there is one
	// worth telling somebody about. Nil is the normal case — most mornings
	// there was nothing to draft, and "0 abordagens prontas" every day is how
	// a good habit becomes a dismissed banner.
	Briefing *OvernightDraftRun `json:"briefing,omitempty"`

	// Empty says the account has no pipeline at all, so the client shows the
	// activation checklist instead of a wall of zeros. A brand-new account
	// reading "0 · 0 · 0 · R$ 0" learns that the product does nothing.
	Empty bool `json:"empty"`

	// Warnings names the sections that failed to load. A blown query
	// degrades to a named gap rather than an empty list that reads as
	// "nothing needs you".
	Warnings []string `json:"warnings,omitempty"`
}

// CommandCenterSuggestion is the single copilot chip.
type CommandCenterSuggestion struct {
	// Prompt is what gets sent to the copilot when tapped — the whole point
	// is that the suggestion is one tap from being done, not a hint the user
	// has to retype.
	Prompt string `json:"prompt"`
	Label  string `json:"label"`
}

// PendingActionDismissal is a snooze.
type PendingActionDismissal struct {
	ID        string
	AccountID string
	UserID    string
	Kind      PendingActionKind
	TargetID  string
	// SnoozedUntil always has a value. There is no permanent dismissal: the
	// deadline a human waved away is still a deadline.
	SnoozedUntil time.Time
	Note         string
}

// DefaultSnooze is how long a dismissal lasts when the caller does not say.
//
// Three days: long enough to clear the list for the week's work, short enough
// that a renewal notice waved away on Monday is back before the window closes.
const DefaultSnooze = 3 * 24 * time.Hour

// MaxSnooze bounds what a client may ask for. Thirty days, because beyond that
// a "snooze" is a deletion with better manners.
const MaxSnooze = 30 * 24 * time.Hour

// Validate normalizes a dismissal and refuses one that cannot be honoured.
func (d *PendingActionDismissal) Validate(now time.Time) error {
	d.Kind = PendingActionKind(strings.TrimSpace(string(d.Kind)))
	d.TargetID = strings.TrimSpace(d.TargetID)
	d.Note = strings.TrimSpace(d.Note)

	if !ValidPendingActionKind(string(d.Kind)) {
		return &AppError{Code: "VALIDATION_ERROR", Message: "tipo de ação inválido"}
	}
	if d.TargetID == "" {
		return &AppError{Code: "VALIDATION_ERROR", Message: "target_id é obrigatório"}
	}
	if d.SnoozedUntil.IsZero() {
		d.SnoozedUntil = now.Add(DefaultSnooze)
	}
	if !d.SnoozedUntil.After(now) {
		// A snooze already in the past is a no-op the client would read as
		// success and then wonder why the row is still there.
		return &AppError{Code: "VALIDATION_ERROR", Message: "snooze deve ser no futuro"}
	}
	if max := now.Add(MaxSnooze); d.SnoozedUntil.After(max) {
		d.SnoozedUntil = max
	}
	return nil
}

// DismissalKey is the identity a snooze is matched on.
type DismissalKey struct {
	Kind     PendingActionKind
	TargetID string
}

// FilterDismissed drops the actions this user has snoozed.
//
// Filtered in Go rather than in each of the six queries: the sources are
// independent statements, and threading a NOT EXISTS through every one of them
// would make adding a source a six-place change.
func FilterDismissed(actions []PendingAction, dismissed map[DismissalKey]struct{}) []PendingAction {
	if len(dismissed) == 0 {
		return actions
	}
	out := make([]PendingAction, 0, len(actions))
	for _, a := range actions {
		if _, snoozed := dismissed[DismissalKey{Kind: a.Kind, TargetID: a.TargetID}]; snoozed {
			continue
		}
		out = append(out, a)
	}
	return out
}

// CommandCenterRepository reads the actions and the counters.
//
// Its own port rather than more methods on the lead/analytics repositories:
// every query here spans a different table and none of them belong to a single
// entity's repository.
type CommandCenterRepository interface {
	// PendingApprovals lists drafts waiting on a human, oldest first.
	PendingApprovals(ctx context.Context, accountID, ownerUserID string, limit int) ([]PendingAction, error)
	// UnownedLeads lists live leads nobody owns. Managers only — the use
	// case decides whether to call it.
	UnownedLeads(ctx context.Context, accountID string, limit int) ([]PendingAction, error)
	// ViewedSignatures lists documents opened and not signed.
	ViewedSignatures(ctx context.Context, accountID string, limit int) ([]PendingAction, error)
	// ContractAlerts lists pending contract deadlines.
	ContractAlerts(ctx context.Context, accountID string, limit int) ([]PendingAction, error)
	// UnconfirmedSales lists AI-extracted sales no human has confirmed.
	UnconfirmedSales(ctx context.Context, accountID string, limit int) ([]PendingAction, error)
	// DueReorders lists customers past their predicted repurchase.
	DueReorders(ctx context.Context, accountID string, limit int) ([]PendingAction, error)
	// WaitingReplies lists open conversations whose customer is waiting on
	// a human, oldest silence first. ownerUserID scopes to one attendant's
	// assignments; empty sees the account (assigned or not).
	WaitingReplies(ctx context.Context, accountID, ownerUserID string, limit int) ([]PendingAction, error)

	// PeriodTotals counts meetings booked and gross closed in [from, to).
	PeriodTotals(ctx context.Context, accountID string, from, to time.Time) (meetings, wonGrossCents int64, currency string, err error)

	// LiveDismissals returns this user's un-expired snoozes.
	LiveDismissals(ctx context.Context, accountID, userID string, now time.Time) (map[DismissalKey]struct{}, error)
	// Dismiss upserts a snooze, moving the deadline when one already exists.
	Dismiss(ctx context.Context, d *PendingActionDismissal) error
}
