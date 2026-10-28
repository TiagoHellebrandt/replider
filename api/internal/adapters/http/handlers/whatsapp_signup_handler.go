package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/observability"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// applyWhatsAppRegistrationStatus stamps the outcome of a /register
// attempt onto the channel config map so the dashboard can surface
// "registro pendente / rate-limited / PIN bloqueado" CTAs without
// re-calling Meta on every page load.
func applyWhatsAppRegistrationStatus(cfg map[string]string, out waadapter.RegistrationOutcome) {
	cfg["registration_status"] = string(out.Status)
	cfg["registration_attempted_at"] = time.Now().UTC().Format(time.RFC3339)
	if out.Code != 0 {
		cfg["registration_error_code"] = strconv.Itoa(out.Code)
	} else {
		delete(cfg, "registration_error_code")
	}
	if out.Kind != "" {
		cfg["registration_error_kind"] = out.Kind
	} else {
		delete(cfg, "registration_error_kind")
	}
	if out.Message != "" {
		cfg["registration_error_message"] = out.Message
	} else {
		delete(cfg, "registration_error_message")
	}
}

// applyWhatsAppSubscribeStatus mirrors applyWhatsAppRegistrationStatus
// for the WABA subscribed_apps step. Stored separately because a phone
// can be CLOUD_API-registered yet not subscribed (or vice versa).
func applyWhatsAppSubscribeStatus(cfg map[string]string, err error) {
	if err == nil {
		cfg["subscribe_status"] = "subscribed"
		delete(cfg, "subscribe_error_message")
		return
	}
	cfg["subscribe_status"] = "failed"
	cfg["subscribe_error_message"] = err.Error()
}

// rememberLastWAPin stashes the PIN that just succeeded against
// /register so subsequent re-registers (token rotation, /deregister
// + /register, PIN reset flow) can default to the same value rather
// than blindly retrying "000000". Meta locks the phone after 3 wrong
// PIN attempts (133008), so each blind retry burns the customer's
// recovery budget. Stored in plaintext alongside access_token —
// equivalent sensitivity since both unlock Cloud API on the number.
func rememberLastWAPin(cfg map[string]string, pin string) {
	if pin == "" {
		return
	}
	cfg["last_pin"] = pin
}

// resolveWAPin picks the PIN to use for /register. Caller-supplied wins
// (admin typed it explicitly); otherwise the last successful PIN; only
// then "000000" as a last resort. Returning the source lets callers
// surface "using saved PIN" vs "trying default" in observability.
func resolveWAPin(supplied string, cfg map[string]string) (pin, source string) {
	if s := strings.TrimSpace(supplied); s != "" {
		return s, "supplied"
	}
	if last := cfg["last_pin"]; last != "" {
		return last, "remembered"
	}
	return "000000", "default"
}

// mapMetaError is the action-agnostic shim kept for legacy call sites
// that haven't migrated to mapMetaErrorAction yet. New handlers must
// pass the action so the catalog can disambiguate generic codes
// (#100 / Subcode=0 means very different things on /verify_code vs
// /create_template) and so a catalog miss surfaces in Sentry with the
// right context tag.
func mapMetaError(err error, fallbackCode, fallbackMsg string) (int, domain.AppError) {
	return mapMetaErrorAction(err, waadapter.ActionUnknown, fallbackCode, fallbackMsg)
}

// mapMetaErrorAction is the catalog-backed mapper. Flow:
//  1. Resolve the catalog entry for (err, action). On hit, use its
//     Kind + DefaultMessage as the canonical answer.
//  2. On miss, fall back to fallbackCode/fallbackMsg AND ship the raw
//     Meta envelope to Sentry so the team can add a catalog entry
//     before the next user trips on the same code.
//  3. Always emit a structured slog.Error/Warn with the full Meta
//     envelope so on-call can see the failure in plain logs without
//     having to open the request payload — the customer asked for
//     this explicitly: "às vezes se eu não abrir a req não consigo
//     ver". One log line per Meta failure, no exceptions.
//  4. HTTP status is derived from Meta's HTTPStatus + a small set of
//     known rate-limit codes — kept stable so the existing 401/403/422/
//     429 contract with the dashboard doesn't change.
func mapMetaErrorAction(err error, action waadapter.MetaAction, fallbackCode, fallbackMsg string) (int, domain.AppError) {
	var meta *waadapter.MetaError
	if errors.As(err, &meta) {
		_, entry, ok := waadapter.ResolveMetaError(err, action)
		code := fallbackCode
		detail := ""
		if ok {
			code = entry.Kind
			detail = entry.DefaultMessage
		}
		// Meta's localized user_msg almost always carries a more
		// specific reason than our catalog default ("A template with
		// this name already exists..." vs the generic "Erro de
		// template Meta."). Prefer it whenever present.
		if meta.UserMsg != "" {
			if meta.UserTitle != "" {
				detail = meta.UserTitle + ": " + meta.UserMsg
			} else {
				detail = meta.UserMsg
			}
		} else if detail == "" {
			detail = meta.Message
		}
		// Always log the canonical Meta envelope. catalog_hit=false
		// rows are the ones we should triage and add to the catalog;
		// they're searchable in logs by `catalog_hit:false action:*`.
		logMetaError(action, code, ok, meta)
		// Catalog miss: ship the raw envelope to Sentry so unknown
		// codes auto-surface instead of silently degrading to the
		// fallback. Matches the per-action context tag so issues are
		// triagable by endpoint.
		if !ok {
			observability.CaptureError(err, map[string]string{
				"component":    "meta_error_catalog",
				"action":       string(action),
				"meta_code":    strconv.Itoa(meta.Code),
				"meta_subcode": strconv.Itoa(meta.Subcode),
				"fb_trace_id":  meta.FBTraceID,
			})
		}
		// Bump the categorized Meta-error counter so dashboards can
		// alert on a sudden spike in TOKEN_EXPIRED / TEMPLATE_PAUSED
		// without grepping logs.
		observability.WhatsAppMetaErrors.WithLabelValues(code, strconv.Itoa(meta.Code)).Inc()
		status := http.StatusBadGateway
		switch {
		case meta.HTTPStatus == http.StatusUnauthorized || meta.Code == 190:
			status = http.StatusUnauthorized
		case meta.HTTPStatus == http.StatusForbidden:
			status = http.StatusForbidden
		case meta.HTTPStatus == http.StatusTooManyRequests,
			meta.Code == 133016, // Cloud API register rate limit
			meta.Code == 133008, // PIN locked (rate-limited variant)
			meta.Code == 131056, // pair rate limit
			meta.Code == 80007,  // app-level rate limit
			meta.Code == 130429: // Cloud API capacity throttle
			// 429 maps to "wait and retry" semantics — the UI can show
			// a cooldown countdown instead of a generic error toast.
			status = http.StatusTooManyRequests
		case meta.HTTPStatus == http.StatusBadRequest:
			// Meta 400 = client/validation error (invalid param, template format, etc.).
			// Return 422 so Cloudflare/proxies forward the response as-is (including CORS headers)
			// instead of substituting their own 502 error page.
			status = http.StatusUnprocessableEntity
		}
		// Surface every Meta-side field so the UI and support can pick
		// the most specific hint available. meta_details surfaces the
		// error_data.details payload that Meta uses for register/migration
		// failures (e.g. "Cannot Migrate Phone Number: missing payment
		// method") — without it the UI only sees the generic top-level
		// "Invalid parameter".
		md := map[string]any{
			"meta_code":       meta.Code,
			"meta_subcode":    meta.Subcode,
			"meta_http":       meta.HTTPStatus,
			"meta_user_title": meta.UserTitle,
			"meta_user_msg":   meta.UserMsg,
			"meta_details":    meta.Details,
			"fb_trace_id":     meta.FBTraceID,
		}
		return status, domain.AppError{Code: code, Message: detail, Metadata: md}
	}
	// Upstream failed but the error wasn't a recognized Meta envelope —
	// treat as a bad gateway so ops/observability can slice 4xx vs 5xx
	// independently of our own bugs. Still log structured so the line
	// shows up in the same `meta error` filter as the catalog ones.
	slog.Error("meta error (non-meta-envelope)",
		"action", string(action),
		"kind", fallbackCode,
		"catalog_hit", false,
		"transport_error", err.Error(),
	)
	return http.StatusBadGateway, domain.AppError{Code: fallbackCode, Message: fallbackMsg + ": " + err.Error()}
}

// logMetaError emits the canonical structured log line for every
// MetaError that mapMetaErrorAction handles. One log per failure with
// every field on-call needs to triage:
//
//   - action      — which Cloud API operation produced the error
//     (verify_code, register_phone, send_template, ...)
//   - kind        — the catalog Kind we resolved to (or fallback when
//     catalog_hit=false)
//   - catalog_hit — false rows are the ones to add to the catalog;
//     filterable in your log shipper as `catalog_hit:false`
//   - meta_code / meta_subcode / meta_http — Meta's numeric envelope
//   - meta_user_title / meta_user_msg — Meta's localized human copy
//   - fb_trace_id — paste this into Meta Business Manager support
//     when escalating, lets Meta look up the exact request server-side
//
// Severity is bumped to Warn for catalog hits (we know the cause) and
// Error for misses (we don't, surfaces in dashboards/alerts).
func logMetaError(action waadapter.MetaAction, kind string, catalogHit bool, meta *waadapter.MetaError) {
	attrs := []any{
		"action", string(action),
		"kind", kind,
		"catalog_hit", catalogHit,
		"meta_code", meta.Code,
		"meta_subcode", meta.Subcode,
		"meta_http", meta.HTTPStatus,
		"meta_user_title", meta.UserTitle,
		"meta_user_msg", meta.UserMsg,
		"meta_message", meta.Message,
		"meta_details", meta.Details,
		"fb_trace_id", meta.FBTraceID,
	}
	if catalogHit {
		slog.Warn("meta error", attrs...)
	} else {
		slog.Error("meta error (catalog miss — add to catalog)", attrs...)
	}
}

// WhatsAppSignupHandler handles the WhatsApp Embedded Signup flow.
// The frontend uses the Facebook JS SDK to launch signup, obtains an
// authorization code, and sends it here. We exchange it for an access
// token, register the phone number, subscribe to webhooks, and create
// a channel — all in one call.
type WhatsAppSignupHandler struct {
	createChannelUC *usecases.CreateChannelUseCase
	channelRepo     domain.ChannelRepository
	waAdapter       *waadapter.WhatsAppAdapter
	appID           string
	appSecret       string
	appBaseURL      string
}

func NewWhatsAppSignupHandler(
	createChannelUC *usecases.CreateChannelUseCase,
	channelRepo domain.ChannelRepository,
	waAdapter *waadapter.WhatsAppAdapter,
	appID, appSecret, appBaseURL string,
) *WhatsAppSignupHandler {
	return &WhatsAppSignupHandler{
		createChannelUC: createChannelUC,
		channelRepo:     channelRepo,
		waAdapter:       waAdapter,
		appID:           appID,
		appSecret:       appSecret,
		appBaseURL:      appBaseURL,
	}
}

type whatsappSignupRequest struct {
	Code          string  `json:"code" binding:"required"`            // Authorization code from FB.login()
	PhoneNumberID string  `json:"phone_number_id" binding:"required"` // From session info listener
	WABAID        string  `json:"waba_id" binding:"required"`         // From session info listener
	Pin           string  `json:"pin"`                                // Optional 6-digit 2FA PIN
	AgentID       *string `json:"agent_id"`                           // Optional agent assignment
	Name          string  `json:"name"`                               // Channel display name
}

// HandleWhatsAppSignup godoc
// @Summary      Complete WhatsApp Embedded Signup
// @Description  Exchanges the authorization code from Meta's Embedded Signup for an access
//
//	token, registers the phone number for Cloud API, subscribes the WABA to
//	webhooks, and creates a WhatsApp channel — all in one request.
//
// @Tags         channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      whatsappSignupRequest  true  "Signup data from frontend"
// @Success      201  {object}  domain.Channel
// @Failure      400  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /channels/whatsapp/signup [post]
func (h *WhatsAppSignupHandler) HandleWhatsAppSignup(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req whatsappSignupRequest
	if !bindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	// Step 1: Exchange authorization code for access token
	accessToken, err := waadapter.ExchangeCodeForToken(ctx, h.appID, h.appSecret, req.Code)
	if err != nil {
		slog.Error("WhatsApp token exchange failed", "error", err)
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "WA_TOKEN_EXCHANGE_FAILED",
			Message: "Failed to exchange authorization code. Please try again.",
		})
		return
	}

	// Step 1.5: Verify the (waba_id, phone_number_id) pair the frontend
	// submitted is consistent. A malicious or buggy client could submit
	// phone_number_id from one account and waba_id from another; without
	// this check we'd persist mismatched IDs and the webhook router
	// (which dispatches template/quality events by WABA) would deliver
	// to the wrong tenant. ListPhoneNumbers is scoped by the freshly
	// exchanged token so this also catches the "wrong WABA in token"
	// case at signup time rather than at first inbound.
	phones, err := waadapter.ListPhoneNumbers(ctx, req.WABAID, accessToken)
	if err != nil {
		slog.Error("WhatsApp signup: failed to list WABA phone numbers",
			"waba_id", req.WABAID, "error", err)
		c.JSON(http.StatusBadGateway, domain.AppError{
			Code:    "WA_WABA_LOOKUP_FAILED",
			Message: "Não foi possível confirmar o WABA. Tente novamente.",
		})
		return
	}
	matched := false
	for _, p := range phones {
		if p.ID == req.PhoneNumberID {
			matched = true
			break
		}
	}
	if !matched {
		slog.Warn("WhatsApp signup: phone_number_id not under waba_id",
			"waba_id", req.WABAID, "phone_number_id", req.PhoneNumberID)
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "WA_PHONE_WABA_MISMATCH",
			Message: "O número informado não pertence à conta WhatsApp Business selecionada.",
		})
		return
	}

	// Step 2: Register phone number for Cloud API messaging.
	//
	// Failure here is non-fatal — we still create/update the channel so
	// the customer doesn't lose their work — but we MUST persist the
	// outcome so the dashboard can show "registro pendente" and offer
	// a retry CTA. Silently swallowing the error (the previous behavior)
	// left customers with channels that looked healthy but never received
	// inbound messages because Meta wasn't routing webhooks to us.
	//
	// PIN resolution prefers a saved last_pin from a prior successful
	// register so re-signups (token rotation, popup re-runs) don't burn
	// retries against the wrong PIN — Meta locks the phone after 3
	// wrong tries (133008).
	existingCfg := map[string]string{}
	if existing, lookupErr := h.channelRepo.GetByPhoneNumberID(ctx, req.PhoneNumberID); lookupErr == nil && existing != nil {
		_ = json.Unmarshal(rawConfigBytes(existing.Config), &existingCfg)
	}
	pin, _ := resolveWAPin(req.Pin, existingCfg)
	registerErr := waadapter.RegisterPhoneNumber(ctx, req.PhoneNumberID, accessToken, pin)
	registerOutcome := waadapter.ClassifyRegisterError(registerErr)
	if registerErr != nil {
		// Route through the canonical Meta logger so the full envelope
		// (subcode, user_msg, error_data.details, fb_trace_id) ends up
		// in the same `meta error` filter as every other Meta failure.
		// Without this, Code=100 register failures hid the real cause
		// (missing payment method, unverified business) inside the raw
		// error string and forced operators to curl Meta manually.
		var metaErr *waadapter.MetaError
		if errors.As(registerErr, &metaErr) {
			_, entry, hit := waadapter.ResolveMetaError(registerErr, waadapter.ActionRegisterPhone)
			kind := registerOutcome.Kind
			if hit && entry.Kind != "" {
				kind = entry.Kind
			}
			logMetaError(waadapter.ActionRegisterPhone, kind, hit, metaErr)
			if !hit {
				observability.CaptureError(registerErr, map[string]string{
					"component":       "whatsapp_signup_register",
					"phone_number_id": req.PhoneNumberID,
					"meta_code":       strconv.Itoa(metaErr.Code),
					"meta_subcode":    strconv.Itoa(metaErr.Subcode),
					"fb_trace_id":     metaErr.FBTraceID,
				})
			}
		} else {
			slog.Error("meta error (non-meta-envelope)",
				"action", string(waadapter.ActionRegisterPhone),
				"kind", registerOutcome.Kind,
				"catalog_hit", false,
				"phone_number_id", req.PhoneNumberID,
				"transport_error", registerErr.Error(),
			)
		}
	}

	// Step 3: Subscribe WABA to receive webhook events
	subscribeErr := waadapter.SubscribeApp(ctx, req.WABAID, accessToken)
	if subscribeErr != nil {
		slog.Warn("WhatsApp WABA subscription failed",
			"waba_id", req.WABAID, "error", subscribeErr)
	}

	// Step 4: Get display phone number for the channel name
	displayPhone, _ := waadapter.GetPhoneNumberDetails(ctx, req.PhoneNumberID, accessToken)

	channelName := req.Name
	if channelName == "" {
		channelName = "WhatsApp"
		if displayPhone != "" {
			channelName = "WhatsApp " + displayPhone
		}
	}

	// Step 5: Create or update the channel with WhatsApp config.
	//
	// Re-signup on a phone Replider already provisioned (token rotation,
	// extra phone added under same WABA, or user re-running the popup
	// after a network blip) must NOT create a duplicate channel — the
	// inbound webhook lookup is keyed on phone_number_id and would
	// arbitrarily pick one row, dropping messages on the others.
	//
	// We dedupe across the whole platform (the new partial unique index
	// enforces it at the DB level too). When a match exists we ensure
	// it belongs to the calling account, then rotate the access_token
	// and re-bind the agent so the user can fix a misassignment in one
	// click instead of deleting + recreating.
	config := map[string]string{
		"phone_number_id": req.PhoneNumberID,
		"waba_id":         req.WABAID,
		"access_token":    accessToken,
	}
	applyWhatsAppRegistrationStatus(config, registerOutcome)
	applyWhatsAppSubscribeStatus(config, subscribeErr)
	if registerErr == nil {
		rememberLastWAPin(config, pin)
	} else if last := existingCfg["last_pin"]; last != "" {
		// Carry the previously-remembered PIN forward so a failed
		// re-register doesn't erase the working PIN we already had.
		config["last_pin"] = last
	}
	configJSON, _ := json.Marshal(config)

	if existing, lookupErr := h.channelRepo.GetByPhoneNumberID(ctx, req.PhoneNumberID); lookupErr == nil && existing != nil {
		if existing.AccountID != accountID {
			c.JSON(http.StatusConflict, domain.AppError{
				Code:    "WA_PHONE_OWNED_ELSEWHERE",
				Message: "Este número já está conectado a outra conta Replider. Solicite a liberação para o suporte.",
			})
			return
		}
		existing.Config = configJSON
		if req.AgentID != nil {
			existing.AgentID = req.AgentID
		}
		if channelName != "" {
			existing.Name = channelName
		}
		if err := h.channelRepo.Update(ctx, existing); err != nil {
			respondError(c, err)
			return
		}
		slog.Info("WhatsApp channel re-signed (token rotated)",
			"channel_id", existing.ID,
			"account_id", accountID,
			"phone_number_id", req.PhoneNumberID)
		c.JSON(http.StatusOK, existing)
		return
	}

	ch, err := h.createChannelUC.Execute(ctx, usecases.CreateChannelDTO{
		AccountID:  accountID,
		AgentID:    req.AgentID,
		Type:       "whatsapp",
		Name:       channelName,
		Config:     configJSON,
		AppBaseURL: h.appBaseURL,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	slog.Info("WhatsApp channel created via Embedded Signup",
		"channel_id", ch.ID,
		"account_id", accountID,
		"phone_number_id", req.PhoneNumberID,
		"display_phone", displayPhone,
	)

	// Mask sensitive config before returning
	ch.IsFullyConfigured = isFullyConfigured(ch.Type, ch.Config)
	ch.Config = maskChannelConfig(ch.Type, ch.Config)
	c.JSON(http.StatusCreated, ch)
}

// HandleListTemplates godoc
// @Summary      List WhatsApp message templates
// @Description  Returns all approved message templates for the channel's WhatsApp Business Account.
// @Tags         channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {array}   waadapter.Template
// @Failure      404  {object}  domain.AppError
// @Router       /channels/{id}/templates [get]
func (h *WhatsAppSignupHandler) HandleListTemplates(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}

	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "templates are only available for WhatsApp channels"})
		return
	}

	// Extract waba_id from config
	var config map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &config)

	wabaID := config["waba_id"]
	if wabaID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_WABA_ID", Message: "channel has no WABA ID configured"})
		return
	}

	accessToken := config["access_token"]
	if accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_TOKEN", Message: "channel has no access token"})
		return
	}

	templates, err := waadapter.ListTemplates(c.Request.Context(), wabaID, accessToken)
	if err != nil {
		slog.Error("failed to list WhatsApp templates",
			"channel_id", channelID,
			"waba_id", wabaID,
			"error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionListTemplates,
			"TEMPLATE_LIST_FAILED", "failed to list templates")
		c.JSON(status, appErr)
		return
	}

	c.JSON(http.StatusOK, templates)
}

// HandleListPhoneNumbers godoc
// @Summary      List WhatsApp phone numbers for a channel's WABA
// @Description  Returns every phone_number entry registered under the
// @Description  channel's WhatsApp Business Account, including the
// @Description  currently connected one. Lets admins see sibling numbers
// @Description  they could connect as a second channel.
// @Tags         channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {array}   waadapter.PhoneNumber
// @Failure      400  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError
// @Router       /channels/{id}/phone-numbers [get]
func (h *WhatsAppSignupHandler) HandleListPhoneNumbers(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	cacheKey := "wa_phone_numbers:" + channelID
	if cached, ok := metaCacheGet(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "phone numbers are only available for WhatsApp channels"})
		return
	}

	var config map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &config)
	wabaID := config["waba_id"]
	if wabaID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_WABA_ID", Message: "channel has no WABA ID configured"})
		return
	}
	accessToken := config["access_token"]
	if accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_TOKEN", Message: "channel has no access token"})
		return
	}

	numbers, err := waadapter.ListPhoneNumbers(c.Request.Context(), wabaID, accessToken)
	if err != nil {
		slog.Error("failed to list WhatsApp phone numbers",
			"channel_id", channelID,
			"waba_id", wabaID,
			"error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionListPhoneNumbers,
			"PHONE_NUMBERS_LIST_FAILED", "failed to list phone numbers")
		c.JSON(status, appErr)
		return
	}
	metaCacheSet(cacheKey, numbers)
	c.JSON(http.StatusOK, numbers)
}

// HandleSendTemplate godoc
// @Summary      Send a WhatsApp template message
// @Description  Sends an approved template message to a contact via the specified channel.
// @Tags         channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Channel ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  domain.AppError
// @Router       /channels/{id}/templates/send [post]
func (h *WhatsAppSignupHandler) HandleSendTemplate(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	var req struct {
		To           string        `json:"to" binding:"required"`
		TemplateName string        `json:"template_name" binding:"required"`
		LanguageCode string        `json:"language_code" binding:"required"`
		Components   []interface{} `json:"components"`
	}
	if !bindJSON(c, &req) {
		return
	}

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}

	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "templates are only available for WhatsApp channels"})
		return
	}

	if err := h.waAdapter.SendTemplate(c.Request.Context(), ch, req.To, req.TemplateName, req.LanguageCode, req.Components); err != nil {
		slog.Error("failed to send WhatsApp template",
			"channel_id", channelID,
			"template", req.TemplateName,
			"error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionSendTemplate,
			"TEMPLATE_SEND_FAILED", "failed to send template")
		c.JSON(status, appErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "template sent"})
}
