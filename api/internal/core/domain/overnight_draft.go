package domain

import (
	"context"
	"strings"
	"time"
)

// Overnight drafting (§6.17, K2).
//
// Filling the approval queue is not real-time work. Nobody watches it happen;
// what a salesperson wants is to open the app at eight and find the approaches
// already written. Two things follow from that:
//
//   - It can run when nothing else competes for the provider, and — where the
//     provider offers a batch endpoint — at roughly half the price.
//   - It becomes the reason to open the app. "Seu SDR trabalhou de madrugada: 40
//     abordagens prontas" is a better morning than an empty screen.
//
// The second is the point. A queue that fills itself silently is a cost saving;
// a queue that tells you it filled itself is a habit.

// OvernightDraftSettings is one account's opt-in.
type OvernightDraftSettings struct {
	AccountID string `json:"-"`
	// Enabled is opt-IN. This spends the account's AI allowance without anyone
	// asking for it that night, so it has to be a decision somebody made.
	Enabled bool `json:"enabled"`
	// FunnelID narrows the run. Empty means every active funnel.
	FunnelID string `json:"funnel_id,omitempty"`
	// NightlyCap is how many to draft per night, on top of (never instead of)
	// the plan's monthly allowance.
	NightlyCap int `json:"nightly_cap"`
	// RunHour is the local hour the run starts, in the ACCOUNT's timezone.
	RunHour int `json:"run_hour"`
	// LastRunOn is the account-local date of the last run, so a worker tick
	// cannot run the same night twice.
	LastRunOn *time.Time `json:"last_run_on,omitempty"`
	EnabledBy string     `json:"enabled_by,omitempty"`
}

// Defaults for a fresh opt-in.
const (
	// DefaultNightlyDraftCap is deliberately modest. The queue is reviewed by a
	// human in the morning, and forty approaches is already more than most
	// people get through before lunch — a bigger default would spend the
	// allowance filling a queue nobody empties.
	DefaultNightlyDraftCap = 40
	// MaxNightlyDraftCap bounds what a client may ask for.
	MaxNightlyDraftCap = 500
	// DefaultOvernightRunHour is 3am local: after the working day everywhere in
	// Brazil, well before anyone opens the app.
	DefaultOvernightRunHour = 3
)

// Validate normalizes the settings and refuses what cannot be honoured.
func (s *OvernightDraftSettings) Validate() error {
	if s.NightlyCap <= 0 {
		s.NightlyCap = DefaultNightlyDraftCap
	}
	if s.NightlyCap > MaxNightlyDraftCap {
		s.NightlyCap = MaxNightlyDraftCap
	}
	if s.RunHour < 0 || s.RunHour > 23 {
		// An out-of-range hour is a client bug, and refusing it outright would
		// turn a typo into a night with no drafts. The default is a better
		// answer than an error nobody reads.
		s.RunHour = DefaultOvernightRunHour
	}
	return nil
}

// DueTonight reports whether this account's run should start now.
//
// Both halves are computed in the ACCOUNT's timezone: the hour it asked for, and
// the date the run belongs to. A UTC comparison would fire at four in the
// afternoon for an account in Manaus, and would run the same "night" twice at the
// date boundary.
func (s *OvernightDraftSettings) DueTonight(now time.Time, timezone string) bool {
	if s == nil || !s.Enabled {
		return false
	}
	local := now.In(LoadLocation(timezone))
	if local.Hour() != s.RunHour {
		return false
	}
	if s.LastRunOn == nil {
		return true
	}
	// Compare calendar dates, not timestamps: the stored value is a date, and a
	// timestamp comparison would let a second tick inside the same hour start a
	// second run.
	last := s.LastRunOn.In(LoadLocation(timezone))
	return last.Year() != local.Year() || last.YearDay() != local.YearDay()
}

// BatchDraftOutcome is what one drafting pass produced, across every funnel
// it touched.
//
// It lives in domain rather than in usecases because the overnight worker —
// a service — consumes it, and services must not import usecases. The
// per-request HTTP shape stays in usecases; this is the part two layers
// agree on.
type BatchDraftOutcome struct {
	Drafted         int
	SkippedOptedOut int
	SkippedBudget   int
	SkippedError    int
	// StoppedReason is set when the pass gave up because every draft was
	// failing the same way. Empty means it ran to the end.
	StoppedReason string
}

// OvernightDraftRun is what one night produced.
type OvernightDraftRun struct {
	ID        string    `json:"id"`
	AccountID string    `json:"-"`
	RanOn     time.Time `json:"ran_on"`

	Drafted         int `json:"drafted"`
	SkippedOptedOut int `json:"skipped_opted_out"`
	SkippedBudget   int `json:"skipped_budget"`
	SkippedError    int `json:"skipped_error"`

	// UsedBatch says whether the provider's batch endpoint did the work. The
	// whole cost argument for running overnight rests on it, and an account
	// whose provider has none gets the convenience without the discount — which
	// the briefing must not imply otherwise.
	UsedBatch bool `json:"used_batch"`

	// StoppedReason is why the night gave up, when it did. Empty covers both
	// a normal run and a legitimately quiet one — an account with nobody
	// left to write to has not failed.
	StoppedReason string `json:"stopped_reason,omitempty"`

	SeenAt     *time.Time `json:"seen_at,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Worth reports whether a run is worth telling somebody about.
//
// A night that drafted nothing is not a briefing, it is noise — and the most
// common reason for it is the honest one: there was nobody left in the funnel to
// write to. Surfacing "0 abordagens prontas" every morning is how a good habit
// becomes a dismissed banner.
//
// A night that BROKE is the exception, and it is the case this predicate got
// wrong for nine nights in production: with only `Drafted > 0` to go on, a
// failed run was worth nothing, so the morning card kept showing the last
// successful night as if it were today's. Silence read as success.
func (r *OvernightDraftRun) Worth() bool {
	return r != nil && (r.Drafted > 0 || r.Blocked())
}

// Blocked reports a night that stopped because something about the ACCOUNT
// made every draft impossible — no catalogue, no provider key. The fix is a
// screen the customer can open, so the card says so instead of counting to
// seventy in silence.
func (r *OvernightDraftRun) Blocked() bool {
	return r != nil && strings.TrimSpace(r.StoppedReason) != ""
}

// BudgetHit reports a run the plan's allowance cut short. Shown alongside the
// count, because "40 prontas" when the customer asked for 100 needs the missing
// 60 explained rather than left as a number that does not add up.
func (r *OvernightDraftRun) BudgetHit() bool {
	return r != nil && r.SkippedBudget > 0
}

// OvernightDraftRepository persists the opt-in and the runs.
type OvernightDraftRepository interface {
	// Settings reads one account's opt-in, or (nil, nil).
	Settings(ctx context.Context, accountID string) (*OvernightDraftSettings, error)
	// SaveSettings upserts the opt-in.
	SaveSettings(ctx context.Context, s *OvernightDraftSettings) error
	// ListEnabled returns every opted-in account with the timezone the schedule
	// is evaluated in, so the worker resolves "is it 3am there?" without a
	// lookup per account.
	ListEnabled(ctx context.Context) ([]OvernightDraftTarget, error)
	// ClaimNight marks the account as run for this local date and returns false
	// when another worker already claimed it. The claim is what makes two pods
	// safe.
	ClaimNight(ctx context.Context, accountID string, localDate time.Time) (bool, error)
	// SaveRun records what the night produced.
	SaveRun(ctx context.Context, run *OvernightDraftRun) error
	// LatestUnseen is the morning briefing: the newest run nobody has been told
	// about yet.
	LatestUnseen(ctx context.Context, accountID string) (*OvernightDraftRun, error)
	// MarkSeen stops the briefing reappearing all day.
	MarkSeen(ctx context.Context, accountID, runID string, at time.Time) error
}

// OvernightDraftTarget is one account the worker may have to run tonight.
type OvernightDraftTarget struct {
	Settings OvernightDraftSettings
	// Timezone is the account's, because the schedule is stated in local hours.
	Timezone string
}
