package domain

import "time"

// MinutesSavedPerBotReply is the coarse "time saved" multiplier applied
// to each automatic bot reply for the digest's headline figure. It is a
// deliberately conservative estimate of the time an attendant would have
// spent reading and typing a single reply by hand.
const MinutesSavedPerBotReply = 2

// WeeklyDigestAccount is one row of the worker's due-list: an account
// that still receives the weekly digest (not opted out, not suspended)
// and is due for a fresh send. Kept minimal — the admin recipient is
// resolved separately so this stays a cheap accounts-only scan.
type WeeklyDigestAccount struct {
	AccountID  string
	Name       string
	PlanID     *string
	LastSentAt *time.Time
}

// WeeklyDigestFAQ is a single "top question" line: the customer message
// text most frequently received in the reporting window, with its count.
type WeeklyDigestFAQ struct {
	Question string
	Count    int
}

// WeeklyDigestStats is the per-account activity summary for one reporting
// window (the trailing 7 days). Drives the email body (US-176).
type WeeklyDigestStats struct {
	// MessagesHandled is the number of automatic bot replies the agent
	// sent on the account's behalf in the window.
	MessagesHandled int
	// TimeSavedMinutes is a coarse estimate (MessagesHandled ×
	// MinutesSavedPerBotReply) framed as "time your team didn't spend
	// typing replies".
	TimeSavedMinutes int
	// ConversionPct is the bot resolution rate: of conversations opened
	// in the window, the share closed without a human takeover, 0..100.
	ConversionPct int
	// TopFAQs are the most frequent inbound customer questions, highest
	// count first (at most a handful).
	TopFAQs []WeeklyDigestFAQ
}

// WeeklyDigest bundles a computed stats summary with the window it
// covers, ready to render into the email.
type WeeklyDigest struct {
	Stats       WeeklyDigestStats
	PeriodStart time.Time
	PeriodEnd   time.Time
}
