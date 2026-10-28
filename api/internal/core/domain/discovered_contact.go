package domain

import (
	"context"
	"strings"
	"time"
)

// Learning the customer list from the customer's own Sent folder.
//
// The product promises to bring in the customers she already has, and until now
// there was no way to do it: no endpoint creates a contact, and the only import
// reads prospecting results. The answer was in her mailbox the whole time — the
// people she has been emailing for years ARE the list.
//
// Two rules shape everything here:
//
//   - Only HEADERS are read (To/Cc/Date). Reading the body of every message she
//     ever sent, to answer "who are your customers", is out of proportion to the
//     question. Recipients and dates answer it.
//   - Nothing becomes a contact until a human says so. A list a machine
//     assembled from somebody's mailbox is exactly what must not act on its own.

// DiscoveredContact is one address the customer has written to.
type DiscoveredContact struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	ChannelID string `json:"channel_id,omitempty"`

	Address string `json:"address"`
	Name    string `json:"name,omitempty"`

	// SentCount, FirstSentAt and LastSentAt are the ranking signal. Somebody
	// emailed 40 times across two years is a customer; somebody emailed once is
	// a stranger who asked a question.
	SentCount   int        `json:"sent_count"`
	FirstSentAt *time.Time `json:"first_sent_at,omitempty"`
	LastSentAt  *time.Time `json:"last_sent_at,omitempty"`
	// LastSubject is the newest sighting's envelope subject (issue #292):
	// what the relationship was last about, feeding the lead dossier so a
	// draft can pick the thread back up instead of opening cold.
	LastSubject string `json:"last_subject,omitempty"`

	// ImportedAt and DismissedAt are kept apart because "not now" is not
	// "already done": collapsing them would either re-offer what she refused or
	// hide what she has not decided.
	ImportedAt  *time.Time `json:"imported_at,omitempty"`
	DismissedAt *time.Time `json:"dismissed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Pending reports a candidate nobody has decided about yet.
func (d *DiscoveredContact) Pending() bool {
	return d.ImportedAt == nil && d.DismissedAt == nil
}

// SentRecipient is one address seen in one sent message, before aggregation.
type SentRecipient struct {
	Address string
	Name    string
	SentAt  time.Time
	// Subject is the message's envelope subject — still envelope-only, never
	// a body. It is what makes the discovered relationship usable as MEMORY
	// ("último assunto: manutenção do tomógrafo") instead of just a count.
	Subject string
}

// DiscoveredContactRepository persists the candidates.
type DiscoveredContactRepository interface {
	// Upsert accumulates a sighting onto the candidate for that address:
	// the count goes up, the date window widens, and a name fills in a blank
	// one. Never resets a decision — a rescan must not un-dismiss what a human
	// refused.
	Upsert(ctx context.Context, accountID, channelID string, r SentRecipient) error
	// ListPending returns undecided candidates, most-emailed first.
	ListPending(ctx context.Context, accountID string, limit, offset int32) ([]*DiscoveredContact, error)
	// GetByAddresses resolves a human's selection, scoped to the account so a
	// request cannot import another tenant's candidate.
	GetByAddresses(ctx context.Context, accountID string, addresses []string) ([]*DiscoveredContact, error)
	// MarkImported and MarkDismissed record the decision.
	MarkImported(ctx context.Context, accountID string, addresses []string, at time.Time) error
	MarkDismissed(ctx context.Context, accountID string, addresses []string, at time.Time) error
	// CountPending feeds the nudge that tells her the list is there at all.
	CountPending(ctx context.Context, accountID string) (int64, error)
}

// SentMailboxNames are the folders a Sent box is called, tried in order when the
// server does not advertise the \Sent special-use attribute (RFC 6154).
//
// The Portuguese names are here because they are what a Brazilian customer's
// mailbox actually uses, and the dotted forms because Dovecot — which Locaweb
// and most cPanel hosts run — nests everything under INBOX with a dot.
var SentMailboxNames = []string{
	"Sent",
	"INBOX.Sent",
	"Sent Items",
	"INBOX.Sent Items",
	"Enviados",
	"INBOX.Enviados",
	"Itens Enviados",
	"INBOX.Itens Enviados",
	"[Gmail]/Sent Mail",
}

// IsAutomatedAddress reports an address no human reads, so it must never be
// offered as a customer.
//
// A conservative list on purpose: a false positive silently hides a real
// customer from the import, which is worse than one obvious robot in a list a
// human is already reviewing. Matched on the local part so a whole domain is
// never excluded — `contato@` at a supplier is a person, `no-reply@` never is.
func IsAutomatedAddress(address string) bool {
	at := strings.LastIndex(address, "@")
	if at <= 0 {
		return false
	}
	local := strings.ToLower(address[:at])
	// Normalized so no-reply, noreply and no_reply are one rule.
	local = strings.NewReplacer("-", "", "_", "", ".", "").Replace(local)
	for _, prefix := range []string{
		"noreply", "donotreply", "naoresponda", "mailerdaemon", "postmaster",
		"bounce", "notificacoes", "notifications", "notificacao",
	} {
		if strings.HasPrefix(local, prefix) {
			return true
		}
	}
	return false
}
