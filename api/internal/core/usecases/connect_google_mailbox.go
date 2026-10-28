package usecases

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// "Conecte o e-mail que você já tem", in one click.
//
// The flow it replaces: find your provider's SMTP hostname, work out the port,
// generate an app password, type four fields — and be refused if the address is
// @gmail.com. What is left is a consent screen.
//
// The channel it produces is an ordinary email channel. Everything downstream —
// the send path, the inbox poller, the Sent-folder scan, the approval queue —
// reads it the same way it reads an SMTP one, because the only thing that
// changed is how the connection authenticates.

// GoogleMailConnector is the OAuth half, kept behind a port so the use case
// stays testable without a network.
type GoogleMailConnector interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (refreshToken, email string, err error)
}

// MailboxChannelStore is the narrow slice of the channel repository this needs.
//
// Narrow rather than the whole interface for the usual reason — connecting a
// mailbox must not be able to delete a channel or read another account's — and
// for a practical one: the full repository is thirteen methods wide, so a test
// double for it is mostly noise about WhatsApp.
type MailboxChannelStore interface {
	ListByAccountID(ctx context.Context, accountID string) ([]*domain.Channel, error)
	Create(ctx context.Context, channel *domain.Channel) error
	MergeCredentials(ctx context.Context, channelID string, config map[string]string) error
}

// mailStateTTL bounds how long a consent screen may sit open. Ten minutes is
// generous for a person clicking through Google and short enough that a state
// string leaking out of a browser history is worth nothing.
const mailStateTTL = 10 * time.Minute

type ConnectGoogleMailboxUseCase struct {
	oauth       GoogleMailConnector
	channels    MailboxChannelStore
	registrar   EmailInboxRegistrar
	mailboxes   domain.EmailMailboxRepository
	stateSecret []byte
	now         func() time.Time
	// nonces makes each signed state single-use (see oauthStateNonces).
	nonces *oauthStateNonces
	// funnelLinker binds the default funnel to the connected mailbox.
	// Optional; nil means the sender falls back to ActiveEmailChannel.
	funnelLinker *FunnelChannelLinker
}

// SetFunnelLinker wires the default-funnel ↔ email-channel link.
func (uc *ConnectGoogleMailboxUseCase) SetFunnelLinker(l *FunnelChannelLinker) {
	uc.funnelLinker = l
}

func NewConnectGoogleMailboxUseCase(
	oauth GoogleMailConnector,
	channels MailboxChannelStore,
	stateSecret []byte,
) *ConnectGoogleMailboxUseCase {
	return &ConnectGoogleMailboxUseCase{
		oauth:       oauth,
		channels:    channels,
		stateSecret: stateSecret,
		now:         time.Now,
		nonces:      newOAuthStateNonces(),
	}
}

// SetInboxRegistrar wires mailbox registration. Optional: without it the
// channel still sends, it just is not polled for replies.
func (uc *ConnectGoogleMailboxUseCase) SetInboxRegistrar(r EmailInboxRegistrar) {
	uc.registrar = r
}

// AuthURL starts the flow.
func (uc *ConnectGoogleMailboxUseCase) AuthURL(accountID string) (string, error) {
	if uc.oauth == nil {
		return "", domain.ErrEmailOAuthUnavailable
	}
	if strings.TrimSpace(accountID) == "" {
		return "", domain.ErrUnauthorized
	}
	exp := uc.now().Add(mailStateTTL).Unix()
	return uc.oauth.AuthCodeURL(uc.signState(accountID, exp)), nil
}

// ConnectResult is what the callback produced, for the log and the redirect.
type ConnectResult struct {
	AccountID string
	ChannelID string
	Email     string
	// Reconnected reports that an existing channel's grant was replaced rather
	// than a new channel created. The distinction matters to the person: one is
	// "connected", the other is "fixed".
	Reconnected bool
}

// HandleCallback validates the state, exchanges the code and persists the
// channel.
//
// The account comes from the SIGNED STATE, never from a session: this endpoint
// is reached as an unauthenticated browser redirect from Google, so the state
// is the only thing tying the consent back to who asked for it.
func (uc *ConnectGoogleMailboxUseCase) HandleCallback(
	ctx context.Context, state, code string,
) (*ConnectResult, error) {
	if uc.oauth == nil {
		return nil, domain.ErrEmailOAuthUnavailable
	}
	accountID, err := uc.verifyState(state)
	if err != nil {
		return nil, err
	}

	refreshToken, email, err := uc.oauth.Exchange(ctx, code)
	if err != nil {
		return &ConnectResult{AccountID: accountID},
			fmt.Errorf("google mailbox connect failed: %w", err)
	}

	out := &ConnectResult{AccountID: accountID, Email: email}

	// Reconnecting the SAME address updates the grant in place. Creating a
	// second channel would split the customer's history across two of them and
	// leave the old one polling with a dead token, which reads to them as the
	// mailbox having broken.
	existing, err := uc.findByAddress(ctx, accountID, email)
	if err != nil {
		return out, err
	}

	config := map[string]string{
		domain.EmailAuthMethodKey:        string(domain.EmailAuthGoogleOAuth),
		domain.EmailOAuthRefreshTokenKey: refreshToken,
		"from_address":                   email,
	}

	if existing != nil {
		out.ChannelID, out.Reconnected = existing.ID, true
		if err := uc.channels.MergeCredentials(ctx, existing.ID, config); err != nil {
			return out, err
		}
	} else {
		cfg := make(map[string]any, len(config))
		for k, v := range config {
			cfg[k] = v
		}
		channel := &domain.Channel{
			ID:           ulid.Make().String(),
			AccountID:    accountID,
			Name:         email,
			Type:         "email",
			Config:       cfg,
			IsActive:     true,
			WebhookToken: ulid.Make().String(),
		}
		if err := uc.channels.Create(ctx, channel); err != nil {
			return out, err
		}
		out.ChannelID = channel.ID
	}

	// The connected mailbox becomes the default funnel's door (durable link;
	// the sender also falls back at send time). Best-effort like the rest of
	// this tail: the consent is stored and the channel sends either way.
	if uc.funnelLinker != nil && out.ChannelID != "" {
		if err := uc.funnelLinker.LinkDefaultToChannel(ctx, accountID, out.ChannelID); err != nil {
			slog.WarnContext(ctx, "google mailbox connect: default funnel not linked to mailbox channel",
				"account_id", accountID, "channel_id", out.ChannelID, "error", err)
		}
	}

	// Registering the inbox is what makes replies arrive. Best-effort: the
	// grant is already stored and the channel already sends, and failing the
	// whole connection over the reading half would throw away a consent the
	// customer just gave. A WARN and on to the proof below — this used to
	// return here, which skipped MarkSMTPProven, which left the mailbox with
	// no warmup stamp, which the send gate reads as "domain not verified" on
	// an address Google itself just vouched for.
	if uc.registrar != nil && out.ChannelID != "" {
		if _, err := uc.registrar.EnsureForChannel(
			ctx, accountID, out.ChannelID, ""); err != nil {
			slog.WarnContext(ctx, "google mailbox connect: inbox not registered — replies will not be read until the channel is edited",
				"account_id", accountID, "channel_id", out.ChannelID, "error", err)
		}
	}

	// The OAuth grant IS the verification of this sending identity: Google
	// authenticated the owner and will sign its own outbound, which is the
	// same trust a successful SMTP test send establishes — so it stamps the
	// same proof, and the proof stamps warmup_started_at (fail-closed guard:
	// a mailbox with no warmup stamp cannot cold-send at all). Best-effort
	// like the registrar above, but the WARN is the trace when it fails —
	// a silent miss here leaves the customer unable to prospect.
	if uc.mailboxes != nil && out.ChannelID != "" {
		if _, err := uc.mailboxes.EnsureForChannel(ctx, accountID, out.ChannelID, email); err != nil {
			slog.WarnContext(ctx, "google mailbox connect: mailbox row not created — warmup not started",
				"account_id", accountID, "channel_id", out.ChannelID, "error", err)
		} else if err := uc.mailboxes.MarkSMTPProven(ctx, accountID, email, time.Now()); err != nil {
			slog.WarnContext(ctx, "google mailbox connect: proof/warmup stamp failed",
				"account_id", accountID, "channel_id", out.ChannelID, "error", err)
		}
	}
	return out, nil
}

// SetMailboxRepo wires the deliverability guard state. Optional setter, same
// seam the test-send use case uses.
func (uc *ConnectGoogleMailboxUseCase) SetMailboxRepo(r domain.EmailMailboxRepository) {
	uc.mailboxes = r
}

// findByAddress looks for an existing email channel on the same address.
func (uc *ConnectGoogleMailboxUseCase) findByAddress(
	ctx context.Context, accountID, email string,
) (*domain.Channel, error) {
	channels, err := uc.channels.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, ch := range channels {
		if ch == nil || ch.Type != "email" {
			continue
		}
		if strings.EqualFold(
			strings.TrimSpace(domain.ChannelConfigString(ch, "from_address")), email) {
			return ch, nil
		}
	}
	return nil, nil
}

// The state is HMAC-signed rather than stored: there is nothing to clean up,
// nothing to look up on the callback, and a forged one is not merely rejected
// but unconstructable without the secret. Same construction as the Calendar
// integration, deliberately — one shape to review, not two.
//
// Shape: base64url(accountID|nonce|exp|hex(hmac(accountID|nonce|exp))). The
// nonce is issued server-side and consumed by verifyState, so a state that
// was already used — or never issued by this process — is refused even
// though its MAC is right (2026-09-01 hardening).
func (uc *ConnectGoogleMailboxUseCase) signState(accountID string, exp int64) string {
	nonce := uc.nonces.issue(time.Unix(exp, 0))
	payload := fmt.Sprintf("%s|%s|%d", accountID, nonce, exp)
	mac := hmac.New(sha256.New, uc.stateSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(
		[]byte(payload + "|" + hex.EncodeToString(mac.Sum(nil))))
}

// ErrEmailOAuthInvalidState is a callback that cannot be trusted: forged,
// tampered with, or simply left open too long.
var ErrEmailOAuthInvalidState = &domain.AppError{
	Code:    "EMAIL_OAUTH_INVALID_STATE",
	Message: "a conexão expirou ou é inválida. Tente conectar de novo",
}

func (uc *ConnectGoogleMailboxUseCase) verifyState(state string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", ErrEmailOAuthInvalidState
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return "", ErrEmailOAuthInvalidState
	}
	accountID, nonce, expStr, gotMAC := parts[0], parts[1], parts[2], parts[3]
	mac := hmac.New(sha256.New, uc.stateSecret)
	mac.Write([]byte(accountID + "|" + nonce + "|" + expStr))
	// Constant-time: a byte-by-byte comparison here leaks how much of a forged
	// MAC was right.
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(gotMAC)) {
		return "", ErrEmailOAuthInvalidState
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || uc.now().Unix() > exp {
		return "", ErrEmailOAuthInvalidState
	}
	// Single use: the nonce is spent here, so replaying this state — even
	// inside its window — is refused.
	if !uc.nonces.consume(nonce) {
		return "", ErrEmailOAuthInvalidState
	}
	return accountID, nil
}
