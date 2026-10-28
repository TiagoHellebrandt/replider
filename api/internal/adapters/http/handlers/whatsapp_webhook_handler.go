package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/gin-gonic/gin"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/observability"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/replider/api/internal/core/usecases"
)

func strPtr(s string) *string { return &s }

// WhatsAppWebhookHandler handles the single global WhatsApp Cloud API webhook.
// As a Tech Provider/BSP, Meta sends ALL messages to one URL; we route by phone_number_id.
type WhatsAppWebhookHandler struct {
	inboundUC    *usecases.InboundWebhookUseCase
	hub          *Hub
	channelRepo  domain.ChannelRepository
	campaignRepo domain.CampaignRepository // optional — nil disables campaign status correlation
	msgRepo      domain.MessageRepository  // optional — nil disables status persistence on outbound rows
	outbox       domain.WAWebhookOutbox    // optional — nil disables raw payload persistence + replay
	events       EventEmitter              // optional — fans out platform events to tenant outbound webhooks
	waAdapter    *waadapter.WhatsAppAdapter
	transcriber  domain.Transcriber    // optional — nil means audio comes through without transcription
	describer    domain.ImageDescriber // optional — nil means images come through without description
	gcsAdapter   *storage.GCSAdapter   // optional — nil means inbound audio keeps the (unplayable) Meta URL
	appSecret    string
	verifyToken  string
	// secretResolver lets BSP / Solution Partner deployments swap the
	// HMAC secret per-WABA. When unset the global appSecret is used
	// for every payload — the single-app default that fits Replider's
	// current single-tenant model. Returning an empty string from the
	// resolver falls back to appSecret too, so a partial setup works.
	secretResolver func(wabaID string) string

	// asyncWG tracks in-flight reply goroutines so a graceful shutdown
	// (SIGTERM during deploy) can wait for AI/send work to finish before
	// the process exits. Without this the API would ack 200 to Meta and
	// then kill the in-flight outbound — the customer's reply would be
	// silently lost with no replay path (the outbox row was already
	// marked processed).
	asyncWG      sync.WaitGroup
	shuttingDown atomic.Bool

	// pushUC, when set, fans out a Web Push to the account's attendants
	// every time a customer message lands. Optional — nil falls back to
	// "ws-only" mode used by tests and dev environments without VAPID.
	// Symmetrical to InboundWebhookHandler.pushUC; previously only the
	// generic Telegram/Instagram path triggered push, so WhatsApp users
	// got dashboard updates but no OS-level notifications.
	pushUC *usecases.PushNotificationUseCase
}

// SetPushUseCase wires the push fan-out for inbound WhatsApp messages.
// Optional — when unset, push delivery is silently skipped.
func (h *WhatsAppWebhookHandler) SetPushUseCase(uc *usecases.PushNotificationUseCase) {
	h.pushUC = uc
}

// EventEmitter is the minimal contract the dispatcher exposes to fan
// out platform events to tenant-managed outbound webhooks (Zapier,
// n8n, custom CRMs). Defined here as a tiny interface to avoid an
// adapter→service import that would invert clean-arch direction.
type EventEmitter interface {
	Emit(accountID, event string, payload any)
}

// SetEventEmitter wires the outbound webhook dispatcher. Without it,
// inbound/outbound events still fan out via WebSocket to the
// dashboard but third-party integrations don't fire.
func (h *WhatsAppWebhookHandler) SetEventEmitter(e EventEmitter) {
	h.events = e
}

// SetOutbox enables raw payload persistence to the wa_webhook_events
// table. With it, every signed webhook is recorded BEFORE processing,
// and the outbox worker can replay rows that failed mid-flight (DB
// blip, AI quota, transcoding crash). Without it, the handler still
// works but loses the safety net.
func (h *WhatsAppWebhookHandler) SetOutbox(o domain.WAWebhookOutbox) {
	h.outbox = o
}

// SetMessageRepo enables persistence of Cloud API status updates onto
// outbound message rows (delivered/read timestamps, failure reason,
// conversation/pricing envelope). Without it the dashboard can't show
// delivery ticks or compute the 24h window.
func (h *WhatsAppWebhookHandler) SetMessageRepo(r domain.MessageRepository) {
	h.msgRepo = r
}

// SetTranscriber wires a speech-to-text adapter. When set, inbound audio
// and voice messages are transcribed before the AI pipeline sees them.
// Without it, the bot receives an empty content and falls back to "não
// consegui entender o áudio".
func (h *WhatsAppWebhookHandler) SetTranscriber(t domain.Transcriber) {
	h.transcriber = t
}

// SetSecretResolver wires a per-WABA app-secret resolver. Used by
// Solution Partner / BSP deployments where each tenant's WABA is
// connected to a different Meta App and therefore signs webhook
// payloads with a different secret. The resolver is consulted on
// every inbound POST; returning "" falls back to the global secret
// (single-app deployment, the Replider default).
func (h *WhatsAppWebhookHandler) SetSecretResolver(fn func(wabaID string) string) {
	h.secretResolver = fn
}

// SetImageDescriber wires a vision adapter. When set, inbound images
// without a caption are described in one sentence (in Portuguese) and
// that description becomes the message content fed to the AI pipeline.
// This is critical for e-commerce use cases where customers routinely
// send product photos as the opener.
func (h *WhatsAppWebhookHandler) SetImageDescriber(d domain.ImageDescriber) {
	h.describer = d
}

// SetGCSAdapter wires the storage adapter used to re-host inbound audio.
// Meta media URLs are short-lived (~15 min) and require the WABA access
// token to download, so the dashboard's audio player cannot fetch them.
// When set, inbound audio is copied to GCS and the message stores the
// object path, which HandleListMessages signs into a playable URL.
func (h *WhatsAppWebhookHandler) SetGCSAdapter(g *storage.GCSAdapter) {
	h.gcsAdapter = g
}

func NewWhatsAppWebhookHandler(
	inboundUC *usecases.InboundWebhookUseCase,
	hub *Hub,
	channelRepo domain.ChannelRepository,
	campaignRepo domain.CampaignRepository,
	waAdapter *waadapter.WhatsAppAdapter,
	appSecret string,
	verifyToken string,
) *WhatsAppWebhookHandler {
	return &WhatsAppWebhookHandler{
		inboundUC:    inboundUC,
		hub:          hub,
		channelRepo:  channelRepo,
		campaignRepo: campaignRepo,
		waAdapter:    waAdapter,
		appSecret:    appSecret,
		verifyToken:  verifyToken,
	}
}

// HandleVerify godoc
// @Summary      WhatsApp webhook verification (Meta challenge)
// @Description  Meta calls this GET endpoint when registering the webhook subscription.
//
//	Returns hub.challenge to confirm ownership.
//
// @Tags         webhooks
// @Produce      plain
// @Param        hub.mode         query  string  true  "subscribe"
// @Param        hub.challenge    query  string  true  "challenge token from Meta"
// @Param        hub.verify_token query  string  true  "must match WHATSAPP_VERIFY_TOKEN env var"
// @Success      200  {string}  string  "echo of hub.challenge"
// @Failure      403  {string}  string  "Forbidden"
// @Router       /webhooks/whatsapp [get]
func (h *WhatsAppWebhookHandler) HandleVerify(c *gin.Context) {
	mode := c.Query("hub.mode")
	challenge := c.Query("hub.challenge")
	token := c.Query("hub.verify_token")

	// Constant-time compare on the verify token. Plain `==` short-circuits
	// on the first byte mismatch and leaks length/prefix info to a
	// network-positioned attacker timing the response. Operators set
	// WHATSAPP_VERIFY_TOKEN themselves, so we can't assume it's
	// high-entropy — defense in depth.
	if mode == "subscribe" && subtle.ConstantTimeCompare([]byte(token), []byte(h.verifyToken)) == 1 {
		c.String(http.StatusOK, challenge)
		return
	}
	c.String(http.StatusForbidden, "Forbidden")
}

// HandleInbound godoc
// @Summary      WhatsApp inbound messages (Tech Provider / BSP)
// @Description  Receives ALL WhatsApp Business messages for the app. Routes each message to
//
//	the correct channel by matching phone_number_id in the payload to a channel
//	in the database. Verifies X-Hub-Signature-256 using WHATSAPP_APP_SECRET.
//	Runs the same two-phase flow as the generic inbound webhook.
//	Always returns HTTP 200 to Meta regardless of processing outcome.
//
// @Tags         webhooks
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /webhooks/whatsapp [post]
// maxWhatsAppWebhookBody is the upper bound on the raw body Meta can
// post. Meta's docs cap webhook payloads at ~5MB (template-review with
// inline media, 100-message batches with media metadata, large catalog
// events). The previous 1MB cap silently 4xx'd legitimate large
// payloads — and the HMAC check downstream would reject the truncated
// body anyway, leaving the customer with a Meta-side retry loop.
const maxWhatsAppWebhookBody = 5 << 20

func (h *WhatsAppWebhookHandler) HandleInbound(c *gin.Context) {
	// LimitReader silently truncates; the trailing 1-byte probe lets us
	// detect when the actual payload exceeds the limit and reject it
	// with a clear 413 instead of running HMAC against the truncated
	// body (which always fails and looks like a signature error to ops).
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWhatsAppWebhookBody+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_PAYLOAD", Message: "failed to read body"})
		return
	}
	defer c.Request.Body.Close()
	if len(body) > maxWhatsAppWebhookBody {
		slog.Warn("WhatsApp webhook body exceeds size cap",
			"size", len(body), "cap", maxWhatsAppWebhookBody)
		observability.WhatsAppWebhookRejected.WithLabelValues("oversize").Inc()
		c.JSON(http.StatusRequestEntityTooLarge, domain.AppError{
			Code:    "PAYLOAD_TOO_LARGE",
			Message: "webhook body exceeds size cap",
		})
		return
	}

	// Verify HMAC signature — app secret must be configured.
	//
	// Per-WABA secret resolution: a BSP-mode deployment (Solution
	// Partner) hosts multiple Meta Apps and each WABA's webhook is
	// signed with the matching app's secret. We peek at entry[0].id
	// (which equals the WABA ID per Meta's spec) and ask the
	// resolver for the right secret; falls back to the global
	// appSecret when the resolver is unset or returns "".
	secret := h.appSecret
	if h.secretResolver != nil {
		var preview struct {
			Entry []struct {
				ID string `json:"id"`
			} `json:"entry"`
		}
		if err := json.Unmarshal(body, &preview); err == nil && len(preview.Entry) > 0 {
			if perWABA := h.secretResolver(preview.Entry[0].ID); perWABA != "" {
				secret = perWABA
			}
		}
	}
	if secret == "" {
		observability.WhatsAppWebhookRejected.WithLabelValues("missing_secret").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "webhook secret not configured"})
		return
	}
	sig := c.GetHeader("X-Hub-Signature-256")
	if !h.waAdapter.VerifySignature(body, sig, secret) {
		observability.WhatsAppWebhookRejected.WithLabelValues("signature").Inc()
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Replay-attack guard. HMAC alone doesn't bind a payload to a
	// point in time — once a body + signature pair leaks (proxy log,
	// MITM, captured network trace), an attacker can re-POST it
	// forever and we'd happily fan out the side effects each time.
	// Dedupe via external_message_id covers re-delivery of the same
	// message, but not events without a wamid (template/quality)
	// nor crafted payloads with reused wamids on a different account.
	// Bound the freshness via Meta's `entry.time` so anything older
	// than MaxWebhookAge is rejected outright.
	if ts := waadapter.EntryTimestampUnix(body); ts > 0 {
		ageSeconds := time.Now().Unix() - ts
		if ageSeconds > waadapter.MaxWebhookAge {
			slog.Warn("WhatsApp webhook rejected: entry.time too old",
				"entry_time", ts, "age_seconds", ageSeconds)
			observability.WhatsAppWebhookRejected.WithLabelValues("replay").Inc()
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	// Persist the raw payload to the outbox before any side effects so
	// transient failures (parser bug, AI quota, DB blip during fanout)
	// leave a replayable trail. The outcome is marked at the end of
	// every return path via the helpers (h.markOutbox*).
	eventID := h.recordOutbox(c.Request.Context(), body, sig)
	// Default-failed semantics: when none of the success paths mark the
	// row before return, the deferred call records the outcome as
	// failed so the worker re-attempts on the next tick.
	processed := false
	skipped := false
	failureMsg := ""
	// asyncOwnsOutbox is set when at least one inbound message was
	// dispatched into a goroutine that runs the AI + send pipeline.
	// In that case the outbox row stays in `pending` until the async
	// work finishes — the goroutine itself marks it. Without this, the
	// inline defer used to mark `processed` while the goroutine was
	// still mid-flight, so a subsequent crash mid-send had no replay
	// path because the outbox already considered the row done.
	asyncOwnsOutbox := false
	defer func() {
		if h.outbox == nil || eventID == "" {
			return
		}
		if asyncOwnsOutbox {
			// async goroutine carries the eventID and marks the row
			// Processed / Failed when it finishes.
			return
		}
		ctx := context.Background()
		switch {
		case processed:
			_ = h.outbox.MarkProcessed(ctx, eventID)
		case skipped:
			_ = h.outbox.MarkSkipped(ctx, eventID)
		default:
			if failureMsg == "" {
				failureMsg = "handler returned without marking the outbox row"
			}
			_ = h.outbox.MarkFailed(ctx, eventID, failureMsg)
		}
	}()

	// Process delivery status updates (sent/delivered/read/failed).
	// We do NOT early-return after statuses — Meta has been observed
	// to coalesce status + message changes into a single webhook
	// delivery. Returning early after statuses dropped any messages
	// in the same batch; flow now continues into the inbound parser
	// after the status loop.
	reqCtx := c.Request.Context()
	statuses := waadapter.ParseStatuses(body)
	if len(statuses) > 0 {
		for _, s := range statuses {
			// Resolve channel to get account_id for WebSocket broadcast
			ch, _ := h.channelRepo.GetByPhoneNumberID(reqCtx, s.PhoneNumberID)
			if ch != nil {
				// Persist Meta's status envelope on the outbound row so
				// the dashboard can show delivered/read ticks, the 24h
				// window indicator, and the failure reason — and so the
				// billing rollup has accurate conversation_origin /
				// pricing_category data per outbound message.
				h.persistWhatsAppStatus(reqCtx, ch, s)
				if h.hub != nil {
					payload, _ := json.Marshal(map[string]interface{}{
						"event":         "message_status",
						"message_id":    s.MessageID,
						"status":        s.Status,
						"recipient":     s.Recipient,
						"failed_reason": s.FailedReason,
					})
					h.hub.BroadcastToAccount(ch.AccountID, payload)
				}
			}
			// Correlate with broadcast campaigns if a recipient row was
			// recorded with this wamid. Best-effort — we skip if the repo
			// is not configured (e.g. e2e tests) or if no row matches
			// (regular outbound replies don't go through campaigns).
			h.applyCampaignStatus(reqCtx, s.MessageID, s.Status)
			observability.WhatsAppStatusUpdates.WithLabelValues(s.PhoneNumberID, s.Status).Inc()
			slog.Debug("WhatsApp status update", "message_id", s.MessageID, "status", s.Status)
		}
		processed = true
		// Fall through to the inbound + meta-event handlers — Meta can
		// merge statuses and messages into a single webhook delivery
		// and we want to process both. The processed flag stays true
		// so the outbox marks this as Processed even when there is no
		// inbound to follow.
	}

	// Non-message webhook fields (template status, quality, review,
	// account capability changes, etc.). Until now these were dropped
	// silently — customers couldn't tell when a template was approved
	// or when their number got flagged. Dispatch each known field;
	// log the rest at INFO so ops at least sees them in the timeline.
	events := waadapter.ParseEvents(body)
	handledMeta := false
	for _, ev := range events {
		if ev.IsMessageEvent() {
			continue
		}
		handledMeta = true
		h.handleMetaEvent(c, ev)
	}

	// Meta delivers up to 100 messages per webhook POST under
	// `entry[].changes[].value.messages[]`. ParseInbound returns just the
	// first; ParseInboundAll surfaces the whole batch so we don't drop
	// messages during burst traffic. Each message is processed
	// independently — one failing won't stall the rest.
	parsedAll := waadapter.ParseInboundAll(body)
	for _, p := range parsedAll {
		// Inbound metric per phone_number_id + type. Empty PhoneNumberID
		// (rare — Meta test webhook with phone_number_id="123456123")
		// still gets counted under the empty label.
		observability.WhatsAppInboundTotal.WithLabelValues(p.PhoneNumberID, p.MessageType).Inc()
	}
	if len(parsedAll) == 0 {
		// Either status-only / event-only / unrecognized — ack 200
		// either way so Meta stops retrying. Pick the most informative
		// label for the response, preserving the `processed` flag that
		// upstream branches (status loop, meta events) may have set.
		var status string
		switch {
		case processed && len(statuses) > 0:
			status = "statuses_processed"
		case handledMeta:
			status = "meta_event_processed"
			processed = true
		default:
			status = "ignored"
			skipped = true
		}
		c.JSON(http.StatusOK, gin.H{"status": status})
		return
	}

	// One tracker per webhook so all async goroutines coordinate
	// their outbox marking through a single counter. Counter starts
	// at 0 — each spawnAsyncReply increments it before launching the
	// goroutine, and the last goroutine to finish writes the final
	// outbox state.
	tracker := &outboxAsyncTracker{eventID: eventID, outbox: h.outbox}

	anyProcessed := false
	allDuplicate := true
	firstFailure := ""
	conversationIDs := make([]string, 0, len(parsedAll))
	for _, parsed := range parsedAll {
		outcome := h.processInboundMessage(c, parsed, tracker)
		if outcome.processed {
			anyProcessed = true
			allDuplicate = allDuplicate && false
			if outcome.conversationID != "" {
				conversationIDs = append(conversationIDs, outcome.conversationID)
			}
		}
		if outcome.asyncSpawned {
			asyncOwnsOutbox = true
		}
		if outcome.duplicate {
			continue
		}
		allDuplicate = false
		if outcome.failureMsg != "" && firstFailure == "" {
			firstFailure = outcome.failureMsg
		}
	}

	switch {
	case anyProcessed:
		processed = true
		// Keep conversation_id (first one) for backwards-compat with the
		// existing test/client surface; add conversation_ids for the
		// batch case so consumers can fan out properly.
		firstConvID := ""
		if len(conversationIDs) > 0 {
			firstConvID = conversationIDs[0]
		}
		c.JSON(http.StatusOK, gin.H{
			"status":           "processed",
			"conversation_id":  firstConvID,
			"conversation_ids": conversationIDs,
		})
	case allDuplicate:
		skipped = true
		c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
	case firstFailure != "":
		failureMsg = firstFailure
		c.JSON(http.StatusOK, gin.H{"status": "error"})
	default:
		skipped = true
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
	}
}

// outboxAsyncTracker coordinates outbox marking when one webhook
// spawns N async reply goroutines. The inline handler skips its
// deferred mark when at least one async ran; each goroutine then
// decrements the counter on completion and the LAST one writes
// MarkProcessed (or MarkFailed if any goroutine reported failure).
//
// Without this, the inline defer marked Processed before the
// goroutines finished — a subsequent send failure had no replay
// path because the row was already considered done.
type outboxAsyncTracker struct {
	eventID string
	pending atomic.Int32
	anyErr  atomic.Bool
	outbox  domain.WAWebhookOutbox
}

// SignalDone is called by each async goroutine when its work
// completes (success OR failure). The tracker writes the final outbox
// state when the last in-flight goroutine signals.
func (t *outboxAsyncTracker) SignalDone(failed bool, reason string) {
	if t == nil {
		return
	}
	if failed {
		t.anyErr.Store(true)
	}
	if t.pending.Add(-1) > 0 {
		return
	}
	if t.outbox == nil || t.eventID == "" {
		return
	}
	ctx := context.Background()
	if t.anyErr.Load() {
		msg := reason
		if msg == "" {
			msg = "async reply send failed"
		}
		_ = t.outbox.MarkFailed(ctx, t.eventID, msg)
		return
	}
	_ = t.outbox.MarkProcessed(ctx, t.eventID)
}

// inboundMessageOutcome reports the result of processing a single
// message inside a webhook batch. duplicate is split out from skipped
// so the batch-level response can still report "duplicate" when every
// message in the batch was already seen.
type inboundMessageOutcome struct {
	processed      bool
	duplicate      bool
	failureMsg     string
	conversationID string
	// asyncSpawned is true when the per-message pipeline kicked off
	// the AI + send goroutine. The batch caller uses this to decide
	// whether the deferred outbox marker fires inline or is delegated
	// to the goroutine, so the row stays in `pending` until the
	// outbound side actually completes.
	asyncSpawned bool
}

// processInboundMessage runs the per-message pipeline (channel lookup,
// media resolution, transcription/description, persistence, broadcast,
// detached AI). Errors and dedupe are reported via the returned outcome
// rather than mutating the gin response; the batch caller decides on
// the final HTTP envelope after every message has been seen.
func (h *WhatsAppWebhookHandler) processInboundMessage(c *gin.Context, parsed *waadapter.ParsedInbound, tracker *outboxAsyncTracker) inboundMessageOutcome {
	return h.processInboundMessageCtx(c.Request.Context(), parsed, tracker)
}

// processInboundMessageCtx is the context-only twin of processInboundMessage
// so the outbox-replay worker (which has no gin.Context) can run the same
// per-message pipeline as the inline handler. Both used to be duplicated;
// keeping a single source of truth means a fix to inline processing also
// applies to replay.
func (h *WhatsAppWebhookHandler) processInboundMessageCtx(ctx context.Context, parsed *waadapter.ParsedInbound, tracker *outboxAsyncTracker) inboundMessageOutcome {
	channel, err := h.channelRepo.GetByPhoneNumberID(ctx, parsed.PhoneNumberID)
	if err != nil || channel == nil {
		// Meta sends a canned test payload with phone_number_id "123456123"
		// when you hit "Test" in the webhook config or immediately after
		// registering a callback URL. It will never match a real channel;
		// logging at WARN just pollutes the feed.
		if parsed.PhoneNumberID == "123456123" {
			slog.Debug("ignored WhatsApp test webhook", "phone_number_id", parsed.PhoneNumberID)
		} else {
			slog.Warn("no active channel found for phone_number_id", "phone_number_id", parsed.PhoneNumberID)
		}
		return inboundMessageOutcome{}
	}

	mediaURL := ""
	if parsed.MediaID != "" {
		u, err := h.waAdapter.GetMediaURL(ctx, channel, parsed.MediaID)
		if err != nil {
			slog.Error("failed to resolve WhatsApp media URL", "channel_id", channel.ID, "error", err)
		} else {
			mediaURL = u
		}
	} else if parsed.LatLon != "" {
		mediaURL = "geo:" + parsed.LatLon
	}

	// Audio/voice: re-host to GCS and transcribe in one pass. The AI
	// pipeline needs text to answer (an empty content field makes the
	// bot ghost the customer — the #1 complaint about WhatsApp bots in
	// BR), and the dashboard needs a playable URL (the Meta media URL
	// is 15-min and auth-gated, so the browser cannot fetch it). The
	// fallback text runs whenever transcription is unavailable so the
	// AI never sees empty input.
	if (parsed.MessageType == "voice" || parsed.MessageType == "audio") && parsed.Text == "" {
		if mediaURL != "" {
			storedURL, transcript := h.rehostAndTranscribeAudio(ctx, channel, mediaURL)
			mediaURL = storedURL
			if transcript != "" {
				parsed.Text = transcript
			} else {
				parsed.Text = "[audio not transcribed]"
			}
		} else {
			parsed.Text = "[audio not transcribed]"
		}
	}

	if parsed.MessageType == "image" && parsed.Text == "" && mediaURL != "" && h.describer != nil {
		desc, err := h.describeImage(services.WithAIAccount(ctx, channel.AccountID), mediaURL)
		if err != nil {
			slog.Warn("image description failed", "channel_id", channel.ID, "error", err)
			parsed.Text = "[imagem enviada pelo cliente]"
		} else {
			parsed.Text = "[imagem do cliente] " + desc
		}
	}

	// Re-host image / sticker to GCS so the dashboard can render them
	// (the raw Meta URL is short-lived and auth-gated). Runs after the
	// optional describer so its live-URL call still goes to Meta. Video
	// and document are deferred — they can be up to 16/100MB, too large
	// to inline-download on the webhook hot path.
	if (parsed.MessageType == "image" || parsed.MessageType == "sticker") && mediaURL != "" {
		if rehosted := h.rehostInboundMedia(ctx, channel, mediaURL); rehosted != "" {
			mediaURL = rehosted
		}
	}

	metaMap := map[string]any{"wa_message_id": parsed.MessageID}
	// Persist the Meta media object id so the dashboard can re-resolve
	// a fresh signed URL when the customer scrolls back to a media
	// message: parsed.MediaURL is short-lived (15 min) and would
	// otherwise 404 on every re-render.
	if parsed.MediaID != "" {
		metaMap["wa_media_id"] = parsed.MediaID
	}
	if len(parsed.Metadata) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(parsed.Metadata, &extra); err == nil {
			for k, v := range extra {
				metaMap[k] = v
			}
		}
	}
	metadata, _ := json.Marshal(metaMap)

	partial, err := h.inboundUC.SaveInbound(ctx, channel, usecases.InboundMessageDTO{
		ChannelToken:      channel.WebhookToken,
		SenderPhone:       parsed.From,
		SenderName:        parsed.SenderName,
		MessageType:       parsed.MessageType,
		Content:           parsed.Text,
		MediaURL:          mediaURL,
		Metadata:          metadata,
		ExternalMessageID: parsed.MessageID,
	})
	if err != nil {
		if err == usecases.ErrDuplicateInbound {
			slog.Debug("WhatsApp duplicate webhook ignored", "wamid", parsed.MessageID)
			return inboundMessageOutcome{duplicate: true}
		}
		slog.Error("WhatsApp SaveInbound failed", "channel_id", channel.ID, "error", err)
		observability.CaptureError(err, map[string]string{
			"component":  "whatsapp_webhook",
			"stage":      "save_inbound",
			"channel_id": channel.ID,
			"wamid":      parsed.MessageID,
		})
		return inboundMessageOutcome{failureMsg: "save_inbound: " + err.Error()}
	}

	h.broadcastWA(partial.AccountID, "new_message", partial.ConversationID, partial.Message)

	// Fan out a web/mobile push so attendants whose dashboard tab is not
	// focused (or backgrounded) still get pinged on a customer reply.
	// Runs detached; failures never block the Meta webhook ack.
	dispatchInboundPushAsync(h.pushUC, partial)

	if partial.IsNewConversation && h.events != nil {
		h.events.Emit(partial.AccountID, "conversation.created", map[string]any{
			"conversation_id": partial.ConversationID,
			"channel_id":      partial.ChannelID,
			"contact_id":      partial.ContactID,
		})
	}

	if partial.IsBotPaused || parsed.Skip {
		// System events, parser errors, reaction-only messages, or
		// human-handled conversations: persisted for transcript
		// visibility but skipped from AI/billing fanout.
		return inboundMessageOutcome{processed: true, conversationID: partial.ConversationID}
	}
	// Reserve a slot in the outbox tracker BEFORE launching so the
	// goroutine's SignalDone can decide it's the last finisher even
	// if it completes before the next iteration's spawnAsyncReply
	// has a chance to increment.
	if tracker != nil {
		tracker.pending.Add(1)
	}
	h.spawnAsyncReply(channel, partial, parsed.MessageID, parsed.From, parsed.Text, tracker)
	return inboundMessageOutcome{
		processed:      true,
		conversationID: partial.ConversationID,
		asyncSpawned:   true,
	}
}

// spawnAsyncReply launches the AI/send pipeline in a tracked goroutine.
// All in-flight work is registered on asyncWG so Shutdown can drain
// before the process exits — without this, Meta has already received
// a 200 ack but the outbound message would be killed mid-send by the
// SIGTERM that ended the process, with no outbox replay path because
// the inbound was already marked processed.
func (h *WhatsAppWebhookHandler) spawnAsyncReply(channel *domain.Channel, partial *usecases.InboundPartialResult, inboundMsgID, recipient, content string, tracker *outboxAsyncTracker) {
	if h.shuttingDown.Load() {
		// Refuse to start new work after shutdown was signalled. The
		// outbox row stays pending; the next instance's worker will
		// pick it up. Decrement the tracker counter so the partial
		// outcome doesn't leave it hanging.
		slog.Warn("WhatsApp async reply skipped: shutdown in progress",
			"conversation_id", partial.ConversationID)
		tracker.SignalDone(true, "shutdown in progress")
		return
	}
	h.asyncWG.Add(1)
	observability.WhatsAppAsyncInflight.Inc()
	go func() {
		defer h.asyncWG.Done()
		defer observability.WhatsAppAsyncInflight.Dec()
		ok := h.processAIAsync(channel, partial, inboundMsgID, recipient, content)
		failureReason := ""
		if !ok {
			failureReason = "async reply send failed"
		}
		tracker.SignalDone(!ok, failureReason)
	}()
}

// Shutdown signals the handler to stop accepting new async work and
// blocks until every in-flight goroutine finishes or the deadline
// elapses. Called from the API's graceful-shutdown path so a SIGTERM
// during deploy doesn't cut off mid-flight outbound replies.
func (h *WhatsAppWebhookHandler) Shutdown(ctx context.Context) {
	h.shuttingDown.Store(true)
	done := make(chan struct{})
	go func() {
		h.asyncWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("WhatsApp async reply drain timed out", "error", ctx.Err())
	}
}

// readReceiptsEnabled reads the per-channel toggle from credentials.
// Default ON — customers usually want a "visto" so they know the
// business saw the message. Tenants opt out by setting
// credentials.read_receipts to "off" (matches the WhatsApp Cloud API
// account-level setting; we keep the toggle per-channel so a tenant
// running multiple numbers with different brand voices can mix).
func readReceiptsEnabled(channel *domain.Channel) bool {
	if channel == nil {
		return true
	}
	v := credentialString(channel, "read_receipts")
	if v == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "off", "false", "no", "0", "disabled":
		return false
	}
	return true
}

// processAIAsync runs the humanized reply pipeline (typing indicator,
// model call, split, send) off the request path. It uses a detached
// context with a generous timeout so the gin request can complete its
// ack without cancelling mid-flight work.
//
// Returns true when every send succeeded (or there was nothing to
// send). Returns false when at least one send failed — the caller
// uses this to decide whether the outbox row should land Processed
// or Failed (Failed = the worker replays it on the next tick).
func (h *WhatsAppWebhookHandler) processAIAsync(channel *domain.Channel, partial *usecases.InboundPartialResult, inboundMsgID, recipient, content string) (ok bool) {
	ok = true
	defer func() {
		if r := recover(); r != nil {
			slog.Error("WhatsApp async AI panic", "channel_id", channel.ID, "recover", r)
			ok = false
		}
	}()

	bgCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Brief "read" pause, then Meta's native typing indicator to the
	// end-user. The read receipt doubles as the typing trigger so only
	// one API call.
	//
	// Read receipts default to ON. Tenants who don't want the customer
	// to see "visto" can opt out per-channel via credentials.read_receipts="off"
	// — useful when the brand voice is "we'll get back to you soon"
	// rather than instant. The flag is read from the channel config
	// each time so a dashboard toggle takes effect immediately.
	time.Sleep(services.ReadDelay())
	if inboundMsgID != "" && readReceiptsEnabled(channel) {
		if err := h.waAdapter.MarkAsReadAndTyping(bgCtx, channel, inboundMsgID); err != nil {
			slog.Debug("WhatsApp mark-as-typing failed", "error", err)
		}
	}
	h.broadcastTypingWA(partial.AccountID, partial.ConversationID, true)
	defer h.broadcastTypingWA(partial.AccountID, partial.ConversationID, false)

	aiCtx, aiCancel := context.WithTimeout(bgCtx, 60*time.Second)
	aiMsg := h.inboundUC.ProcessAI(aiCtx, partial, content)
	aiCancel()

	if aiMsg != nil && aiMsg.HandoffRequested {
		// Silent hand-off for WhatsApp: no outbound, no bubble. The
		// conversation is already paused server-side; surface it to
		// the dashboard so an attendant can take over. No outbound
		// expected, so the outbox marks Processed.
		h.broadcastHandoffWA(partial.AccountID, partial.ConversationID)
		contactName := ""
		if partial.Message != nil && partial.Message.SenderName != nil {
			contactName = *partial.Message.SenderName
		}
		dispatchHandoffPushAsync(h.pushUC, partial.AccountID, partial.ConversationID, contactName)
		return true
	}
	if aiMsg == nil || aiMsg.Message == nil || aiMsg.Message.Content == nil {
		// No reply (AI declined / empty). Still counts as success on
		// the outbox side because there was no send to fail.
		return true
	}
	h.broadcastWA(partial.AccountID, "new_message", partial.ConversationID, aiMsg.Message)

	// Split long replies into 2-4 bubbles and space them out so the
	// conversation reads like a real person's typing cadence rather than
	// a chatbot dropping a paragraph in one shot.
	bubbles := services.SplitForHuman(*aiMsg.Message.Content)
	for i, bubble := range bubbles {
		time.Sleep(services.TypingDelay(bubble))
		out := &domain.Message{
			Content: strPtr(bubble),
			Type:    "text",
		}
		if i == 0 && aiMsg.Message.Type != "" {
			out.Type = aiMsg.Message.Type
		}
		// Anchor the FIRST bubble as a quoted reply to the inbound that
		// triggered this AI run. Subsequent bubbles read as typing
		// continuation, so quoting all of them would clutter the
		// thread. The wa_reply_to metadata is consumed by the adapter
		// and forwarded as the Cloud API `context.message_id`.
		if i == 0 && inboundMsgID != "" {
			out.Metadata = map[string]string{"wa_reply_to": inboundMsgID}
		}
		sendStart := time.Now()
		wamid, sendErr := h.waAdapter.SendMessageAndGetID(bgCtx, channel, recipient, out)
		// Outbound metrics: per-type duration histogram + per-channel
		// outcome counter so dashboards can show "send failure rate"
		// without scanning logs. phone_number_id may be empty when
		// the channel lacks creds; we still record under the empty
		// label so totals stay accurate.
		phoneID := credentialString(channel, "phone_number_id")
		observability.WhatsAppSendDuration.WithLabelValues(out.Type).Observe(time.Since(sendStart).Seconds())
		if sendErr != nil {
			observability.WhatsAppOutboundTotal.WithLabelValues(phoneID, out.Type, "failure").Inc()
		} else {
			observability.WhatsAppOutboundTotal.WithLabelValues(phoneID, out.Type, "success").Inc()
		}
		// Each bubble is a distinct WhatsApp message with its own
		// wamid. We record one MessageAttempt per bubble so the later
		// status webhook (delivered/read/failed) can correlate against
		// the specific bubble — the parent messages.external_message_id
		// only carries one id and would lose status for bubbles 2..N.
		// Failed sends still get an attempt row (with NULL wamid) so
		// ops can count failures without scanning logs.
		now := time.Now()
		attempt := &domain.MessageAttempt{
			ID:          ulid.Make().String(),
			MessageID:   aiMsg.Message.ID,
			ChannelID:   channel.ID,
			BubbleIndex: int16(i),
		}
		if sendErr == nil {
			w := wamid
			attempt.ExternalMessageID = &w
			attempt.SentAt = &now
		} else {
			r := sendErr.Error()
			attempt.FailedAt = &now
			attempt.FailedReason = &r
		}
		if h.msgRepo != nil {
			if err := h.msgRepo.CreateMessageAttempt(bgCtx, attempt); err != nil {
				slog.Warn("failed to record message_attempt",
					"channel_id", channel.ID, "wamid", wamid, "error", err)
			}
		}
		if sendErr != nil {
			slog.Error("failed to send WhatsApp AI reply", "channel_id", channel.ID, "error", sendErr)
			observability.CaptureError(sendErr, map[string]string{
				"component":       "whatsapp_webhook",
				"stage":           "send_reply",
				"channel_id":      channel.ID,
				"conversation_id": partial.ConversationID,
			})
			return false
		}
		slog.Debug("WhatsApp AI reply bubble sent",
			"channel_id", channel.ID,
			"conversation_id", partial.ConversationID,
			"wamid", wamid,
			"bubble_index", i)
		if i < len(bubbles)-1 {
			time.Sleep(services.BetweenMessagesDelay())
		}
	}
	return true
}

// ProcessPayload runs the side-effect pipeline for an inbound webhook
// without any HTTP coupling. It's the same logic HandleInbound runs
// inline, lifted out so the outbox worker can replay rows that errored
// mid-flight (parser bug, AI quota, DB blip during fanout).
//
// Returns:
//
//	(processed=true, nil) — every side effect committed.
//	(processed=false, nil) — recognised but intentionally not actionable
//	    (Meta test webhook, system event, deduped retry, status-only
//	    delivery update).
//	(_, err) — transient failure; caller decides whether to retry.
func (h *WhatsAppWebhookHandler) ProcessPayload(ctx context.Context, body []byte) (bool, error) {
	// Status updates (sent/delivered/read/failed) — fire-and-forget;
	// even partial DB failures don't justify a retry.
	statuses := waadapter.ParseStatuses(body)
	if len(statuses) > 0 {
		for _, s := range statuses {
			ch, _ := h.channelRepo.GetByPhoneNumberID(ctx, s.PhoneNumberID)
			if ch != nil {
				h.persistWhatsAppStatus(ctx, ch, s)
				if h.hub != nil {
					payload, _ := json.Marshal(map[string]interface{}{
						"event":         "message_status",
						"message_id":    s.MessageID,
						"status":        s.Status,
						"recipient":     s.Recipient,
						"failed_reason": s.FailedReason,
					})
					h.hub.BroadcastToAccount(ch.AccountID, payload)
				}
			}
			h.applyCampaignStatus(ctx, s.MessageID, s.Status)
		}
		return true, nil
	}

	// Non-message events (template/quality/review). Best-effort, won't
	// fail the webhook.
	events := waadapter.ParseEvents(body)
	handledMeta := false
	for _, ev := range events {
		if ev.IsMessageEvent() {
			continue
		}
		handledMeta = true
		h.handleMetaEventCtx(ctx, ev)
	}

	// Replay path runs the SAME per-message pipeline as the inline
	// handler — single source of truth means a fix to ParseInbound
	// (multi-message batch), audio fallback, etc. covers the worker
	// too. Without this, replay used to silently drop messages 2..N
	// and skip the audio fallback we just added inline.
	parsedAll := waadapter.ParseInboundAll(body)
	if len(parsedAll) == 0 {
		return handledMeta, nil
	}
	anyProcessed := false
	allDuplicate := true
	var firstErr error
	// Replay path runs inside the worker's transactional state — the
	// outbox is already known to be `pending` and the worker itself
	// owns the final mark via the return value, so a per-event
	// tracker isn't needed. Pass nil; the per-message helper Add()s
	// to the tracker only when one is provided.
	for _, parsed := range parsedAll {
		outcome := h.processInboundMessageCtx(ctx, parsed, nil)
		if outcome.processed {
			anyProcessed = true
			allDuplicate = false
		}
		if !outcome.duplicate {
			allDuplicate = false
		}
		if outcome.failureMsg != "" && firstErr == nil {
			firstErr = fmt.Errorf("%s", outcome.failureMsg)
		}
	}
	if firstErr != nil && !anyProcessed {
		return false, firstErr
	}
	if anyProcessed {
		return true, nil
	}
	// allDuplicate (already-seen retry) and any other recognised-but-
	// no-op outcome both surface as `processed=false, err=nil` so the
	// worker marks the row Skipped (not Failed) and stops retrying.
	_ = allDuplicate
	return false, nil
}

// handleMetaEventCtx mirrors handleMetaEvent but takes a plain context.
// Used by the outbox replay worker that lacks a gin.Context.
func (h *WhatsAppWebhookHandler) handleMetaEventCtx(ctx context.Context, ev waadapter.ChangeEvent) {
	h.handleMetaEventInternal(ctx, ev)
}

// recordOutbox persists the raw signed payload to wa_webhook_events so
// the async worker can replay it if the inline processing path errors
// out (parser bug, AI quota, DB blip mid-fanout). Returns the eventID
// for later mark calls; empty string when the outbox isn't configured
// or the insert fails — handler proceeds either way to avoid blocking
// the inbound flow on a non-critical store.
func (h *WhatsAppWebhookHandler) recordOutbox(ctx context.Context, body []byte, sig string) string {
	if h.outbox == nil {
		return ""
	}
	id := ulid.Make().String()
	if err := h.outbox.Insert(ctx, id, body, sig); err != nil {
		// Sentry escalation: silent outbox failure means replay safety
		// is OFF for this delivery — a transient AI/DB error after this
		// point loses the inbound forever. Ops needs to see it now,
		// not when the customer complains about a missing message.
		slog.Warn("wa outbox insert failed; processing will continue without replay safety",
			"error", err, "size", len(body))
		observability.CaptureError(err, map[string]string{
			"component": "whatsapp_webhook",
			"stage":     "outbox_insert",
		})
		return ""
	}
	return id
}

// persistWhatsAppStatus writes Meta's status envelope onto the matching
// outbound message row. Best-effort: a missing repo (test wiring) or
// unmatched wamid (status for an outbound we never persisted, e.g.
// orphan template send) is logged and dropped, never propagated.
func (h *WhatsAppWebhookHandler) persistWhatsAppStatus(ctx context.Context, ch *domain.Channel, s waadapter.ParsedStatus) {
	if h.msgRepo == nil || s.MessageID == "" {
		return
	}
	upd := domain.WhatsAppStatusUpdate{
		ChannelID:          ch.ID,
		ExternalMessageID:  s.MessageID,
		FailedReason:       s.FailedReason,
		WAConversationID:   s.ConversationID,
		ConversationOrigin: s.ConversationOrigin,
		PricingCategory:    s.PricingCategory,
	}
	// Stamp the right timestamp based on which transition this status
	// represents. Meta sends multiple statuses per outbound (sent →
	// delivered → read → failed) so we set only the column that matches.
	// Earlier code dropped `sent` and `failed` entirely — the dashboard
	// could only render delivered / read, never the leading checkmark
	// or the failure timestamp.
	now := parseEpoch(s.Timestamp)
	switch s.Status {
	case "sent":
		upd.SentAt = &now
	case "delivered":
		upd.DeliveredAt = &now
	case "read":
		upd.ReadAt = &now
	case "failed":
		upd.FailedAt = &now
	}
	if s.PricingCategory != "" {
		b := s.PricingBillable
		upd.PricingBillable = &b
	}
	if s.ConversationExpires != "" {
		t := parseEpoch(s.ConversationExpires)
		upd.WAConversationExpiresAt = &t
	}
	// Apply onto the parent messages row (legacy single-bubble path)
	// AND onto message_attempts (split-bubble path). Cloud API only
	// returns one wamid per outbound; whichever table owns it sees a
	// real update, the other COALESCE/NULLIF guards mean a no-op.
	// Doing both lets us avoid an upfront lookup and keeps each call
	// idempotent.
	if err := h.msgRepo.ApplyWhatsAppStatus(ctx, upd); err != nil {
		slog.Warn("failed to persist WhatsApp status (parent)",
			"wamid", s.MessageID, "channel_id", ch.ID, "error", err)
	}
	if err := h.msgRepo.ApplyMessageAttemptStatus(ctx, upd); err != nil {
		slog.Warn("failed to persist WhatsApp status (attempt)",
			"wamid", s.MessageID, "channel_id", ch.ID, "error", err)
	}
}

// parseEpoch turns a Meta-style epoch-second string into time.Time.
// Returns the zero value on parse failure so the COALESCE in the SQL
// upsert leaves the column untouched.
func parseEpoch(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(secs, 0).UTC()
}

// handleMetaEvent dispatches non-message webhook fields. Until now Meta
// events for template approvals, quality drops and review state were
// dropped silently — customers had to refresh the dashboard or notice
// rate-limit errors before realising. We now log every field at INFO,
// and for high-impact ones (template status, quality flag, review
// decision) broadcast a WebSocket event so the Flutter dashboard can
// invalidate caches and surface a banner immediately.
func (h *WhatsAppWebhookHandler) handleMetaEvent(c *gin.Context, ev waadapter.ChangeEvent) {
	h.handleMetaEventInternal(c.Request.Context(), ev)
}

// fanoutAccounts returns the set of account IDs that should receive a
// given Meta event. Quality updates carrying a display_phone_number
// can be pinned to a single channel; everything else (template, review,
// account-level) goes to every active channel under the WABA so two
// tenants sharing the WABA both see their own state.
func (h *WhatsAppWebhookHandler) fanoutAccounts(ctx context.Context, wabaID, displayPhoneNumber string) []string {
	channels, err := h.channelRepo.ListByWABAID(ctx, wabaID)
	if err != nil {
		slog.Warn("failed to list channels for WABA fan-out",
			"waba_id", wabaID, "error", err)
		return nil
	}
	if len(channels) == 0 {
		return nil
	}
	if displayPhoneNumber != "" {
		// Pin to the channel whose display_phone_number matches; only
		// fall back to the full set when no channel records the number
		// (legacy rows, channels created before display_phone caching).
		var hits []string
		for _, ch := range channels {
			if cred := credentialString(ch, "display_phone_number"); cred == displayPhoneNumber {
				hits = append(hits, ch.AccountID)
			}
		}
		if len(hits) > 0 {
			return dedupe(hits)
		}
	}
	out := make([]string, 0, len(channels))
	for _, ch := range channels {
		out = append(out, ch.AccountID)
	}
	return dedupe(out)
}

func credentialString(ch *domain.Channel, key string) string {
	if ch == nil || ch.Config == nil {
		return ""
	}
	raw, ok := ch.Config.([]byte)
	if !ok {
		if rm, ok := ch.Config.(json.RawMessage); ok {
			raw = []byte(rm)
		}
	}
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (h *WhatsAppWebhookHandler) emitMetaEventToAccounts(accounts []string, event string, data map[string]any) {
	for _, accountID := range accounts {
		h.broadcastMetaEvent(accountID, event, data)
	}
}

// handleMetaEventInternal is the shared body invoked by handleMetaEvent
// (gin context) and handleMetaEventCtx (worker / replay path). It fans
// out to every account tied to the WABA, so a WABA shared between two
// tenants no longer leaks template/quality events from one to the other.
func (h *WhatsAppWebhookHandler) handleMetaEventInternal(ctx context.Context, ev waadapter.ChangeEvent) {
	switch ev.Field {
	case "message_template_status_update":
		if t, ok := ev.AsTemplateStatus(); ok {
			slog.Info("WhatsApp template status update",
				"waba_id", ev.WABAID,
				"template_name", t.MessageTemplate.Name,
				"language", t.MessageTemplate.Language,
				"event", t.Event,
				"reason", t.Reason)
			accounts := h.fanoutAccounts(ctx, ev.WABAID, "")
			h.emitMetaEventToAccounts(accounts, "wa_template_status", map[string]any{
				"template_name": t.MessageTemplate.Name,
				"language":      t.MessageTemplate.Language,
				"event":         t.Event,
				"reason":        t.Reason,
			})
		}
	case "phone_number_quality_update":
		if q, ok := ev.AsQualityUpdate(); ok {
			slog.Warn("WhatsApp phone quality update",
				"waba_id", ev.WABAID,
				"display_phone_number", q.DisplayPhoneNumber,
				"event", q.Event,
				"current_limit", q.CurrentLimit,
				"current_quality", q.Quality)
			accounts := h.fanoutAccounts(ctx, ev.WABAID, q.DisplayPhoneNumber)
			h.emitMetaEventToAccounts(accounts, "wa_quality_update", map[string]any{
				"display_phone_number": q.DisplayPhoneNumber,
				"event":                q.Event,
				"current_limit":        q.CurrentLimit,
				"current_quality":      q.Quality,
			})
		}
	case "account_review_update":
		if r, ok := ev.AsAccountReviewUpdate(); ok {
			slog.Warn("WhatsApp account review update",
				"waba_id", ev.WABAID,
				"decision", r.DecisionEvent)
			accounts := h.fanoutAccounts(ctx, ev.WABAID, "")
			h.emitMetaEventToAccounts(accounts, "wa_account_review", map[string]any{
				"decision": r.DecisionEvent,
			})
		}
	case "account_update", "business_capability_update",
		"phone_number_name_update", "template_category_update", "security":
		slog.Info("WhatsApp meta event received",
			"field", ev.Field, "waba_id", ev.WABAID, "payload", string(ev.Payload))
	default:
		slog.Info("WhatsApp meta event (unknown field)",
			"field", ev.Field, "waba_id", ev.WABAID, "payload", string(ev.Payload))
	}
}

// broadcastMetaEvent pushes a Meta-side event (template approved,
// quality drop, review decision) to every dashboard connected for the
// account. accountID may be empty when the WABA hasn't been bound to a
// channel yet — the broadcast is then a no-op by design.
func (h *WhatsAppWebhookHandler) broadcastMetaEvent(accountID, event string, data map[string]any) {
	if accountID == "" {
		return
	}
	if h.hub != nil {
		body := map[string]any{"event": event}
		for k, v := range data {
			body[k] = v
		}
		payload, _ := json.Marshal(body)
		h.hub.BroadcastToAccount(accountID, payload)
	}
	// Translate the dashboard-internal event name to the public webhook
	// contract so tenant integrations don't see our WS-internal naming.
	if h.events != nil {
		if outbound, ok := metaEventToWebhookName(event); ok {
			h.events.Emit(accountID, outbound, data)
		}
	}
}

// metaEventToWebhookName maps the internal WS event names ("wa_*")
// emitted by handleMetaEvent to the stable public webhook event names
// from services/outbound_webhook_dispatcher.go. Keeping the mapping
// explicit (instead of renaming WS events) prevents breaking the
// existing dashboard listeners.
func metaEventToWebhookName(internal string) (string, bool) {
	switch internal {
	case "wa_quality_update":
		return "whatsapp.quality_changed", true
	case "wa_template_status":
		return "whatsapp.template_status_changed", true
	}
	return "", false
}

func (h *WhatsAppWebhookHandler) broadcastWA(accountID, event, conversationID string, msg *domain.Message) {
	if h.hub != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"event":           event,
			"conversation_id": conversationID,
			"message":         msg,
		})
		h.hub.BroadcastToAccount(accountID, payload)
	}
	// Fan out to tenant outbound webhooks. Inbound (`new_message` from
	// the customer) maps to `message.received`; AI / agent outbound
	// reply maps to `message.sent`. Skip when no emitter is wired
	// (test harnesses, deployments without the dispatcher).
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

// Stable webhook event names exposed to tenants. Mirrors the
// constants in services/outbound_webhook_dispatcher.go — kept here
// too so the handler doesn't import the services package just to
// reference one string.
const (
	EventMessageReceived = "message.received"
	EventMessageSent     = "message.sent"
)

func (h *WhatsAppWebhookHandler) broadcastTypingWA(accountID, conversationID string, isTyping bool) {
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

// broadcastHandoffWA signals the dashboard that the bot yielded on a
// silent-handoff agent for a WhatsApp conversation. Server-side the
// conversation is already paused; this just lights up the tile.
func (h *WhatsAppWebhookHandler) broadcastHandoffWA(accountID, conversationID string) {
	if h.hub == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event":           "human_handoff_requested",
		"conversation_id": conversationID,
	})
	h.hub.BroadcastToAccount(accountID, payload)
}

// applyCampaignStatus correlates a WhatsApp status update (delivered/read/
// failed) with a broadcast recipient. It is a no-op when:
//   - the campaignRepo is not wired
//   - the message id does not belong to a campaign send (regular outbound)
//   - the status is not one we track per recipient (e.g. "sent" — already
//     marked at send time by the worker)
//
// Increments the campaign aggregate counter on every status transition so
// the dashboard report stays accurate.
func (h *WhatsAppWebhookHandler) applyCampaignStatus(ctx context.Context, wamid, status string) {
	if h.campaignRepo == nil || wamid == "" {
		return
	}
	var rec domain.RecipientStatus
	switch status {
	case "delivered":
		rec = domain.RecipientDelivered
	case "read":
		rec = domain.RecipientRead
	case "failed":
		rec = domain.RecipientFailed
	default:
		return // ignore "sent" (set at send time) and any future statuses
	}

	// Locate the campaign first so we can update its counter even though
	// the recipient update only needs the wamid.
	camp, err := h.campaignRepo.GetCampaignByProviderMsgID(ctx, wamid)
	if err != nil {
		slog.Warn("campaign correlation failed", "wamid", wamid, "error", err)
		return
	}
	if camp == nil {
		return // not a broadcast — regular reply
	}

	if err := h.campaignRepo.UpdateRecipientStatusByProviderID(ctx, wamid, rec); err != nil {
		slog.Warn("recipient status update failed", "wamid", wamid, "error", err)
		return
	}
	if err := h.campaignRepo.IncrementCounter(ctx, camp.ID, rec); err != nil {
		slog.Warn("campaign counter increment failed", "campaign_id", camp.ID, "error", err)
	}
}

// rehostAndTranscribeAudio downloads the inbound audio once, then both
// re-hosts it to GCS (so the dashboard can play it from a signed URL —
// the Meta media URL is 15-min and auth-gated) and transcribes the same
// bytes. It returns the URL to persist as media_url (the GCS object
// path on success, the original URL on any failure) and the transcript
// (empty when transcription is unavailable or fails). Never errors:
// every degradation falls back so the message is still saved.
func (h *WhatsAppWebhookHandler) rehostAndTranscribeAudio(ctx context.Context, channel *domain.Channel, mediaURL string) (storedURL string, transcript string) {
	storedURL = mediaURL

	dlCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	audio, contentType, err := h.waAdapter.DownloadMedia(dlCtx, channel, mediaURL)
	if err != nil {
		slog.Warn("inbound audio: download failed — dashboard playback unavailable",
			"channel_id", channel.ID, "error", err)
		return storedURL, ""
	}

	// Re-host to GCS. HandleListMessages signs object paths into 30-min
	// read URLs the browser can fetch.
	if h.gcsAdapter != nil {
		objectPath := fmt.Sprintf("accounts/%s/inbound_media/%s%s",
			channel.AccountID, ulid.Make().String(), mediaExt(contentType))
		if upErr := h.gcsAdapter.UploadBytes(ctx, objectPath, audio, contentType); upErr != nil {
			slog.Warn("inbound audio: GCS re-host failed — keeping Meta URL",
				"channel_id", channel.ID, "error", upErr)
		} else {
			storedURL = objectPath
		}
	}

	// Transcribe the same bytes — no second download.
	if h.transcriber != nil {
		// The account rides the context so the BYOK transcriber can pick
		// the customer's own key.
		tCtx, tCancel := context.WithTimeout(services.WithAIAccount(ctx, channel.AccountID), 12*time.Second)
		defer tCancel()
		// pt hints Brazilian Portuguese — Whisper auto-detects otherwise
		// but slower and less accurate on short clips.
		if text, tErr := h.transcriber.Transcribe(tCtx, bytes.NewReader(audio), "pt"); tErr != nil {
			slog.Warn("audio transcription failed", "channel_id", channel.ID, "error", tErr)
		} else {
			transcript = text
		}
	}
	return storedURL, transcript
}

// mediaExt maps a content type to a file extension. The extension is
// cosmetic — the browser renders from the stored Content-Type — but it
// keeps GCS objects recognisable.
func mediaExt(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	case strings.Contains(ct, "mpeg"), strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "mp4"), strings.Contains(ct, "m4a"), strings.Contains(ct, "aac"):
		return ".m4a"
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	default:
		return ".bin"
	}
}

// rehostInboundMedia copies non-audio inbound media (image, sticker) to
// GCS so the dashboard can render it from a signed read URL. Meta media
// URLs are 15-min and require the WABA token — the browser cannot fetch
// them. No-op when GCS is unconfigured; returns "" on any failure so
// the caller keeps the original URL.
func (h *WhatsAppWebhookHandler) rehostInboundMedia(ctx context.Context, channel *domain.Channel, mediaURL string) string {
	if h.gcsAdapter == nil {
		return ""
	}
	dlCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, contentType, err := h.waAdapter.DownloadMedia(dlCtx, channel, mediaURL)
	if err != nil {
		slog.Warn("inbound media re-host: download failed",
			"channel_id", channel.ID, "error", err)
		return ""
	}
	objectPath := fmt.Sprintf("accounts/%s/inbound_media/%s%s",
		channel.AccountID, ulid.Make().String(), mediaExt(contentType))
	if err := h.gcsAdapter.UploadBytes(ctx, objectPath, data, contentType); err != nil {
		slog.Warn("inbound media re-host: GCS upload failed",
			"channel_id", channel.ID, "error", err)
		return ""
	}
	return objectPath
}

// describeImage resolves the Meta CDN URL to a one-sentence description
// via the vision adapter. Same 20s-budget reasoning as the audio path
// — 10s leaves headroom for downstream SaveInbound + broadcast.
func (h *WhatsAppWebhookHandler) describeImage(ctx context.Context, mediaURL string) (string, error) {
	descCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return h.describer.Describe(descCtx, mediaURL, "pt")
}
