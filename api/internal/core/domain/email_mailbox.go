package domain

import (
	"context"
	"time"
)

// Deliverability guards (US-M007).
//
// The damage this prevents is permanent: a fresh domain that sends 500 cold
// emails on day one is burned at Gmail and Outlook, and later good behaviour
// never fully undoes it. The asset at risk belongs to the CUSTOMER, and they
// generally do not know the rules — so the platform enforces them instead of
// documenting them and hoping.
//
// Three independent gates, in the order they bite:
//
//	1. domain not authenticated (US-M006)  → cold sending refused outright
//	2. warmup unknown (never stamped)      → cold sending refused — NULL
//	   means "don't know how old this identity is", and not knowing never
//	   releases (same posture as the ClamAV gate: no scanner, no upload)
//	3. daily cap reached                   → the send is deferred, not lost;
//	   while the mailbox warms, the ceiling starts tiny and grows daily
//
// (3) defers rather than fails because the caller is a queue: the approach
// still goes out, just tomorrow. There is deliberately no longer a TOTAL
// cold block in the first days — the growing daily ceiling from day one IS
// the warmup (a handful of sends on day one teaches the provider gradually;
// a hard block followed by a burst teaches it nothing).

// Warmup knobs. Vars, not consts: both are env-configurable
// (EMAIL_WARMUP_RAMP_DAYS / EMAIL_WARMUP_DAILY_CAP via SetEmailWarmupConfig),
// because ramp length and full-speed ceiling are deliverability policy, and
// policy that requires a deploy to tune is policy nobody tunes.
var (
	// DefaultDailyCap is the standard cold-outreach ceiling per mailbox at
	// full speed. Configurable per mailbox too: a warmed mailbox on a mature
	// domain carries more, and a customer with a real deliverability
	// consultant should not be argued with.
	DefaultDailyCap = 25

	// WarmupDuration is how long a new mailbox ramps. Three weeks is the
	// consensus floor; shorter ramps are what get domains flagged.
	WarmupDuration = 21 * 24 * time.Hour
)

// SetEmailWarmupConfig applies the env-configured warmup policy. Zero or
// negative values keep the defaults — misconfiguration must never disable
// the guard.
func SetEmailWarmupConfig(rampDays, dailyCap int) {
	if rampDays > 0 {
		WarmupDuration = time.Duration(rampDays) * 24 * time.Hour
	}
	if dailyCap > 0 {
		DefaultDailyCap = dailyCap
	}
}

// WarmupRampDays is the configured ramp length in days, for messages and UI.
func WarmupRampDays() int { return int(WarmupDuration / (24 * time.Hour)) }

const (
	// WarmupInitialCap is the ceiling on day one of warmup — the whole cold
	// budget of a brand-new identity.
	WarmupInitialCap = 5

	// SMTPProvenDailyCap is the ceiling for a mailbox proven only by a
	// successful send through the CUSTOMER's own provider (Locaweb, Titan,
	// Workspace…), with none of our DNS records published.
	//
	// Deliberately NOT the configurable daily_cap: that column is an operator
	// decision about a domain authenticated for a transport we control and can
	// monitor. Here we are borrowing someone else's mailbox, under a per-day
	// limit their provider sets and does not tell us. Staying well under it is
	// the whole point — blowing through it gets the address throttled or
	// blocked, and that address is the one they answer customers from.
	SMTPProvenDailyCap = 25
)

// EmailMailbox is one sending identity and its guard state.
type EmailMailbox struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	ChannelID string `json:"channel_id"`
	Address   string `json:"address"`
	Domain    string `json:"domain"`

	DailyCap int `json:"daily_cap"`

	WarmupStartedAt   *time.Time `json:"warmup_started_at,omitempty"`
	WarmupCompletedAt *time.Time `json:"warmup_completed_at,omitempty"`

	SentToday int        `json:"sent_today"`
	SentDate  *time.Time `json:"sent_date,omitempty"`

	DNSVerifiedAt *time.Time `json:"dns_verified_at,omitempty"`

	// SMTPVerifiedAt is the last time the customer's OWN provider accepted an
	// authenticated send from us. It proves the identity for the low tier
	// without any DNS: their provider publishes SPF/DKIM for the domain and
	// signs the message, so ours would say nothing about this mail.
	SMTPVerifiedAt *time.Time `json:"smtp_verified_at,omitempty"`

	// Bulk sending (hybrid model, 2026-08-25): one-by-one approvals ALWAYS
	// leave through the customer's own mailbox; batches require the domain
	// registered and verified on the platform's bulk provider (Resend), so
	// the per-message cost only exists for accounts that asked for volume.
	//
	// BulkDomainID is the provider's id for this mailbox's domain;
	// BulkVerifiedAt stamps the provider reporting the DNS verified.
	// Forward-only, like DNSVerifiedAt.
	BulkDomainID   *string    `json:"bulk_domain_id,omitempty"`
	BulkVerifiedAt *time.Time `json:"bulk_verified_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BulkReady answers whether batch sends may leave through the bulk provider
// for this mailbox's domain.
func (m *EmailMailbox) BulkReady() bool {
	return m != nil && m.BulkDomainID != nil && m.BulkVerifiedAt != nil
}

// SendDecision is the answer to "may this mailbox send right now?".
type SendDecision struct {
	Allowed bool `json:"allowed"`
	// Reason is machine-readable so the queue can distinguish "try again
	// tomorrow" from "this will never work until the human acts".
	Reason SendBlockReason `json:"reason,omitempty"`
	// RetryAfter is set when the block is temporal (cap reached, warmup
	// day): the caller reschedules instead of dropping the send.
	RetryAfter *time.Time `json:"retry_after,omitempty"`
	// Remaining is today's leftover budget when allowed.
	Remaining int `json:"remaining"`
	// Used and Cap describe today's budget in both outcomes, so a refusal
	// can tell the person HOW MUCH and WHY instead of a bare "limite
	// atingido" they cannot plan around.
	Used int `json:"used"`
	Cap  int `json:"cap"`
}

type SendBlockReason string

const (
	BlockNone SendBlockReason = ""
	// BlockDNSUnverified now means "this identity has been proven by NEITHER
	// route": no successful send through the customer's own provider and none
	// of our records published. Kept under the original name because the
	// client already maps it, and the fix is the same either way — prove the
	// mailbox.
	BlockDNSUnverified SendBlockReason = "dns_unverified"
	// BlockWarmupUnknown means warmup_started_at is NULL: nothing recorded
	// when this identity started existing for the receiving providers. NULL
	// is "don't know", and not knowing never releases a cold send — the
	// column sat unwritten for months while this guard silently passed,
	// which is exactly the contact_ref_id disease.
	BlockWarmupUnknown SendBlockReason = "warmup_not_started"
	BlockDailyCap      SendBlockReason = "daily_cap_reached"
)

// EffectiveDailyCap is the ceiling for `now`, accounting for warmup ramp.
//
// The ramp is linear from WarmupInitialCap to the configured cap across
// WarmupDuration. Linear is a deliberate simplification of what warmup
// services do (they also vary volume by hour and mix in engagement); it is
// the part that matters most and the part we can honestly enforce ourselves.
func (m *EmailMailbox) EffectiveDailyCap(now time.Time) int {
	// Proven only by a send through the customer's own provider: their ceiling,
	// not ours, and we do not know what it is. No warmup ramp either — that
	// ramp teaches a receiving provider about a NEW sending identity, and this
	// address has been established at their provider for years. Ramping it
	// would throttle a mailbox nobody at Gmail considers new.
	if !m.DomainAuthenticated() {
		return SMTPProvenDailyCap
	}

	cap := m.DailyCap
	if cap <= 0 {
		cap = DefaultDailyCap
	}
	if !m.IsWarming(now) {
		return cap
	}
	elapsed := now.Sub(*m.WarmupStartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	progress := float64(elapsed) / float64(WarmupDuration)
	if progress > 1 {
		return cap
	}
	ramped := float64(WarmupInitialCap) + progress*float64(cap-WarmupInitialCap)
	if int(ramped) < WarmupInitialCap {
		return WarmupInitialCap
	}
	return int(ramped)
}

// IsWarming reports whether the mailbox is still inside its ramp.
func (m *EmailMailbox) IsWarming(now time.Time) bool {
	if m.WarmupStartedAt == nil {
		return false
	}
	if m.WarmupCompletedAt != nil {
		return false
	}
	return now.Sub(*m.WarmupStartedAt) < WarmupDuration
}

// DomainAuthenticated reports whether OUR records are published (US-M006) —
// the proof that matters when we are the one delivering to Gmail.
//
// It is NOT the only way an identity can be proven; see SMTPProven. Callers
// deciding "may this send" want SendingProven, and callers deciding "which
// ceiling applies" want this one.
func (m *EmailMailbox) DomainAuthenticated() bool { return m.DNSVerifiedAt != nil }

// SMTPProven reports whether the customer's own provider accepted an
// authenticated send from us. That provider publishes SPF/DKIM for the domain
// and signs the message, so this is the honest proof for that transport — and
// asking for our records instead would be asking about mail we do not send.
func (m *EmailMailbox) SMTPProven() bool { return m.SMTPVerifiedAt != nil }

// SendingProven reports whether the identity has been proven by EITHER route.
// Neither means nothing is known about this address beyond somebody typing it
// into a form, which is not a reason to put it in front of a stranger.
func (m *EmailMailbox) SendingProven() bool { return m.DomainAuthenticated() || m.SMTPProven() }

// WarmupDay is the 1-based day of the ramp, for messages and UI. 0 when the
// ramp never started.
func (m *EmailMailbox) WarmupDay(now time.Time) int {
	if m.WarmupStartedAt == nil {
		return 0
	}
	day := int(now.Sub(*m.WarmupStartedAt)/(24*time.Hour)) + 1
	if day < 1 {
		day = 1
	}
	return day
}

// sentTodayAt returns today's counter, treating a counter from a previous
// day as zero. The stored counter is only meaningful together with its date.
func (m *EmailMailbox) sentTodayAt(now time.Time) int {
	if m.SentDate == nil {
		return 0
	}
	y1, mo1, d1 := m.SentDate.UTC().Date()
	y2, mo2, d2 := now.UTC().Date()
	if y1 == y2 && mo1 == mo2 && d1 == d2 {
		return m.SentToday
	}
	return 0
}

// CanSend answers the send question for one message.
//
// `cold` distinguishes a first touch from a reply. Conflating them would
// either block legitimate replies during warmup (making the product useless
// for a week) or let cold blasts through (defeating the point).
func (m *EmailMailbox) CanSend(now time.Time, cold bool) SendDecision {
	if cold && !m.SendingProven() {
		return SendDecision{Reason: BlockDNSUnverified}
	}
	// FAIL CLOSED on an unknown warmup. Every proof route (DNS verification,
	// SMTP test send, OAuth connect) stamps warmup_started_at, so NULL here
	// is a mailbox that skipped verification or predates the stamp — and a
	// guard that cannot answer "how old is this identity?" must refuse, not
	// wave through. This exact NULL let cold blasts pass for months.
	if cold && m.WarmupStartedAt == nil {
		return SendDecision{Reason: BlockWarmupUnknown}
	}

	cap := m.EffectiveDailyCap(now)
	used := m.sentTodayAt(now)
	if used >= cap {
		next := NextUTCMidnight(now)
		return SendDecision{Reason: BlockDailyCap, RetryAfter: &next, Used: used, Cap: cap}
	}
	return SendDecision{Allowed: true, Remaining: cap - used, Used: used, Cap: cap}
}

// EmailWarmupStatus is the warmup picture a screen renders on an email
// channel: whether the ramp started, where it is, and today's budget.
type EmailWarmupStatus struct {
	Started   bool `json:"started"`
	Warming   bool `json:"warming"`
	Day       int  `json:"day,omitempty"`
	RampDays  int  `json:"ramp_days"`
	DailyCap  int  `json:"daily_cap"`
	SentToday int  `json:"sent_today"`
	Remaining int  `json:"remaining"`
}

// NewEmailWarmupStatus derives the screen-facing state from the guard state,
// so the UI and the send gate can never disagree about today's budget.
func NewEmailWarmupStatus(m *EmailMailbox, now time.Time) *EmailWarmupStatus {
	if m == nil {
		return nil
	}
	cap := m.EffectiveDailyCap(now)
	used := m.sentTodayAt(now)
	remaining := cap - used
	if remaining < 0 {
		remaining = 0
	}
	return &EmailWarmupStatus{
		Started:   m.WarmupStartedAt != nil,
		Warming:   m.IsWarming(now),
		Day:       m.WarmupDay(now),
		RampDays:  WarmupRampDays(),
		DailyCap:  cap,
		SentToday: used,
		Remaining: remaining,
	}
}

// NextUTCMidnight is when a daily budget resets. UTC (not the account's
// timezone) because the counter is stored per UTC date — one clock for the
// cap keeps "did we already send today?" unambiguous across DST.
func NextUTCMidnight(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
}

// EmailMailboxRepository persists the guard state.
type EmailMailboxRepository interface {
	// EnsureForChannel returns the mailbox row for a channel's from
	// address, creating it on first use so no explicit setup step can be
	// skipped.
	EnsureForChannel(ctx context.Context, accountID, channelID, address string) (*EmailMailbox, error)
	GetByAddress(ctx context.Context, accountID, address string) (*EmailMailbox, error)
	ListByAccount(ctx context.Context, accountID string) ([]*EmailMailbox, error)
	// RecordSend increments today's counter ATOMICALLY, resetting it when
	// the stored date is not today. Returns the post-increment count.
	//
	// Atomicity is the point: two concurrent sends that both read-then-write
	// would each see the same count and blow through the cap.
	RecordSend(ctx context.Context, mailboxID string, now time.Time) (int, error)
	// MarkDNSVerified stamps a successful US-M006 verification.
	MarkDNSVerified(ctx context.Context, accountID, address string, at time.Time) error
	// MarkSMTPProven stamps a successful authenticated send through the
	// customer's own provider — the proof that unlocks the low tier with no
	// DNS at all.
	MarkSMTPProven(ctx context.Context, accountID, address string, at time.Time) error
	// ClearSMTPProof revokes it for one channel, because a credential change
	// means we no longer proved anything about the transport we would use.
	ClearSMTPProof(ctx context.Context, accountID, channelID string) error
	// StartWarmup / CompleteWarmup drive the ramp.
	StartWarmup(ctx context.Context, accountID, address string, at time.Time) error
	CompleteWarmup(ctx context.Context, accountID, address string, at time.Time) error
	SetDailyCap(ctx context.Context, accountID, address string, cap int) error
	// SetBulkDomain records the bulk provider's domain id for this mailbox's
	// domain (hybrid sending). MarkBulkVerified stamps the provider reporting
	// the records verified — forward-only, same posture as MarkDNSVerified.
	SetBulkDomain(ctx context.Context, accountID, address, domainID string) error
	MarkBulkVerified(ctx context.Context, accountID, address string, at time.Time) error
}

var (
	ErrMailboxNotFound = &AppError{Code: "EMAIL_MAILBOX_NOT_FOUND", Message: "sending mailbox not found"}
	ErrEmailDailyCap   = &AppError{
		Code:    "EMAIL_DAILY_CAP_REACHED",
		Message: "limite diário de envios desta caixa atingido — o restante sai amanhã",
	}
	ErrEmailWarmupNotStarted = &AppError{
		Code: "EMAIL_WARMUP_NOT_STARTED",
		Message: "esta caixa nunca teve o aquecimento iniciado — verifique o domínio " +
			"(DNS) ou reconecte a caixa. Sem saber a idade da identidade, a " +
			"prospecção fria não sai",
	}
	// ErrBulkSendingUnavailable: the platform has no bulk provider key
	// configured (RESEND_API_KEY). Infrastructure gap, not a customer one.
	ErrBulkSendingUnavailable = &AppError{
		Code:    "BULK_SENDING_UNAVAILABLE",
		Message: "envio em volume não está disponível na plataforma no momento",
	}
	// ErrBulkSendingNotReady: this account's domain is not registered+verified
	// with the bulk provider — batches are refused until it is (individual
	// approvals keep flowing through the customer's own mailbox).
	ErrBulkSendingNotReady = &AppError{
		Code: "BULK_SENDING_NOT_READY",
		Message: "envio em lote exige o domínio configurado para volume — ative em " +
			"Configurações → Canais, ou aprove uma a uma pela sua própria caixa",
	}
)
