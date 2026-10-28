package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	tgadapter "github.com/replider/api/internal/adapters/channels/telegram"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/replider/api/internal/core/usecases"
)

type ChannelHandler struct {
	createUC    *usecases.CreateChannelUseCase
	listUC      *usecases.ListChannelsUseCase
	updateUC    *usecases.UpdateChannelUseCase
	deleteUC    *usecases.DeleteChannelUseCase
	toggleUC    *usecases.ToggleChannelUseCase
	channelRepo domain.ChannelRepository
	appBaseURL  string
	// ngrokDetection lets resolveBaseURL probe the local ngrok API when
	// appBaseURL is not HTTPS. Local dev only; production never touches
	// localhost:4040.
	ngrokDetection bool
	tgAdapter      *tgadapter.TelegramAdapter
	// emailDNS is optional (SetEmailDNSVerifier) — see email_dns_handler.go.
	emailDNS *services.EmailDNSVerifier
	// dnsProvisioners keys one-click DNS writers by provider slug. Empty is
	// the normal state for a zone we cannot write to: the customer publishes
	// the records by hand, which is what the instructions are for.
	dnsProvisioners map[string]domain.DNSProvisioner
	// mailboxRepo is where a passing DNS verification lands (see
	// recordDNSVerified). Optional: unwired means the screen still reports the
	// truth about the records, it just cannot lift the send gate.
	mailboxRepo domain.EmailMailboxRepository
	// testSendUC proves an email channel's credentials. Optional — nil answers
	// 503 rather than nil-dereferencing mid-request.
	testSendUC *usecases.TestSendEmailChannelUseCase
	// bulkProvider is the platform's bulk email provider (hybrid sending).
	// Optional — nil/unconfigured answers BULK_SENDING_UNAVAILABLE (503).
	bulkProvider domain.BulkDomainProvider
	// inboxCursors exposes the IMAP poller's last outcome per email channel.
	// Optional: unwired means the list simply carries no imap_* fields.
	inboxCursors EmailInboxCursorReader
}

// SetEmailMailboxRepo wires the mailbox that records a passing verification.
func (h *ChannelHandler) SetEmailMailboxRepo(r domain.EmailMailboxRepository) { h.mailboxRepo = r }

// EmailInboxCursorReader is the one read the channel list needs over the
// poller's bookkeeping: satisfied by *postgres.EmailInboxCursorRepository.
// Narrow on purpose — the domain repository is write-side for the worker.
type EmailInboxCursorReader interface {
	GetByChannel(ctx context.Context, channelID string) (*domain.EmailInboxCursor, error)
}

// SetEmailInboxCursorReader wires the IMAP poll state onto email channel
// responses (imap_last_error / imap_failure_count / imap_last_polled_at).
func (h *ChannelHandler) SetEmailInboxCursorReader(r EmailInboxCursorReader) { h.inboxCursors = r }

// applyEmailInboxHealth attaches the poller's last outcome to an email
// channel's response. Best-effort like applyEmailWarmup: a read failure or
// an unregistered inbox leaves the fields absent.
func (h *ChannelHandler) applyEmailInboxHealth(c *gin.Context, ch *domain.Channel) {
	if h.inboxCursors == nil || ch == nil || ch.Type != "email" {
		return
	}
	cursor, err := h.inboxCursors.GetByChannel(c.Request.Context(), ch.ID)
	if err != nil || cursor == nil {
		return
	}
	ch.IMAPLastError = cursor.LastError
	ch.IMAPFailureCount = cursor.FailureCount
	ch.IMAPLastPolledAt = cursor.LastPolledAt
}

// SetEmailTestSendUseCase wires the credential probe.
func (h *ChannelHandler) SetEmailTestSendUseCase(uc *usecases.TestSendEmailChannelUseCase) {
	h.testSendUC = uc
}

func NewChannelHandler(
	createUC *usecases.CreateChannelUseCase,
	listUC *usecases.ListChannelsUseCase,
	updateUC *usecases.UpdateChannelUseCase,
	deleteUC *usecases.DeleteChannelUseCase,
	toggleUC *usecases.ToggleChannelUseCase,
	channelRepo domain.ChannelRepository,
	appBaseURL string,
	tgAdapter *tgadapter.TelegramAdapter,
) *ChannelHandler {
	return &ChannelHandler{
		createUC:    createUC,
		listUC:      listUC,
		updateUC:    updateUC,
		deleteUC:    deleteUC,
		toggleUC:    toggleUC,
		channelRepo: channelRepo,
		appBaseURL:  appBaseURL,
		tgAdapter:   tgAdapter,
	}
}

// verifyChannelOwnership checks that the channel belongs to the requesting account.
func (h *ChannelHandler) verifyChannelOwnership(c *gin.Context, ch *domain.Channel, accountID string) bool {
	if ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return false
	}
	return true
}

type createChannelRequest struct {
	AgentID *string     `json:"agent_id"` // optional — nil means human-only channel
	Type    string      `json:"type" binding:"required"`
	Name    string      `json:"name" binding:"required"`
	Config  interface{} `json:"config" swaggertype:"object"`
}

type updateChannelRequest struct {
	AgentID *string     `json:"agent_id"` // can be set to null to unassign agent
	Name    *string     `json:"name"`
	Config  interface{} `json:"config" swaggertype:"object"`
}

// HandleListChannels godoc
// @Summary      List Channels
// @Description  Returns all channels belonging to the authenticated account.
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.Channel
// @Failure      401  {object}  domain.AppError
// @Router       /channels [get]
func (h *ChannelHandler) HandleListChannels(c *gin.Context) {
	accountID := c.GetString("accountID")
	channels, err := h.listUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		slog.Error("failed to list channels", "account_id", accountID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}
	if channels == nil {
		channels = []*domain.Channel{}
	}
	for _, ch := range channels {
		decorateChannelResponse(ch)
		h.applyEmailWarmup(c, ch)
		h.applyEmailInboxHealth(c, ch)
	}
	c.JSON(http.StatusOK, channels)
}

// applyEmailWarmup attaches the mailbox's warmup/budget state to an email
// channel's response. Best-effort: a read failure leaves the field absent
// (the screen then shows nothing) — but the SEND gate reads the row itself,
// so a missing decoration can never loosen the guard.
func (h *ChannelHandler) applyEmailWarmup(c *gin.Context, ch *domain.Channel) {
	if h.mailboxRepo == nil || ch == nil || ch.Type != "email" {
		return
	}
	address := emailFromAddressOf(ch)
	if address == "" {
		return
	}
	mailbox, err := h.mailboxRepo.GetByAddress(c.Request.Context(), ch.AccountID, address)
	if err != nil || mailbox == nil {
		return
	}
	ch.EmailWarmup = domain.NewEmailWarmupStatus(mailbox, time.Now())
}

// HandleCreateChannel godoc
// @Summary      Create Channel
// @Description  Creates a new messaging channel. agent_id is optional — omit for human-only channels.
// @Tags         Channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      createChannelRequest  true  "Channel data"
// @Success      201  {object}  domain.Channel
// @Failure      400  {object}  domain.AppError
// @Failure      401  {object}  domain.AppError
// @Router       /channels [post]
func (h *ChannelHandler) HandleCreateChannel(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req createChannelRequest
	if !bindJSON(c, &req) {
		return
	}

	ch, err := h.createUC.Execute(c.Request.Context(), usecases.CreateChannelDTO{
		AccountID:  accountID,
		AgentID:    req.AgentID,
		Type:       req.Type,
		Name:       req.Name,
		Config:     marshalRaw(req.Config),
		AppBaseURL: h.appBaseURL,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	decorateChannelResponse(ch)
	c.JSON(http.StatusCreated, ch)
}

// HandleUpdateChannel godoc
// @Summary      Update Channel
// @Description  Updates a channel's name, config, or assigned agent. Set agent_id to null to unassign.
// @Tags         Channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string               true  "Channel ID"
// @Param        body  body      updateChannelRequest  true  "Update data"
// @Success      200  {object}  domain.Channel
// @Failure      400  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "code: CHANNEL_NOT_FOUND"
// @Router       /channels/{id} [put]
func (h *ChannelHandler) HandleUpdateChannel(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	var req updateChannelRequest
	if !bindJSON(c, &req) {
		return
	}

	ch, err := h.updateUC.Execute(c.Request.Context(), usecases.UpdateChannelDTO{
		AccountID: accountID,
		ChannelID: channelID,
		AgentID:   req.AgentID,
		Name:      req.Name,
		Config:    marshalRaw(req.Config),
	})
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			if appErr.Code == domain.ErrChannelNotFound.Code {
				c.JSON(http.StatusNotFound, appErr)
				return
			}
			c.JSON(http.StatusBadRequest, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}
	metaCacheInvalidate(channelID)
	decorateChannelResponse(ch)
	c.JSON(http.StatusOK, ch)
}

// HandleDeleteChannel godoc
// @Summary      Delete Channel
// @Description  Deletes a channel. Only the owning account can delete.
// @Tags         Channels
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  domain.AppError  "code: CHANNEL_NOT_FOUND"
// @Router       /channels/{id} [delete]
func (h *ChannelHandler) HandleDeleteChannel(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	metaCacheInvalidate(channelID)
	if err := h.deleteUC.Execute(c.Request.Context(), accountID, channelID); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			if appErr.Code == domain.ErrChannelNotFound.Code {
				c.JSON(http.StatusNotFound, appErr)
				return
			}
			c.JSON(http.StatusBadRequest, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "channel deleted"})
}

// HandleToggleChannel godoc
// @Summary      Toggle Channel
// @Description  Flips the is_active state of a channel. Only the owning account can toggle.
// @Tags         Channels
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  domain.Channel
// @Failure      404  {object}  domain.AppError  "code: CHANNEL_NOT_FOUND"
// @Router       /channels/{id}/toggle [patch]
func (h *ChannelHandler) HandleToggleChannel(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	metaCacheInvalidate(channelID)
	ch, err := h.toggleUC.Execute(c.Request.Context(), accountID, channelID)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			if appErr.Code == domain.ErrChannelNotFound.Code {
				c.JSON(http.StatusNotFound, appErr)
				return
			}
			c.JSON(http.StatusBadRequest, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}
	decorateChannelResponse(ch)
	c.JSON(http.StatusOK, ch)
}

// HandleTestChannel godoc
// @Summary      Test Channel Connection
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  domain.AppError
// @Router       /channels/{id}/test [post]
func (h *ChannelHandler) HandleTestChannel(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if !h.verifyChannelOwnership(c, ch, accountID) {
		return
	}

	switch ch.Type {
	case "telegram":
		configBytes := rawConfigBytes(ch.Config)
		var tgConfig struct {
			BotToken string `json:"bot_token"`
		}
		if err := json.Unmarshal(configBytes, &tgConfig); err != nil || tgConfig.BotToken == "" {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "VALIDATION_ERROR", Message: "bot_token not configured"})
			return
		}

		resp, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", tgConfig.BotToken))
		if err != nil {
			c.JSON(http.StatusBadGateway, domain.AppError{Code: "CONNECTION_ERROR", Message: "failed to reach Telegram API"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var tgResp struct {
			OK     bool `json:"ok"`
			Result struct {
				FirstName string `json:"first_name"`
				Username  string `json:"username"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &tgResp); err != nil || !tgResp.OK {
			c.JSON(http.StatusBadGateway, domain.AppError{Code: "CONNECTION_ERROR", Message: "invalid Telegram bot token"})
			return
		}

		botName := tgResp.Result.FirstName
		if tgResp.Result.Username != "" {
			botName = "@" + tgResp.Result.Username
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "bot_name": botName})

	case "whatsapp":
		// Real WhatsApp health check: hit the Graph API for the
		// phone number's display details. Succeeds only when the
		// access token is valid AND the phone_number_id is correct
		// AND the token has the whatsapp_business_messaging scope.
		// Matches what the Flutter "Testar Conexão" button needs to
		// show a green/red state during the Meta App Review demo.
		configBytes := rawConfigBytes(ch.Config)
		var waCfg struct {
			PhoneNumberID string `json:"phone_number_id"`
			WABAID        string `json:"waba_id"`
			AccessToken   string `json:"access_token"`
		}
		_ = json.Unmarshal(configBytes, &waCfg)
		// Use specific error codes so the Flutter WhatsAppError
		// humanizer can render the right pt-BR copy + reconnect CTA
		// (MISSING_TOKEN / MISSING_WABA_ID both hit isMissingCredentials
		// in whatsapp_error.dart). Older channels created before the
		// multi-field manual form / Embedded Signup landed will have
		// only phone_number_id — surface the missing piece by name.
		if waCfg.PhoneNumberID == "" {
			slog.Warn("WhatsApp channel missing phone_number_id",
				"channel_id", ch.ID,
				"has_waba", waCfg.WABAID != "",
				"has_token", waCfg.AccessToken != "")
			c.JSON(http.StatusBadRequest, domain.AppError{
				Code:    "MISSING_PHONE_NUMBER_ID",
				Message: "Canal sem phone_number_id. Reconecte via Meta Embedded Signup.",
			})
			return
		}
		if waCfg.AccessToken == "" {
			slog.Warn("WhatsApp channel missing access_token",
				"channel_id", ch.ID,
				"phone_number_id", waCfg.PhoneNumberID,
				"has_waba", waCfg.WABAID != "")
			c.JSON(http.StatusBadRequest, domain.AppError{
				Code:    "MISSING_TOKEN",
				Message: "Canal sem access token. Reconecte via Meta Embedded Signup.",
			})
			return
		}
		display, err := waadapter.GetPhoneNumberDetails(
			c.Request.Context(), waCfg.PhoneNumberID, waCfg.AccessToken)
		if err != nil {
			slog.Error("WhatsApp connection test failed",
				"channel_id", ch.ID,
				"phone_number_id", waCfg.PhoneNumberID,
				"error", err)
			status, appErr := mapMetaErrorAction(err, waadapter.ActionTestChannel,
				"CONNECTION_ERROR", "failed to reach Meta Graph API")
			c.JSON(status, appErr)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":              true,
			"display_number":  display,
			"phone_number_id": waCfg.PhoneNumberID,
		})

	case "email":
		// Until this branch existed, an email channel fell through to the
		// default below and was told "ok" — a connection test that reported
		// success without opening a single socket. The customer learned their
		// password was wrong when a real proposal failed to reach a real
		// prospect, which is the worst possible moment and the worst possible
		// messenger.
		//
		// The probe goes to the channel's own from_address, so testing is free
		// of consequences for anyone outside the account, and a success is also
		// the PROOF that unlocks cold sending with no DNS work.
		if h.testSendUC == nil {
			respondError(c, &domain.AppError{
				Code:    "EMAIL_TEST_SEND_UNAVAILABLE",
				Message: "envio de teste não configurado",
			})
			return
		}
		out, err := h.testSendUC.Execute(c.Request.Context(), accountID, ch.ID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":                 true,
			"sent_to":            out.SentTo,
			"cold_send_unlocked": out.ColdSendUnlocked,
			"daily_cap":          out.DailyCap,
		})

	default:
		// Still an unconditional yes for the types with nothing to dial
		// (webchat) and the ones whose probe was never written (sms,
		// instagram). Left alone rather than quietly changed: those buttons
		// have their own screens depending on this shape.
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// HandleWebhookStatus godoc
// @Summary      Telegram Webhook Status
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  domain.AppError
// @Router       /channels/{id}/webhook-status [get]
func (h *ChannelHandler) HandleWebhookStatus(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	cacheKey := "wa_webhook_status:" + channelID
	if cached, ok := metaCacheGet(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if !h.verifyChannelOwnership(c, ch, accountID) {
		return
	}
	switch ch.Type {
	case "telegram":
		info, err := h.tgAdapter.GetWebhookInfo(c.Request.Context(), ch)
		if err != nil {
			slog.Error("failed to get telegram webhook info", "channel_id", ch.ID, "error", err)
			c.JSON(http.StatusBadGateway, domain.AppError{Code: "TELEGRAM_ERROR", Message: "failed to get webhook info from Telegram"})
			return
		}

		baseURL := h.resolveBaseURL()
		expectedURL := baseURL + "/v1/webhooks/" + ch.WebhookToken
		// Stale errors: Telegram keeps last_error_message until a
		// successful delivery (which won't happen on a quiet bot).
		// If the reported error is older than 10 minutes, treat as
		// recovered — the fix is deployed and the badge shouldn't
		// stick around forever.
		errorIsStale := info.LastErrorDate > 0 &&
			time.Since(time.Unix(info.LastErrorDate, 0)) > 10*time.Minute
		isHealthy := info.URL == expectedURL && (info.LastErrorMessage == "" || errorIsStale)

		payload := gin.H{
			"channel_id":           ch.ID,
			"channel_name":         ch.Name,
			"registered_url":       info.URL,
			"expected_url":         expectedURL,
			"is_healthy":           isHealthy,
			"pending_update_count": info.PendingUpdateCount,
			"last_error_message":   info.LastErrorMessage,
			"last_error_date":      info.LastErrorDate,
		}
		metaCacheSet(cacheKey, payload)
		c.JSON(http.StatusOK, payload)
	case "whatsapp":
		// WhatsApp webhooks are global (Meta posts to a single URL
		// identified by phone_number_id). Health = the WABA's
		// subscribed_apps list contains our app. If the signup's
		// SubscribeApp call silently failed, this endpoint is how the
		// UI finds out.
		var cfg map[string]string
		_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
		wabaID := cfg["waba_id"]
		accessToken := cfg["access_token"]
		if wabaID == "" || accessToken == "" {
			slog.Warn("WhatsApp channel missing creds for webhook status",
				"channel_id", ch.ID,
				"has_waba", wabaID != "",
				"has_token", accessToken != "")
			c.JSON(http.StatusOK, gin.H{
				"channel_id":   ch.ID,
				"channel_name": ch.Name,
				"is_healthy":   false,
				"subscribed":   false,
				"reason":       "Canal sem waba_id ou access_token. Reconecte via Meta Embedded Signup.",
			})
			return
		}
		apps, err := waadapter.ListSubscribedApps(c.Request.Context(), wabaID, accessToken)
		if err != nil {
			slog.Error("failed to list WhatsApp subscribed apps",
				"channel_id", ch.ID,
				"waba_id", wabaID,
				"error", err)
			status, appErr := mapMetaErrorAction(err, waadapter.ActionListSubscribed,
				"WEBHOOK_STATUS_FAILED", "failed to fetch subscribed apps")
			c.JSON(status, appErr)
			return
		}
		// Fetch phone-level platform_type. Subscribed-apps can show
		// Replider as "inscrito" while the phone itself is still
		// platform_type=NOT_APPLICABLE, meaning a previous Cloud API
		// app (e.g. the customer's legacy integration) still owns the
		// /register slot and Meta will NOT route inbound to us. The UI
		// needs this flag to surface a "migrar número" CTA.
		phoneNumberID := cfg["phone_number_id"]
		var phoneStatus *waadapter.PhoneNumberStatus
		if phoneNumberID != "" {
			if s, err := waadapter.GetPhoneNumberStatus(c.Request.Context(), phoneNumberID, accessToken); err == nil {
				phoneStatus = s
			} else {
				slog.Warn("failed to fetch WhatsApp phone status",
					"channel_id", ch.ID, "phone_number_id", phoneNumberID, "error", err)
			}
		}
		platformOK := phoneStatus != nil && phoneStatus.PlatformType == "CLOUD_API"
		healthy := len(apps) > 0 && platformOK
		resp := gin.H{
			"channel_id":      ch.ID,
			"channel_name":    ch.Name,
			"is_healthy":      healthy,
			"subscribed":      len(apps) > 0,
			"subscribed_apps": apps,
			// Echo back the IDs the Flutter UI uses to build deep links
			// to Meta Business Manager (payment, 2FA, templates, etc.) —
			// users don't know how to navigate the Meta menus.
			"waba_id":         wabaID,
			"phone_number_id": phoneNumberID,
		}
		if phoneStatus != nil {
			resp["platform_type"] = phoneStatus.PlatformType
			resp["webhook_application"] = phoneStatus.WebhookApplication
			resp["needs_register"] = !platformOK
			resp["quality_rating"] = phoneStatus.QualityRating
			resp["messaging_limit_tier"] = phoneStatus.MessagingLimitTier
			resp["name_status"] = phoneStatus.NameStatus
		}
		// Persisted /register outcome from the last attempt (signup,
		// /channels/.../register, or post-verify register chain). Lets
		// the dashboard show "rate-limited (cooldown ends ~Xh)" or
		// "PIN bloqueado — use SMS" without forcing a new Meta call
		// just to re-derive the diagnosis.
		if v := cfg["registration_status"]; v != "" {
			resp["registration_status"] = v
		}
		if v := cfg["registration_error_code"]; v != "" {
			resp["registration_error_code"] = v
		}
		if v := cfg["registration_error_kind"]; v != "" {
			resp["registration_error_kind"] = v
		}
		if v := cfg["registration_error_message"]; v != "" {
			resp["registration_error_message"] = v
		}
		if v := cfg["registration_attempted_at"]; v != "" {
			resp["registration_attempted_at"] = v
		}
		if v := cfg["subscribe_status"]; v != "" {
			resp["subscribe_status"] = v
		}
		if v := cfg["subscribe_error_message"]; v != "" {
			resp["subscribe_error_message"] = v
		}
		// Business verification status — surfaced to the dashboard so it
		// can warn "Sua empresa não foi verificada pela Meta" + show a
		// "Verificar empresa" CTA. In Brazil, register fails with an
		// opaque #100 until the business is verified AND a payment method
		// is added — exposing the status here is the only way the user
		// understands the chicken-and-egg without reading Meta's docs.
		// Best-effort: a Meta failure here doesn't block the diagnostic.
		if owner, ownerErr := waadapter.GetWABAOwnerInfo(c.Request.Context(), wabaID, accessToken); ownerErr == nil && owner != nil {
			resp["business_id"] = owner.BusinessID
			resp["business_name"] = owner.BusinessName
			resp["business_verification_status"] = string(owner.VerificationStatus)
		} else if ownerErr != nil {
			slog.Warn("failed to fetch WABA owner info",
				"channel_id", ch.ID, "waba_id", wabaID, "error", ownerErr)
		}
		metaCacheSet(cacheKey, resp)
		c.JSON(http.StatusOK, resp)
	default:
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NOT_SUPPORTED", Message: "webhook status is only available for Telegram or WhatsApp channels"})
		return
	}
}

// HandleWebhookRegister godoc
// @Summary      Force re-register Telegram webhook
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  domain.AppError
// @Router       /channels/{id}/webhook-register [post]
func (h *ChannelHandler) HandleWebhookRegister(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if !h.verifyChannelOwnership(c, ch, accountID) {
		return
	}
	if ch.WebhookToken == "" && ch.Type == "telegram" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NO_WEBHOOK_TOKEN", Message: "channel has no webhook token"})
		return
	}

	configBytes := rawConfigBytes(ch.Config)

	switch ch.Type {
	case "telegram":
		baseURL := h.resolveBaseURL()
		if baseURL == "" {
			c.JSON(http.StatusBadGateway, domain.AppError{Code: "NO_PUBLIC_URL", Message: "no HTTPS base URL available and ngrok not detected"})
			return
		}
		var tgConfig struct {
			BotToken string `json:"bot_token"`
		}
		if err := json.Unmarshal(configBytes, &tgConfig); err != nil || tgConfig.BotToken == "" {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "VALIDATION_ERROR", Message: "bot_token not configured"})
			return
		}
		webhookURL := baseURL + "/v1/webhooks/" + ch.WebhookToken
		if err := tgadapter.RegisterWebhook(c.Request.Context(), tgConfig.BotToken, webhookURL, ch.WebhookToken); err != nil {
			slog.Error("failed to register telegram webhook", "channel_id", ch.ID, "error", err)
			c.JSON(http.StatusBadGateway, domain.AppError{Code: "TELEGRAM_ERROR", Message: "failed to register webhook with Telegram"})
			return
		}
		metaCacheInvalidate(ch.ID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "webhook_url": webhookURL})
	case "whatsapp":
		// WhatsApp: Re-run the `SubscribeApp` step from signup. Fixes
		// channels where the initial subscription was marked non-fatal
		// and silently failed — webhooks weren't arriving but the UI
		// had no way to recover short of a full reconnect.
		var waCfg struct {
			WABAID      string `json:"waba_id"`
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(configBytes, &waCfg); err != nil || waCfg.WABAID == "" || waCfg.AccessToken == "" {
			slog.Warn("WhatsApp channel missing creds for webhook register",
				"channel_id", ch.ID,
				"has_waba", waCfg.WABAID != "",
				"has_token", waCfg.AccessToken != "")
			c.JSON(http.StatusBadRequest, domain.AppError{
				Code:    "MISSING_TOKEN",
				Message: "Canal sem waba_id ou access_token. Reconecte via Meta Embedded Signup.",
			})
			return
		}
		if err := waadapter.SubscribeApp(c.Request.Context(), waCfg.WABAID, waCfg.AccessToken); err != nil {
			slog.Error("failed to re-subscribe WABA to app",
				"channel_id", ch.ID,
				"waba_id", waCfg.WABAID,
				"error", err)
			status, appErr := mapMetaErrorAction(err, waadapter.ActionSubscribeApp,
				"WA_SUBSCRIBE_FAILED", "failed to subscribe WABA to app")
			c.JSON(status, appErr)
			return
		}
		metaCacheInvalidate(ch.ID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "subscribed": true})
	default:
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NOT_SUPPORTED", Message: "webhook register is only available for Telegram or WhatsApp channels"})
	}
}

// SetNgrokDetection enables the local ngrok tunnel probe (dev only).
func (h *ChannelHandler) SetNgrokDetection(on bool) { h.ngrokDetection = on }

func (h *ChannelHandler) resolveBaseURL() string {
	if strings.HasPrefix(h.appBaseURL, "https://") {
		return h.appBaseURL
	}
	if !h.ngrokDetection {
		return ""
	}
	return detectNgrokURLFromLocal()
}

func detectNgrokURLFromLocal() string {
	resp, err := http.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		Tunnels []struct {
			PublicURL string `json:"public_url"`
		} `json:"tunnels"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return ""
	}
	for _, t := range result.Tunnels {
		if strings.HasPrefix(t.PublicURL, "https://") {
			return t.PublicURL
		}
	}
	return ""
}

func maskChannelConfig(channelType string, config interface{}) interface{} {
	configBytes := rawConfigBytes(config)
	if len(configBytes) == 0 {
		return config
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(configBytes, &configMap); err != nil {
		return config
	}

	// Mask every credential-shaped key (*_token/*_secret/*_key/password)
	// instead of a hand-kept list — a new channel type's secret (e.g.
	// Twilio auth_token) must never leak to the browser just because
	// nobody remembered to extend the list.
	for key, val := range configMap {
		if !domain.SensitiveConfigKey(key) {
			continue
		}
		if strVal, ok := val.(string); ok && len(strVal) > 4 {
			configMap[key] = strVal[:4] + strings.Repeat("*", len(strVal)-4)
		} else {
			configMap[key] = "****"
		}
	}

	return configMap
}

// igTokenExpiryWarnWindow is how close to `token_expires_at` a channel
// gets before TokenExpiringSoon flips true — 10 days gives the tenant
// a comfortable window to re-connect before Meta kills the token.
const igTokenExpiryWarnWindow = 10 * 24 * time.Hour

// decorateChannelResponse derives the response-only health fields
// (IsFullyConfigured + token expiry) and masks credentials. Must run
// on every channel read path BEFORE serialization — masking last, so
// the health derivation still sees real config values.
func decorateChannelResponse(ch *domain.Channel) {
	ch.IsFullyConfigured = isFullyConfigured(ch.Type, ch.Config)
	applyTokenHealth(ch)
	ch.Config = maskChannelConfig(ch.Type, ch.Config)
}

// applyTokenHealth surfaces the config's `token_expires_at` stamp
// (written by the channel use cases when an Instagram access token is
// set/rotated — Meta long-lived tokens die after ~60 days) as derived
// response fields, flagging TokenExpiringSoon when the deadline is
// inside the warn window or already past.
func applyTokenHealth(ch *domain.Channel) {
	raw := rawConfigBytes(ch.Config)
	if len(raw) == 0 {
		return
	}
	var cfg struct {
		TokenExpiresAt string `json:"token_expires_at"`
	}
	if json.Unmarshal(raw, &cfg) != nil || cfg.TokenExpiresAt == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, cfg.TokenExpiresAt)
	if err != nil {
		return
	}
	ch.TokenExpiresAt = &t
	ch.TokenExpiringSoon = time.Until(t) < igTokenExpiryWarnWindow
}

// isFullyConfigured reports whether the channel has every credential
// needed to actually call the upstream provider. The Flutter UI uses
// this to paint "Reconectar" badges on stub channels that were created
// before the embedded signup completed (or where a manual form was
// submitted with only a phone_number_id).
func isFullyConfigured(channelType string, config interface{}) bool {
	configBytes := rawConfigBytes(config)
	if len(configBytes) == 0 {
		return false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(configBytes, &m); err != nil {
		return false
	}
	has := func(key string) bool {
		v, ok := m[key]
		if !ok {
			return false
		}
		s, isStr := v.(string)
		return isStr && s != ""
	}
	switch channelType {
	case "whatsapp":
		return has("phone_number_id") && has("waba_id") && has("access_token")
	case "telegram":
		return has("bot_token")
	case "email":
		// US-M003: without all four the channel cannot send at all, so it
		// must not read as ready in the UI.
		return has("from_address") && has("smtp_host") && has("smtp_username") && has("smtp_password")
	case "instagram":
		return has("page_id") && has("access_token")
	case "sms":
		return has("account_sid") && has("auth_token") && has("from_number")
	case "webchat":
		return true
	case "internal":
		// In-product delivery — nothing external to configure.
		return true
	}
	return true
}

// HandleWhatsAppRegisterPhone runs /{phone_number_id}/register on the
// current Cloud API app so Meta starts routing inbound messages to our
// webhook. Required when Embedded Signup's best-effort register step
// silently failed (e.g. number was still on another app) — without
// this, the WABA can be "subscribed" to Replider while the phone stays
// platform_type=NOT_APPLICABLE and no messages arrive.
//
// PIN is the two-step-verification PIN set in WhatsApp Business
// Manager. If the number never had one, Meta accepts "000000" on the
// first call and locks the number to that PIN going forward.
//
// @Summary      Register WhatsApp phone on Cloud API (PIN required)
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Channel ID"
// @Param        body  body  object{pin=string}  true  "Two-step verification PIN (6 digits)"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  domain.AppError
// @Failure      404   {object}  domain.AppError
// @Router       /channels/{id}/whatsapp/register [post]
func (h *ChannelHandler) HandleWhatsAppRegisterPhone(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")
	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NOT_SUPPORTED", Message: "only whatsapp channels can be registered"})
		return
	}
	var req struct {
		Pin string `json:"pin"`
	}
	_ = c.ShouldBindJSON(&req)

	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
	pin, _ := resolveWAPin(req.Pin, cfg)
	phoneNumberID := cfg["phone_number_id"]
	accessToken := cfg["access_token"]
	if phoneNumberID == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "MISSING_CREDS",
			Message: "Canal sem phone_number_id ou access_token. Reconecte via Meta Embedded Signup.",
		})
		return
	}
	registerErr := waadapter.RegisterPhoneNumber(c.Request.Context(), phoneNumberID, accessToken, pin)
	registerOutcome := waadapter.ClassifyRegisterError(registerErr)
	applyWhatsAppRegistrationStatus(cfg, registerOutcome)
	if registerErr == nil {
		rememberLastWAPin(cfg, pin)
	}
	// Re-subscribe WABA as a belt-and-suspenders — some accounts lose
	// subscribed_apps when /register flips platform_type. Only attempt
	// when /register succeeded; otherwise the subscribe call will most
	// likely race the same Meta-side condition.
	var subscribeErr error
	if registerErr == nil && cfg["waba_id"] != "" {
		subscribeErr = waadapter.SubscribeApp(c.Request.Context(), cfg["waba_id"], accessToken)
		if subscribeErr != nil {
			slog.Warn("WABA re-subscribe after register failed", "channel_id", ch.ID, "error", subscribeErr)
		}
		applyWhatsAppSubscribeStatus(cfg, subscribeErr)
	}
	if persistErr := h.persistChannelConfig(c.Request.Context(), ch, cfg); persistErr != nil {
		slog.Warn("failed to persist registration status", "channel_id", ch.ID, "error", persistErr)
	}
	metaCacheInvalidate(ch.ID)
	if registerErr != nil {
		slog.Error("WhatsApp phone register failed",
			"channel_id", ch.ID, "phone_number_id", phoneNumberID,
			"status", registerOutcome.Status,
			"meta_code", registerOutcome.Code,
			"error", registerErr)
		status, appErr := mapMetaErrorAction(registerErr, waadapter.ActionRegisterPhone,
			"WA_REGISTER_FAILED", "failed to register phone on Cloud API")
		c.JSON(status, appErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "registered": true})
}

// persistChannelConfig writes the mutated config map back to channels.config.
// Used after /register and /verify-code so the dashboard sees the latest
// registration_status without a separate save call.
//
// We use MergeCredentials (PostgreSQL JSONB `||`) instead of a full
// Update — the admin path here would otherwise overwrite whatever the
// token-refresh worker had written to access_token between the read
// (which loaded ch.Config) and the write. Best-effort: callers log on
// failure but do not surface the error to the client, since the
// upstream Meta call already succeeded (or failed) — the only
// consequence of a persistence miss is a stale dashboard hint.
func (h *ChannelHandler) persistChannelConfig(ctx context.Context, ch *domain.Channel, cfg map[string]string) error {
	return h.channelRepo.MergeCredentials(ctx, ch.ID, cfg)
}

// HandleWhatsAppDeregister releases the phone from its current Cloud API
// owner so /register can run cleanly against our app with a brand-new
// PIN. Meta only accepts this when the caller's token is issued for
// the app that currently owns the number — for numbers stuck on a
// previous provider (Catholize, Chatwoot, ...) the call 400s and the
// customer has to go through the old provider or Meta Business Support.
//
// @Summary      Deregister WhatsApp phone from its Cloud API owner
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200 {object}  map[string]interface{}
// @Router       /channels/{id}/whatsapp/deregister [post]
func (h *ChannelHandler) HandleWhatsAppDeregister(c *gin.Context) {
	accountID := c.GetString("accountID")
	ch, err := h.channelRepo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NOT_SUPPORTED", Message: "only whatsapp channels"})
		return
	}
	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
	phoneNumberID := cfg["phone_number_id"]
	accessToken := cfg["access_token"]
	if phoneNumberID == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_CREDS", Message: "canal sem credenciais"})
		return
	}
	if err := waadapter.DeregisterPhoneNumber(c.Request.Context(), phoneNumberID, accessToken); err != nil {
		slog.Error("WhatsApp deregister failed", "channel_id", ch.ID, "error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionDeregisterPhone,
			"WA_DEREGISTER_FAILED", "falha ao liberar o número")
		c.JSON(status, appErr)
		return
	}
	metaCacheInvalidate(ch.ID)
	// After deregister the phone goes back to platform_type=NOT_APPLICABLE
	// — flag the channel as pending so the dashboard prompts the user to
	// re-run /register (typically with a fresh PIN).
	applyWhatsAppRegistrationStatus(cfg, waadapter.RegistrationOutcome{Status: waadapter.RegistrationStatusPending})
	if persistErr := h.persistChannelConfig(c.Request.Context(), ch, cfg); persistErr != nil {
		slog.Warn("failed to persist post-deregister status", "channel_id", ch.ID, "error", persistErr)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deregistered": true})
}

// HandleWhatsAppRequestCode triggers Meta's /request_code so the customer
// can reset the Cloud API 2FA PIN even when the previous integration
// set one we don't know. The real code arrives on the customer's
// phone via SMS or voice; they feed it into HandleWhatsAppVerifyCode.
//
// @Summary      Request a Cloud API verification code (SMS / voice)
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Channel ID"
// @Param        body  body  object{code_method=string,language=string}  true  "code_method=SMS|VOICE"
// @Success      200   {object}  map[string]interface{}
// @Router       /channels/{id}/whatsapp/request-code [post]
func (h *ChannelHandler) HandleWhatsAppRequestCode(c *gin.Context) {
	accountID := c.GetString("accountID")
	ch, err := h.channelRepo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NOT_SUPPORTED", Message: "only whatsapp channels"})
		return
	}
	var req struct {
		CodeMethod string `json:"code_method"`
		Language   string `json:"language"`
	}
	_ = c.ShouldBindJSON(&req)
	method := strings.ToUpper(strings.TrimSpace(req.CodeMethod))
	if method != "SMS" && method != "VOICE" {
		method = "SMS"
	}
	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
	phoneNumberID := cfg["phone_number_id"]
	accessToken := cfg["access_token"]
	if phoneNumberID == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_CREDS", Message: "canal sem credenciais"})
		return
	}
	if err := waadapter.RequestVerificationCode(c.Request.Context(), phoneNumberID, accessToken, method, req.Language); err != nil {
		slog.Error("WhatsApp request_code failed", "channel_id", ch.ID, "error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionRequestCode,
			"WA_REQUEST_CODE_FAILED", "could not enviar o código")
		c.JSON(status, appErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "code_method": method})
}

// HandleWhatsAppVerifyCode confirms the code Meta sent and, if a PIN
// is provided, immediately re-registers the phone so the caller takes
// over Cloud API ownership in a single round-trip.
//
// @Summary      Verify code + optionally register with new PIN
// @Tags         Channels
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Channel ID"
// @Param        body  body  object{code=string,new_pin=string}  true  "6-digit verification code + new PIN"
// @Success      200   {object}  map[string]interface{}
// @Router       /channels/{id}/whatsapp/verify-code [post]
func (h *ChannelHandler) HandleWhatsAppVerifyCode(c *gin.Context) {
	accountID := c.GetString("accountID")
	ch, err := h.channelRepo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NOT_SUPPORTED", Message: "only whatsapp channels"})
		return
	}
	var req struct {
		Code   string `json:"code"`
		NewPin string `json:"new_pin"`
	}
	_ = c.ShouldBindJSON(&req)
	code := strings.TrimSpace(req.Code)
	if code == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "VALIDATION_ERROR", Message: "code é obrigatório"})
		return
	}
	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
	phoneNumberID := cfg["phone_number_id"]
	accessToken := cfg["access_token"]
	if phoneNumberID == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_CREDS", Message: "canal sem credenciais"})
		return
	}
	if err := waadapter.VerifyCode(c.Request.Context(), phoneNumberID, accessToken, code); err != nil {
		slog.Error("WhatsApp verify_code failed", "channel_id", ch.ID, "error", err)
		// Catalog handles the (#100) "Invalid parameter" disambiguation
		// for this endpoint via the ActionVerifyCode entry — no manual
		// rewrite needed.
		status, appErr := mapMetaErrorAction(err, waadapter.ActionVerifyCode,
			"WA_VERIFY_CODE_FAILED", "código inválido ou expirado")
		c.JSON(status, appErr)
		return
	}
	// Chain into /register when caller supplied a new PIN. Without this
	// the UI would need a second round-trip and the number stays
	// platform_type=NOT_APPLICABLE in the interim.
	registered := false
	newPin := strings.TrimSpace(req.NewPin)
	if newPin != "" {
		registerErr := waadapter.RegisterPhoneNumber(c.Request.Context(), phoneNumberID, accessToken, newPin)
		registerOutcome := waadapter.ClassifyRegisterError(registerErr)
		applyWhatsAppRegistrationStatus(cfg, registerOutcome)
		if registerErr == nil {
			rememberLastWAPin(cfg, newPin)
		}
		if registerErr == nil && cfg["waba_id"] != "" {
			subscribeErr := waadapter.SubscribeApp(c.Request.Context(), cfg["waba_id"], accessToken)
			applyWhatsAppSubscribeStatus(cfg, subscribeErr)
		}
		if persistErr := h.persistChannelConfig(c.Request.Context(), ch, cfg); persistErr != nil {
			slog.Warn("failed to persist post-verify registration status",
				"channel_id", ch.ID, "error", persistErr)
		}
		if registerErr != nil {
			slog.Error("WhatsApp post-verify register failed",
				"channel_id", ch.ID,
				"status", registerOutcome.Status,
				"meta_code", registerOutcome.Code,
				"error", registerErr)
			status, appErr := mapMetaErrorAction(registerErr, waadapter.ActionRegisterPhone,
				"WA_REGISTER_FAILED", "falha ao registrar com novo PIN")
			c.JSON(status, appErr)
			return
		}
		registered = true
	}
	metaCacheInvalidate(ch.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "verified": true, "registered": registered})
}

func rawConfigBytes(config interface{}) []byte {
	switch v := config.(type) {
	case []byte:
		return v
	case json.RawMessage:
		return []byte(v)
	default:
		b, _ := json.Marshal(v)
		return b
	}
}
