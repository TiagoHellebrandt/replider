package email

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/replider/api/internal/core/domain"
)

// The Google side of "connect the mailbox you already have".
//
// Two jobs, and they are deliberately in one file: the consent handshake that
// obtains a refresh token, and the exchange that turns that token into the
// short-lived credential SMTP and IMAP actually authenticate with. Splitting
// them invites a second scope list, and a scope granted at consent but absent
// at use fails as "authentication failed" hours later.

// GmailScopes is the access this asks for, and it is the whole list.
//
// `https://mail.google.com/` is required rather than merely convenient: it is
// the ONLY scope Google honours for SMTP and IMAP XOAUTH2. The narrower
// gmail.send / gmail.readonly scopes work with the Gmail REST API and are
// rejected by the mail protocols, which is a mismatch that costs an app-review
// cycle to discover.
//
// It is a restricted scope, so a production app needs Google's verification.
// Until that lands the flow works for accounts listed as test users, which is
// the same position the Calendar integration is in.
var GmailScopes = []string{
	"https://mail.google.com/",
	"openid",
	"email",
}

// GoogleMailOAuth drives consent and mints access tokens.
type GoogleMailOAuth struct {
	cfg *oauth2.Config

	// cache holds live access tokens per channel. Without it every send and
	// every poll would spend a network round trip on a token that is valid for
	// an hour — and Google rate-limits token minting far more tightly than it
	// rate-limits mail.
	mu    sync.Mutex
	cache map[string]*oauth2.Token
}

// NewGoogleMailOAuth returns nil when the platform has no Google client
// configured. Callers treat nil as "feature unavailable" rather than falling
// back to something weaker.
func NewGoogleMailOAuth(clientID, clientSecret, redirectURL string) *GoogleMailOAuth {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return nil
	}
	return &GoogleMailOAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       GmailScopes,
		},
		cache: map[string]*oauth2.Token{},
	}
}

var _ domain.EmailOAuthTokenSource = (*GoogleMailOAuth)(nil)

// AuthCodeURL builds the consent URL.
//
// access_type=offline plus prompt=consent because Google issues a refresh token
// only on the FIRST consent unless asked again — so a customer reconnecting a
// mailbox after revoking access would otherwise complete the flow, receive no
// refresh token, and end up with a channel that works until the first access
// token expires and never again.
func (g *GoogleMailOAuth) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// Exchange completes the handshake.
//
// The email is read back from Google rather than taken from anything the user
// typed: it becomes the channel's from_address, so somebody who consents with a
// different account than they meant to gets a channel that says which one,
// instead of one that quietly sends as the wrong person.
func (g *GoogleMailOAuth) Exchange(
	ctx context.Context, code string,
) (refreshToken, email string, err error) {
	token, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		// Without it the connection lasts an hour. Refusing here is what stops
		// a channel being created that will start failing after lunch.
		return "", "", errors.New("google returned no refresh token")
	}
	// From the id_token rather than a userinfo request: the address is already
	// in the response we just received, and one fewer network call is one fewer
	// way for the last step of a consent flow to fail after consent was given.
	if idToken, ok := token.Extra("id_token").(string); ok {
		email = emailFromIDToken(idToken)
	}
	if email == "" {
		return "", "", errors.New("google returned no account address")
	}
	return token.RefreshToken, email, nil
}

// emailFromIDToken pulls the address out of the JWT payload without verifying
// the signature — safe here and only here, because the token came straight
// from Google's token endpoint over TLS in response to our own client secret,
// not from a caller. Never use this on a token that arrived from a client.
func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(claims.Email))
}

// AccessToken returns a token valid right now for this channel.
func (g *GoogleMailOAuth) AccessToken(ctx context.Context, channel *domain.Channel) (string, error) {
	if channel == nil {
		return "", domain.ErrEmailOAuthNotConnected
	}
	refresh := strings.TrimSpace(
		domain.ChannelConfigString(channel, domain.EmailOAuthRefreshTokenKey))
	if refresh == "" {
		return "", domain.ErrEmailOAuthNotConnected
	}

	if tok := g.cached(channel.ID); tok != "" {
		return tok, nil
	}

	token, err := g.cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		// A refresh that fails is a grant that is gone — revoked in the
		// customer's Google account, or expired because the app is still
		// unverified and Google caps those at seven days. Both are "connect it
		// again", and neither is a transport fault worth retrying silently.
		var retrieve *oauth2.RetrieveError
		if errors.As(err, &retrieve) {
			return "", domain.ErrEmailOAuthNotConnected
		}
		return "", err
	}
	g.store(channel.ID, token)
	return token.AccessToken, nil
}

// cached returns a token with time left on it, or "".
func (g *GoogleMailOAuth) cached(channelID string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	tok, ok := g.cache[channelID]
	if !ok || tok == nil {
		return ""
	}
	// A minute of headroom: a token that expires mid-SMTP-session fails the
	// send, and the clock we are comparing against is not Google's.
	if time.Until(tok.Expiry) < time.Minute {
		delete(g.cache, channelID)
		return ""
	}
	return tok.AccessToken
}

func (g *GoogleMailOAuth) store(channelID string, token *oauth2.Token) {
	if token == nil || token.AccessToken == "" || token.Expiry.IsZero() {
		// No expiry means no way to know when it stops working. Not cached:
		// minting one per send is wasteful, serving a dead one is worse.
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cache[channelID] = token
}

// Forget drops a channel's cached token. Called when a send or a poll fails
// with an auth error (the provider refused the token we hold), so the next
// attempt refreshes from the stored grant instead of retrying a token already
// known to be dead for the rest of its hour. A replaced grant takes effect
// the same way.
func (g *GoogleMailOAuth) Forget(channelID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.cache, channelID)
}
