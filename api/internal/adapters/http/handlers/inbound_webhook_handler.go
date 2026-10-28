package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	emailchannel "github.com/replider/api/internal/adapters/channels/email"
	igadapter "github.com/replider/api/internal/adapters/channels/instagram"
	"github.com/replider/api/internal/adapters/channels/telegram"
	tgadapter "github.com/replider/api/internal/adapters/channels/telegram"
	twadapter "github.com/replider/api/internal/adapters/channels/twilio"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/observability"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// maxTelegramWebhookAgeSec bounds how stale a Telegram update may be
// before the receiver treats it as a replay. 1h mirrors the WhatsApp
// guard (whatsapp.MaxWebhookAge); message-id dedupe is the second line
// of defence for anything inside the window.
const maxTelegramWebhookAgeSec = 1 * 60 * 60

// InboundWebhookHandler handles incoming messages from external channels (WhatsApp, Telegram, etc.)
type InboundWebhookHandler struct {
	inboundUC   *usecases.InboundWebhookUseCase
	hub         *Hub
	channelRepo domain.ChannelRepository
	msgRepo     domain.MessageRepository // optional: nil disables send-failure stamping
	waAdapter   *waadapter.WhatsAppAdapter
	tgAdapter   *tgadapter.TelegramAdapter
	igAdapter   *igadapter.InstagramAdapter
	smsAdapter  *twadapter.TwilioAdapter
	pushUC      *usecases.PushNotificationUseCase // optional: nil when push is disabled
	events      EventEmitter                      // optional: nil disables outbound integration fan-out
	// igAppSecret validates X-Hub-Signature-256 on Instagram inbound
	// payloads (Meta HMAC, same scheme as WhatsApp). Fail-closed: a
	// channel with no per-channel `app_secret` config key AND no global
	// secret gets a 403 on every delivery — manual-token channels on a
	// tenant's own Meta app must carry their app's secret as channel
	// config. The per-channel key overrides this global.
	igAppSecret string
	// emailWebhookSecret validates the Standard Webhooks signature on inbound
	// mail (Resend and friends). Same fail-closed policy as igAppSecret: with
	// no per-channel `webhook_secret` and no global secret, every delivery is
	// refused. The path had NO check at all until 15/09/2026, which let a
	// forged POST assert its own SPF/DKIM verdict — see the case "email"
	// block for why that one mattered more than the others.
	emailWebhookSecret string
	// attachmentUC is optional (setter-wired): nil means inbound files are
	// parsed and then dropped, which is the pre-US-M005-followup behaviour.
	attachmentUC *usecases.MessageAttachmentUseCase
	// riskUC is optional: nil means inbound email is delivered without the
	// impersonation checks, which is the pre-B1 behaviour.
	riskUC *usecases.EmailRiskUseCase
	// bounceUC is optional: nil means a delivery failure is recognised and
	// dropped rather than suppressing the address. Recognising it is still
	// worth doing unwired — it keeps MAILER-DAEMON out of the inbox.
	bounceUC *usecases.EmailBounceUseCase
	// triageUC is optional: nil leaves every conversation uncategorised,
	// which is the pre-B3 behaviour — a chronological inbox.
	triageUC *usecases.InboxTriageUseCase
}

// SetEmailRiskUseCase wires the inbound impersonation checks. Optional.
func (h *InboundWebhookHandler) SetEmailRiskUseCase(uc *usecases.EmailRiskUseCase) {
	h.riskUC = uc
}

// SetAttachmentUseCase wires inbound attachment storage. Optional so the
// handler constructor and its test call sites stay untouched — the same
// pattern as every other optional dependency here.
func (h *InboundWebhookHandler) SetAttachmentUseCase(uc *usecases.MessageAttachmentUseCase) {
	h.attachmentUC = uc
}

// SetInstagramAppSecret wires the global Meta app secret used to verify
// X-Hub-Signature-256 on inbound Instagram webhooks. Optional — see the
// igAppSecret field doc for the fallback semantics.
func (h *InboundWebhookHandler) SetInstagramAppSecret(secret string) {
	h.igAppSecret = secret
}

// SetEmailWebhookSecret wires the platform-wide inbound-mail signing secret.
// Optional — see the emailWebhookSecret field doc for the fallback semantics.
func (h *InboundWebhookHandler) SetEmailWebhookSecret(secret string) {
	h.emailWebhookSecret = secret
}

// SetMessageRepo wires the message repo so outbound send failures can
// stamp failed_at + failed_reason on the row. Optional — without it
// the dashboard still flips the bubble via the WS event but the DB
// stays inconsistent until ApplyWhatsAppStatus catches up.
func (h *InboundWebhookHandler) SetMessageRepo(r domain.MessageRepository) {
	h.msgRepo = r
}

// SetPushUseCase wires the push dispatcher. Optional — when unset, push
// fan-out is skipped silently (useful in tests and when VAPID keys are
// absent).
func (h *InboundWebhookHandler) SetPushUseCase(uc *usecases.PushNotificationUseCase) {
	h.pushUC = uc
}

// SetEventEmitter wires the outbound webhook dispatcher so Telegram /
// Instagram inbound also fan out to tenant integrations alongside the
// dashboard WS broadcast. WhatsApp uses its own dedicated handler.
func (h *InboundWebhookHandler) SetEventEmitter(e EventEmitter) {
	h.events = e
}

// NewInboundWebhookHandler creates a new handler for channel webhooks
func NewInboundWebhookHandler(
	inboundUC *usecases.InboundWebhookUseCase,
	hub *Hub,
	channelRepo domain.ChannelRepository,
	waAdapter *waadapter.WhatsAppAdapter,
	tgAdapter *tgadapter.TelegramAdapter,
	igAdapter *igadapter.InstagramAdapter,
	smsAdapter *twadapter.TwilioAdapter,
) *InboundWebhookHandler {
	return &InboundWebhookHandler{
		inboundUC:   inboundUC,
		hub:         hub,
		channelRepo: channelRepo,
		waAdapter:   waAdapter,
		tgAdapter:   tgAdapter,
		igAdapter:   igAdapter,
		smsAdapter:  smsAdapter,
	}
}

// inboundItem is one normalized inbound message, whatever transport it arrived
// on.
//
// Package-level rather than local to the webhook handler because the mailbox
// poller (IMAP) produces the same thing: it reads mail nobody pushed to us and
// has to travel the identical path afterwards. Two copies of that path is how
// one arrival route quietly stops storing attachments, or stops running the
// impersonation checks, and nobody notices for months.
type inboundItem struct {
	from, content, msgType, senderName, senderUsername, mediaURL, externalMsgID string
	runAI                                                                       bool
	// metadata carries channel-specific fields that must survive onto
	// the stored message. Email (US-M005) uses it for subject +
	// threading headers, without which a reply starts a new thread.
	metadata []byte
	// attachments are stored AFTER the message row exists, since each
	// row references its message. Nil for every channel but email.
	attachments []usecases.IncomingAttachment
	// emailRisk carries what the impersonation checks need. Zero for
	// every channel but email.
	emailReplyTo string
	emailAuth    *domain.InboundAuthVerdict
	// Inbox triage (§6.10 camada 3). The subject carries most of what a
	// category can be read from, and List-Unsubscribe is the honest
	// signal that something is bulk mail.
	emailSubject    string
	listUnsubscribe bool
}

// HandleInboundWebhook godoc
// @Summary      Receive inbound messages from external channels
// @Description  Public webhook endpoint. Receives messages from WhatsApp/Telegram/Instagram (JSON) and Twilio SMS (form-encoded, X-Twilio-Signature validated, TwiML ack) via a unique channel token.
//
//	Uses a two-phase flow: Phase 1 saves the inbound message and broadcasts it to dashboard
//	clients via WebSocket so the agent sees it immediately. Phase 2 (if the bot is active)
//	sends a typing indicator, runs AI processing (RAG + tool calling), clears the typing
//	indicator, broadcasts the AI response via WebSocket, and dispatches the reply through the
//	appropriate channel adapter (WhatsApp, Telegram, etc.).
//
// @Tags         webhooks
// @Accept       json
// @Produce      json
// @Param        channel_token path string true "Unique webhook token for the channel"
// @Success      200  {object}  map[string]string  "Message processed"
// @Failure      400  {object}  domain.AppError     "Invalid payload"
// @Failure      403  {object}  domain.AppError     "Invalid or missing webhook signature (Instagram/SMS/Telegram)"
// @Failure      404  {object}  domain.AppError     "Channel not found"
// @Failure      500  {object}  domain.AppError     "Processing error"
// @Router       /webhooks/{channel_token} [post]
func (h *InboundWebhookHandler) HandleInboundWebhook(c *gin.Context) {
	// Defense in depth: any panic inside SaveInbound (domain code) or
	// the channel adapter path should not escape to the HTTP writer as
	// a 500. Telegram's watchdog flips the webhook to "Webhook down" on
	// any 5xx, so we eat panics, log them, and 200 the provider.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("inbound webhook panic",
				"channel_token", c.Param("channel_token"),
				"recover", r)
			if !c.Writer.Written() {
				c.JSON(http.StatusOK, gin.H{"status": "recovered"})
			}
		}
	}()

	channelToken := c.Param("channel_token")

	// Resolve the channel BEFORE reading the body: the size a payload may
	// legitimately reach depends on the channel type, and the cap has to be
	// applied while reading, not after.
	channel, err := h.channelRepo.GetByWebhookToken(c.Request.Context(), channelToken)
	if err != nil || channel == nil {
		observability.WebhookFailuresTotal.WithLabelValues("unknown", "channel_not_found").Inc()
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}

	// A truncated body is worse than a rejected one: it fails to parse and
	// acks as "unparseable", which is silence — the exact failure mode the
	// attachment work exists to remove. So the limit is read with one extra
	// byte and an over-limit payload is reported, never trimmed.
	limit := maxInboundBodyBytes(channel.Type)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, limit+1))
	if err != nil {
		observability.WebhookFailuresTotal.WithLabelValues("unknown", "read_body").Inc()
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_PAYLOAD", Message: "failed to read body"})
		return
	}
	defer c.Request.Body.Close()
	if int64(len(body)) > limit {
		slog.Error("inbound webhook payload over the size limit, dropped",
			"channel_type", channel.Type, "channel_id", channel.ID,
			"limit_bytes", limit, "read_bytes", len(body))
		observability.WebhookFailuresTotal.WithLabelValues(channel.Type, "body_too_large").Inc()
		// 413 rather than 200: the provider SHOULD know this one failed.
		// Acking it would lose the message with no trace anywhere.
		c.JSON(http.StatusRequestEntityTooLarge,
			domain.AppError{Code: "PAYLOAD_TOO_LARGE", Message: "webhook payload too large"})
		return
	}

	// Validate Telegram webhook secret token. The token is a per-channel
	// ULID so brute force is infeasible (2^128 space), but plain `==`
	// short-circuits on the first byte mismatch and leaks length/prefix
	// info to a network-positioned attacker. subtle.ConstantTimeCompare
	// runs in time independent of the inputs and is the standard
	// pattern for any secret comparison.
	if channel.Type == "telegram" {
		secretToken := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(secretToken), []byte(channel.WebhookToken)) != 1 {
			slog.Warn("Telegram webhook secret token mismatch", "channel_id", channel.ID, "remote_addr", c.ClientIP())
			observability.WebhookFailuresTotal.WithLabelValues("telegram", "invalid_secret").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SECRET", Message: "invalid webhook secret"})
			return
		}
	}

	// Parse into normalized inbound items based on channel type. Most
	// channels deliver exactly one message per webhook, but Instagram
	// batches (entry[]/messaging[] both fan out, and Meta retries
	// re-deliver whole batches) and an MMS carries N attachments —
	// processing only the first item silently drops the rest.
	var items []inboundItem
	var tgUserID int64

	switch channel.Type {
	case "telegram":
		parsed, ok := telegram.ParseUpdate(body)
		if !ok {
			// Telegram sends non-message updates (e.g. bot added to group) — acknowledge silently
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		// Replay guard: a captured webhook re-POSTed later carries a
		// stale message.date. The secret-token check alone does not
		// bound payload age. Ack with 200 so an attacker can't probe,
		// but drop the message. Skip when date is absent (0).
		if parsed.Date > 0 && time.Now().Unix()-parsed.Date > maxTelegramWebhookAgeSec {
			slog.Warn("Telegram webhook rejected as stale (possible replay)",
				"channel_id", channel.ID, "message_age_sec", time.Now().Unix()-parsed.Date,
				"remote_addr", c.ClientIP())
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		item := inboundItem{
			from:           parsed.ChatID,
			content:        parsed.Text,
			senderName:     parsed.SenderName,
			senderUsername: parsed.Username,
			msgType:        parsed.MediaType,
			externalMsgID:  parsed.MessageID,
			runAI:          true,
		}
		tgUserID = parsed.UserID

		// Resolve media file to downloadable URL via Telegram getFile API
		if parsed.MediaFileID != "" {
			url, err := h.tgAdapter.GetFileURL(c.Request.Context(), channel, parsed.MediaFileID)
			if err != nil {
				slog.Error("failed to resolve Telegram file URL", "channel_id", channel.ID, "error", err)
			} else {
				item.mediaURL = url
			}
		} else if parsed.LatLon != "" {
			// Location: encode as geo URI so the client can render a map link
			item.mediaURL = "geo:" + parsed.LatLon
		}
		items = append(items, item)

	case "instagram":
		// X-Hub-Signature-256 validation, same Meta HMAC scheme as the
		// WhatsApp webhook. Secret resolution: per-channel `app_secret`
		// config key (tenant's own Meta app) → global INSTAGRAM_APP_SECRET.
		// Fail CLOSED when no secret is configured anywhere — same policy
		// as SMS/WhatsApp: an unvalidated public endpoint accepts forged
		// DMs from anyone who learns the channel token. Manual-token
		// tenants must enter their Meta app's secret as the channel's
		// `app_secret` config key.
		igSecret := resolveInstagramSecret(channel, h.igAppSecret)
		if igSecret == "" {
			slog.Error("Instagram webhook rejected: no app secret configured — set INSTAGRAM_APP_SECRET or the channel's app_secret config",
				"channel_id", channel.ID)
			observability.WebhookFailuresTotal.WithLabelValues("instagram", "missing_app_secret").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SIGNATURE", Message: "invalid webhook signature"})
			return
		}
		sig := c.GetHeader("X-Hub-Signature-256")
		if !igadapter.VerifySignature(body, sig, igSecret) {
			slog.Warn("Instagram webhook signature mismatch",
				"channel_id", channel.ID, "remote_addr", c.ClientIP())
			observability.WebhookFailuresTotal.WithLabelValues("instagram", "invalid_signature").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SIGNATURE", Message: "invalid webhook signature"})
			return
		}

		messages, parseErr := igadapter.ParseWebhookPayload(body)
		if parseErr != nil || len(messages) == 0 {
			// Non-message events (read receipts, reactions) and bot-echo
			// deliveries (message.is_echo — our own outbound reflected
			// back) land here: parsed fine, zero actionable messages.
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		// Meta's Graph API is the only source of a human-readable name
		// for an IGSID — the webhook itself carries none. Cache per
		// request so a batch from one sender costs one profile call.
		igNames := map[string]string{}
		for _, msg := range messages {
			senderName := msg.SenderName
			if senderName == "" {
				senderName = h.instagramSenderName(c.Request.Context(), channel, msg.SenderID, igNames)
			}
			items = append(items, inboundItem{
				from:          msg.SenderID,
				content:       msg.Text,
				senderName:    senderName,
				msgType:       msg.MediaType,
				mediaURL:      msg.MediaURL,
				externalMsgID: msg.MessageID,
				runAI:         true,
			})
		}

	case "email":
		// Standard Webhooks signature (what Resend sends), same fail-closed
		// policy as SMS/Instagram/WhatsApp. This endpoint used to check
		// NOTHING: the only secret was the channel token in the URL, and a
		// secret in a URL leaks through proxy logs, Referer headers, browser
		// history and the copy button in our own interface.
		//
		// It mattered more here than elsewhere. Below, ParseInboundWebhook
		// fills InboundEmail.Auth from the POST BODY, while the IMAP path
		// verifies DKIM against the raw message. So a forged POST could
		// declare `spf=pass; dkim=pass` and AssessEmailRisk would grade it
		// clean — the supplier-impersonation fraud that risk layer exists to
		// catch, walking in through the one door with no lock. With the
		// signature required, the body can only have come from the provider
		// this account chose, and trusting its verdict becomes a decision
		// about that provider rather than about a stranger.
		mailSecret := resolveEmailWebhookSecret(channel, h.emailWebhookSecret)
		if mailSecret == "" {
			slog.Error("email webhook rejected: no signing secret configured — set EMAIL_WEBHOOK_SECRET or the channel's webhook_secret config",
				"channel_id", channel.ID)
			observability.WebhookFailuresTotal.WithLabelValues("email", "missing_webhook_secret").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SIGNATURE", Message: "invalid webhook signature"})
			return
		}
		if !emailchannel.VerifyWebhookSignature(mailSecret, c.GetHeader, body, time.Now()) {
			slog.Warn("email webhook signature mismatch",
				"channel_id", channel.ID, "remote_addr", c.ClientIP())
			observability.WebhookFailuresTotal.WithLabelValues("email", "invalid_signature").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SIGNATURE", Message: "invalid webhook signature"})
			return
		}

		// A delivery FAILURE is checked first, because both of its shapes
		// were mishandled below otherwise: a provider event has no From and
		// would be acked as "ignored" (the bounce vanishing), and a DSN
		// parses as an ordinary message — becoming a conversation with
		// MAILER-DAEMON as the contact, which the agent then replies to.
		if bounce, ok := emailchannel.ParseBounce(body); ok {
			h.recordBounce(c, channel, bounce)
			return
		}

		// The inbound mail provider (Resend/SendGrid/Mailgun/Postmark)
		// POSTs the parsed message as JSON. The sender identity is an
		// EMAIL ADDRESS, not a phone — the identity resolver (US-F007)
		// keys on the channel type, so the same person writing from mail
		// and from WhatsApp lands on one contact.
		parsed, ok := emailchannel.ParseInboundWebhook(body)
		if !ok {
			// No usable sender: ack rather than 500, or the provider
			// retries an unattributable message forever.
			observability.WebhookFailuresTotal.WithLabelValues("email", "unparseable").Inc()
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		// Delivery reports and autoresponders: the shared gate (also run by
		// the IMAP poller — see inbound_email_fetch.go). Acked either way,
		// because a provider retries a non-2xx forever.
		switch verdict, bounce := h.emailDeliveryGate(c.Request.Context(), channel, parsed); verdict {
		case emailGateBounce:
			c.JSON(http.StatusOK, gin.H{"status": "bounce_recorded", "kind": string(bounce.Kind)})
			return
		case emailGateDropped:
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}

		// Strip the quoted history: without this every reply re-feeds the
		// whole thread to the AI (paying for the same tokens repeatedly)
		// and the dashboard shows a wall of quoted text instead of the one
		// sentence the person wrote.
		item, ok := h.emailInboundItem(c.Request.Context(), channel, parsed)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}
		items = append(items, item)

	case "sms":
		// Twilio posts application/x-www-form-urlencoded (From, To,
		// Body, MessageSid, ...). X-Twilio-Signature is HMAC-SHA1 over
		// the full request URL + sorted form params, keyed by the
		// channel's OWN auth_token (BYO Twilio). The token is required
		// for the channel to function at all, so — unlike the Instagram
		// fail-open path — a channel that can't validate fails closed.
		form, parseErr := url.ParseQuery(string(body))
		if parseErr != nil {
			observability.WebhookFailuresTotal.WithLabelValues("sms", "invalid_form").Inc()
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_PAYLOAD", Message: "invalid form body"})
			return
		}
		authToken := credentialString(channel, "auth_token")
		if authToken == "" {
			slog.Warn("SMS webhook on channel without auth_token config",
				"channel_id", channel.ID)
			observability.WebhookFailuresTotal.WithLabelValues("sms", "missing_auth_token").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SIGNATURE", Message: "invalid webhook signature"})
			return
		}
		if !twadapter.ValidateSignature(authToken, webhookRequestURL(c), form, c.GetHeader("X-Twilio-Signature")) {
			slog.Warn("Twilio webhook signature mismatch",
				"channel_id", channel.ID, "remote_addr", c.ClientIP())
			observability.WebhookFailuresTotal.WithLabelValues("sms", "invalid_signature").Inc()
			c.JSON(http.StatusForbidden, domain.AppError{Code: "INVALID_SIGNATURE", Message: "invalid webhook signature"})
			return
		}
		sms, ok := twadapter.ParseInboundForm(form)
		if !ok {
			// Status callbacks pointed at the message URL, or payloads
			// with no From/MessageSid — ack so Twilio doesn't retry.
			ackTwiML(c)
			return
		}
		item := inboundItem{
			from:          sms.From, // E.164 phone is the sender identity
			content:       sms.Body,
			externalMsgID: sms.MessageSID,
			msgType:       "text",
			runAI:         true,
		}
		if len(sms.Media) > 0 {
			item.mediaURL = sms.Media[0].URL
			item.msgType = sms.Media[0].Type
		}
		items = append(items, item)
		// Every additional MMS attachment becomes its own media message.
		// External id gets a #n suffix so the per-attachment rows still
		// dedupe on Twilio webhook retries; only the first item (the one
		// carrying the text body) drives the AI so a 3-photo MMS doesn't
		// trigger three bot replies.
		if len(sms.Media) > 1 {
			for i, m := range sms.Media[1:] {
				items = append(items, inboundItem{
					from:          sms.From,
					msgType:       m.Type,
					mediaURL:      m.URL,
					externalMsgID: sms.MessageSID + "#" + strconv.Itoa(i+1),
					runAI:         false,
				})
			}
		}

	default: // "whatsapp" and legacy normalized format
		var payload InboundPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			observability.WebhookFailuresTotal.WithLabelValues(channel.Type, "invalid_json").Inc()
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_PAYLOAD", Message: "invalid JSON"})
			return
		}
		item := inboundItem{
			from:    payload.From,
			content: payload.Content,
			msgType: payload.Type,
			runAI:   true,
		}
		if item.msgType == "" {
			item.msgType = "text"
		}
		items = append(items, item)
	}

	// Phase 1: save each inbound message (creates/finds conversation).
	// Items are independent — a duplicate or persist failure never blocks
	// its batch siblings (an Instagram retry re-delivers the WHOLE batch,
	// so a partially-processed batch must still ingest the missed tail).
	type aiJob struct {
		partial *usecases.InboundPartialResult
		from    string
		content string
	}
	var (
		aiJobs  []aiJob
		firstOK *usecases.InboundPartialResult
		lastErr error
	)
	for _, it := range items {
		partial, runAI, err := h.ingestInbound(c.Request.Context(), channel, channelToken, it)
		if err != nil {
			if err == usecases.ErrDuplicateInbound {
				// Provider retried a webhook we already processed; skip
				// so the retry loop stops and nothing else fires for it.
				slog.Debug("duplicate inbound webhook ignored",
					"channel_type", channel.Type, "external_message_id", it.externalMsgID)
				continue
			}
			// Keep going, ACK 200 at the end even on persistence failures.
			// Returning 5xx makes Telegram mark the webhook unhealthy
			// (getWebhookInfo flips is_healthy=false with "Wrong response
			// from the webhook: 500"), which the UI then surfaces as
			// "Webhook down" even though the webhook itself is reachable
			// and the problem is downstream. We still log at ERROR so ops
			// sees the storage failure.
			lastErr = err
			observability.WebhookFailuresTotal.WithLabelValues(channel.Type, "persist_failed").Inc()
			slog.Error("failed to process inbound webhook",
				"channel_id", channel.ID,
				"channel_type", channel.Type,
				"external_message_id", it.externalMsgID,
				"error", err)
			continue
		}
		if firstOK == nil {
			firstOK = partial
		}
		if runAI {
			aiJobs = append(aiJobs, aiJob{partial: partial, from: it.from, content: it.content})
		}
	}

	// Durability escape hatch: Instagram and SMS providers natively
	// redeliver on a 5xx (Meta's webhook retry budget; Twilio's retry
	// schedule) and — unlike WhatsApp — have no outbox/replay path on this
	// route. When NOTHING persisted (a single-item webhook whose one
	// SaveInbound failed, the overwhelming common case), ACKing 200 would
	// lose the customer's message forever. Surface a 500 so the provider
	// retries; a redelivery re-runs SaveInbound cleanly. Telegram is
	// excluded on purpose — its watchdog flips the webhook to "unhealthy"
	// on any 5xx — and so is webchat/default (synchronous UI, no retry).
	// Scoped to firstOK == nil so a partial-batch failure (some items
	// already saved + AI-dispatched below) never re-drives those siblings.
	if firstOK == nil && lastErr != nil && (channel.Type == "instagram" || channel.Type == "sms") {
		observability.WebhookFailuresTotal.WithLabelValues(channel.Type, "ack_5xx_for_retry").Inc()
		c.JSON(http.StatusInternalServerError, domain.AppError{
			Code:    "INBOUND_PERSIST_FAILED",
			Message: "failed to persist inbound message; provider should retry",
		})
		return
	}

	// Ack the provider before the AI runs. Telegram retries undelivered
	// updates aggressively and Meta's webhook budget is 20s; synchronous
	// AI work past those thresholds produced the duplicate-inbound storm
	// that dedupe now covers. Keep the ack on the request path and move
	// AI + outbound send to a detached goroutine. Twilio parses the
	// response body as TwiML, so SMS gets an empty <Response/> instead
	// of JSON (which would log error 12300 in the Twilio console).
	switch {
	case channel.Type == "sms":
		ackTwiML(c)
	case firstOK != nil:
		c.JSON(http.StatusOK, gin.H{
			"status":          "processed",
			"conversation_id": firstOK.ConversationID,
		})
	case lastErr != nil:
		c.JSON(http.StatusOK, gin.H{
			"status": "error",
			"detail": lastErr.Error(),
		})
	default: // every item was a provider retry we had already processed
		c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
	}

	// Telegram profile (photo + bio) can change over time — keep the
	// stored copy in sync without hammering the Bot API on every message
	// by only re-fetching when the cached data is absent or >24h old.
	if channel.Type == "telegram" && tgUserID != 0 && firstOK != nil && avatarNeedsRefresh(firstOK.ContactAvatarFetchedAt) {
		go h.refreshTelegramProfileAsync(channel, firstOK.AccountID, firstOK.ConversationID, items[0].from)
	}

	for _, job := range aiJobs {
		go h.processAIAsync(channel, job.partial, job.from, job.content)
	}
}

// emailInboundItem maps a parsed message onto the normalized item.
//
// Shared by the webhook route and the mailbox poller. Every field here is read
// by something downstream — the threading headers keep a reply in its thread,
// the auth verdict feeds the impersonation checks, the inline flag stops a
// signature logo being counted as "1 anexo" — so a field set on one arrival
// route and not the other is a feature that silently works for half the
// customers.
//
// Reports false when there is nothing worth storing: no text after the quoted
// history is stripped, and no subject either.
func (h *InboundWebhookHandler) emailInboundItem(
	ctx context.Context,
	channel *domain.Channel,
	parsed emailchannel.InboundEmail,
) (inboundItem, bool) {
	content := emailchannel.StripQuotedReply(parsed.Body)
	if strings.TrimSpace(content) == "" && parsed.Subject == "" {
		return inboundItem{}, false
	}
	// Inline furniture (signature logos, tracking pixels) is stored
	// too — it is what "view original" needs to render — but flagged so
	// the UI never counts it as "1 anexo". Training someone to ignore
	// the attachment indicator is how the real contract slips past.
	var files []usecases.IncomingAttachment
	for _, a := range parsed.Attachments {
		if len(a.Content) == 0 {
			// URL-hosted (Mailgun's default). Fetching it means an
			// outbound request to a provider-supplied address, which
			// has to go through the SSRF-guarded client — not wired
			// yet, so it is logged rather than silently skipped.
			slog.WarnContext(ctx, "inbound attachment came as a URL, not bytes — not stored",
				"channel_id", channel.ID, "filename", a.Filename)
			continue
		}
		files = append(files, usecases.IncomingAttachment{
			Filename:    a.SafeFilename(),
			ContentType: a.ContentType,
			Content:     a.Content,
			IsInline:    a.IsInline(),
			ContentID:   a.ContentID,
		})
	}

	// The original markup, kept for "view original". Uploaded BEFORE the
	// message exists and keyed by its own id, so the path can ride in the
	// metadata and no second write is needed to attach it. A failed
	// upload just means the button will not appear — the text, which is
	// what the pipeline and the reader actually use, is already safe.
	var htmlPath string
	if h.attachmentUC != nil && parsed.HTML != "" {
		htmlPath = h.attachmentUC.StoreOriginalHTML(ctx,
			channel.AccountID, ulid.Make().String(), parsed.HTML)
	}

	return inboundItem{
		from:            parsed.From,
		senderName:      parsed.FromName,
		content:         content,
		msgType:         "text",
		externalMsgID:   parsed.MessageID,
		metadata:        emailInboundMetadata(parsed, htmlPath),
		attachments:     files,
		emailReplyTo:    parsed.ReplyTo,
		emailSubject:    parsed.Subject,
		listUnsubscribe: parsed.ListUnsubscribe,
		emailAuth: &domain.InboundAuthVerdict{
			Trustworthy: parsed.Auth.Trustworthy(),
			Suspicious:  parsed.Auth.Suspicious(),
		},
		// Email NEVER answers itself (2026-08-26, owner decision): this is
		// B2B hospital-equipment sales, the ticket is high and an
		// auto-reply that gets a nuance wrong costs a deal. The AI drafts
		// in the approval queue; a human sends every email. Chat channels
		// keep their bot — the stakes and the medium are different.
		runAI: false,
	}, true
}

// ingestInbound persists one inbound item and runs everything that must happen
// before the AI does: attachments, impersonation checks, triage, the dashboard
// broadcast and the push.
//
// Extracted so the mailbox poller can travel the same path as a webhook. It
// returns whether the AI should run rather than dispatching it, because the two
// callers differ on exactly that: a webhook acks the provider FIRST and answers
// afterwards, while a poller has nobody to ack to.
//
// ErrDuplicateInbound is returned unwrapped — both callers treat it as "already
// handled", and neither should log it as a failure.
func (h *InboundWebhookHandler) ingestInbound(
	ctx context.Context,
	channel *domain.Channel,
	channelToken string,
	it inboundItem,
) (*usecases.InboundPartialResult, bool, error) {
	partial, err := h.inboundUC.SaveInbound(ctx, channel, usecases.InboundMessageDTO{
		ChannelToken:      channelToken,
		SenderPhone:       it.from,
		SenderName:        it.senderName,
		SenderUsername:    it.senderUsername,
		MessageType:       it.msgType,
		Content:           it.content,
		MediaURL:          it.mediaURL,
		Metadata:          it.metadata,
		ExternalMessageID: it.externalMsgID,
		Subject:           it.emailSubject,
		ListUnsubscribe:   it.listUnsubscribe,
	})
	if err != nil {
		return nil, false, err
	}

	// Attachments go in after the message row exists (they reference
	// it). Best-effort and non-blocking for the reply: a file that
	// fails to store must not stop the AI from answering a customer who
	// is waiting, and the use case logs every drop.
	if len(it.attachments) > 0 && h.attachmentUC != nil && partial.Message != nil {
		h.attachmentUC.Store(ctx, partial.AccountID, partial.Message.ID, it.attachments)
	}

	// Impersonation checks. After the message exists so the assessment
	// can reference it, and never able to fail the delivery: refusing to
	// deliver mail because a registry lookup timed out would be a worse
	// product than not checking at all.
	var risk domain.EmailRiskAssessment
	if h.riskUC != nil && it.emailAuth != nil && partial.Message != nil {
		risk = h.riskUC.Assess(ctx, usecases.EmailRiskParams{
			AccountID:   partial.AccountID,
			MessageID:   partial.Message.ID,
			FromAddress: it.from,
			ReplyTo:     it.emailReplyTo,
			Auth:        *it.emailAuth,
			Body:        it.content,
		})
	}

	// Inbox triage (§6.10 camada 3). After the risk verdict, because a
	// message we believe is a scam must be filed as quarantine and not
	// as the ordinary supplier invoice it is dressed up as — that
	// mistake IS the fraud working.
	if h.triageUC != nil && partial.Message != nil {
		h.triageUC.Classify(ctx, usecases.TriageInput{
			AccountID:             partial.AccountID,
			ConversationID:        partial.ConversationID,
			ContactID:             partial.ContactID,
			ChannelID:             partial.ChannelID,
			Subject:               it.emailSubject,
			Body:                  it.content,
			Quarantined:           risk.Quarantined(),
			HasPaymentInstruction: domain.HasPaymentInstruction(domain.ExtractPaymentDetails(it.content)),
			ListUnsubscribe:       it.listUnsubscribe,
		})
	}

	// Broadcast inbound message immediately — agent sees the message before AI responds
	h.broadcastEvent(partial.AccountID, "new_message", partial.ConversationID, partial.Message)
	if partial.IsNewConversation && h.events != nil {
		h.events.Emit(partial.AccountID, "conversation.created", map[string]any{
			"conversation_id": partial.ConversationID,
			"channel_id":      partial.ChannelID,
			"contact_id":      partial.ContactID,
		})
	}

	// Fan out a web/mobile push so attendants whose dashboard tab is not
	// focused still get pinged. Runs off the request path; push failures
	// must never block the provider ack.
	dispatchInboundPushAsync(h.pushUC, partial)

	return partial, it.runAI && !partial.IsBotPaused, nil
}

// instagramSenderName resolves an IGSID to a display name via the Graph
// API, best-effort: any failure logs a WARN and returns "" so the
// pipeline degrades to today's behavior (contact shows the raw IGSID)
// instead of dropping the message. The per-request cache bounds a batch
// to one profile call per distinct sender — including negative results,
// so one failing IGSID can't stall the batch on repeated fetches.
func (h *InboundWebhookHandler) instagramSenderName(ctx context.Context, channel *domain.Channel, igsid string, cache map[string]string) string {
	if h.igAdapter == nil {
		return ""
	}
	if name, ok := cache[igsid]; ok {
		return name
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	name, err := h.igAdapter.FetchSenderName(fetchCtx, channel, igsid)
	if err != nil {
		slog.Warn("failed to resolve Instagram sender profile",
			"channel_id", channel.ID, "error", err)
		name = ""
	}
	cache[igsid] = name
	return name
}

// ackTwiML acknowledges a Twilio webhook with an empty TwiML document.
// Twilio parses the response body of a message webhook as TwiML: a JSON
// body logs error 12300 (invalid content-type) in the tenant's Twilio
// console on every inbound SMS. <Response/> means "no reply actions" —
// the actual reply goes out via the REST API on the async AI path.
func ackTwiML(c *gin.Context) {
	c.Data(http.StatusOK, "text/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?><Response/>`))
}

// webhookRequestURL reconstructs the absolute URL Twilio signed. Behind
// the nginx proxy the request arrives over plain HTTP, so the scheme
// comes from X-Forwarded-Proto (set in nginx/snippets/proxy.conf); the
// Host header survives the proxy hop (proxy_set_header Host $host).
func webhookRequestURL(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + c.Request.Host + c.Request.RequestURI
}

// resolveInstagramSecret picks the HMAC secret for an Instagram
// channel: the per-channel `app_secret` config key wins (tenant using
// their own Meta app), otherwise the global INSTAGRAM_APP_SECRET.
// Empty means "no validation possible" — the caller decides policy.
func resolveInstagramSecret(channel *domain.Channel, globalSecret string) string {
	if s := credentialString(channel, "app_secret"); s != "" {
		return s
	}
	return globalSecret
}

// resolveEmailWebhookSecret: the tenant's own provider secret first, then the
// platform's. Same shape as the Instagram resolver, and the key is named
// `webhook_secret` on purpose — domain.SensitiveConfigKey matches the
// `_secret` suffix, so it is encrypted at rest, masked on read and preserved
// across an edit that does not retype it, with no per-key handling to add.
func resolveEmailWebhookSecret(channel *domain.Channel, globalSecret string) string {
	if s := credentialString(channel, "webhook_secret"); s != "" {
		return s
	}
	return globalSecret
}

// HandleWebhookVerify godoc
// @Summary      Meta webhook verification (hub.challenge) for channel webhooks
// @Description  Meta calls this GET endpoint when the webhook subscription for an
//
//	Instagram channel is registered (App Dashboard → Webhooks → Instagram,
//	subscribe to the `messages` field). Callback URL is
//	/v1/webhooks/{channel_token} and the Verify Token the operator enters in
//	the Meta dashboard must be the SAME channel token. Echoes hub.challenge
//	on match.
//
// @Tags         webhooks
// @Produce      plain
// @Param        channel_token    path   string  true  "Unique webhook token for the channel"
// @Param        hub.mode         query  string  true  "subscribe"
// @Param        hub.challenge    query  string  true  "challenge token from Meta"
// @Param        hub.verify_token query  string  true  "must equal the channel token"
// @Success      200  {string}  string  "echo of hub.challenge"
// @Failure      403  {string}  string  "Forbidden"
// @Router       /webhooks/{channel_token} [get]
func (h *InboundWebhookHandler) HandleWebhookVerify(c *gin.Context) {
	channelToken := c.Param("channel_token")
	mode := c.Query("hub.mode")
	challenge := c.Query("hub.challenge")
	token := c.Query("hub.verify_token")

	// The channel must exist — a random token can't fish a 200 out of
	// this endpoint. 403 (not 404) for the unknown-channel case too, so
	// the response doesn't distinguish "no such channel" from "wrong
	// verify token" (no enumeration oracle).
	channel, err := h.channelRepo.GetByWebhookToken(c.Request.Context(), channelToken)
	if err != nil || channel == nil {
		c.String(http.StatusForbidden, "Forbidden")
		return
	}

	// Constant-time compare mirrors HandleVerify on the WhatsApp
	// handler: plain == short-circuits on the first mismatching byte
	// and leaks prefix info to a timing attacker.
	if mode == "subscribe" && subtle.ConstantTimeCompare([]byte(token), []byte(channel.WebhookToken)) == 1 {
		c.String(http.StatusOK, challenge)
		return
	}
	c.String(http.StatusForbidden, "Forbidden")
}

// avatarNeedsRefresh returns true when the stored Telegram avatar is
// missing or older than 24 hours. Kept as a named helper so both the
// handler code path and tests read the same threshold.
func avatarNeedsRefresh(fetchedAt *time.Time) bool {
	if fetchedAt == nil {
		return true
	}
	return time.Since(*fetchedAt) > 24*time.Hour
}

// refreshTelegramProfileAsync fetches the current profile photo + bio
// via the Bot API's getChat endpoint and persists them. Runs off the
// request path so the webhook ack stays fast; broadcasts a
// conversation_updated event so connected dashboards re-render the
// avatar + bio without a reload.
func (h *InboundWebhookHandler) refreshTelegramProfileAsync(channel *domain.Channel, accountID, conversationID, chatID string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("telegram profile refresh panic",
				"channel_id", channel.ID, "recover", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := h.tgAdapter.GetChatInfo(ctx, channel, chatID)
	if err != nil {
		slog.Debug("telegram getChat lookup failed",
			"channel_id", channel.ID, "chat_id", chatID, "error", err)
		return
	}
	if err := h.inboundUC.UpdateContactAvatar(ctx, conversationID, accountID, info.PhotoURL, info.Bio); err != nil {
		slog.Warn("failed to persist refreshed telegram profile",
			"channel_id", channel.ID, "conversation_id", conversationID, "error", err)
		return
	}
	// Let connected dashboards swap the avatar/bio without a page refresh.
	payload, _ := json.Marshal(map[string]interface{}{
		"event":              "conversation_updated",
		"conversation_id":    conversationID,
		"contact_avatar_url": info.PhotoURL,
		"contact_bio":        info.Bio,
	})
	if h.hub != nil {
		h.hub.BroadcastToAccount(accountID, payload)
	}
}

// processAIAsync runs the AI pipeline off the request path so the webhook
// ack is fast. Uses a detached context so gin finishing the response does
// not cancel mid-flight model work.
func (h *InboundWebhookHandler) processAIAsync(channel *domain.Channel, partial *usecases.InboundPartialResult, recipient, content string) {
	h.processAIAsyncCtx(context.Background(), channel, partial, recipient, content)
}

// processAIAsyncCtx is the same work rooted at a caller-supplied context, so a
// caller that has one (the mailbox poller) keeps its trace and logging values
// instead of starting a background context that no log line can be tied back to.
//
// The parent must already be detached from any cancellation the caller is about
// to trigger — see the call site.
func (h *InboundWebhookHandler) processAIAsyncCtx(parent context.Context, channel *domain.Channel, partial *usecases.InboundPartialResult, recipient, content string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("inbound async AI panic", "channel_id", channel.ID, "recover", r)
		}
	}()

	bgCtx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()

	h.broadcastTyping(partial.AccountID, partial.ConversationID, true)
	defer h.broadcastTyping(partial.AccountID, partial.ConversationID, false)

	aiCtx, aiCancel := context.WithTimeout(bgCtx, 60*time.Second)
	aiResult := h.inboundUC.ProcessAI(aiCtx, partial, content)
	aiCancel()

	h.deliverAIResult(bgCtx, channel, partial, recipient, aiResult)
}

// deliverAIResult broadcasts and sends whatever the AI pipeline produced.
//
// Extracted so the DEFERRED path (K1: a reply generated minutes later by the
// debounce worker) goes through the exact same broadcast, send-switch and
// send-failure handling as an immediate one. A second copy of this switch is how
// one channel type silently stops reporting failed sends.
func (h *InboundWebhookHandler) deliverAIResult(
	ctx context.Context,
	channel *domain.Channel,
	partial *usecases.InboundPartialResult,
	recipient string,
	aiResult *usecases.ProcessAIResult,
) {
	if aiResult != nil && aiResult.HandoffRequested {
		// Silent hand-off: the agent is in SilentOnUnknown mode and
		// chose not to answer. Surface the thread to the dashboard so
		// an attendant picks it up; the customer sees no bot message.
		h.broadcastHandoff(partial.AccountID, partial.ConversationID)
		// Push a distinct notification on top of the inbound push so
		// the attendant's OS-level notification reads "AI handed off"
		// instead of looking like a normal new-message ping. Re-uses
		// the conversation tag so the OS coalesces the two pushes.
		contactName := ""
		if partial.Message != nil && partial.Message.SenderName != nil {
			contactName = *partial.Message.SenderName
		}
		dispatchHandoffPushAsync(h.pushUC, partial.AccountID, partial.ConversationID, contactName)
		return
	}
	if aiResult == nil || aiResult.Message == nil {
		return
	}
	aiMsg := aiResult.Message

	// Broadcast AI response to the dashboard
	h.broadcastEvent(partial.AccountID, "new_message", partial.ConversationID, aiMsg)

	if aiResult.LimitReached {
		// Usage limit hit — dashboard-only notice. Skip external send.
		h.broadcastUsageLimitReached(partial.AccountID, partial.ConversationID)
		return
	}

	if aiResult.UsageAlert != nil {
		h.broadcastUsageAlert(partial.AccountID, aiResult.UsageAlert)
	}

	outMsg := &domain.Message{Content: aiMsg.Content, Type: aiMsg.Type, MediaURL: aiMsg.MediaURL}
	if outMsg.Type == "" {
		outMsg.Type = "text"
	}
	var sendErr error
	switch channel.Type {
	case "telegram":
		sendErr = h.tgAdapter.SendMessage(ctx, channel, recipient, outMsg)
	case "instagram":
		sendErr = h.igAdapter.SendMessage(ctx, channel, recipient, outMsg)
	case "sms":
		sendErr = h.smsAdapter.SendMessage(ctx, channel, recipient, outMsg)
	default:
		sendErr = h.waAdapter.SendMessage(ctx, channel, recipient, outMsg)
	}
	if sendErr != nil {
		// The dashboard already saw an optimistic "new_message" for
		// this row above — without a follow-up event the attendant
		// thinks the customer received the reply. Stamp the failure
		// on the DB row and broadcast a status event so the bubble
		// flips to "send failed" without a manual refresh.
		reason := sendErr.Error()
		slog.Error("failed to send AI reply via channel",
			"channel_type", channel.Type, "channel_id", channel.ID,
			"message_id", aiMsg.ID, "error", sendErr)
		if h.msgRepo != nil {
			if mErr := h.msgRepo.MarkSendFailed(ctx, aiMsg.ID, reason); mErr != nil {
				slog.Warn("failed to mark message as send-failed",
					"message_id", aiMsg.ID, "error", mErr)
			}
		}
		h.broadcastSendFailed(partial.AccountID, partial.ConversationID, aiMsg.ID, reason)
	}
}

// broadcastSendFailed tells the dashboard that an outbound message it
// already rendered optimistically actually failed at the channel send
// step. The bubble can flip to a "send failed — tap to retry" state.
func (h *InboundWebhookHandler) broadcastSendFailed(accountID, conversationID, messageID, reason string) {
	if h.hub == nil {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"event":           "message_send_failed",
		"conversation_id": conversationID,
		"message_id":      messageID,
		"failed_reason":   reason,
	})
	if err != nil {
		slog.Warn("failed to marshal message_send_failed payload",
			"conversation_id", conversationID, "message_id", messageID, "error", err)
		return
	}
	h.hub.BroadcastToAccount(accountID, payload)
}

// broadcastEvent sends a named WebSocket event to all dashboard clients for the account.
func (h *InboundWebhookHandler) broadcastEvent(accountID, event, conversationID string, msg *domain.Message) {
	if h.hub != nil {
		payload, err := json.Marshal(map[string]interface{}{
			"event":           event,
			"conversation_id": conversationID,
			"message":         msg,
		})
		if err != nil {
			slog.Warn("failed to marshal broadcast event", "error", err)
		} else {
			h.hub.BroadcastToAccount(accountID, payload)
		}
	}
	// Mirror to outbound integrations. The WS event names are
	// dashboard-internal ("new_message"); the public stream uses
	// stable strings ("message.received" / "message.sent") so renaming
	// the WS layer can never break tenant integrations.
	if h.events != nil && msg != nil {
		evt := EventMessageReceived
		if msg.Direction == "outbound" {
			evt = EventMessageSent
		}
		h.events.Emit(accountID, evt, map[string]any{
			"conversation_id": conversationID,
			"message":         msg,
		})
	}
}

// broadcastTyping sends a typing/typing_stopped event to dashboard clients.
func (h *InboundWebhookHandler) broadcastTyping(accountID, conversationID string, isTyping bool) {
	if h.hub == nil {
		return
	}
	event := "typing_stopped"
	if isTyping {
		event = "typing"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event":           event,
		"conversation_id": conversationID,
	})
	h.hub.BroadcastToAccount(accountID, payload)
}

// broadcastHandoff tells dashboard clients that the bot yielded on a
// silent-handoff agent. The conversation is already marked bot-paused
// server-side; this event is what makes the tile pulse/highlight so
// an attendant notices.
func (h *InboundWebhookHandler) broadcastHandoff(accountID, conversationID string) {
	if h.hub == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event":           "human_handoff_requested",
		"conversation_id": conversationID,
	})
	h.hub.BroadcastToAccount(accountID, payload)
}

// broadcastUsageLimitReached sends a usage_limit_reached event to dashboard clients.
func (h *InboundWebhookHandler) broadcastUsageLimitReached(accountID, conversationID string) {
	if h.hub == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event":           "usage_limit_reached",
		"conversation_id": conversationID,
	})
	h.hub.BroadcastToAccount(accountID, payload)
}

// broadcastUsageAlert sends a usage_alert event to dashboard clients when usage crosses a threshold.
func (h *InboundWebhookHandler) broadcastUsageAlert(accountID string, alert *usecases.UsageAlertInfo) {
	if h.hub == nil || alert == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event":           "usage_alert",
		"percent_used":    alert.PercentUsed,
		"tokens_used":     alert.TokensUsed,
		"effective_limit": alert.EffectiveLimit,
	})
	h.hub.BroadcastToAccount(accountID, payload)
}

// InboundPayload is the normalized inbound message format (WhatsApp / legacy)
type InboundPayload struct {
	From     string          `json:"from"`
	Type     string          `json:"type"`
	Content  string          `json:"content"`
	Metadata json.RawMessage `json:"metadata"`
}

// maxInboundBodyBytes is the per-channel cap on a webhook body.
//
// Email is the outlier by an order of magnitude: providers inline attachments
// as base64, which inflates by ~33%, so a single 25MB document arrives as
// ~34MB of JSON. The old flat 1MB silently truncated those payloads into
// unparseable JSON — the message was acked as "ignored" and the attachment
// vanished with no error anywhere.
//
// 40MB leaves headroom over MaxAttachmentBytes (25MB) for the base64 overhead
// plus the message body and headers. Everything else keeps the original 1MB:
// a Telegram update or a Twilio form has no business being larger, and a
// generous cap on those is just a cheaper way to exhaust memory.
func maxInboundBodyBytes(channelType string) int64 {
	if channelType == "email" {
		return 40 << 20
	}
	return 1 << 20
}

// emailInboundMetadata merges the threading headers with the sender's
// authentication verdict and the original markup.
//
// The verdict rides on the message rather than in its own column because it
// is display-and-audit data, not something anything queries or filters on.
// It is recorded even when every check came back unknown: "nobody checked"
// has to be visible, since the alternative is a UI that silently reads as
// verified.
func emailInboundMetadata(parsed emailchannel.InboundEmail, htmlPath string) []byte {
	meta := map[string]any{}
	if raw := parsed.ThreadMetadata(); len(raw) > 0 {
		if err := json.Unmarshal(raw, &meta); err != nil {
			// Threading metadata is built by us, so this cannot normally
			// fail; losing the auth verdict because of it would be worse
			// than losing the thread link.
			slog.Warn("email thread metadata unreadable", "error", err)
			meta = map[string]any{}
		}
	}
	meta["email_auth"] = map[string]any{
		"spf":         string(parsed.Auth.SPF),
		"dkim":        string(parsed.Auth.DKIM),
		"dmarc":       string(parsed.Auth.DMARC),
		"trustworthy": parsed.Auth.Trustworthy(),
		"suspicious":  parsed.Auth.Suspicious(),
		"raw":         parsed.Auth.Raw,
	}
	// Where a human answer should GO (not a threading header). Read back
	// by usecases.EmailReplyRecipient on the reply path.
	if parsed.ReplyTo != "" {
		meta[emailchannel.MetaReplyTo] = parsed.ReplyTo
	}
	if parsed.HadHTMLOnly {
		meta["email_html_only"] = true
	}
	// Present only when the markup was actually stored. The client shows
	// "view original" off this key, so an absent key means the button does
	// not appear rather than appearing and failing.
	if htmlPath != "" {
		meta["email_html_path"] = htmlPath
	}
	out, err := json.Marshal(meta)
	if err != nil {
		slog.Error("could not encode email metadata", "error", err)
		return parsed.ThreadMetadata()
	}
	return out
}

// SetInboxTriageUseCase wires inbox categorisation.
func (h *InboundWebhookHandler) SetInboxTriageUseCase(uc *usecases.InboxTriageUseCase) {
	h.triageUC = uc
}

// SetBounceUseCase wires delivery-failure handling.
func (h *InboundWebhookHandler) SetBounceUseCase(uc *usecases.EmailBounceUseCase) {
	h.bounceUC = uc
}

// recordBounce handles a delivery failure and acks.
//
// Always 200, even when nothing is wired: a webhook that errors gets retried
// by the provider forever, and no retry can supply a missing use case.
func (h *InboundWebhookHandler) recordBounce(c *gin.Context, channel *domain.Channel, b domain.EmailBounce) {
	h.recordBounceCtx(c.Request.Context(), channel, b)
	c.JSON(http.StatusOK, gin.H{"status": "bounce_recorded", "kind": string(b.Kind)})
}

// DispatchDebounced delivers a reply the debounce worker generated (K1).
//
// Implements usecases.DebouncedDispatcher. The worker owns the generation but not
// the sending — the channel adapters live here — so the result comes back through
// this seam and joins the same delivery path as an immediate reply.
//
// The recipient is the conversation's contact external id, which is what the
// inbound pipeline stores and what every adapter's SendMessage expects.
func (h *InboundWebhookHandler) DispatchDebounced(
	ctx context.Context,
	partial *usecases.InboundPartialResult,
	result *usecases.ProcessAIResult,
) {
	if partial == nil || result == nil {
		return
	}
	channel, err := h.channelRepo.GetByID(ctx, partial.ChannelID)
	if err != nil || channel == nil {
		// The reply is already persisted and broadcast to the dashboard by the
		// pipeline; without a channel it simply cannot leave. Logged at ERROR
		// because from the customer's side this is a message that was never
		// answered.
		slog.ErrorContext(ctx, "debounced reply: channel gone, cannot deliver",
			"channel_id", partial.ChannelID,
			"conversation_id", partial.ConversationID, "error", err)
		return
	}
	if strings.TrimSpace(partial.ContactID) == "" {
		slog.ErrorContext(ctx, "debounced reply: no recipient on the conversation",
			"conversation_id", partial.ConversationID)
		return
	}
	h.deliverAIResult(ctx, channel, partial, partial.ContactID, result)
}

// NotifyDebounce tells the dashboard when the AI will answer (K1).
//
// Implements usecases.DebounceNotifier. A zero respondAt means the wait is over:
// the banner clears rather than counting down against a moment that already
// passed.
func (h *InboundWebhookHandler) NotifyDebounce(
	accountID, conversationID string, respondAt time.Time, messageCount int,
) {
	if h.hub == nil {
		return
	}
	payload := map[string]any{
		"event":           "ai_debounce",
		"conversation_id": conversationID,
		"message_count":   messageCount,
	}
	if !respondAt.IsZero() {
		payload["responds_at"] = respondAt.UTC().Format(time.RFC3339)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("failed to marshal ai_debounce payload",
			"conversation_id", conversationID, "error", err)
		return
	}
	h.hub.BroadcastToAccount(accountID, encoded)
}
