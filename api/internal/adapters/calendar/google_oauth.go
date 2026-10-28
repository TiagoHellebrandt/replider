package calendar

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/replider/api/internal/core/domain"
)

// GoogleOAuth drives the per-account "Conectar Google Calendar" flow:
// authorization URL, code exchange, and API calls authenticated with
// the account's refresh token instead of the platform service account.
type GoogleOAuth struct {
	cfg *oauth2.Config
}

// NewGoogleOAuth returns nil when the OAuth client credentials are not
// configured — callers treat nil as "feature disabled" (503 to users).
func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &GoogleOAuth{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			calendar.CalendarEventsScope,
			"https://www.googleapis.com/auth/calendar.calendarlist.readonly",
			"openid",
			"email",
		},
	}}
}

// AuthCodeURL builds the consent-screen URL. access_type=offline +
// prompt=consent force Google to issue a refresh token even when the
// user already granted access before (re-connect flow).
func (g *GoogleOAuth) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// ExchangeCode trades the callback code for tokens. Returns the refresh
// token plus the Google account e-mail parsed from the id_token payload
// (best-effort, display-only — "" when absent).
func (g *GoogleOAuth) ExchangeCode(ctx context.Context, code string) (refreshToken, email string, err error) {
	tok, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("google oauth exchange failed: %w", err)
	}
	if tok.RefreshToken == "" {
		return "", "", fmt.Errorf("google oauth exchange returned no refresh token")
	}
	if idToken, ok := tok.Extra("id_token").(string); ok {
		email = emailFromIDToken(idToken)
	}
	return tok.RefreshToken, email, nil
}

// ListCalendars lists the calendars the connected Google account can
// see, for the dashboard's picker.
func (g *GoogleOAuth) ListCalendars(ctx context.Context, refreshToken string) ([]domain.GoogleUserCalendar, error) {
	srv, err := g.serviceForToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	list, err := srv.CalendarList.List().MinAccessRole("writer").Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list google calendars: %w", err)
	}
	out := make([]domain.GoogleUserCalendar, 0, len(list.Items))
	for _, item := range list.Items {
		out = append(out, domain.GoogleUserCalendar{
			ID:      item.Id,
			Summary: item.Summary,
			Primary: item.Primary,
		})
	}
	return out, nil
}

// serviceForToken builds a Calendar client from a stored refresh token.
// The oauth2 TokenSource mints/refreshes access tokens transparently.
func (g *GoogleOAuth) serviceForToken(ctx context.Context, refreshToken string) (*calendar.Service, error) {
	ts := g.cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	srv, err := calendar.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("unable to build google calendar client: %w", err)
	}
	return srv, nil
}

// emailFromIDToken pulls the email claim out of a Google id_token
// WITHOUT signature verification — the token just arrived first-hand
// from Google's token endpoint over TLS and the value is display-only,
// never used for authorization.
func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
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
	return claims.Email
}
