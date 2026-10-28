package domain

import (
	"context"
	"time"
)

// SellerSettings holds one seller's commission rates and activity state
// (US-007, vendedores v3). Rates are percents of net profit. An absent
// row means the defaults: own 50, inbound 20, active. The activity
// clause: an inactive AM keeps a 15% residual until residual_until
// (deactivation + 12 months), then earns nothing.
type SellerSettings struct {
	UserID                string  `json:"user_id"`
	CommissionRateOwn     float64 `json:"commission_rate_own"`
	CommissionRateInbound float64 `json:"commission_rate_inbound"`
	// IsSeller is the EXPLICIT owner grant (review-fixes US-014): only
	// flagged users reach the /seller panel, mint ref codes, have their
	// codes resolve at signup or get stamped by convert-lead. DEFAULT
	// FALSE — a row created as a side-effect (pix key, activity clause)
	// never makes someone payable by itself.
	IsSeller      bool       `json:"is_seller"`
	IsActiveAM    bool       `json:"is_active_am"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	ResidualUntil *time.Time `json:"residual_until,omitempty"`
	PixKey        *string    `json:"pix_key,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Commission ledger reasons. standard/residual/sla_reduced/
// bonus_upgrade are the mutually-exclusive profit-share reasons the
// cycle recompute manages (bonus_upgrade REPLACES standard in a cycle
// with a plan upgrade — US-008); bonus_nps/bonus_retention are additive
// fixed-amount rows living alongside. adjustment is the delta a
// recompute records when the (account, cycle)'s profit share was
// already PAID — settled money is immutable, so the difference (which
// can be negative) becomes its own pending row instead of a second
// full profit share.
const (
	CommissionReasonStandard       = "standard"
	CommissionReasonResidual       = "residual"
	CommissionReasonSLAReduced     = "sla_reduced"
	CommissionReasonBonusUpgrade   = "bonus_upgrade"
	CommissionReasonBonusNPS       = "bonus_nps"
	CommissionReasonBonusRetention = "bonus_retention"
	CommissionReasonAdjustment     = "adjustment"
)

const (
	CommissionStatusPending = "pending"
	CommissionStatusPaid    = "paid"
)

// CommissionEntry is one commission_ledger row: rate applied to one
// account's net profit for one cycle (or a bonus). amount_cents is
// integer BRL cents, never negative — a negative base records the base
// and pays 0.
type CommissionEntry struct {
	ID                 string     `json:"id"`
	SellerUserID       string     `json:"seller_user_id"`
	AccountID          string     `json:"account_id"`
	Cycle              string     `json:"cycle"`
	BaseNetProfitCents int64      `json:"base_net_profit_cents"`
	RateApplied        float64    `json:"rate_applied"`
	AmountCents        int64      `json:"amount_cents"`
	Reason             string     `json:"reason"`
	Status             string     `json:"status"`
	PaidAt             *time.Time `json:"paid_at,omitempty"`
	Note               *string    `json:"note,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	// Joined on listing reads for display; empty elsewhere.
	SellerEmail string `json:"seller_email,omitempty"`
	AccountName string `json:"account_name,omitempty"`
}

// Owner-editable bonus amounts (US-008), integer BRL cents in
// platform_settings. 0 disables the bonus.
const (
	// SettingBonusNPSCents pays when a carteira account answers NPS
	// 9-10 inside the cycle (max 1 per account per calendar quarter).
	SettingBonusNPSCents = "bonus_nps_cents"
	// SettingBonusRetentionCents pays once when a seller-linked account
	// completes 12 months with an active subscription.
	SettingBonusRetentionCents = "bonus_retention_cents"
)

// SLA escalation actions (US-009). The ladder is mechanical: breach
// cycle 1 -> warning (ledger untouched); 2nd consecutive -> reduced
// (profit share at 20%, reason 'sla_reduced'); 3rd+ -> flag_reallocation
// (owner decides manually, nothing automatic). A breach-free cycle
// resets the counter.
const (
	SLAActionNone             = "none"
	SLAActionWarning          = "warning"
	SLAActionReduced          = "reduced"
	SLAActionFlagReallocation = "flag_reallocation"
)

// Owner-editable SLA parameters in platform_settings (US-009 seeds in
// 000129). The metric: % of the seller's assigned platform-account
// conversations in the cycle whose first HUMAN response took more than
// sla_first_response_minutes of business time (window
// sla_business_start_hour..sla_business_end_hour in sla_timezone,
// weekends excluded); breach when that % exceeds sla_breach_pct.
const (
	SettingSLAFirstResponseMinutes = "sla_first_response_minutes"
	SettingSLABreachPct            = "sla_breach_pct"
	SettingSLABusinessStartHour    = "sla_business_start_hour"
	SettingSLABusinessEndHour      = "sla_business_end_hour"
	SettingSLATimezone             = "sla_timezone"
)

// SellerSLAState is one seller_sla_state row: the measured metric plus
// the escalation step for one cycle.
type SellerSLAState struct {
	ID                    string    `json:"id"`
	SellerUserID          string    `json:"seller_user_id"`
	Cycle                 string    `json:"cycle"`
	TotalConversations    int       `json:"total_conversations"`
	BreachedConversations int       `json:"breached_conversations"`
	Breach                bool      `json:"breach"`
	ConsecutiveBreaches   int       `json:"consecutive_breaches"`
	Action                string    `json:"action"`
	ComputedAt            time.Time `json:"computed_at"`
	// Joined on listing reads for display; empty elsewhere.
	SellerEmail string `json:"seller_email,omitempty"`
}

// SLAConversation is one assigned platform-account conversation the
// SLA metric measures: when it arrived and when a human first replied
// (nil = still unanswered, which counts as a breach once the business
// window blows past the threshold).
type SLAConversation struct {
	ConversationID       string
	AssignedTo           string
	ContactName          string
	CreatedAt            time.Time
	FirstHumanResponseAt *time.Time
	// CustomerAccountID is set on internal-channel support threads
	// (canal-interno US-009): all of one customer account's per-user
	// threads collapse into a single SLA unit so thread-per-user never
	// multiplies the account's weight in the seller ladder. Empty for
	// regular conversations, which stay one unit each.
	CustomerAccountID string
}

// SellerDetractorFlag surfaces detractor NPS (score 0-6) on a seller's
// carteira inside the cycle. Informational only — it NEVER changes a
// commission rate (PRD: human review, no automatic cut).
type SellerDetractorFlag struct {
	SellerUserID       string `json:"seller_user_id"`
	SellerEmail        string `json:"seller_email"`
	DetractorResponses int    `json:"detractor_responses"`
}

// CommissionBase is one (seller, account) pair the cycle recompute
// iterates: the account's computed net profit joined with its current
// seller link (US-002), lead source track and the objective-event
// signals the bonus rules (US-008) read.
type CommissionBase struct {
	SellerUserID   string
	AccountID      string
	LeadSource     *string
	NetProfitCents int64
	// HasPlanUpgrade: a 'plan_upgraded' billing event landed inside the
	// cycle — the profit share uses the own-lead rate that month.
	HasPlanUpgrade bool
	// HasPromoterNPS: the account answered NPS with score 9-10 inside
	// the cycle.
	HasPromoterNPS bool
	// AccountCreatedAt + SubscriptionStatus feed the 12-month retention
	// bonus check.
	AccountCreatedAt   time.Time
	SubscriptionStatus string
}

// SellerCarteiraAccount is one carteira row of the seller panel
// (US-010): the linked account with its plan/MRR, attribution track and
// the last CLOSED cycle's computed numbers. The Last* pointers are nil
// when that cycle was never computed for the account (no
// account_monthly_profit / ledger rows yet).
type SellerCarteiraAccount struct {
	AccountID          string     `json:"account_id"`
	Name               string     `json:"name"`
	PlanName           string     `json:"plan_name"`
	MRRCents           int64      `json:"mrr_cents"`
	Currency           string     `json:"currency"`
	LeadSource         *string    `json:"lead_source,omitempty"`
	SubscriptionStatus string     `json:"subscription_status"`
	SellerLinkedAt     *time.Time `json:"seller_linked_at,omitempty"`
	// Platform conversation that converted this inbound lead (US-004
	// stamp) — the co-pilot deep link (US-015). Nil for ?ref= signups.
	ConvertedConversationID *string `json:"converted_conversation_id,omitempty"`
	// Last closed cycle: computed net profit, the seller's TOTAL
	// commission on the account (profit share + additive bonuses) and
	// the profit-share rate/reason actually applied.
	LastCycleNetProfitCents  *int64   `json:"last_cycle_net_profit_cents,omitempty"`
	LastCycleCommissionCents *int64   `json:"last_cycle_commission_cents,omitempty"`
	LastCycleRateApplied     *float64 `json:"last_cycle_rate_applied,omitempty"`
	LastCycleReason          *string  `json:"last_cycle_reason,omitempty"`
	// Engine-only columns for the co-pilot suggestion engine (review-fixes
	// US-022): fetched in the same carteira query so the engine never does
	// a per-account GetByID / usage lookup (N+1). Never serialized.
	CreatedAt                        time.Time `json:"-"`
	PlanID                           *string   `json:"-"`
	MaxConversationsPerMonthOverride *int      `json:"-"`
	MaxAITokensPerMonthOverride      *int      `json:"-"`
	// Latest-cycle usage counters (account_usage_summary view; nil when
	// the account has no usage row yet).
	UsageConversations *int32 `json:"-"`
	UsageAITokens      *int64 `json:"-"`
	UsageAddonTokens   *int64 `json:"-"`
}

// EngineAccount reconstructs the minimal domain.Account the co-pilot
// engine needs for the EffectiveMax* limit math — the carteira query
// carries these columns precisely so no per-account fetch is required.
func (c *SellerCarteiraAccount) EngineAccount() *Account {
	return &Account{
		ID:                               c.AccountID,
		PlanID:                           c.PlanID,
		CreatedAt:                        c.CreatedAt,
		MaxConversationsPerMonthOverride: c.MaxConversationsPerMonthOverride,
		MaxAITokensPerMonthOverride:      c.MaxAITokensPerMonthOverride,
	}
}

// CommissionLedgerCurrency: every commission_ledger amount and monthly
// profit figure is BRL cents by construction (USD revenue converts at
// the manual rate before it enters the ledger; payouts are PIX). The
// seller panel formats profit/commission from this constant-carrying
// payload field instead of hardcoding — if the ledger ever goes
// multi-currency this is the single seam.
const CommissionLedgerCurrency = "brl"

// SellerStatementLine is one commission_ledger row of the seller's own
// statement (US-011) joined with the cycle's full profit breakdown so
// the seller can audit every number: revenue − Stripe fee − AI cost −
// tax − infra = net profit × rate = commission. The breakdown pointers
// are nil on additive bonus rows' missing profit (never computed) —
// the profit-share row of a computed cycle always carries them.
type SellerStatementLine struct {
	AccountID   string     `json:"account_id"`
	AccountName string     `json:"account_name"`
	Reason      string     `json:"reason"`
	RateApplied float64    `json:"rate_applied"`
	AmountCents int64      `json:"amount_cents"`
	Status      string     `json:"status"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	// Full profit breakdown of the account's cycle (account_monthly_profit).
	RevenueCents   *int64 `json:"revenue_cents,omitempty"`
	StripeFeeCents *int64 `json:"stripe_fee_cents,omitempty"`
	TokenCostCents *int64 `json:"token_cost_cents,omitempty"`
	TaxCents       *int64 `json:"tax_cents,omitempty"`
	InfraCents     *int64 `json:"infra_cents,omitempty"`
	NetProfitCents *int64 `json:"net_profit_cents,omitempty"`
}

// SellerPayoutSummary is one seller's line on the owner payout screen
// (US-013): the cycle's ledger grouped per seller with the PIX key the
// owner transfers to. Adjustment* carry PENDING entries left in EARLIER
// cycles (a late recompute after that cycle was settled) — they surface
// as "ajuste" and settle together with the current cycle.
type SellerPayoutSummary struct {
	SellerUserID      string  `json:"seller_user_id"`
	SellerName        string  `json:"seller_name"`
	SellerEmail       string  `json:"seller_email"`
	PixKey            *string `json:"pix_key,omitempty"`
	PendingCents      int64   `json:"pending_cents"`
	PendingEntries    int     `json:"pending_entries"`
	PaidCents         int64   `json:"paid_cents"`
	AdjustmentCents   int64   `json:"adjustment_cents"`
	AdjustmentEntries int     `json:"adjustment_entries"`
	TotalDueCents     int64   `json:"total_due_cents"`
}

// CommissionRepository owns seller_settings + commission_ledger.
type CommissionRepository interface {
	// InCycleTx runs fn inside one DB transaction holding the per-cycle
	// advisory locks (deduped, acquired in ascending order so multi-lock
	// holders can never deadlock). Serializes recomputes and payouts
	// touching the same cycle (US-006): the check-then-insert sequences
	// (SumPaidProfitShare/HasBonusEntry -> UpsertEntry) and the payout's
	// settle all observe a stable ledger. fn's repo is transaction-bound;
	// an error rolls everything back.
	InCycleTx(ctx context.Context, cycles []string, fn func(CommissionRepository) error) error
	// GetSellerSettings returns nil, nil when the seller has no row
	// (callers apply the 50/20/active defaults).
	GetSellerSettings(ctx context.Context, userID string) (*SellerSettings, error)
	// UpsertSellerActivity writes the activity-clause state, keeping
	// existing rates/pix_key when the row already exists.
	UpsertSellerActivity(ctx context.Context, userID string, isActive bool, deactivatedAt, residualUntil *time.Time) error
	// SetSellerFlag grants/revokes the explicit seller flag (US-014),
	// keeping every other column when the row already exists.
	SetSellerFlag(ctx context.Context, userID string, isSeller bool) error
	// ListCommissionBases joins the cycle's account_monthly_profit rows
	// with their accounts' current seller link plus the bonus signals
	// (plan upgrade + promoter NPS) inside [from, to).
	ListCommissionBases(ctx context.Context, cycle string, from, to time.Time) ([]CommissionBase, error)
	// UpsertEntry writes on UNIQUE(seller, account, cycle, reason) —
	// idempotent under recompute — and NEVER modifies a paid row.
	UpsertEntry(ctx context.Context, e *CommissionEntry) error
	// DeleteStaleProfitShare removes the account+cycle's PENDING
	// profit-share entries (standard/residual/sla_reduced/
	// bonus_upgrade/adjustment — the mutually-exclusive set the
	// recompute manages) that no longer match the freshly computed
	// (seller, reason) — covers seller reallocation and activity-state
	// changes between recomputes. Paid rows and the additive bonus_nps/
	// bonus_retention rows are never touched.
	DeleteStaleProfitShare(ctx context.Context, accountID, cycle, keepSellerUserID, keepReason string) error
	// SumPaidProfitShare totals the (account, cycle)'s PAID profit-share
	// money (the mutually-exclusive reasons plus settled adjustments)
	// under ANY seller. Existence means the cycle's profit already
	// settled: a recompute must record a delta adjustment (US-004)
	// instead of minting a new full pending row. paidSellerUserID is
	// the seller of the most recently SETTLED profit-share row — a
	// clawback (negative delta) must be charged to whoever actually
	// received the money, never the account's current seller (US-023:
	// after an A→B reallocation a downward recompute debits A, not B).
	SumPaidProfitShare(ctx context.Context, accountID, cycle string) (paidCents int64, paidSellerUserID string, hasPaid bool, err error)
	// HasBonusEntry reports whether the account already has a ledger
	// entry (any status, any SELLER — a bonus granted before a
	// reallocation keeps blocking after it) with the given reason in any
	// of the cycles — nil cycles means any cycle ever. Backs the
	// 1-per-quarter NPS cap and the one-time retention bonus; recompute
	// idempotency comes from this check-first (the granted row itself
	// blocks the re-grant).
	HasBonusEntry(ctx context.Context, accountID, reason string, cycles []string) (bool, error)
	// ListEntriesByCycle returns the cycle's ledger with seller email +
	// account name joined for display.
	ListEntriesByCycle(ctx context.Context, cycle string) ([]*CommissionEntry, error)

	// ListSellerStatement returns the seller's ledger rows for one cycle
	// with account name + the cycle's profit breakdown joined (US-011).
	ListSellerStatement(ctx context.Context, sellerUserID, cycle string) ([]SellerStatementLine, error)
	// ListSellerCycles returns the distinct cycles the seller has ledger
	// entries in, newest first — the statement's cycle selector.
	ListSellerCycles(ctx context.Context, sellerUserID string) ([]string, error)
	// UpsertSellerPixKey writes only the seller's PIX key (nil clears),
	// preserving rates and activity state.
	UpsertSellerPixKey(ctx context.Context, userID string, pixKey *string) error

	// ListPayoutSummaries groups the ledger per seller for the owner
	// payout screen (US-013): the cycle's pending/paid totals plus the
	// pending amounts left in earlier cycles (late-recompute
	// adjustments), with the PIX key and user identity joined.
	ListPayoutSummaries(ctx context.Context, cycle string) ([]SellerPayoutSummary, error)
	// MarkPayoutPaid settles a seller's pending entries up to and
	// including the cycle (adjustments from earlier cycles ride along):
	// status -> 'paid' with paid_at + optional note. Only 'pending' rows
	// match, so marking twice is a no-op. Runs in its own transaction
	// holding the advisory locks of every cycle it settles (US-006), so
	// a concurrent recompute can neither delete a row mid-settle nor
	// re-mint a full pending row for money being paid. Returns the rows
	// updated and their summed amount — the total is revalidated from
	// the live locked rows, never a stale screen snapshot.
	MarkPayoutPaid(ctx context.Context, sellerUserID, cycle string, paidAt time.Time, note *string) (int64, int64, error)

	// ListSLAConversations returns the platform account's ASSIGNED
	// conversations created inside [from, to) with their first human
	// response timestamp (US-009 metric source).
	ListSLAConversations(ctx context.Context, accountID string, from, to time.Time) ([]SLAConversation, error)
	// GetSLAState returns nil, nil when the seller has no row for the
	// cycle (no data = no breach, counter at zero).
	GetSLAState(ctx context.Context, sellerUserID, cycle string) (*SellerSLAState, error)
	// UpsertSLAState writes on UNIQUE(seller_user_id, cycle), keeping
	// the original id — recompute-idempotent like the ledger.
	UpsertSLAState(ctx context.Context, s *SellerSLAState) error
	// ListSLAStatesByCycle returns the cycle's SLA states with seller
	// email joined for display.
	ListSLAStatesByCycle(ctx context.Context, cycle string) ([]*SellerSLAState, error)
	// ListDetractorFlags counts detractor NPS (score 0-6) per seller
	// over the carteira inside [from, to) — informational only.
	ListDetractorFlags(ctx context.Context, from, to time.Time) ([]SellerDetractorFlag, error)
}

// OwnerSellerRow is one platform-account user on the owner panel's
// Vendedores screen (owner-sellers US-001): identity + seller state +
// rotation membership + carteira size, joined in one read so the list
// renders without N+1 calls. InRotation is computed by the use case
// (the auto-assign "empty participants = everyone" default lives
// there), never by SQL.
type OwnerSellerRow struct {
	UserID        string  `json:"user_id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	Role          *string `json:"role"`
	IsActive      bool    `json:"is_active"`
	IsSeller      bool    `json:"is_seller"`
	IsActiveAM    bool    `json:"is_active_am"`
	SellerRefCode *string `json:"seller_ref_code,omitempty"`
	// ShareURL is the full marketing-site signup link for SellerRefCode
	// (webBaseURL + /register?ref=CODE) — built by the use case so the
	// app never hardcodes the site origin.
	ShareURL *string `json:"share_url,omitempty"`
	PixKey   *string `json:"pix_key,omitempty"`
	// CarteiraCount is the number of customer accounts currently linked
	// to this seller (accounts.seller_user_id).
	CarteiraCount int64 `json:"carteira_count"`
	InRotation    bool  `json:"in_rotation"`
}
