package usecases

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// Connecting the mailbox she already has. What matters here is not the OAuth
// dance — it is that the callback cannot be driven by anyone who did not start
// the flow, and that reconnecting does not leave her with two channels.

type fakeMailConnector struct {
	lastState    string
	refreshToken string
	email        string
	err          error
}

func (f *fakeMailConnector) AuthCodeURL(state string) string {
	f.lastState = state
	return "https://accounts.google.com/o/oauth2/auth?state=" + state
}

func (f *fakeMailConnector) Exchange(context.Context, string) (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	return f.refreshToken, f.email, nil
}

type fakeChannelStore struct {
	channels []*domain.Channel
	merged   map[string]map[string]string
	created  []*domain.Channel
}

func newFakeChannelStore(existing ...*domain.Channel) *fakeChannelStore {
	return &fakeChannelStore{channels: existing, merged: map[string]map[string]string{}}
}

func (f *fakeChannelStore) ListByAccountID(context.Context, string) ([]*domain.Channel, error) {
	return f.channels, nil
}

func (f *fakeChannelStore) Create(_ context.Context, c *domain.Channel) error {
	f.created = append(f.created, c)
	f.channels = append(f.channels, c)
	return nil
}

func (f *fakeChannelStore) MergeCredentials(_ context.Context, id string, cfg map[string]string) error {
	f.merged[id] = cfg
	return nil
}

func newConnectUC(conn GoogleMailConnector, store MailboxChannelStore) *ConnectGoogleMailboxUseCase {
	return NewConnectGoogleMailboxUseCase(conn, store, []byte("test-state-secret"))
}

func TestConnectGoogleMailbox_CreatesTheChannelFromWhatGoogleSaid(t *testing.T) {
	conn := &fakeMailConnector{refreshToken: "refresh-abc", email: "vendas@clinicaaurora.com"}
	store := newFakeChannelStore()
	uc := newConnectUC(conn, store)

	authURL, err := uc.AuthURL("acc-1")
	require.NoError(t, err)
	require.Contains(t, authURL, "state=")

	res, err := uc.HandleCallback(context.Background(), conn.lastState, "code-1")
	require.NoError(t, err)
	assert.False(t, res.Reconnected)

	require.Len(t, store.created, 1)
	created := store.created[0]
	assert.Equal(t, "email", created.Type)
	assert.Equal(t, "acc-1", created.AccountID)
	// The address comes from Google, not from anything typed: consenting with
	// the wrong account must produce a channel that says so.
	assert.Equal(t, "vendas@clinicaaurora.com",
		domain.ChannelConfigString(created, "from_address"))
	assert.Equal(t, string(domain.EmailAuthGoogleOAuth),
		domain.ChannelConfigString(created, domain.EmailAuthMethodKey))
	assert.Equal(t, "refresh-abc",
		domain.ChannelConfigString(created, domain.EmailOAuthRefreshTokenKey))
	// No password, no host, no port. That is the feature.
	assert.Empty(t, domain.ChannelConfigString(created, "smtp_password"))
	assert.Empty(t, domain.ChannelConfigString(created, "smtp_host"))
}

// A gmail.com address is the whole point: the SMTP path refuses it, and this
// one must not.
func TestConnectGoogleMailbox_AFreeProviderAddressIsAccepted(t *testing.T) {
	conn := &fakeMailConnector{refreshToken: "r", email: "clinica.aurora@gmail.com"}
	store := newFakeChannelStore()

	uc := newConnectUC(conn, store)
	_, err := uc.HandleCallback(
		context.Background(), mustState(t, uc, conn, "acc-1"), "code")
	require.NoError(t, err)
	require.Len(t, store.created, 1)
	assert.Equal(t, "clinica.aurora@gmail.com",
		domain.ChannelConfigString(store.created[0], "from_address"))
}

// Reconnecting the same address replaces the grant in place. A second channel
// would split her history and leave the first one polling with a dead token.
func TestConnectGoogleMailbox_ReconnectingUpdatesInsteadOfDuplicating(t *testing.T) {
	existing := &domain.Channel{
		ID: "chan-1", AccountID: "acc-1", Type: "email",
		Config: map[string]any{"from_address": "Vendas@ClinicaAurora.com"},
	}
	conn := &fakeMailConnector{refreshToken: "refresh-new", email: "vendas@clinicaaurora.com"}
	store := newFakeChannelStore(existing)
	uc := newConnectUC(conn, store)

	res, err := uc.HandleCallback(context.Background(), mustState(t, uc, conn, "acc-1"), "code")
	require.NoError(t, err)

	assert.True(t, res.Reconnected)
	assert.Empty(t, store.created, "a second channel was created for the same mailbox")
	// Case-insensitively matched: the address Google reports is lower-cased and
	// the stored one was typed by a person.
	require.Contains(t, store.merged, "chan-1")
	assert.Equal(t, "refresh-new",
		store.merged["chan-1"][domain.EmailOAuthRefreshTokenKey])
}

// The callback is an unauthenticated browser redirect. The signed state is the
// only thing tying it back to an account, so a forged one has to be worthless.
func TestConnectGoogleMailbox_ARefusedStateCreatesNothing(t *testing.T) {
	conn := &fakeMailConnector{refreshToken: "r", email: "a@b.com"}
	store := newFakeChannelStore()
	uc := newConnectUC(conn, store)

	for _, state := range []string{
		"",
		"not-base64!!",
		// Right shape, wrong signature: this is the forgery that matters.
		encodeState("acc-victim", time.Now().Add(time.Hour).Unix(), "deadbeef"),
	} {
		_, err := uc.HandleCallback(context.Background(), state, "code")
		assert.ErrorIs(t, err, ErrEmailOAuthInvalidState, "state %q was accepted", state)
	}
	assert.Empty(t, store.created)
}

// A consent screen left open all afternoon is not a valid callback.
func TestConnectGoogleMailbox_AnExpiredStateIsRefused(t *testing.T) {
	conn := &fakeMailConnector{refreshToken: "r", email: "a@b.com"}
	store := newFakeChannelStore()
	uc := newConnectUC(conn, store)

	uc.now = func() time.Time { return time.Now().Add(-time.Hour) }
	state, err := uc.AuthURL("acc-1")
	require.NoError(t, err)
	_ = state
	uc.now = time.Now

	_, err = uc.HandleCallback(context.Background(), conn.lastState, "code")
	assert.ErrorIs(t, err, ErrEmailOAuthInvalidState)
	assert.Empty(t, store.created)
}

// Google issues a refresh token only on first consent unless asked again. A
// connection without one works until the first access token expires and then
// stops, which is worse than not connecting.
func TestConnectGoogleMailbox_AGrantWithNoRefreshTokenIsRefused(t *testing.T) {
	conn := &fakeMailConnector{err: errors.New("google returned no refresh token")}
	store := newFakeChannelStore()
	uc := newConnectUC(conn, store)

	_, err := uc.HandleCallback(context.Background(), mustState(t, uc, conn, "acc-1"), "code")
	require.Error(t, err)
	assert.Empty(t, store.created)
}

// Without a Google client the feature is unavailable, not broken.
func TestConnectGoogleMailbox_UnconfiguredSaysSo(t *testing.T) {
	uc := newConnectUC(nil, newFakeChannelStore())

	_, err := uc.AuthURL("acc-1")
	assert.ErrorIs(t, err, domain.ErrEmailOAuthUnavailable)
	_, err = uc.HandleCallback(context.Background(), "x", "y")
	assert.ErrorIs(t, err, domain.ErrEmailOAuthUnavailable)
}

// mustState drives AuthURL to mint a valid state for the given account.
// mustState mints a state on the SAME use case that will receive the
// callback: the nonce inside it is single-use and lives in that instance.
func mustState(t *testing.T, uc *ConnectGoogleMailboxUseCase, conn *fakeMailConnector, accountID string) string {
	t.Helper()
	_, err := uc.AuthURL(accountID)
	require.NoError(t, err)
	return conn.lastState
}

// encodeState builds a state string with an arbitrary MAC, for the forgery case.
func encodeState(accountID string, exp int64, mac string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join([]string{
		accountID, strconv.FormatInt(exp, 10), mac,
	}, "|")))
}

// A registrar that cannot register the inbox must not stop the proof: the
// warmup stamp is what lets the mailbox send at all, and it used to be
// skipped whenever the reading half failed — "EMAIL_DOMAIN_NOT_VERIFIED" on
// an address Google had just vouched for.
type failingInboxRegistrar struct{ calls int }

func (f *failingInboxRegistrar) EnsureForChannel(context.Context, string, string, string) (*domain.EmailInboxCursor, error) {
	f.calls++
	return nil, errors.New("cursor table unavailable")
}

func TestConnectGoogleMailbox_RegistrarFailureStillStampsTheProof(t *testing.T) {
	conn := &fakeMailConnector{refreshToken: "refresh-abc", email: "dona@gmail.com"}
	store := newFakeChannelStore()
	uc := newConnectUC(conn, store)
	registrar := &failingInboxRegistrar{}
	uc.SetInboxRegistrar(registrar)
	mailboxes := &fakeMailboxRepo{mailbox: &domain.EmailMailbox{ID: "mb-1", Address: "dona@gmail.com"}}
	uc.SetMailboxRepo(mailboxes)

	_, err := uc.AuthURL("acc-1")
	require.NoError(t, err)
	res, err := uc.HandleCallback(context.Background(), conn.lastState, "code-1")
	require.NoError(t, err)
	require.NotEmpty(t, res.ChannelID)

	assert.Equal(t, 1, registrar.calls, "the inbox registration was attempted")
	require.NotNil(t, mailboxes.smtpProvenAt, "the OAuth grant must still count as the sending proof")
}
