package domain

import (
	"context"
	"strings"
)

// Connecting the mailbox she ALREADY has (pillar 1).
//
// Until now "connect your email" meant: find out your SMTP server, generate an
// app password, type a host, a port, a username and that password — and, if the
// address was @gmail.com, be refused outright, because a free mailbox cannot
// pass DMARC alignment for a domain the customer controls. That rule is correct
// for cold outbound FROM a domain we send on behalf of, and it made the product
// unusable for most small businesses, whose company email IS a Gmail account.
//
// Sending THROUGH the mailbox instead of on its behalf dissolves the problem:
// Google signs its own outbound, so alignment is Google's to satisfy, and the
// reply lands in her real inbox because it never left it. What it costs is an
// OAuth handshake instead of a password — which is also the part that makes the
// setup one click instead of five fields.
//
// The SMTP path is untouched and stays the answer for a company that sends from
// its own domain at volume.

// EmailAuthMethod is how a mail channel proves who it is.
type EmailAuthMethod string

const (
	// EmailAuthSMTP is host + username + password, the original path. The
	// zero value on purpose: every channel that exists today is one of these
	// and none of them carry the field.
	EmailAuthSMTP EmailAuthMethod = ""
	// EmailAuthGoogleOAuth is a Google account connected by consent. The
	// channel stores a refresh token, never a password.
	EmailAuthGoogleOAuth EmailAuthMethod = "google_oauth"
)

// EmailAuthMethodKey is the channel-config key. Not a credential, so it is
// deliberately NOT suffixed in a way that makes it sensitive — the UI needs to
// read it back to know which form to render.
const EmailAuthMethodKey = "auth_method"

// EmailOAuthRefreshTokenKey holds the long-lived grant.
//
// The "_token" suffix is load-bearing: SensitiveConfigKey matches on it, so the
// value is encrypted at rest by the channel repository and masked on every read
// without a single line of per-key handling. Renaming this to something that
// does not end in _token/_secret/_key would silently store the customer's
// mailbox grant in the clear.
const EmailOAuthRefreshTokenKey = "oauth_refresh_token" //nolint:gosec // G101: this is the config KEY name, not a credential — the value it addresses is what gets encrypted

// Google's submission and IMAP endpoints. Constants rather than user input:
// asking somebody to type "smtp.gmail.com" is the setup this whole path exists
// to remove, and a typo there is indistinguishable from a wrong password.
const (
	GoogleSMTPHost = "smtp.gmail.com"
	GoogleSMTPPort = "587"
	GoogleIMAPHost = "imap.gmail.com"
	GoogleIMAPPort = "993"
)

// EmailAuthMethodOf reports how this channel authenticates.
func EmailAuthMethodOf(channel *Channel) EmailAuthMethod {
	switch strings.TrimSpace(strings.ToLower(ChannelConfigString(channel, EmailAuthMethodKey))) {
	case string(EmailAuthGoogleOAuth):
		return EmailAuthGoogleOAuth
	default:
		return EmailAuthSMTP
	}
}

// IsOAuthEmail is the one question every call site asks.
func IsOAuthEmail(channel *Channel) bool {
	return EmailAuthMethodOf(channel) == EmailAuthGoogleOAuth
}

// EmailOAuthTokenSource turns a channel's stored grant into a short-lived
// access token.
//
// A port rather than a concrete client because the refresh has to happen in two
// places that must not diverge — the SMTP send and the IMAP poll — and because
// an expired grant needs to surface as one error the whole product recognises
// rather than as "authentication failed" from two protocols.
type EmailOAuthTokenSource interface {
	// AccessToken returns a token valid right now, refreshing if needed.
	AccessToken(ctx context.Context, channel *Channel) (string, error)
}

// ErrEmailOAuthNotConnected is what an unconfigured or revoked grant reports.
//
// Distinct from a wrong password on purpose: the fix is "connect it again",
// and a mailbox whose owner revoked our access will otherwise fail silently
// every five minutes forever.
var ErrEmailOAuthNotConnected = &AppError{
	Code:    "EMAIL_OAUTH_NOT_CONNECTED",
	Message: "a conexão com o Google expirou ou foi revogada. Conecte a conta de e-mail de novo",
}

// ErrEmailOAuthUnavailable is the platform not being configured for it —
// an operator problem, never the customer's.
var ErrEmailOAuthUnavailable = &AppError{
	Code:    "EMAIL_OAUTH_UNAVAILABLE",
	Message: "conexão com Google não está configurada nesta instalação",
}
