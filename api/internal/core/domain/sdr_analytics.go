package domain

import (
	"context"
	"strings"
	"time"
)

// SDR analytics (§6.12, I1).
//
// The screen that answers "is it working?" with numbers instead of a
// feeling. Five cuts of the same period — funnel, strategy, seller, channel,
// signal — and every number is clickable, because a metric nobody can open is
// a metric nobody trusts.
//
// Two rules run through the whole file:
//
//  1. A rate over zero is NIL, never 0%. Zero replies out of zero approaches
//     is unknown; rendering 0% condemns a strategy nobody used yet.
//  2. The period is bounded in the ACCOUNT's timezone. "Hoje" for a clinic in
//     Manaus is not the platform's today, and a report that silently shifts by
//     hours is a report someone screenshots and argues with.

// SDRPeriod is the window a report covers, resolved and named.
type SDRPeriod struct {
	// Key is the requested preset ("today", "7d", "30d", "mtd").
	Key string `json:"key"`
	// From is inclusive, To exclusive — the half-open convention every
	// aggregation in this codebase uses, so a row on the boundary is
	// counted exactly once.
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Timezone names the frame the bounds were computed in, so a number can
	// be checked rather than trusted.
	Timezone string `json:"timezone"`
}

// SDR period presets. Deliberately a closed set: an arbitrary from/to invites
// a comparison against a previous period that does not exist, and every
// preset here has an unambiguous "previous".
const (
	SDRPeriodToday = "today"
	SDRPeriod7d    = "7d"
	SDRPeriod30d   = "30d"
	SDRPeriodMTD   = "mtd"
)

// DefaultSDRPeriod is what the screen opens on. 7d, not 30d: a salesperson
// looks at this to decide what to do today, and a month-long average hides the
// week the reply rate collapsed.
const DefaultSDRPeriod = SDRPeriod7d

// ResolveSDRPeriod turns a preset into concrete bounds in the account's
// timezone.
//
// An unknown or empty key falls back to the default rather than erroring: this
// feeds a dashboard, and a typo in a saved deep link should render the default
// view, not a broken screen. The returned Key always reports what was actually
// used, so the UI can highlight the right chip.
func ResolveSDRPeriod(key, tz string, now time.Time) SDRPeriod {
	loc := LoadLocation(tz)
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	// To is "now", not end-of-day: a funnel that counts the rest of today as
	// elapsed reports a conversion rate against leads that have not had
	// their chance yet.
	to := local

	switch strings.ToLower(strings.TrimSpace(key)) {
	case SDRPeriodToday:
		return SDRPeriod{Key: SDRPeriodToday, From: midnight, To: to, Timezone: loc.String()}
	case SDRPeriod30d:
		return SDRPeriod{Key: SDRPeriod30d, From: midnight.AddDate(0, 0, -29), To: to, Timezone: loc.String()}
	case SDRPeriodMTD:
		first := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
		return SDRPeriod{Key: SDRPeriodMTD, From: first, To: to, Timezone: loc.String()}
	default:
		// 7d includes today, so the window is 6 days back — otherwise
		// "7 dias" silently covers eight.
		return SDRPeriod{Key: SDRPeriod7d, From: midnight.AddDate(0, 0, -6), To: to, Timezone: loc.String()}
	}
}

// Previous is the immediately preceding window of the same length, used for
// the "vs. período anterior" deltas.
func (p SDRPeriod) Previous() SDRPeriod {
	span := p.To.Sub(p.From)
	return SDRPeriod{Key: p.Key, From: p.From.Add(-span), To: p.From, Timezone: p.Timezone}
}

// SDRFunnelStep is one rung of the cohort funnel.
//
// The cohort is the leads that ENTERED in the period, followed to wherever
// they are now — not "things that happened this week to whatever lead". Only
// the cohort reading makes a conversion rate mean anything: mixing a reply
// earned this week with an approach sent last month produces a rate that can
// exceed 100% and explains nothing.
type SDRFunnelStep struct {
	// Metric is the machine name, and doubles as the drill-down key.
	Metric string `json:"metric"`
	Label  string `json:"label"`
	Count  int64  `json:"count"`
	// ConversionFromPrevious is count/previous as a 0..1 fraction. Nil on
	// the first step (nothing to convert from) and whenever the previous
	// step is zero.
	ConversionFromPrevious *float64 `json:"conversion_from_previous,omitempty"`
	// ConversionFromEntry is count/first-step, the number a salesperson
	// actually quotes ("de cada 100 leads, 4 fecham").
	ConversionFromEntry *float64 `json:"conversion_from_entry,omitempty"`
	// Incomplete flags a step that counts MORE leads than the step above
	// it. That is not a funnel — it means the rung above was never
	// recorded for some leads (a sale entered by hand, an approach sent
	// before instrumentation). Surfaced rather than clamped: a silently
	// capped number is a lie, and this one is a genuine data-quality fact
	// the operator can act on.
	Incomplete bool `json:"incomplete,omitempty"`
}

// Funnel step metrics. These strings are the contract between the funnel, the
// drill-down endpoint and the deep links in the UI — renaming one breaks a
// saved link, so treat them as API.
const (
	SDRMetricEntered    = "entered"
	SDRMetricResearched = "researched"
	SDRMetricApproached = "approached"
	SDRMetricReplied    = "replied"
	SDRMetricQualified  = "qualified"
	SDRMetricWon        = "won"
)

// SDRFunnelLadder is the order the rungs are reported in, with the label each
// carries. Order is the whole meaning of a funnel, so it lives in one place.
//
// "Pesquisado" is deliberately NOT a rung. Enrichment is optional — plenty of
// good sales skip it and go straight to the approach — so putting it in the
// ladder would make every such account's "abordados" step count more leads
// than the step above it, and the Incomplete flag would cry data-quality wolf
// at a perfectly normal workflow. It is reported in the totals instead.
func SDRFunnelLadder() []struct {
	Metric string
	Label  string
} {
	return []struct {
		Metric string
		Label  string
	}{
		{SDRMetricEntered, "Entraram"},
		{SDRMetricApproached, "Abordados"},
		{SDRMetricReplied, "Responderam"},
		{SDRMetricQualified, "Qualificados"},
		{SDRMetricWon, "Ganhos"},
	}
}

// SDRFunnelCounts is the raw per-rung tally the repository returns, keyed by
// metric. Building the ladder (rates, anomaly flags) is domain work, so it
// happens once here rather than in SQL.
type SDRFunnelCounts map[string]int64

// BuildFunnel turns raw counts into the reportable ladder.
func BuildFunnel(counts SDRFunnelCounts) []SDRFunnelStep {
	ladder := SDRFunnelLadder()
	steps := make([]SDRFunnelStep, 0, len(ladder))

	var entry int64
	for i, rung := range ladder {
		count := counts[rung.Metric]
		step := SDRFunnelStep{Metric: rung.Metric, Label: rung.Label, Count: count}
		if i == 0 {
			entry = count
		} else {
			prev := steps[i-1].Count
			if prev > 0 {
				rate := float64(count) / float64(prev)
				step.ConversionFromPrevious = &rate
			}
			step.Incomplete = count > prev
			if entry > 0 {
				overall := float64(count) / float64(entry)
				step.ConversionFromEntry = &overall
			}
		}
		steps = append(steps, step)
	}
	return steps
}

// SDRRate is the shared "counted out of counted" shape used by every
// breakdown row, so reply rate and win rate can never be computed two
// different ways.
type SDRRate struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

// Value is the fraction, or nil when there is no denominator.
func (r SDRRate) Value() *float64 {
	if r.Denominator == 0 {
		return nil
	}
	v := float64(r.Numerator) / float64(r.Denominator)
	return &v
}

// SDRStrategyRow is one strategy version's results inside the period.
//
// Version is part of the identity, not decoration: an approach sent under
// version 2 must not be credited to version 3's rewritten arguments.
type SDRStrategyRow struct {
	StrategyID string `json:"strategy_id"`
	Name       string `json:"name"`
	Version    int    `json:"version"`
	Status     string `json:"status"`

	Approaches int64 `json:"approaches"`
	Replied    int64 `json:"replied"`
	Won        int64 `json:"won"`

	ReplyRate *float64 `json:"reply_rate,omitempty"`
	WinRate   *float64 `json:"win_rate,omitempty"`

	// WonGrossCents is what the wins were worth. A strategy with a lower
	// win rate and bigger deals can be the better one, and a screen that
	// only shows rates hides that.
	WonGrossCents int64  `json:"won_gross_cents"`
	Currency      string `json:"currency,omitempty"`
}

// SDRSellerRow is one salesperson's period.
type SDRSellerRow struct {
	// SellerUserID is empty for work nobody owns. Reported as its own row
	// rather than dropped — unowned leads piling up IS the finding.
	SellerUserID string `json:"seller_user_id"`
	Name         string `json:"name"`

	LeadsOwned int64 `json:"leads_owned"`
	Approaches int64 `json:"approaches"`
	Replied    int64 `json:"replied"`
	Won        int64 `json:"won"`

	ReplyRate *float64 `json:"reply_rate,omitempty"`

	Sales           int64  `json:"sales"`
	GrossCents      int64  `json:"gross_cents"`
	NetCents        int64  `json:"net_cents"`
	Currency        string `json:"currency,omitempty"`
	CommissionCents int64  `json:"commission_cents"`
}

// SDRChannelRow is one channel's period, with what it cost.
type SDRChannelRow struct {
	// Channel is the channel TYPE (whatsapp, telegram, sms…), not one
	// channel row: a customer with three WhatsApp numbers thinks in
	// "WhatsApp".
	Channel    string   `json:"channel"`
	Approaches int64    `json:"approaches"`
	Replied    int64    `json:"replied"`
	ReplyRate  *float64 `json:"reply_rate,omitempty"`

	// AICostCents is the AI spend attributable to this channel, converted
	// from the immutable per-message stamp at the platform rate. It is NOT
	// the whole cost of the channel — see SDRCostDisclosure.
	AICostCents int64 `json:"ai_cost_cents"`
}

// SDRCostDisclosure states, in the payload, exactly what the cost column
// covers.
//
// This exists because the alternative is worse than showing nothing: the
// carrier's and Meta's per-message fees are billed on the CUSTOMER's own
// provider account and we never see the invoice. Presenting our AI cost as
// "custo do canal" would understate their real spend, and estimating their
// carrier fee from a price table we do not control would invent a number that
// shows up next to a real invoice and loses the argument.
type SDRCostDisclosure struct {
	// AIOnly is always true today. It is a field rather than a doc comment
	// so the UI renders the caveat from the payload and can stop when
	// carrier metering exists.
	AIOnly bool `json:"ai_only"`
	// RateCents is the USD→BRL rate applied to the AI stamp.
	RateCents int `json:"rate_cents"`
	// Currency the cost figures are in.
	Currency string `json:"currency"`
}

// SDRSignalRow is one signal SOURCE's period: how many public buying signals
// arrived, how many somebody acted on, how many closed.
type SDRSignalRow struct {
	Source   string   `json:"source"`
	Signals  int64    `json:"signals"`
	Worked   int64    `json:"worked"`
	Won      int64    `json:"won"`
	WorkRate *float64 `json:"work_rate,omitempty"`
	WinRate  *float64 `json:"win_rate,omitempty"`
	// ValueCents sums the disclosed values of the signals. Tenders that did
	// not disclose a value contribute zero, so this is a floor, never a
	// total — DisclosedCount says how much of the set it covers.
	ValueCents     int64 `json:"value_cents"`
	DisclosedCount int64 `json:"disclosed_count"`
}

// SDRTotals is the headline strip: the four numbers that go at the top.
type SDRTotals struct {
	LeadsEntered int64 `json:"leads_entered"`
	// Researched is enrichment runs in the window, reported here rather than
	// as a funnel rung because researching is optional — see
	// SDRFunnelLadder.
	Researched int64    `json:"researched"`
	Approaches int64    `json:"approaches"`
	Replies    int64    `json:"replies"`
	Meetings   int64    `json:"meetings"`
	Sales      int64    `json:"sales"`
	GrossCents int64    `json:"gross_cents"`
	NetCents   int64    `json:"net_cents"`
	Currency   string   `json:"currency,omitempty"`
	ReplyRate  *float64 `json:"reply_rate,omitempty"`
	// AvgReplySeconds is nil when nobody replied. An average over zero
	// replies rendered as "0s" would be the most flattering possible lie.
	AvgReplySeconds *int `json:"avg_reply_seconds,omitempty"`
}

// SDRDelta is one headline number compared with the previous window.
//
// Direction is carried explicitly because "up" is not universally good: reply
// LATENCY going up is worse, and letting the UI guess per metric is how a
// green arrow ends up on a regression.
type SDRDelta struct {
	Metric   string `json:"metric"`
	Current  int64  `json:"current"`
	Previous int64  `json:"previous"`
	// Better is nil when there is nothing to compare (no previous period
	// data) — a first-week account should see "—", not a fake +100%.
	Better *bool `json:"better,omitempty"`
}

// SDRAnalytics is the whole report.
type SDRAnalytics struct {
	Period     SDRPeriod         `json:"period"`
	Totals     SDRTotals         `json:"totals"`
	Deltas     []SDRDelta        `json:"deltas,omitempty"`
	Funnel     []SDRFunnelStep   `json:"funnel"`
	Strategies []SDRStrategyRow  `json:"strategies"`
	Sellers    []SDRSellerRow    `json:"sellers"`
	Channels   []SDRChannelRow   `json:"channels"`
	Signals    []SDRSignalRow    `json:"signals"`
	Cost       SDRCostDisclosure `json:"cost"`
	// Warnings names the sections that failed to load. A blown aggregate
	// degrades to a named gap instead of taking the screen down or, worse,
	// rendering as an empty section that reads like "you did nothing".
	Warnings []string `json:"warnings,omitempty"`
}

// SDRLeadRow is one row of a drill-down list: the leads behind a number.
type SDRLeadRow struct {
	LeadID      string    `json:"lead_id"`
	ContactID   string    `json:"contact_id"`
	ContactName string    `json:"contact_name"`
	Status      string    `json:"status"`
	Source      string    `json:"source,omitempty"`
	OwnerName   string    `json:"owner_name,omitempty"`
	StrategyID  string    `json:"strategy_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	// GrossCents is the sale this lead produced, when it produced one.
	GrossCents int64  `json:"gross_cents,omitempty"`
	Currency   string `json:"currency,omitempty"`
}

// SDRDrillFilter is a drill-down request: which rung, narrowed by which cut.
//
// One filter type for every drill-down in the report, so "open the number"
// works identically whether the click came from the funnel, a strategy row or
// a channel row.
type SDRDrillFilter struct {
	AccountID string
	Period    SDRPeriod
	// Metric is a funnel rung (SDRMetric*). Required: a drill-down with no
	// rung is just the lead list, which already exists.
	Metric string
	// The optional cuts. Empty means "no narrowing"; SellerUserID has a
	// sentinel for the unowned bucket because "" already means "any".
	StrategyID   string
	SellerUserID string
	Channel      string
	SignalSource string

	Limit  int32
	Offset int32
}

// SDRUnownedSeller is the sentinel that narrows a drill-down to leads nobody
// owns. Needed because the empty string already means "every seller", and the
// unowned bucket is the one an operator most wants to open.
const SDRUnownedSeller = "none"

// ValidSDRMetric guards the value before it reaches SQL.
//
// Deliberately does NOT trim: it validates the exact string the caller is
// about to use. A guard that accepts "won " while the caller queries with the
// space is a guard that passed on a value nothing matches.
func ValidSDRMetric(v string) bool {
	switch v {
	case SDRMetricEntered, SDRMetricResearched, SDRMetricApproached,
		SDRMetricReplied, SDRMetricQualified, SDRMetricWon:
		return true
	}
	return false
}

// SDRAnalyticsRepository aggregates the report.
//
// Deliberately its own port rather than more methods on AnalyticsRepository:
// that interface is implemented by a dozen test mocks, and every cut here is
// SDR-specific.
type SDRAnalyticsRepository interface {
	// Funnel counts the cohort of leads that entered in [from, to) at each
	// rung, following them to wherever they are NOW.
	Funnel(ctx context.Context, accountID string, from, to time.Time) (SDRFunnelCounts, error)
	// Strategies tallies per strategy VERSION.
	Strategies(ctx context.Context, accountID string, from, to time.Time) ([]SDRStrategyRow, error)
	Sellers(ctx context.Context, accountID string, from, to time.Time) ([]SDRSellerRow, error)
	Channels(ctx context.Context, accountID string, from, to time.Time) ([]SDRChannelRow, error)
	Signals(ctx context.Context, accountID string, from, to time.Time) ([]SDRSignalRow, error)
	// SalesTotals sums the sales closed in the window.
	SalesTotals(ctx context.Context, accountID string, from, to time.Time) (sales, grossCents, netCents int64, currency string, err error)
	// AICostUSDByChannel is the AI spend in the window keyed by channel
	// TYPE, from the immutable per-message stamp. One method rather than a
	// total plus a breakdown, so the two can never disagree — the total is
	// the sum of the map.
	AICostUSDByChannel(ctx context.Context, accountID string, from, to time.Time) (map[string]float64, error)
	// DrillLeads lists the leads behind one number.
	DrillLeads(ctx context.Context, f SDRDrillFilter) ([]SDRLeadRow, error)
}
