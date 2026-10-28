package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/memory"
	imapclient "github.com/emersion/go-imap/client"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tested against a real IMAP server in-process rather than a hand-written fake.
// The rules that matter here — PEEK not touching \Seen, UID semantics, read-only
// SELECT — are the server's behaviour, and a fake would just assert that our
// mock agrees with our code.

// startIMAPServer boots an in-memory IMAP server and returns a fetcher pointed
// at it. The memory backend ships with one user (username/password) and one
// message in INBOX.
func startIMAPServer(t *testing.T) (*IMAPFetcher, *memory.Mailbox, string, string) {
	t.Helper()

	be := memory.New()
	srv := imapserver.New(be)
	srv.AllowInsecureAuth = true

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = listener.Close()
	})

	user, err := be.Login(nil, "username", "password")
	require.NoError(t, err)
	mbox, err := user.GetMailbox("INBOX")
	require.NoError(t, err)

	fetcher := NewIMAPFetcher()
	// The in-process server speaks plaintext; production always dials implicit
	// TLS (see dialTLS), which is why this seam exists only for tests.
	fetcher.dial = func(ctx context.Context, addr string, _ *tls.Config) (*imapclient.Client, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		return imapclient.New(conn)
	}

	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return fetcher, mbox.(*memory.Mailbox), host, port
}

func testConfig(host, port string) domain.EmailInboxConfig {
	return domain.EmailInboxConfig{
		Host: host, Port: port,
		Username: "username", Password: "password",
		Mailbox: "INBOX",
	}
}

func appendMessage(t *testing.T, mbox *memory.Mailbox, from, subject, body string) {
	t.Helper()
	raw := strings.Join([]string{
		"From: " + from,
		"Subject: " + subject,
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
	require.NoError(t, mbox.CreateMessage(nil, time.Now(), strings.NewReader(raw)))
}

// A first connection reports the top of the mailbox WITHOUT returning its
// contents, so the caller can anchor there instead of importing history.
func TestIMAPFetcher_AnchorsWithoutReadingHistory(t *testing.T) {
	f, mbox, host, port := startIMAPServer(t)
	appendMessage(t, mbox, "ana@clinica.com.br", "antiga", "história")

	// afterUID is the top, so nothing is above the cursor.
	first, err := f.Fetch(context.Background(), testConfig(host, port), 0, 0, 10)
	require.NoError(t, err)
	assert.NotZero(t, first.UIDValidity, "the caller needs a generation to anchor to")
	assert.NotZero(t, first.HighestUID)

	anchored, err := f.Fetch(context.Background(), testConfig(host, port),
		first.UIDValidity, first.HighestUID, 10)
	require.NoError(t, err)
	assert.Empty(t, anchored.Messages, "nothing above the anchor")
}

func TestIMAPFetcher_ReturnsOnlyMessagesAboveTheCursor(t *testing.T) {
	f, mbox, host, port := startIMAPServer(t)

	base, err := f.Fetch(context.Background(), testConfig(host, port), 0, 0, 10)
	require.NoError(t, err)
	anchor := base.HighestUID

	appendMessage(t, mbox, "ana@clinica.com.br", "Re: proposta", "pode mandar")

	got, err := f.Fetch(context.Background(), testConfig(host, port),
		base.UIDValidity, anchor, 10)

	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	assert.Greater(t, got.Messages[0].UID, anchor)
	assert.Contains(t, string(got.Messages[0].RawMessage), "pode mandar")
	// Ascending, so a partial ingest can advance to the last one handled.
	assert.Equal(t, got.Messages[0].UID, got.HighestUID)
}

// THE rule. Fetching a body without PEEK sets \Seen, and the salesperson's
// unread badge empties itself every time we poll — their inbox is a tool they
// use, and we are a guest in it.
func TestIMAPFetcher_DoesNotMarkMessagesAsRead(t *testing.T) {
	f, mbox, host, port := startIMAPServer(t)

	base, err := f.Fetch(context.Background(), testConfig(host, port), 0, 0, 10)
	require.NoError(t, err)

	appendMessage(t, mbox, "ana@clinica.com.br", "Re: proposta", "resposta nova")

	got, err := f.Fetch(context.Background(), testConfig(host, port),
		base.UIDValidity, base.HighestUID, 10)
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)

	// Read the flags back through the protocol, which is the only view that
	// matters — it is what the customer's own mail client sees.
	unseen := countUnseen(t, f, host, port)
	assert.Equal(t, 1, unseen, "the reply must still be unread for the human")
}

// countUnseen asks the server how many messages carry no \Seen flag.
func countUnseen(t *testing.T, f *IMAPFetcher, host, port string) int {
	t.Helper()
	ctx := context.Background()
	c, err := f.dial(ctx, net.JoinHostPort(host, port), nil)
	require.NoError(t, err)
	defer func() { _ = c.Logout() }()
	require.NoError(t, c.Login("username", "password"))
	_, err = c.Select("INBOX", true)
	require.NoError(t, err)

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Search(criteria)
	require.NoError(t, err)
	return len(ids)
}

// A rebuilt mailbox means the cursor's UIDs address different messages. The
// fetcher reports the new generation and reads nothing, leaving the decision
// with the caller rather than re-importing or silently skipping.
func TestIMAPFetcher_ReportsAChangedGenerationInsteadOfGuessing(t *testing.T) {
	f, mbox, host, port := startIMAPServer(t)
	appendMessage(t, mbox, "ana@clinica.com.br", "oi", "corpo")

	got, err := f.Fetch(context.Background(), testConfig(host, port),
		999999, /* a validity the server never issued */
		0, 10)

	require.NoError(t, err)
	assert.NotEqual(t, uint32(999999), got.UIDValidity)
	assert.Empty(t, got.Messages, "no reading on a cursor that means nothing")
	assert.NotZero(t, got.HighestUID, "but the caller still gets somewhere to anchor")
}

// A backlog is paged, and the caller is told there is more so it comes straight
// back instead of draining one page per interval.
func TestIMAPFetcher_PagesABacklogAndSaysThereIsMore(t *testing.T) {
	f, mbox, host, port := startIMAPServer(t)

	base, err := f.Fetch(context.Background(), testConfig(host, port), 0, 0, 10)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		appendMessage(t, mbox, "ana@clinica.com.br",
			fmt.Sprintf("assunto %d", i), fmt.Sprintf("corpo %d", i))
	}

	page, err := f.Fetch(context.Background(), testConfig(host, port),
		base.UIDValidity, base.HighestUID, 2)

	require.NoError(t, err)
	assert.Len(t, page.Messages, 2)
	assert.True(t, page.More)

	rest, err := f.Fetch(context.Background(), testConfig(host, port),
		base.UIDValidity, page.Messages[1].UID, 10)
	require.NoError(t, err)
	assert.Len(t, rest.Messages, 3)
	assert.False(t, rest.More)
}

// A wrong password surfaces as an error carrying the server's words, not as an
// empty mailbox — "nothing arrived" and "we could not log in" are different
// problems with different fixes.
func TestIMAPFetcher_ReportsBadCredentials(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	cfg := testConfig(host, port)
	cfg.Password = "errada"

	_, err := f.Fetch(context.Background(), cfg, 0, 0, 10)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "login")
}

func TestIMAPFetcher_RefusesToDialWithNoHost(t *testing.T) {
	f := NewIMAPFetcher()
	_, err := f.Fetch(context.Background(), domain.EmailInboxConfig{}, 0, 0, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

// An empty mailbox is a normal answer, not an error: a brand-new sending address
// has nothing in it yet.
func TestIMAPFetcher_AnEmptyMailboxIsNotAnError(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	createEmptyMailbox(t, f, host, port, "Vazia")
	cfg := testConfig(host, port)
	cfg.Mailbox = "Vazia"

	got, err := f.Fetch(context.Background(), cfg, 0, 0, 10)

	require.NoError(t, err)
	assert.Empty(t, got.Messages)
	assert.Zero(t, got.HighestUID)
}

// The port and folder defaults keep the customer's form at the fields they
// already filled in for sending.
func TestIMAPFetcher_DefaultsPortAndMailbox(t *testing.T) {
	assert.Equal(t, "993", DefaultIMAPPort)
	assert.Equal(t, "INBOX", DefaultMailbox)
}

// createEmptyMailbox makes a folder with nothing in it, which is what a
// brand-new sending address looks like.
func createEmptyMailbox(t *testing.T, f *IMAPFetcher, host, port, name string) {
	t.Helper()
	c, err := f.dial(context.Background(), net.JoinHostPort(host, port), nil)
	require.NoError(t, err)
	defer func() { _ = c.Logout() }()
	require.NoError(t, c.Login("username", "password"))
	require.NoError(t, c.Create(name))
}
