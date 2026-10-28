package usecases

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// GoogleCalendarOAuth is the narrow port the use case needs from the
// OAuth adapter (satisfied by calendar.GoogleOAuth). Kept as an
// interface so unit tests run without Google.
type GoogleCalendarOAuth interface {
	AuthCodeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (refreshToken, email string, err error)
	ListCalendars(ctx context.Context, refreshToken string) ([]domain.GoogleUserCalendar, error)
}

// GoogleCalendarIntegrationUseCase drives the per-account "Conectar
// Google Calendar" flow. The OAuth state is HMAC-signed (accountID +
// expiry) so the public callback can recover WHICH account initiated
// the flow without trusting anything client-supplied.
type GoogleCalendarIntegrationUseCase struct {
	repo        domain.GoogleCalendarIntegrationRepository
	oauth       GoogleCalendarOAuth // nil = feature not configured
	stateSecret []byte
	now         func() time.Time
	// nonces makes each signed state single-use (see oauthStateNonces).
	nonces *oauthStateNonces
}

func NewGoogleCalendarIntegrationUseCase(
	repo domain.GoogleCalendarIntegrationRepository,
	oauth GoogleCalendarOAuth,
	stateSecret string,
) *GoogleCalendarIntegrationUseCase {
	return &GoogleCalendarIntegrationUseCase{
		repo:        repo,
		oauth:       oauth,
		stateSecret: []byte(stateSecret),
		now:         time.Now,
		nonces:      newOAuthStateNonces(),
	}
}

// stateTTL bounds how long a consent screen can sit open before the
// callback is refused.
const stateTTL = 15 * time.Minute

// AuthURL returns the Google consent-screen URL for this account.
func (uc *GoogleCalendarIntegrationUseCase) AuthURL(accountID string) (string, error) {
	if uc.oauth == nil {
		return "", domain.ErrGCalNotConfigured
	}
	exp := uc.now().Add(stateTTL).Unix()
	return uc.oauth.AuthCodeURL(uc.signState(accountID, exp)), nil
}

// HandleCallback validates the state, exchanges the code and persists
// the integration. Returns the accountID so the handler can log it.
func (uc *GoogleCalendarIntegrationUseCase) HandleCallback(ctx context.Context, state, code string) (string, error) {
	if uc.oauth == nil {
		return "", domain.ErrGCalNotConfigured
	}
	accountID, err := uc.verifyState(state)
	if err != nil {
		return "", err
	}
	refreshToken, email, err := uc.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return accountID, fmt.Errorf("google calendar connect failed: %w", err)
	}
	if err := uc.repo.Save(ctx, &domain.GoogleCalendarIntegration{
		AccountID:    accountID,
		RefreshToken: refreshToken,
		GoogleEmail:  email,
	}); err != nil {
		return accountID, err
	}
	return accountID, nil
}

// IntegrationStatus is the handler-facing projection — no token.
type IntegrationStatus struct {
	Configured  bool       `json:"configured"`
	Connected   bool       `json:"connected"`
	GoogleEmail string     `json:"google_email,omitempty"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
}

func (uc *GoogleCalendarIntegrationUseCase) Status(ctx context.Context, accountID string) (*IntegrationStatus, error) {
	out := &IntegrationStatus{Configured: uc.oauth != nil}
	integ, err := uc.repo.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if integ != nil {
		out.Connected = true
		out.GoogleEmail = integ.GoogleEmail
		out.ConnectedAt = &integ.ConnectedAt
	}
	return out, nil
}

func (uc *GoogleCalendarIntegrationUseCase) Disconnect(ctx context.Context, accountID string) error {
	return uc.repo.Delete(ctx, accountID)
}

// ListCalendars lists the connected Google account's calendars where
// the user can write (the picker only offers bookable targets).
func (uc *GoogleCalendarIntegrationUseCase) ListCalendars(ctx context.Context, accountID string) ([]domain.GoogleUserCalendar, error) {
	if uc.oauth == nil {
		return nil, domain.ErrGCalNotConfigured
	}
	integ, err := uc.repo.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if integ == nil {
		return nil, domain.ErrGCalNotConnected
	}
	return uc.oauth.ListCalendars(ctx, integ.RefreshToken)
}

// signState produces base64url(accountID|nonce|exp|hex(hmac(accountID|nonce|exp))).
// The nonce is issued server-side and consumed by verifyState, so a state
// is accepted exactly once (2026-09-01 hardening).
func (uc *GoogleCalendarIntegrationUseCase) signState(accountID string, exp int64) string {
	nonce := uc.nonces.issue(time.Unix(exp, 0))
	payload := fmt.Sprintf("%s|%s|%d", accountID, nonce, exp)
	mac := hmac.New(sha256.New, uc.stateSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + hex.EncodeToString(mac.Sum(nil))))
}

func (uc *GoogleCalendarIntegrationUseCase) verifyState(state string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", domain.ErrGCalInvalidState
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return "", domain.ErrGCalInvalidState
	}
	accountID, nonce, expStr, gotMAC := parts[0], parts[1], parts[2], parts[3]
	payload := accountID + "|" + nonce + "|" + expStr
	mac := hmac.New(sha256.New, uc.stateSecret)
	mac.Write([]byte(payload))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(gotMAC)) {
		return "", domain.ErrGCalInvalidState
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || uc.now().Unix() > exp {
		return "", domain.ErrGCalInvalidState
	}
	// Single use: a replayed state is refused even inside its window.
	if !uc.nonces.consume(nonce) {
		return "", domain.ErrGCalInvalidState
	}
	return accountID, nil
}
