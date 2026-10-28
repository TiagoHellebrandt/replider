package email

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Against the real in-process server, because the two things that matter here
// are the server's behaviour: which folder answers to \Sent, and that an
// ENVELOPE fetch does not touch \Seen.

// createSentFolder makes a folder and puts messages in it, returning the fetcher
// config pointed at it.
func createSentFolder(t *testing.T, f *IMAPFetcher, host, port, name string, recipients ...string) {
	t.Helper()
	c, err := f.dial(context.Background(), net.JoinHostPort(host, port), nil)
	require.NoError(t, err)
	defer func() { _ = c.Logout() }()
	require.NoError(t, c.Login("username", "password"))
	require.NoError(t, c.Create(name))

	for i, to := range recipients {
		raw := strings.Join([]string{
			"From: contato@clinica.com.br",
			"To: " + to,
			fmt.Sprintf("Subject: proposta %d", i),
			"Date: Mon, 01 Jun 2026 09:00:00 -0300",
			"Content-Type: text/plain; charset=utf-8",
			"",
			"segue nossa proposta",
		}, "\r\n")
		require.NoError(t, c.Append(name, nil, time.Now(), strings.NewReader(raw)))
	}
}

func TestScanSent_FindsTheFolderByItsName(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	// "INBOX.Sent" is the Dovecot shape, which is what Locaweb and most cPanel
	// hosts serve.
	createSentFolder(t, f, host, port, "INBOX.Sent",
		"ana@clinicaaurora.com.br", "compras@hospital.com.br")

	cfg := testConfig(host, port)
	cfg.Mailbox = "" // let it discover
	got, err := f.ScanSent(context.Background(), cfg, 0, 50)

	require.NoError(t, err)
	assert.Equal(t, "INBOX.Sent", got.Mailbox)
	require.Len(t, got.Recipients, 2)
	addresses := []string{got.Recipients[0].Address, got.Recipients[1].Address}
	assert.Contains(t, addresses, "ana@clinicaaurora.com.br")
	assert.Contains(t, addresses, "compras@hospital.com.br")
}

// A Portuguese mailbox is what a Brazilian customer actually has.
func TestScanSent_FindsAPortugueseFolderName(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	createSentFolder(t, f, host, port, "INBOX.Enviados", "ana@clinicaaurora.com.br")

	cfg := testConfig(host, port)
	cfg.Mailbox = ""
	got, err := f.ScanSent(context.Background(), cfg, 0, 50)

	require.NoError(t, err)
	assert.Equal(t, "INBOX.Enviados", got.Mailbox)
	assert.Len(t, got.Recipients, 1)
}

// Named rather than guessed: picking an arbitrary folder would import the wrong
// people, and "we could not find it" is an answer a customer can act on.
func TestScanSent_SaysSoWhenThereIsNoSentFolder(t *testing.T) {
	f, _, host, port := startIMAPServer(t)

	cfg := testConfig(host, port)
	cfg.Mailbox = ""
	_, err := f.ScanSent(context.Background(), cfg, 0, 50)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Sent")
}

// The date comes from the envelope, which is what ranks a candidate as a
// long-standing customer rather than a one-off.
func TestScanSent_CarriesTheSendDate(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	createSentFolder(t, f, host, port, "Sent", "ana@clinicaaurora.com.br")

	cfg := testConfig(host, port)
	cfg.Mailbox = "Sent"
	got, err := f.ScanSent(context.Background(), cfg, 0, 50)

	require.NoError(t, err)
	require.Len(t, got.Recipients, 1)
	assert.False(t, got.Recipients[0].SentAt.IsZero())
	assert.Equal(t, 2026, got.Recipients[0].SentAt.Year())
	// The subject travels too (issue #292): still envelope-only, and it is
	// what turns "37 messages" into "the relationship was about X".
	assert.Equal(t, "proposta 0", got.Recipients[0].Subject)
}

// Cc counts: a proposal copied to the buyer's boss means the boss is a contact
// too. Bcc deliberately does not — a blind copy is the one recipient she chose
// to keep private, and surfacing it would reveal something she hid on purpose.
func TestScanSent_ReadsToAndCcButNotBcc(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	c, err := f.dial(context.Background(), net.JoinHostPort(host, port), nil)
	require.NoError(t, err)
	require.NoError(t, c.Login("username", "password"))
	require.NoError(t, c.Create("Sent"))
	raw := strings.Join([]string{
		"From: contato@clinica.com.br",
		"To: ana@clinicaaurora.com.br",
		"Cc: chefe@clinicaaurora.com.br",
		"Bcc: sigiloso@interno.com.br",
		"Date: Mon, 01 Jun 2026 09:00:00 -0300",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"proposta",
	}, "\r\n")
	require.NoError(t, c.Append("Sent", nil, time.Now(), strings.NewReader(raw)))
	_ = c.Logout()

	cfg := testConfig(host, port)
	cfg.Mailbox = "Sent"
	got, err := f.ScanSent(context.Background(), cfg, 0, 50)

	require.NoError(t, err)
	var addresses []string
	for _, r := range got.Recipients {
		addresses = append(addresses, r.Address)
	}
	assert.Contains(t, addresses, "ana@clinicaaurora.com.br")
	assert.Contains(t, addresses, "chefe@clinicaaurora.com.br")
	assert.NotContains(t, addresses, "sigiloso@interno.com.br")
}

// The scan reads ENVELOPES, not bodies — so it cannot set \Seen either. Her Sent
// folder looks exactly as she left it.
func TestScanSent_LeavesTheFolderUntouched(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	createSentFolder(t, f, host, port, "Sent",
		"ana@clinicaaurora.com.br", "compras@hospital.com.br")

	cfg := testConfig(host, port)
	cfg.Mailbox = "Sent"
	_, err := f.ScanSent(context.Background(), cfg, 0, 50)
	require.NoError(t, err)

	c, err := f.dial(context.Background(), net.JoinHostPort(host, port), nil)
	require.NoError(t, err)
	defer func() { _ = c.Logout() }()
	require.NoError(t, c.Login("username", "password"))
	status, err := c.Select("Sent", true)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), status.Messages, "nothing removed")

	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.SeenFlag}
	seen, err := c.Search(criteria)
	require.NoError(t, err)
	assert.Empty(t, seen, "an envelope fetch must not mark anything read")
}

// A long history is walked NEWEST first and in pages, so an interrupted walk has
// still collected the correspondents that matter.
func TestScanSent_WalksNewestFirstInPages(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	var recipients []string
	for i := 0; i < 5; i++ {
		recipients = append(recipients, fmt.Sprintf("cliente%d@empresa.com.br", i))
	}
	createSentFolder(t, f, host, port, "Sent", recipients...)

	cfg := testConfig(host, port)
	cfg.Mailbox = "Sent"

	// The WALK is newest-first: the first page holds the two most recently sent
	// messages, whatever order the server chooses to hand them back in. That is
	// the guarantee that makes an interrupted scan useful.
	first, err := f.ScanSent(context.Background(), cfg, 0, 2)
	require.NoError(t, err)
	require.Len(t, first.Recipients, 2)
	assert.False(t, first.Done)
	assert.ElementsMatch(t,
		[]string{"cliente4@empresa.com.br", "cliente3@empresa.com.br"},
		addressesOf(first.Recipients))
	assert.NotZero(t, first.LowestScannedUID)

	second, err := f.ScanSent(context.Background(), cfg, first.LowestScannedUID, 2)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"cliente2@empresa.com.br", "cliente1@empresa.com.br"},
		addressesOf(second.Recipients))

	third, err := f.ScanSent(context.Background(), cfg, second.LowestScannedUID, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"cliente0@empresa.com.br"}, addressesOf(third.Recipients))
	assert.True(t, third.Done, "the bottom is reached and the caller is told")
}

func TestScanSent_AnEmptySentFolderIsNotAnError(t *testing.T) {
	f, _, host, port := startIMAPServer(t)
	createSentFolder(t, f, host, port, "Sent")

	cfg := testConfig(host, port)
	cfg.Mailbox = "Sent"
	got, err := f.ScanSent(context.Background(), cfg, 0, 50)

	require.NoError(t, err)
	assert.Empty(t, got.Recipients)
	assert.True(t, got.Done)
}

func TestScanSent_RefusesWithNoHost(t *testing.T) {
	f := NewIMAPFetcher()
	_, err := f.ScanSent(context.Background(), domain.EmailInboxConfig{}, 0, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func addressesOf(recipients []domain.SentRecipient) []string {
	out := make([]string, 0, len(recipients))
	for _, r := range recipients {
		out = append(out, r.Address)
	}
	return out
}
