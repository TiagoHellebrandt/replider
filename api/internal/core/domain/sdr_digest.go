package domain

import (
	"context"
	"fmt"
	"time"
)

// Actionable daily digest (US-M010).
//
// The digest exists to answer "what needs me today?" in one notification.
// So it is built from things a person can ACT on, each with a deep link —
// not from a summary of what happened, which reads well and changes
// nothing.

// SDRDigest is one account's daily state.
type SDRDigest struct {
	AccountID string `json:"account_id"`
	// PendingApprovals is the human-in-the-loop backlog (US-M020).
	PendingApprovals int64 `json:"pending_approvals"`
	// NewReplies is prospects who answered since the last digest — the
	// most time-sensitive item, because a reply left cold reads as being
	// ignored by the company that just cold-approached them.
	NewReplies int64 `json:"new_replies"`
	// StalledLeads were approached and never answered, past the patience
	// window. They are the follow-up list.
	StalledLeads int64 `json:"stalled_leads"`
	// UnownedLeads is live leads nobody is responsible for.
	UnownedLeads int64 `json:"unowned_leads"`
}

// StalledLeadAfter is how long an unanswered approach waits before it
// counts as stalled. Three business-ish days: long enough that a busy
// decision-maker has not simply been in meetings, short enough that the
// follow-up still references something they remember.
const StalledLeadAfter = 72 * time.Hour

// HasWork reports whether there is anything worth interrupting someone
// for. A digest that fires on an empty day is how a daily notification
// becomes something people mute.
func (d SDRDigest) HasWork() bool {
	return d.PendingApprovals > 0 || d.NewReplies > 0 ||
		d.StalledLeads > 0 || d.UnownedLeads > 0
}

// DigestLine is one actionable row: what it is, how many, and where to go.
type DigestLine struct {
	Kind     string `json:"kind"`
	Count    int64  `json:"count"`
	Text     string `json:"text"`
	ClickURL string `json:"click_url"`
}

// Lines renders the digest as ordered actionable rows, most urgent first:
// a reply waiting beats a backlog, which beats a follow-up, which beats
// housekeeping. Empty items are omitted rather than shown as zero — "0
// respostas novas" is noise pretending to be information.
func (d SDRDigest) Lines() []DigestLine {
	var out []DigestLine
	if d.NewReplies > 0 {
		out = append(out, DigestLine{
			Kind:     "new_replies",
			Count:    d.NewReplies,
			Text:     fmt.Sprintf("%d resposta(s) nova(s) esperando você", d.NewReplies),
			ClickURL: "/home",
		})
	}
	if d.PendingApprovals > 0 {
		out = append(out, DigestLine{
			Kind:     "pending_approvals",
			Count:    d.PendingApprovals,
			Text:     fmt.Sprintf("%d abordagem(ns) aguardando aprovação", d.PendingApprovals),
			ClickURL: "/prospect?tab=queue",
		})
	}
	if d.StalledLeads > 0 {
		out = append(out, DigestLine{
			Kind:     "stalled_leads",
			Count:    d.StalledLeads,
			Text:     fmt.Sprintf("%d lead(s) sem resposta há mais de 3 dias", d.StalledLeads),
			ClickURL: "/funnel",
		})
	}
	if d.UnownedLeads > 0 {
		out = append(out, DigestLine{
			Kind:     "unowned_leads",
			Count:    d.UnownedLeads,
			Text:     fmt.Sprintf("%d lead(s) sem dono", d.UnownedLeads),
			ClickURL: "/funnel",
		})
	}
	return out
}

// Headline is the notification title's body text: the single most urgent
// line, because a push notification has room for one thing.
func (d SDRDigest) Headline() string {
	lines := d.Lines()
	if len(lines) == 0 {
		return ""
	}
	return lines[0].Text
}

// DigestCandidate is one account the digest may be due for, carrying the
// timezone the window has to be measured in.
//
// The timezone travels WITH the row rather than being fetched per account:
// the due-query already has to read it to decide the account is due, and a
// second lookup per account is how a daily job becomes N round trips.
type DigestCandidate struct {
	AccountID string
	Timezone  string
}

// SDRDigestRepository builds the digest and tracks who has been sent one.
type SDRDigestRepository interface {
	// ListDue returns accounts that opted in, whose OWN local clock has
	// reached sendHour, and that have not been sent a digest since their
	// own local midnight.
	//
	// Both conditions are per-account and both resolve in the ACCOUNT's
	// timezone — the whole point of the digest is that it lands at the
	// start of somebody's working day, and the server's day is not theirs.
	// The window is a DB filter, not a worker bookkeeping detail, so a
	// restart cannot double-send.
	ListDue(ctx context.Context, sendHour, limit int) ([]DigestCandidate, error)
	// Build computes one account's digest. repliesSince bounds "new"
	// replies to the period the digest covers.
	Build(ctx context.Context, accountID string, repliesSince time.Time, stalledBefore time.Time) (*SDRDigest, error)
	// MarkSent stamps the account so today's digest is not repeated.
	MarkSent(ctx context.Context, accountID string, at time.Time) error
}
