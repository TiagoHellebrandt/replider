package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// TokenInfo carries the subset of /debug_token fields we use to decide
// whether a stored access_token still has runway. `IsValid=false` is
// the loud signal — Meta revoked the token (account ban, password
// change, app uninstall) and we must alert the tenant. `ExpiresAt`
// zero means the token never expires (System User permanent token);
// the caller skips refresh in that case.
type TokenInfo struct {
	IsValid   bool
	ExpiresAt time.Time
	Scopes    []string
	UserID    string
	AppID     string
}

// DebugToken introspects an access_token via Meta's /debug_token
// endpoint. The endpoint itself is authenticated with an app-level
// access_token (`{app_id}|{app_secret}`) since user-level tokens can't
// inspect themselves. Used by the refresh worker to decide whether to
// rotate before expiry.
func DebugToken(ctx context.Context, appID, appSecret, accessToken string) (*TokenInfo, error) {
	apiURL := fmt.Sprintf(
		"%s/%s/debug_token?input_token=%s&access_token=%s|%s",
		metaGraphBaseURL, metaAPIVersion, accessToken, appID, appSecret,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: build debug_token request: %w", err)
	}
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: debug_token call: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, ParseMetaError(resp.StatusCode, body)
	}
	var env struct {
		Data struct {
			AppID     string   `json:"app_id"`
			Type      string   `json:"type"`
			IsValid   bool     `json:"is_valid"`
			ExpiresAt int64    `json:"expires_at"`
			Scopes    []string `json:"scopes"`
			UserID    string   `json:"user_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("whatsapp: parse debug_token: %w", err)
	}
	info := &TokenInfo{
		IsValid: env.Data.IsValid,
		Scopes:  env.Data.Scopes,
		UserID:  env.Data.UserID,
		AppID:   env.Data.AppID,
	}
	if env.Data.ExpiresAt > 0 {
		info.ExpiresAt = time.Unix(env.Data.ExpiresAt, 0).UTC()
	}
	return info, nil
}

// ExchangeForLongLivedToken extends a soon-to-expire user access_token
// to a fresh 60d window. Same endpoint we use for the initial Embedded
// Signup code exchange, with grant_type=fb_exchange_token. Returns
// the rotated token; the caller persists it on the channel.
func ExchangeForLongLivedToken(ctx context.Context, appID, appSecret, currentToken string) (string, error) {
	apiURL := fmt.Sprintf(
		"%s/%s/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s",
		metaGraphBaseURL, metaAPIVersion, appID, appSecret, currentToken,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("whatsapp: build token-exchange request: %w", err)
	}
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp: token-exchange call: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", ParseMetaError(resp.StatusCode, body)
	}
	var env struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("whatsapp: parse token exchange: %w", err)
	}
	if env.AccessToken == "" {
		return "", fmt.Errorf("whatsapp: empty access_token in exchange response")
	}
	return env.AccessToken, nil
}

// ResumableUploadMedia uploads a binary asset (image / video /
// document) via Meta's Resumable Upload API and returns the media
// handle that template HEADER components require. Two-call flow:
//
//  1. POST /{app-id}/uploads?file_length=N&file_type=…  → upload_id
//  2. POST /{upload_id}  body: binary; header file_offset:0 → handle (h)
//
// The session init authenticates with the app access token
// (`{app_id}|{app_secret}`), not the user/system-user token, because
// the upload bucket is per-app, not per-WABA. The handle returned by
// step 2 is what callers pass as `header.example.header_handle` when
// creating a template.
//
// Meta caps inputs at 100 MB / 25 MB / 16 MB depending on type;
// caller is expected to enforce upstream so we don't burn bandwidth
// on a doomed upload.
func ResumableUploadMedia(
	ctx context.Context,
	appID, appSecret string,
	fileBytes []byte,
	mimeType string,
) (string, error) {
	if appID == "" || appSecret == "" {
		return "", fmt.Errorf("whatsapp: resumable upload needs app id + secret")
	}
	appToken := appID + "|" + appSecret

	// Step 1: open the upload session.
	initURL := fmt.Sprintf(
		"%s/%s/%s/uploads?file_length=%d&file_type=%s&access_token=%s",
		metaGraphBaseURL, metaAPIVersion, appID, len(fileBytes),
		url.QueryEscape(mimeType), url.QueryEscape(appToken),
	)
	initReq, err := http.NewRequestWithContext(ctx, http.MethodPost, initURL, nil)
	if err != nil {
		return "", fmt.Errorf("whatsapp: build upload init: %w", err)
	}
	initResp, err := metaHTTPClient.Do(initReq)
	if err != nil {
		return "", fmt.Errorf("whatsapp: upload init call: %w", err)
	}
	defer initResp.Body.Close()
	initBody, _ := io.ReadAll(initResp.Body)
	if initResp.StatusCode >= 400 {
		return "", ParseMetaError(initResp.StatusCode, initBody)
	}
	var initOut struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(initBody, &initOut); err != nil || initOut.ID == "" {
		return "", fmt.Errorf("whatsapp: parse upload init: %s", string(initBody))
	}

	// Step 2: stream the bytes. file_offset:0 because we never resume
	// partial uploads — Meta's session expires in 24h and the cost of
	// re-uploading a media file is small enough that resume is over-
	// engineering for the volumes Replider sees.
	uploadURL := fmt.Sprintf("%s/%s/%s",
		metaGraphBaseURL, metaAPIVersion, initOut.ID)
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(fileBytes))
	if err != nil {
		return "", fmt.Errorf("whatsapp: build upload chunk: %w", err)
	}
	upReq.Header.Set("Authorization", "OAuth "+appToken)
	upReq.Header.Set("file_offset", "0")
	upResp, err := metaHTTPClient.Do(upReq)
	if err != nil {
		return "", fmt.Errorf("whatsapp: upload chunk call: %w", err)
	}
	defer upResp.Body.Close()
	upBody, _ := io.ReadAll(upResp.Body)
	if upResp.StatusCode >= 400 {
		return "", ParseMetaError(upResp.StatusCode, upBody)
	}
	var upOut struct {
		Handle string `json:"h"`
	}
	if err := json.Unmarshal(upBody, &upOut); err != nil || upOut.Handle == "" {
		return "", fmt.Errorf("whatsapp: parse upload chunk: %s", string(upBody))
	}
	return upOut.Handle, nil
}

// EmbeddedSignupResult holds the data returned after a successful
// WhatsApp Embedded Signup token exchange.
type EmbeddedSignupResult struct {
	AccessToken        string `json:"access_token"`
	PhoneNumberID      string `json:"phone_number_id"`
	WABAID             string `json:"waba_id"`
	DisplayPhoneNumber string `json:"display_phone_number"`
}

// ExchangeCodeForToken exchanges a Facebook Login authorization code for
// a long-lived access token via the Meta Graph API.
func ExchangeCodeForToken(ctx context.Context, appID, appSecret, code string) (string, error) {
	apiURL := fmt.Sprintf(
		"https://graph.facebook.com/%s/oauth/access_token?client_id=%s&client_secret=%s&code=%s",
		metaAPIVersion, appID, appSecret, code,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("whatsapp: failed to create token exchange request: %w", err)
	}

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp: token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       *struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("whatsapp: failed to parse token exchange response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("whatsapp: token exchange failed: %s (code %d)", result.Error.Message, result.Error.Code)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("whatsapp: empty access token in response")
	}

	slog.Info("WhatsApp token exchanged successfully")
	return result.AccessToken, nil
}

// RegisterPhoneNumber registers the phone number for Cloud API messaging.
func RegisterPhoneNumber(ctx context.Context, phoneNumberID, accessToken, pin string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/register", metaGraphBaseURL, metaAPIVersion, phoneNumberID)

	payload, _ := json.Marshal(map[string]string{
		"messaging_product": "whatsapp",
		"pin":               pin,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("whatsapp: failed to create register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: register request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, body)
	}

	slog.Info("WhatsApp phone number registered", "phone_number_id", phoneNumberID)
	return nil
}

// SubscribeApp subscribes the WABA to receive webhook events. Idempotent
// on Meta's side: re-running on an already-subscribed WABA returns 200
// (sometimes with `{"success":true}`, sometimes with `{"data":[...]}`),
// so we treat any 2xx as success rather than parsing for `success: true`
// and surfacing a misleading error on the second call.
func SubscribeApp(ctx context.Context, wabaID, accessToken string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/subscribed_apps", metaGraphBaseURL, metaAPIVersion, wabaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return fmt.Errorf("whatsapp: failed to create subscribe request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: subscribe request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, body)
	}

	slog.Info("WhatsApp WABA subscribed to webhooks", "waba_id", wabaID)
	return nil
}

// SubscribedApp is one entry in the WABA's subscribed_apps list.
type SubscribedApp struct {
	ID   string `json:"id"`
	Name string `json:"whatsapp_business_api_data,omitempty"`
}

// ListSubscribedApps returns the apps currently subscribed to receive
// webhooks for a WABA. Used by the webhook-status endpoint so the
// Flutter UI can surface "✅ webhook ativo" vs "❌ não inscrito".
func ListSubscribedApps(ctx context.Context, wabaID, accessToken string) ([]SubscribedApp, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/subscribed_apps", metaAPIVersion, wabaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, ParseMetaError(resp.StatusCode, body)
	}
	var result struct {
		Data []struct {
			WhatsAppBusinessAPIData struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"whatsapp_business_api_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("whatsapp: failed to parse subscribed_apps: %w", err)
	}
	apps := make([]SubscribedApp, 0, len(result.Data))
	for _, d := range result.Data {
		apps = append(apps, SubscribedApp{
			ID:   d.WhatsAppBusinessAPIData.ID,
			Name: d.WhatsAppBusinessAPIData.Name,
		})
	}
	return apps, nil
}

// GetPhoneNumberDetails fetches the display phone number for a phone_number_id.
func GetPhoneNumberDetails(ctx context.Context, phoneNumberID, accessToken string) (string, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s?fields=display_phone_number,verified_name", metaAPIVersion, phoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		VerifiedName       string `json:"verified_name"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.DisplayPhoneNumber, nil
}

// PhoneNumberStatus is the subset of /{phone_number_id} fields needed
// to decide whether a number is ready to receive messages on OUR app.
// `PlatformType == "CLOUD_API"` means Meta is routing inbound webhooks
// to whoever called /register last; anything else (NOT_APPLICABLE,
// ON_PREMISES) means the number is either unregistered or still owned
// by a previous Cloud API app, and we need to run /register to take
// over routing.
//
// QualityRating + MessagingLimitTier expose Meta's spam-risk signals so
// the dashboard can warn the tenant before the number gets banned.
type PhoneNumberStatus struct {
	ID                     string `json:"id"`
	DisplayPhoneNumber     string `json:"display_phone_number"`
	VerifiedName           string `json:"verified_name"`
	CodeVerificationStatus string `json:"code_verification_status"`
	PlatformType           string `json:"platform_type"`
	WebhookApplication     string `json:"webhook_application,omitempty"`
	QualityRating          string `json:"quality_rating,omitempty"`       // GREEN / YELLOW / RED
	MessagingLimitTier     string `json:"messaging_limit_tier,omitempty"` // TIER_1K / TIER_10K / TIER_100K / TIER_UNLIMITED
	NameStatus             string `json:"name_status,omitempty"`          // APPROVED / PENDING_REVIEW / DECLINED ...
}

// GetPhoneNumberStatus fetches the registration/platform state of a
// phone number so the diagnostics UI can tell the difference between
// "ready to go" and "still registered elsewhere, needs migration", and
// can surface quality risks.
func GetPhoneNumberStatus(ctx context.Context, phoneNumberID, accessToken string) (*PhoneNumberStatus, error) {
	apiURL := fmt.Sprintf(
		"%s/%s/%s?fields=display_phone_number,verified_name,code_verification_status,platform_type,webhook_configuration,quality_rating,throughput,messaging_limit_tier,name_status",
		metaGraphBaseURL, metaAPIVersion, phoneNumberID,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, ParseMetaError(resp.StatusCode, body)
	}
	var raw struct {
		ID                     string `json:"id"`
		DisplayPhoneNumber     string `json:"display_phone_number"`
		VerifiedName           string `json:"verified_name"`
		CodeVerificationStatus string `json:"code_verification_status"`
		PlatformType           string `json:"platform_type"`
		WebhookConfiguration   struct {
			Application string `json:"application"`
		} `json:"webhook_configuration"`
		QualityRating      string `json:"quality_rating"`
		MessagingLimitTier string `json:"messaging_limit_tier"`
		NameStatus         string `json:"name_status"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("whatsapp: parse phone status: %w", err)
	}
	return &PhoneNumberStatus{
		ID:                     raw.ID,
		DisplayPhoneNumber:     raw.DisplayPhoneNumber,
		VerifiedName:           raw.VerifiedName,
		CodeVerificationStatus: raw.CodeVerificationStatus,
		PlatformType:           raw.PlatformType,
		WebhookApplication:     raw.WebhookConfiguration.Application,
		QualityRating:          raw.QualityRating,
		MessagingLimitTier:     raw.MessagingLimitTier,
		NameStatus:             raw.NameStatus,
	}, nil
}

// RequestVerificationCode asks Meta to send a fresh verification code
// to the phone number via SMS or a voice call. Used as the first step
// of a "forgot the 2FA PIN" flow — the number was registered to a
// previous Cloud API app that set a PIN we don't know, and this is
// the only way to take over without the legacy tenant's cooperation.
//
// codeMethod is either "SMS" or "VOICE". languageCode is a BCP-47 tag
// ("pt_BR", "en_US") that controls the code's delivery language.
func RequestVerificationCode(ctx context.Context, phoneNumberID, accessToken, codeMethod, languageCode string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/request_code", metaGraphBaseURL, metaAPIVersion, phoneNumberID)
	form := url.Values{}
	form.Set("code_method", codeMethod)
	if languageCode == "" {
		languageCode = "pt_BR"
	}
	form.Set("language", languageCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("whatsapp: build request_code: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: request_code call: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, body)
	}
	return nil
}

// VerifyCode confirms the code Meta just sent to the phone. On success
// the number becomes "verified" and the caller can run /register with
// a brand-new PIN — without needing the old one.
func VerifyCode(ctx context.Context, phoneNumberID, accessToken, code string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/verify_code", metaGraphBaseURL, metaAPIVersion, phoneNumberID)
	// Strip whitespace, dashes, dots — users frequently paste "754-223"
	// from the SMS text or "754 223" from autofill, and Meta's
	// /verify_code rejects anything that isn't pure digits with a
	// generic (#100) "Invalid parameter" that has no subcode and no
	// user_msg, so we'd never know that's the reason.
	digits := make([]byte, 0, 6)
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	code = string(digits)
	form := url.Values{}
	form.Set("code", code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("whatsapp: build verify_code: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: verify_code call: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, body)
	}
	return nil
}

// DeregisterPhoneNumber releases the phone from the current Cloud API
// owner. Used as the first step of a "migrate from another app" flow
// when the caller has the OLD app's access token. Meta treats /register
// on an already-owned number as an error; explicit /deregister is how
// the number becomes free to re-register.
func DeregisterPhoneNumber(ctx context.Context, phoneNumberID, accessToken string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/deregister", metaGraphBaseURL, metaAPIVersion, phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return fmt.Errorf("whatsapp: build deregister request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: deregister request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, body)
	}
	return nil
}

// PhoneNumber is one entry returned by ListPhoneNumbers. Mirrors the
// subset of the Meta Graph `phone_numbers` edge the UI needs — the full
// payload has ~15 fields but most are irrelevant to our flows.
type PhoneNumber struct {
	ID                     string `json:"id"`
	DisplayPhoneNumber     string `json:"display_phone_number"`
	VerifiedName           string `json:"verified_name"`
	QualityRating          string `json:"quality_rating,omitempty"`
	CodeVerificationStatus string `json:"code_verification_status,omitempty"`
	NameStatus             string `json:"name_status,omitempty"`
}

// ListPhoneNumbers returns all phone numbers registered under a WABA.
// The caller (channels handler) uses this to show admins which numbers
// live under their WhatsApp Business Account so they can connect a
// second channel or spot stale numbers.
func ListPhoneNumbers(ctx context.Context, wabaID, accessToken string) ([]PhoneNumber, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/phone_numbers?fields=id,display_phone_number,verified_name,quality_rating,code_verification_status,name_status&limit=50", metaAPIVersion, wabaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: failed to create phone numbers request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: phone numbers request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("whatsapp: phone numbers request failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	var result struct {
		Data []PhoneNumber `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("whatsapp: failed to parse phone numbers response: %w", err)
	}
	return result.Data, nil
}

// BusinessVerificationStatus mirrors the verification_status field
// Meta exposes on owned_business / owner_business_info edges. Values
// observed in practice: "verified", "not_verified", "pending",
// "expired", "rejected", "revoked". Unknown values pass through so
// the dashboard can branch defensively without a hard switch.
type BusinessVerificationStatus string

const (
	BusinessVerified    BusinessVerificationStatus = "verified"
	BusinessNotVerified BusinessVerificationStatus = "not_verified"
	BusinessPending     BusinessVerificationStatus = "pending"
)

// WABAOwnerInfo bundles the owner Business and its verification
// status. ID is needed so the dashboard can deep-link directly to
// the right Security Center page (instead of dumping the user at
// the BM root and forcing them to pick).
type WABAOwnerInfo struct {
	BusinessID         string                     `json:"business_id"`
	BusinessName       string                     `json:"business_name"`
	VerificationStatus BusinessVerificationStatus `json:"verification_status"`
}

// GetWABAOwnerInfo fetches the owner business + verification status
// for a WABA. Used to surface a "Verifique sua empresa" warning in
// the dashboard when status != verified — the BR chicken-and-egg
// (no payment method without verification, no register without
// payment method) is invisible until we expose this.
//
// Falls back to on_behalf_of_business_info when owner_business_info
// is empty (some WABAs created via shared partner flows surface only
// the BSP edge).
func GetWABAOwnerInfo(ctx context.Context, wabaID, accessToken string) (*WABAOwnerInfo, error) {
	apiURL := fmt.Sprintf("%s/%s/%s?fields=owner_business_info,on_behalf_of_business_info",
		metaGraphBaseURL, metaAPIVersion, wabaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: build owner-info request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: owner-info request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, ParseMetaError(resp.StatusCode, body)
	}

	var raw struct {
		OwnerBusinessInfo struct {
			ID                 string                     `json:"id"`
			Name               string                     `json:"name"`
			VerificationStatus BusinessVerificationStatus `json:"verification_status"`
		} `json:"owner_business_info"`
		OnBehalfOfBusinessInfo struct {
			ID                 string                     `json:"id"`
			Name               string                     `json:"name"`
			VerificationStatus BusinessVerificationStatus `json:"verification_status"`
		} `json:"on_behalf_of_business_info"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("whatsapp: parse owner-info: %w", err)
	}

	pick := raw.OwnerBusinessInfo
	if pick.ID == "" {
		pick.ID = raw.OnBehalfOfBusinessInfo.ID
		pick.Name = raw.OnBehalfOfBusinessInfo.Name
		pick.VerificationStatus = raw.OnBehalfOfBusinessInfo.VerificationStatus
	}
	if pick.ID == "" {
		return nil, fmt.Errorf("whatsapp: WABA %s has no owner business info", wabaID)
	}
	return &WABAOwnerInfo{
		BusinessID:         pick.ID,
		BusinessName:       pick.Name,
		VerificationStatus: pick.VerificationStatus,
	}, nil
}

// maxTemplateListPages bounds how many pages of templates we follow
// before stopping. WhatsApp Business Manager UI shows 50/page; even
// the heaviest enterprise tenants stay well below 1000 templates, so
// 20 pages × 100 = 2000 is a generous ceiling that prevents a runaway
// loop if Meta returns a self-referential pagination cursor.
const maxTemplateListPages = 20

// ListTemplates returns ALL approved message templates for a WABA,
// following Meta's `paging.next` until the dataset is exhausted (or
// the safety cap fires). Without pagination, tenants with more than
// 100 templates only saw the first page in the dashboard's send-as-
// template picker, which silently broke campaigns that targeted
// older templates by name.
func ListTemplates(ctx context.Context, wabaID, accessToken string) ([]Template, error) {
	apiURL := fmt.Sprintf("%s/%s/%s/message_templates?fields=id,name,status,language,category,components,rejected_reason&limit=100",
		metaGraphBaseURL, metaAPIVersion, wabaID)

	var all []Template
	for page := 0; page < maxTemplateListPages; page++ {
		body, err := getJSON(ctx, apiURL, accessToken)
		if err != nil {
			return nil, err
		}
		var result struct {
			Data   []Template `json:"data"`
			Paging struct {
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("whatsapp: failed to parse template list response: %w", err)
		}
		all = append(all, result.Data...)
		if result.Paging.Next == "" {
			return all, nil
		}
		apiURL = result.Paging.Next
	}
	return all, nil
}

// getJSON does an authenticated GET and returns the raw body, or a
// typed MetaError on 4xx/5xx. Shared between ListTemplates and any
// other Graph API endpoint that needs paginated GETs.
func getJSON(ctx context.Context, apiURL, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, ParseMetaError(resp.StatusCode, body)
	}
	return body, nil
}

// Template represents a WhatsApp message template.
// RejectedReason is populated only when Status == "REJECTED".
type Template struct {
	ID             string        `json:"id,omitempty"`
	Name           string        `json:"name"`
	Status         string        `json:"status"`
	Language       string        `json:"language"`
	Category       string        `json:"category"`
	Components     []interface{} `json:"components"`
	RejectedReason string        `json:"rejected_reason,omitempty"`
}

// SendTemplate sends a pre-approved template message via the Cloud API.
func (a *WhatsAppAdapter) SendTemplate(ctx context.Context, channel *domain.Channel, to string, templateName, languageCode string, components []interface{}) error {
	_, err := a.SendTemplateAndGetID(ctx, channel, to, templateName, languageCode, components)
	return err
}

// SendTemplateAndGetID is identical to SendTemplate but returns the WhatsApp
// message id (wamid.XXX) so the caller can correlate later status updates
// (delivered/read/failed) coming on the inbound webhook. Used by the
// broadcast/campaign worker.
func (a *WhatsAppAdapter) SendTemplateAndGetID(ctx context.Context, channel *domain.Channel, to string, templateName, languageCode string, components []interface{}) (string, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}
	accessToken, err := a.resolveAccessToken(cfg)
	if err != nil {
		return "", err
	}
	to = normalizeWAPhone(to)
	if to == "" {
		return "", fmt.Errorf("whatsapp: empty recipient phone after normalization")
	}
	// Anti-ban opt-out guard (US-025): abort BEFORE any Meta call when the
	// recipient has opted out — this low-level template path is what the
	// campaign/approach worker drives, the highest-risk cold-outbound surface.
	if err := a.ensureNotOptedOut(ctx, channel, to); err != nil {
		return "", err
	}
	if !a.pairLimiter.Allow(channel.ID, to) {
		return "", &MetaError{Code: 131056, Message: "pair rate limit (in-process limiter)"}
	}

	apiURL := fmt.Sprintf("%s/%s/%s/messages", metaGraphBaseURL, metaAPIVersion, cfg.PhoneNumberID)

	tmpl := map[string]interface{}{
		"name":     templateName,
		"language": map[string]string{"code": languageCode},
	}
	if len(components) > 0 {
		tmpl["components"] = components
	}
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "template",
		"template":          tmpl,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp: template send failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", ParseMetaError(resp.StatusCode, respBody)
	}

	wamid := extractFirstMessageID(respBody)
	if wamid == "" {
		return "", fmt.Errorf("whatsapp: empty messages array in response: %s", string(respBody))
	}
	slog.Info("WhatsApp template sent",
		"template", templateName,
		"language", languageCode,
		"phone_number_id", cfg.PhoneNumberID,
		"wamid", wamid)
	return wamid, nil
}

// MarkAsRead sends a read receipt for a specific message.
func (a *WhatsAppAdapter) MarkAsRead(ctx context.Context, channel *domain.Channel, messageID string) error {
	return a.sendReadStatus(ctx, channel, messageID, false)
}

// MarkAsReadAndTyping sends a read receipt AND a typing indicator in the
// same call (Meta coalesces them). The typing bubble stays visible for up
// to 25s or until the next outbound message is sent, whichever comes first.
// Use this before shipping an AI reply so the end-user sees the bot
// "composing" instead of a sudden instant response — the single biggest
// tell that they're talking to automation.
func (a *WhatsAppAdapter) MarkAsReadAndTyping(ctx context.Context, channel *domain.Channel, messageID string) error {
	return a.sendReadStatus(ctx, channel, messageID, true)
}

func (a *WhatsAppAdapter) sendReadStatus(ctx context.Context, channel *domain.Channel, messageID string, typing bool) error {
	cfg, err := parseConfig(channel)
	if err != nil {
		return err
	}
	accessToken, err := a.resolveAccessToken(cfg)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", metaAPIVersion, cfg.PhoneNumberID)

	body := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        messageID,
	}
	if typing {
		body["typing_indicator"] = map[string]string{"type": "text"}
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: mark as read failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Warn("WhatsApp mark as read failed", "status", resp.StatusCode, "body", string(respBody))
	}
	return nil
}
