// Package captcha provides CAPTCHA token verification for abuse-sensitive
// public endpoints (e.g. signup). The concrete implementation talks to
// Cloudflare Turnstile's siteverify API, but the middleware that consumes it
// depends only on a small interface (middleware.CaptchaVerifier), so tests and
// alternative providers (hCaptcha, reCAPTCHA) can swap in without touching the
// HTTP layer.
package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// turnstileVerifyURL is Cloudflare's server-side token validation endpoint.
const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// TurnstileVerifier validates Cloudflare Turnstile response tokens against the
// siteverify API using the account's secret key.
type TurnstileVerifier struct {
	secret string
	client *http.Client
	// url is the siteverify endpoint, overridable in tests.
	url string
}

// NewTurnstileVerifier builds a verifier bound to the given secret. A nil
// client falls back to a sane 10s-timeout client so a slow Cloudflare never
// hangs a signup request past the handler deadline. Returns nil when secret is
// empty so callers can treat "no secret configured" as "captcha disabled".
func NewTurnstileVerifier(secret string, client *http.Client) *TurnstileVerifier {
	if strings.TrimSpace(secret) == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &TurnstileVerifier{secret: secret, client: client, url: turnstileVerifyURL}
}

// turnstileResponse is the subset of the siteverify JSON we care about.
type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify posts the token (and optional client IP) to Cloudflare and reports
// whether the challenge passed. A transport/decoding error is returned so the
// caller can distinguish "couldn't check" from "checked and failed" — the
// middleware fails closed on either, but logs them differently.
func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if v == nil {
		// No verifier configured → nothing to check. Treat as pass so a
		// missing secret never hard-blocks signup; the middleware is
		// responsible for not invoking a nil verifier in the first place.
		return true, nil
	}
	if strings.TrimSpace(token) == "" {
		return false, nil
	}

	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.url, strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("build turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("call turnstile siteverify: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("turnstile siteverify returned status %d", resp.StatusCode)
	}

	var out turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("decode turnstile response: %w", err)
	}

	if !out.Success {
		slog.Debug("turnstile verification failed", "error_codes", out.ErrorCodes)
	}
	return out.Success, nil
}
