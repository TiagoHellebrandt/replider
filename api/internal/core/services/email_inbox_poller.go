package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// EmailInboxPoller reads the customer's mailbox on a timer.
//
// Every other channel is pushed to us and retried by its provider. A mailbox is
// neither: if we do not go and look, the reply simply sits there and the
// salesperson finds out from the customer that they answered days ago.
//
// The shape of this worker is dictated by one fact: the mailbox belongs to a
// person who is using it at the same time. So it reads, and only reads — the
// fetcher cannot mark, move or delete — and it paces itself, because providers
// cap concurrent IMAP sessions per account and a poller that opens one
// connection per channel at once is a poller that locks the customer out of
// their own webmail.
type EmailInboxPoller struct {
	cursors  domain.EmailInboxCursorRepository
	fetcher  domain.EmailInboxFetcher
	channels EmailPollChannelResolver
	ingest   EmailInboxIngestor

	interval time.Duration
	// concurrency caps simultaneous IMAP sessions across ALL accounts. Small on
	// purpose: this is I/O waiting on someone else's mail server, and the cost
	// of being slow is a reply arriving a minute later, while the cost of being
	// greedy is a customer's provider refusing their own login.
	concurrency int
	// batch is how many messages one poll ingests. A backlog drains over
	// several polls instead of one long transaction, and `More` makes the
	// poller come straight back rather than waiting a full interval.
	batch int
	// maxFailures is how many consecutive failures before an inbox is polled
	// on a slower cadence. A wrong password otherwise burns a connection
	// attempt every interval forever, which providers read as an attack.
	maxFailures int
	// tokens mints the access token for a mailbox connected by consent. Nil
	// means the platform has no Google client configured; such a channel then
	// fails its poll with a named reason rather than attempting an empty
	// password, which the provider would count as a bad login.
	tokens domain.EmailOAuthTokenSource

	now func() time.Time
}

// EmailPollChannelResolver reads the channel a cursor belongs to. Narrow port
// rather than the full repository so the poller cannot mutate a channel.
type EmailPollChannelResolver interface {
	GetByID(ctx context.Context, id string) (*domain.Channel, error)
}

// EmailInboxIngestor hands one fetched message to the inbound pipeline.
//
// The implementation lives in the HTTP layer, next to the webhook path it
// shares: a mailbox message and a webhook message must be stored, checked and
// broadcast identically, and the only way to guarantee that is for both to call
// the same code.
type EmailInboxIngestor interface {
	// IngestFetchedEmail returns whether anything was persisted. A duplicate is
	// (false, nil): already handled, nothing to report.
	IngestFetchedEmail(ctx context.Context, channel *domain.Channel, raw []byte) (bool, error)
}

func NewEmailInboxPoller(
	cursors domain.EmailInboxCursorRepository,
	fetcher domain.EmailInboxFetcher,
	channels EmailPollChannelResolver,
	ingest EmailInboxIngestor,
) *EmailInboxPoller {
	return &EmailInboxPoller{
		cursors:     cursors,
		fetcher:     fetcher,
		channels:    channels,
		ingest:      ingest,
		interval:    2 * time.Minute,
		concurrency: 4,
		batch:       25,
		maxFailures: 5,
		now:         time.Now,
	}
}

// SetInterval overrides the poll cadence (tests, and a future per-plan tier).
func (p *EmailInboxPoller) SetInterval(d time.Duration) {
	if d > 0 {
		p.interval = d
	}
}

// Start runs until ctx is done. Ticks immediately so a freshly connected inbox
// does not wait a full interval to prove itself.
// SetOAuthTokens wires the token source. Optional setter so the constructor
// and its call sites stay untouched.
func (p *EmailInboxPoller) SetOAuthTokens(t domain.EmailOAuthTokenSource) { p.tokens = t }

func (p *EmailInboxPoller) Start(ctx context.Context) {
	if p.cursors == nil || p.fetcher == nil || p.channels == nil || p.ingest == nil {
		slog.Warn("email inbox poller not wired; mailbox replies will not be read")
		return
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.RunOnce(ctx)
		}
	}
}

// RunOnce polls every inbox that is due.
func (p *EmailInboxPoller) RunOnce(ctx context.Context) {
	now := p.now()
	due, err := p.cursors.ListDue(ctx, now.Add(-p.interval), 200)
	if err != nil {
		slog.ErrorContext(ctx, "email inbox poller: could not list due inboxes", "error", err)
		return
	}
	if len(due) == 0 {
		return
	}

	// A fixed pool rather than one goroutine per inbox: see `concurrency`.
	work := make(chan *domain.EmailInboxCursor)
	var wg sync.WaitGroup
	workers := p.concurrency
	if workers > len(due) {
		workers = len(due)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cursor := range work {
				p.pollOne(ctx, cursor)
			}
		}()
	}
	for _, cursor := range due {
		select {
		case <-ctx.Done():
			close(work)
			wg.Wait()
			return
		case work <- cursor:
		}
	}
	close(work)
	wg.Wait()
}

// pollOne reads and ingests one mailbox.
func (p *EmailInboxPoller) pollOne(ctx context.Context, cursor *domain.EmailInboxCursor) {
	now := p.now()

	// A repeatedly failing inbox is polled on a slower cadence rather than
	// disabled: the usual cause is a rotated password, and the customer fixes
	// it in the channel form without knowing they also have to re-enable
	// anything. Backing off protects their provider from a login attempt every
	// two minutes, which reads as an attack.
	if cursor.FailureCount >= p.maxFailures && !p.backoffElapsed(cursor, now) {
		return
	}

	channel, err := p.channels.GetByID(ctx, cursor.ChannelID)
	if err != nil || channel == nil {
		p.fail(ctx, cursor, now, "canal não encontrado")
		return
	}
	if !channel.IsActive {
		// Paused channel: stamp the attempt so it stops being due every tick,
		// without recording a failure it did not have.
		_ = p.cursors.Advance(ctx, cursor.ChannelID, cursor.Mailbox,
			cursor.UIDValidity, cursor.LastUID, now)
		return
	}

	cfg, ok := inboxConfigFromChannel(channel, cursor.Mailbox)
	if !ok {
		p.fail(ctx, cursor, now, "canal sem host/usuário de IMAP configurado")
		return
	}
	// Refreshed per poll, not per channel: an access token lives about an hour
	// and this loop outlives thousands of them.
	if domain.IsOAuthEmail(channel) {
		if p.tokens == nil {
			p.fail(ctx, cursor, now, domain.ErrEmailOAuthUnavailable.Message)
			return
		}
		token, tokErr := p.tokens.AccessToken(ctx, channel)
		if tokErr != nil {
			p.fail(ctx, cursor, now, tokErr.Error())
			return
		}
		cfg.AccessToken = token
	}

	result, err := p.fetcher.Fetch(ctx, cfg, cursor.UIDValidity, cursor.LastUID, p.batch)
	if err != nil {
		// The provider refused the token we hold: drop it from the cache so
		// the next poll refreshes from the grant instead of retrying a token
		// already known to be dead for the rest of its hour.
		if errors.Is(err, domain.ErrEmailOAuthNotConnected) {
			if f, ok := p.tokens.(interface{ Forget(string) }); ok {
				f.Forget(channel.ID)
			}
		}
		p.fail(ctx, cursor, now, err.Error())
		return
	}

	// First connection, or a mailbox whose UID space the server rebuilt. Anchor
	// to the TOP rather than reading everything: importing years of history
	// would land hundreds of strangers in the pipeline as brand-new
	// conversations, each one looking like a lead nobody has answered.
	if !cursor.Anchored() || result.UIDValidity != cursor.UIDValidity {
		if result.UIDValidity == 0 {
			p.fail(ctx, cursor, now, "servidor não informou UIDVALIDITY")
			return
		}
		if err := p.cursors.Reanchor(ctx, cursor.ChannelID, cursor.Mailbox,
			result.UIDValidity, result.HighestUID, now); err != nil {
			slog.ErrorContext(ctx, "email inbox poller: could not anchor cursor",
				"channel_id", cursor.ChannelID, "error", err)
		}
		return
	}

	// Ingest in UID order and remember the last one that actually landed, so a
	// failure halfway through leaves the cursor pointing at the last message we
	// really handled — the rest come back next poll instead of being skipped.
	ingested := cursor.LastUID
	var ingestErr error
	for _, msg := range result.Messages {
		if ctx.Err() != nil {
			break
		}
		if _, err := p.ingest.IngestFetchedEmail(ctx, channel, msg.RawMessage); err != nil {
			slog.ErrorContext(ctx, "email inbox poller: could not ingest message",
				"channel_id", cursor.ChannelID, "uid", msg.UID, "error", err)
			ingestErr = err
			break
		}
		ingested = msg.UID
	}

	if ingested > cursor.LastUID || len(result.Messages) == 0 {
		if err := p.cursors.Advance(ctx, cursor.ChannelID, cursor.Mailbox,
			result.UIDValidity, ingested, now); err != nil {
			slog.ErrorContext(ctx, "email inbox poller: could not advance cursor",
				"channel_id", cursor.ChannelID, "error", err)
		}
	}

	// An ingest failure is a poll failure. Recorded AFTER the advance, which
	// resets the counter, so the partial progress is kept and the failure
	// still sticks: without this the cursor sat on the failed UID with no
	// last_error, no backoff, and the same message was retried every two
	// minutes forever — blocking every later reply in that mailbox with
	// nothing in the panel to say why.
	if ingestErr != nil {
		p.fail(ctx, cursor, now, ingestErr.Error())
		return
	}

	// A page-sized batch with more waiting: come straight back rather than
	// letting a backlog drain one page per interval.
	if result.More && ctx.Err() == nil {
		p.pollOne(ctx, &domain.EmailInboxCursor{
			ChannelID:   cursor.ChannelID,
			AccountID:   cursor.AccountID,
			Mailbox:     cursor.Mailbox,
			UIDValidity: result.UIDValidity,
			LastUID:     ingested,
		})
	}
}

// backoffElapsed reports whether a failing inbox has waited long enough.
//
// Linear in the failure count and capped, because the cause is almost always a
// credential the customer will fix by hand: an exponential curve would leave a
// corrected password unnoticed for hours.
func (p *EmailInboxPoller) backoffElapsed(cursor *domain.EmailInboxCursor, now time.Time) bool {
	if cursor.LastPolledAt == nil {
		return true
	}
	extra := time.Duration(cursor.FailureCount) * p.interval
	if max := 30 * time.Minute; extra > max {
		extra = max
	}
	return now.Sub(*cursor.LastPolledAt) >= extra
}

func (p *EmailInboxPoller) fail(ctx context.Context, cursor *domain.EmailInboxCursor, now time.Time, reason string) {
	if err := p.cursors.RecordFailure(ctx, cursor.ChannelID, cursor.Mailbox, now, reason); err != nil {
		slog.ErrorContext(ctx, "email inbox poller: could not record failure",
			"channel_id", cursor.ChannelID, "error", err)
	}
	slog.WarnContext(ctx, "email inbox poll failed",
		"channel_id", cursor.ChannelID, "mailbox", cursor.Mailbox, "reason", reason)
}

// inboxConfigFromChannel builds the connection from the channel's own config.
//
// Reuses the SMTP credentials, and defaults the host to the SMTP host: every
// provider that hosts a mailbox authenticates both protocols against one
// account, and on Locaweb the host is literally the same name. So connecting the
// inbox costs the customer no extra fields in the common case, and `imap_host` /
// `imap_port` exist only for the providers that split them.
func inboxConfigFromChannel(channel *domain.Channel, mailbox string) (domain.EmailInboxConfig, bool) {
	get := func(key string) string {
		return strings.TrimSpace(domain.ChannelConfigString(channel, key))
	}
	cfg := domain.EmailInboxConfig{
		Host:     get("imap_host"),
		Port:     get("imap_port"),
		Username: get("smtp_username"),
		Password: get("smtp_password"),
		Mailbox:  mailbox,
	}
	if domain.IsOAuthEmail(channel) {
		// Nothing was typed on this path, so nothing is read from config: the
		// endpoint is Google's and the account IS the address. The token is
		// filled in by the caller, which is the only place that can refresh it.
		cfg.Host = domain.GoogleIMAPHost
		cfg.Port = domain.GoogleIMAPPort
		cfg.Username = get("from_address")
		cfg.Password = ""
		return cfg, cfg.Username != ""
	}
	if cfg.Host == "" {
		cfg.Host = get("smtp_host")
	}
	if cfg.Username == "" {
		cfg.Username = get("from_address")
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		return cfg, false
	}
	return cfg, true
}
