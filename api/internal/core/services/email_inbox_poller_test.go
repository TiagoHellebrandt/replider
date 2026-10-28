package services

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The poller reads a mailbox a human is using at the same time. Everything below
// pins a rule that exists because getting it wrong loses a customer's reply or
// vandalises their inbox.

type fakeInboxCursorRepo struct {
	mu       sync.Mutex
	cursors  []*domain.EmailInboxCursor
	listErr  error
	advanced []cursorMove
	anchored []cursorMove
	failures []string
}

type cursorMove struct {
	channelID   string
	uidValidity uint32
	lastUID     uint32
}

func (f *fakeInboxCursorRepo) EnsureForChannel(context.Context, string, string, string) (*domain.EmailInboxCursor, error) {
	return nil, nil
}

func (f *fakeInboxCursorRepo) Advance(_ context.Context, channelID, _ string, uidValidity, lastUID uint32, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advanced = append(f.advanced, cursorMove{channelID, uidValidity, lastUID})
	return nil
}

func (f *fakeInboxCursorRepo) Reanchor(_ context.Context, channelID, _ string, uidValidity, lastUID uint32, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.anchored = append(f.anchored, cursorMove{channelID, uidValidity, lastUID})
	return nil
}

func (f *fakeInboxCursorRepo) RecordFailure(_ context.Context, _, _ string, _ time.Time, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, reason)
	return nil
}

func (f *fakeInboxCursorRepo) ListDue(context.Context, time.Time, int32) ([]*domain.EmailInboxCursor, error) {
	return f.cursors, f.listErr
}

type fakeInboxFetcher struct {
	mu sync.Mutex
	// results are returned in order, so a test can model "first page, then the
	// rest" for the backlog case.
	results []domain.EmailFetchResult
	err     error
	calls   []inboxFetchCall
	// inflight tracks concurrent sessions, which is the number providers cap.
	inflight, maxInflight int
	hold                  time.Duration
}

type inboxFetchCall struct {
	host, username, mailbox string
	knownValidity, afterUID uint32
}

func (f *fakeInboxFetcher) Fetch(_ context.Context, cfg domain.EmailInboxConfig, knownUIDValidity, afterUID uint32, _ int) (domain.EmailFetchResult, error) {
	f.mu.Lock()
	f.inflight++
	if f.inflight > f.maxInflight {
		f.maxInflight = f.inflight
	}
	f.calls = append(f.calls, inboxFetchCall{cfg.Host, cfg.Username, cfg.Mailbox, knownUIDValidity, afterUID})
	var out domain.EmailFetchResult
	if len(f.results) > 0 {
		out = f.results[0]
		f.results = f.results[1:]
	}
	err := f.err
	f.mu.Unlock()

	if f.hold > 0 {
		time.Sleep(f.hold)
	}

	f.mu.Lock()
	f.inflight--
	f.mu.Unlock()
	return out, err
}

type fakeInboxChannelResolver struct {
	channels map[string]*domain.Channel
	err      error
}

func (f *fakeInboxChannelResolver) GetByID(_ context.Context, id string) (*domain.Channel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.channels[id], nil
}

type fakeInboxIngestor struct {
	mu       sync.Mutex
	received [][]byte
	err      error
	failAt   int
}

func (f *fakeInboxIngestor) IngestFetchedEmail(_ context.Context, _ *domain.Channel, raw []byte) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAt > 0 && len(f.received) == f.failAt-1 {
		return false, errors.New("ingest exploded")
	}
	f.received = append(f.received, raw)
	return true, f.err
}

func inboxChannelFixture(t *testing.T, id string, cfg map[string]string) *domain.Channel {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	return &domain.Channel{
		ID: id, AccountID: "acc-1", Type: "email", IsActive: true,
		Config: json.RawMessage(raw),
	}
}

func newPollerFixture(t *testing.T, cursor *domain.EmailInboxCursor, results ...domain.EmailFetchResult) (
	*EmailInboxPoller, *fakeInboxCursorRepo, *fakeInboxFetcher, *fakeInboxIngestor,
) {
	t.Helper()
	cursors := &fakeInboxCursorRepo{cursors: []*domain.EmailInboxCursor{cursor}}
	fetcher := &fakeInboxFetcher{results: results}
	ingestor := &fakeInboxIngestor{}
	channels := &fakeInboxChannelResolver{channels: map[string]*domain.Channel{
		cursor.ChannelID: inboxChannelFixture(t, cursor.ChannelID, map[string]string{
			"from_address":  "contato@clinica.com.br",
			"smtp_host":     "email-ssl.com.br",
			"smtp_username": "contato@clinica.com.br",
			"smtp_password": "segredo",
		}),
	}}
	p := NewEmailInboxPoller(cursors, fetcher, channels, ingestor)
	p.now = func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) }
	return p, cursors, fetcher, ingestor
}

// A first connection anchors to the TOP of the mailbox. Reading everything would
// import years of history as brand-new conversations — hundreds of strangers
// arriving as leads nobody has answered.
func TestEmailPoller_AFirstConnectionAnchorsToTheTopInsteadOfImportingHistory(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX"},
		domain.EmailFetchResult{UIDValidity: 42, HighestUID: 9000},
	)

	p.RunOnce(context.Background())

	require.Len(t, cursors.anchored, 1)
	assert.Equal(t, cursorMove{"ch-1", 42, 9000}, cursors.anchored[0])
	assert.Empty(t, ingestor.received, "nothing historical is ingested")
}

// The mailbox's own credentials are reused, and the host defaults to the SMTP
// one — on Locaweb it is literally the same name, so connecting the inbox costs
// the customer no extra fields.
func TestEmailPoller_ReusesTheSendingCredentials(t *testing.T) {
	p, _, fetcher, _ := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{UIDValidity: 42, HighestUID: 100},
	)

	p.RunOnce(context.Background())

	require.Len(t, fetcher.calls, 1)
	assert.Equal(t, "email-ssl.com.br", fetcher.calls[0].host)
	assert.Equal(t, "contato@clinica.com.br", fetcher.calls[0].username)
	assert.Equal(t, "INBOX", fetcher.calls[0].mailbox)
	// The cursor travels down so the fetcher can ask only for what is above it.
	assert.Equal(t, uint32(42), fetcher.calls[0].knownValidity)
	assert.Equal(t, uint32(100), fetcher.calls[0].afterUID)
}

func TestEmailPoller_IngestsNewMailAndAdvancesPastIt(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{
			UIDValidity: 42, HighestUID: 102,
			Messages: []domain.FetchedEmail{
				{UID: 101, RawMessage: []byte("From: a@b.com\r\n\r\num")},
				{UID: 102, RawMessage: []byte("From: c@d.com\r\n\r\ndois")},
			},
		},
	)

	p.RunOnce(context.Background())

	assert.Len(t, ingestor.received, 2)
	require.Len(t, cursors.advanced, 1)
	assert.Equal(t, cursorMove{"ch-1", 42, 102}, cursors.advanced[0])
}

// A failure halfway through must leave the cursor at the last message that
// really landed. Advancing past the failure would silently lose it; not
// advancing at all would re-deliver the ones that worked.
func TestEmailPoller_AFailureMidBatchAdvancesOnlyPastWhatLanded(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{
			UIDValidity: 42, HighestUID: 103,
			Messages: []domain.FetchedEmail{
				{UID: 101, RawMessage: []byte("um")},
				{UID: 102, RawMessage: []byte("dois")},
				{UID: 103, RawMessage: []byte("tres")},
			},
		},
	)
	ingestor.failAt = 2 // the second message explodes

	p.RunOnce(context.Background())

	assert.Len(t, ingestor.received, 1)
	require.Len(t, cursors.advanced, 1)
	assert.Equal(t, uint32(101), cursors.advanced[0].lastUID)
}

// The server rebuilt the mailbox: the old UIDs address different messages now,
// so the cursor is garbage. Re-anchor rather than reading a range that would be
// arbitrary in either direction.
func TestEmailPoller_AChangedUIDValidityReanchorsInsteadOfGuessing(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{UIDValidity: 77, HighestUID: 5},
	)

	p.RunOnce(context.Background())

	require.Len(t, cursors.anchored, 1)
	assert.Equal(t, cursorMove{"ch-1", 77, 5}, cursors.anchored[0])
	assert.Empty(t, cursors.advanced)
	assert.Empty(t, ingestor.received)
}

// A backlog drains without waiting a full interval per page.
func TestEmailPoller_ComesStraightBackWhenMoreIsWaiting(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{
			UIDValidity: 42, HighestUID: 200, More: true,
			Messages: []domain.FetchedEmail{{UID: 101, RawMessage: []byte("um")}},
		},
		domain.EmailFetchResult{
			UIDValidity: 42, HighestUID: 200,
			Messages: []domain.FetchedEmail{{UID: 102, RawMessage: []byte("dois")}},
		},
	)

	p.RunOnce(context.Background())

	assert.Len(t, ingestor.received, 2, "the second page is read in the same run")
	require.Len(t, cursors.advanced, 2)
	assert.Equal(t, uint32(102), cursors.advanced[1].lastUID)
}

// A wrong password is recorded with the provider's own words, so "why did
// nothing arrive?" is answerable without reading logs.
func TestEmailPoller_RecordsWhyAPollFailed(t *testing.T) {
	p, cursors, fetcher, _ := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
	)
	fetcher.err = errors.New("imap login: 535 Authentication failed")

	p.RunOnce(context.Background())

	require.Len(t, cursors.failures, 1)
	assert.Contains(t, cursors.failures[0], "535 Authentication failed")
	assert.Empty(t, cursors.advanced)
}

// A channel with no credentials yet is a form half filled in, not an error the
// customer needs to see repeated every two minutes.
func TestEmailPoller_ReportsAnUnconfiguredInboxWithoutDialing(t *testing.T) {
	cursors := &fakeInboxCursorRepo{cursors: []*domain.EmailInboxCursor{{
		ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
	}}}
	fetcher := &fakeInboxFetcher{}
	channels := &fakeInboxChannelResolver{channels: map[string]*domain.Channel{
		"ch-1": inboxChannelFixture(t, "ch-1", map[string]string{"from_address": "a@b.com"}),
	}}
	p := NewEmailInboxPoller(cursors, fetcher, channels, &fakeInboxIngestor{})

	p.RunOnce(context.Background())

	assert.Empty(t, fetcher.calls, "no session is opened without credentials")
	require.Len(t, cursors.failures, 1)
}

// A paused channel stops being polled, and that is not a failure — recording one
// would fill the counter and trip the backoff for a channel nobody broke.
func TestEmailPoller_APausedChannelIsSkippedNotFailed(t *testing.T) {
	cursors := &fakeInboxCursorRepo{cursors: []*domain.EmailInboxCursor{{
		ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
		UIDValidity: 42, LastUID: 100,
	}}}
	fetcher := &fakeInboxFetcher{}
	ch := inboxChannelFixture(t, "ch-1", map[string]string{
		"smtp_host": "email-ssl.com.br", "smtp_username": "a@b.com", "smtp_password": "x",
	})
	ch.IsActive = false
	channels := &fakeInboxChannelResolver{channels: map[string]*domain.Channel{"ch-1": ch}}
	p := NewEmailInboxPoller(cursors, fetcher, channels, &fakeInboxIngestor{})

	p.RunOnce(context.Background())

	assert.Empty(t, fetcher.calls)
	assert.Empty(t, cursors.failures)
	assert.Len(t, cursors.advanced, 1, "the attempt is stamped so it stops being due")
}

// Providers cap concurrent IMAP sessions per account, and a poller that opens
// one per channel at once locks the customer out of their own webmail.
func TestEmailPoller_BoundsConcurrentSessions(t *testing.T) {
	var due []*domain.EmailInboxCursor
	channels := map[string]*domain.Channel{}
	for i := 0; i < 12; i++ {
		id := string(rune('a'+i)) + "-ch"
		due = append(due, &domain.EmailInboxCursor{
			ChannelID: id, AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 1,
		})
		channels[id] = inboxChannelFixture(t, id, map[string]string{
			"smtp_host": "email-ssl.com.br", "smtp_username": "a@b.com", "smtp_password": "x",
		})
	}
	fetcher := &fakeInboxFetcher{hold: 5 * time.Millisecond}
	p := NewEmailInboxPoller(&fakeInboxCursorRepo{cursors: due}, fetcher,
		&fakeInboxChannelResolver{channels: channels}, &fakeInboxIngestor{})

	p.RunOnce(context.Background())

	assert.Len(t, fetcher.calls, 12, "every inbox is still polled")
	assert.LessOrEqual(t, fetcher.maxInflight, p.concurrency)
}

// A repeatedly failing inbox backs off instead of attempting a login every
// interval forever — providers read that as an attack.
func TestEmailPoller_BacksOffAfterRepeatedFailures(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	justPolled := now.Add(-10 * time.Second)

	cursors := &fakeInboxCursorRepo{cursors: []*domain.EmailInboxCursor{{
		ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
		UIDValidity: 42, LastUID: 1,
		FailureCount: 5, LastPolledAt: &justPolled,
	}}}
	fetcher := &fakeInboxFetcher{}
	channels := &fakeInboxChannelResolver{channels: map[string]*domain.Channel{
		"ch-1": inboxChannelFixture(t, "ch-1", map[string]string{
			"smtp_host": "email-ssl.com.br", "smtp_username": "a@b.com", "smtp_password": "x",
		}),
	}}
	p := NewEmailInboxPoller(cursors, fetcher, channels, &fakeInboxIngestor{})
	p.now = func() time.Time { return now }

	p.RunOnce(context.Background())

	assert.Empty(t, fetcher.calls, "still inside the backoff window")

	// Long enough later, it tries again: the usual cause is a password the
	// customer fixes by hand, and they should not have to re-enable anything.
	long := now.Add(2 * time.Hour)
	p.now = func() time.Time { return long }
	p.RunOnce(context.Background())
	assert.Len(t, fetcher.calls, 1)
}

// An unwired poller says so instead of pretending to work.
func TestEmailPoller_UnwiredStartsNothing(t *testing.T) {
	p := NewEmailInboxPoller(nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Start(ctx) // returns immediately rather than blocking on a ticker
}

// A message that cannot be ingested is a poll failure and has to be recorded
// as one. Before this the loop only broke: no last_error, no failure count,
// no backoff — and the same UID was retried every two minutes forever, with
// every later reply in the mailbox stuck behind it and nothing in the panel
// to say why.
func TestEmailPoller_AnIngestFailureIsRecordedSoBackoffEngages(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{
			UIDValidity: 42, HighestUID: 101,
			// More is set on purpose: a failed page must NOT be re-fetched
			// straight away, or the poller spins on the same broken UID.
			More:     true,
			Messages: []domain.FetchedEmail{{UID: 101, RawMessage: []byte("um")}},
		},
	)
	ingestor.failAt = 1 // the very first message explodes

	p.RunOnce(context.Background())

	assert.Equal(t, []string{"ingest exploded"}, cursors.failures)
	// The cursor does not move past the message that failed.
	assert.Empty(t, cursors.advanced)
}

// Partial progress is kept AND the failure sticks: the advance (which resets
// the counter) happens first, the failure is stamped after it.
func TestEmailPoller_AFailureMidBatchKeepsProgressAndRecordsTheFailure(t *testing.T) {
	p, cursors, _, ingestor := newPollerFixture(t,
		&domain.EmailInboxCursor{
			ChannelID: "ch-1", AccountID: "acc-1", Mailbox: "INBOX",
			UIDValidity: 42, LastUID: 100,
		},
		domain.EmailFetchResult{
			UIDValidity: 42, HighestUID: 102,
			Messages: []domain.FetchedEmail{
				{UID: 101, RawMessage: []byte("um")},
				{UID: 102, RawMessage: []byte("dois")},
			},
		},
	)
	ingestor.failAt = 2

	p.RunOnce(context.Background())

	require.Len(t, cursors.advanced, 1)
	assert.Equal(t, uint32(101), cursors.advanced[0].lastUID)
	assert.Equal(t, []string{"ingest exploded"}, cursors.failures)
}
