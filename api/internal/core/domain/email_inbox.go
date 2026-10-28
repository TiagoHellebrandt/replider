package domain

import (
	"context"
	"time"
)

// Reading the customer's mailbox (US-M005 follow-up).
//
// Every other channel here is pushed to us: WhatsApp, Telegram, Instagram and
// SMS all POST a webhook, and the provider retries until we ack. The mailbox a
// customer already owns does none of that — Locaweb, Titan and Workspace are
// mail servers, not webhook senders — so replies only reach the product if we
// go and fetch them.
//
// The alternative was asking the customer to change their domain's MX so mail
// arrives at us instead. That is a domain-wide change for a tool only the sales
// team uses: the finance department's invoices and the owner's mail would start
// flowing through us, and our downtime would become the company's downtime.
// Reading one mailbox with one password is scoped to exactly the person using
// the product, needs no DNS, and is undone by revoking one credential.

// EmailInboxCursor is where the poller stopped reading one folder.
type EmailInboxCursor struct {
	ChannelID string `json:"channel_id"`
	AccountID string `json:"account_id"`
	Mailbox   string `json:"mailbox"`

	// UIDValidity is the server's generation number for this folder's UID
	// space. A change invalidates LastUID completely — the same number may now
	// point at a different message — so the poller re-anchors instead of
	// reading a range that would be arbitrary.
	UIDValidity uint32 `json:"uid_validity"`
	// LastUID is the highest UID already ingested. Zero alongside a set
	// UIDValidity means "anchored, nothing read": a first connection starts at
	// the top of the mailbox rather than importing years of history that would
	// arrive as hundreds of brand-new conversations.
	LastUID uint32 `json:"last_uid"`

	LastPolledAt *time.Time `json:"last_polled_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
	FailureCount int        `json:"failure_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Anchored reports whether the cursor has a usable position.
func (c *EmailInboxCursor) Anchored() bool { return c.UIDValidity != 0 }

// EmailInboxCursorRepository persists poller positions.
type EmailInboxCursorRepository interface {
	// EnsureForChannel creates the row on first use, unanchored. Implicit
	// because an explicit "register this inbox" step is a step somebody
	// forgets, and the symptom is silence rather than an error.
	EnsureForChannel(ctx context.Context, accountID, channelID, mailbox string) (*EmailInboxCursor, error)
	// Advance moves the cursor forward and clears the failure state. Never
	// backwards: a lower UID would re-ingest messages the pipeline already
	// deduped, and re-anchoring is an explicit call rather than a side effect.
	Advance(ctx context.Context, channelID, mailbox string, uidValidity, lastUID uint32, at time.Time) error
	// Reanchor replaces the position wholesale, for the case where the server
	// changed UIDVALIDITY and the old cursor addresses nothing meaningful.
	Reanchor(ctx context.Context, channelID, mailbox string, uidValidity, lastUID uint32, at time.Time) error
	// RecordFailure stamps the attempt and increments the run. The message is
	// stored so "why did nothing arrive?" is answerable from the row.
	RecordFailure(ctx context.Context, channelID, mailbox string, at time.Time, reason string) error
	// ListDue returns cursors not polled since `before`, oldest first, with
	// never-polled ones first of all.
	ListDue(ctx context.Context, before time.Time, limit int32) ([]*EmailInboxCursor, error)
}

// FetchedEmail is one message read from a mailbox.
//
// Carries the UID so the caller can advance the cursor past exactly what it
// ingested, and RawMessage so the MIME is parsed once, above the transport.
type FetchedEmail struct {
	UID uint32
	// RawMessage is the full RFC 5322 message, headers included. Kept whole
	// rather than pre-split because the threading headers, the
	// Authentication-Results the customer's own server stamped, and the
	// attachments all live in it — and a transport that decided which parts
	// mattered would be the wrong layer to decide that.
	RawMessage []byte
	// InternalDate is when the server received it, used for nothing but
	// ordering and diagnostics: the Date header is written by the sender and
	// can say anything.
	InternalDate time.Time
}

// EmailInboxFetcher reads new messages from one mailbox.
//
// Deliberately narrow. Three things this port must never grow: marking messages
// read, moving them, and deleting them. The salesperson is working the same
// mailbox in their own client all day; a fetcher that touches state they can see
// is a fetcher that steals their unread badge, loses their filing, or destroys
// mail. Read-only is not a limitation here, it is the contract.
type EmailInboxFetcher interface {
	// Fetch returns messages with a UID strictly greater than afterUID, up to
	// limit, and the folder's current UIDVALIDITY.
	//
	// A UIDVALIDITY different from the one passed in means the caller's cursor
	// is meaningless; implementations report the new value and return no
	// messages, leaving the decision to re-anchor with the caller rather than
	// silently re-reading a mailbox.
	Fetch(ctx context.Context, cfg EmailInboxConfig, knownUIDValidity, afterUID uint32, limit int) (EmailFetchResult, error)
}

// EmailFetchResult is one poll's outcome.
type EmailFetchResult struct {
	// UIDValidity is the folder's generation as the server reports it NOW.
	UIDValidity uint32
	// Messages are ordered by ascending UID, so a partial ingest can still
	// advance the cursor to the last one it actually handled.
	Messages []FetchedEmail
	// HighestUID is the top of the mailbox, whether or not it was returned.
	// This is what anchors a first connection: starting from the newest message
	// means a new channel does not import years of history as new conversations.
	HighestUID uint32
	// More reports that the mailbox held messages past `limit`, so the poller
	// can come back immediately instead of waiting a full interval while a
	// backlog drains one page per tick.
	More bool
}

// EmailInboxConfig is what it takes to open one mailbox.
type EmailInboxConfig struct {
	Host string
	Port string
	// Username and Password are the SAME credentials the channel already sends
	// with. Every provider that hosts a mailbox authenticates both protocols
	// against one account, so asking for a second pair would be asking the
	// customer to invent one.
	Username string
	Password string
	// AccessToken, when set, replaces Password and switches the mechanism to
	// XOAUTH2 — the mailbox connected by consent rather than by an app
	// password. Mutually exclusive with Password by construction: a consented
	// channel stores no password.
	AccessToken string
	Mailbox     string
}

// EmailSentScanResult is one page of a backwards walk through the Sent folder.
type EmailSentScanResult struct {
	// Mailbox is the folder that was actually read, so the caller can record
	// which name worked rather than rediscovering it every page.
	Mailbox     string
	UIDValidity uint32
	// HighestUID is the top of the folder as seen on this page.
	HighestUID uint32
	// LowestScannedUID is where the next page continues from, exclusive and
	// downwards. Zero when nothing was read.
	LowestScannedUID uint32
	// Recipients are every To/Cc address seen, one entry per message per
	// address — aggregation belongs to the caller, which is the layer that
	// knows what to exclude.
	//
	// The ORDER within a page follows the server's, not the walk's: a FETCH
	// answers in its own sequence and re-sorting would cost a pass for nothing,
	// since the caller aggregates. The newest-first guarantee is about WHICH
	// messages a page contains, not how they are listed inside it.
	Recipients []SentRecipient
	// Done reports that the bottom of the folder was reached.
	Done bool
}

// EmailSentScanner walks a Sent folder backwards, reading envelopes only.
//
// Separate from EmailInboxFetcher because they answer different questions and
// must be allowed to differ: one is a forward stream of things to reply to, the
// other is a bounded look backwards to learn a customer list. Folding them
// together would mean one set of paging rules serving neither well.
type EmailSentScanner interface {
	ScanSent(ctx context.Context, cfg EmailInboxConfig, beforeUID uint32, limit int) (EmailSentScanResult, error)
}
