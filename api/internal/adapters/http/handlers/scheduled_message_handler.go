package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// tzOffsetRe matches an RFC3339 trailing offset like "+02:00", "-0300" or
// "+0000". Used to reject send_at values that omit the timezone, which
// Go's time package otherwise silently parses as UTC — a user in BRT
// (-03:00) asking for "10am tomorrow" would have it fire at 7am local.
var tzOffsetRe = regexp.MustCompile(`[+-]\d{2}:?\d{2}$`)

// tzAwareTime is time.Time but only unmarshals from RFC3339 strings
// that include an explicit timezone (offset or trailing Z).
type tzAwareTime struct{ time.Time }

func (t *tzAwareTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if !strings.HasSuffix(s, "Z") && !tzOffsetRe.MatchString(s) {
		return fmt.Errorf("send_at must include a timezone offset (Z or ±HH:MM)")
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("send_at must be RFC3339 with timezone: %w", err)
	}
	t.Time = parsed
	return nil
}

// ScheduledMessageHandler exposes CRUD over outbound_scheduled. The
// dashboard's "send tomorrow at 10am" form posts here; the dispatcher
// worker drains rows when send_at falls due.
type ScheduledMessageHandler struct {
	repo        domain.OutboundScheduledRepository
	channelRepo domain.ChannelRepository
	auditLog    domain.AuditLogRepository
}

func NewScheduledMessageHandler(repo domain.OutboundScheduledRepository, channelRepo domain.ChannelRepository) *ScheduledMessageHandler {
	return &ScheduledMessageHandler{repo: repo, channelRepo: channelRepo}
}

// SetAuditLog wires the audit-log writer. When set, every create /
// cancel emits a row so the LGPD audit trail covers scheduled sends
// alongside the regular CRUD.
func (h *ScheduledMessageHandler) SetAuditLog(a domain.AuditLogRepository) {
	h.auditLog = a
}

type scheduleMessageRequest struct {
	ChannelID      string      `json:"channel_id" binding:"required"`
	ConversationID *string     `json:"conversation_id"`
	Recipient      string      `json:"recipient" binding:"required"`
	SendAt         tzAwareTime `json:"send_at" binding:"required"`
	// Payload is the full domain.Message shape — Type/Content/MediaURL
	// /Metadata. Stored opaque so adding new types later doesn't need
	// a schema change here.
	Payload map[string]interface{} `json:"payload" binding:"required"`
}

// HandleSchedule godoc
// @Summary      Schedule an outbound message for future delivery
// @Tags         scheduled-messages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  scheduleMessageRequest  true  "Channel + recipient + send_at + payload"
// @Success      201   {object}  domain.OutboundScheduled
// @Failure      400   {object}  domain.AppError
// @Failure      404   {object}  domain.AppError
// @Router       /scheduled-messages [post]
func (h *ScheduledMessageHandler) HandleSchedule(c *gin.Context) {
	accountID := getAccountID(c)
	var req scheduleMessageRequest
	if !bindJSON(c, &req) {
		return
	}

	// send_at in the past is almost certainly a client clock-skew bug;
	// reject so the dispatcher doesn't fire instantly without operator
	// intent. Allow up to 60s of skew so a "schedule now" form that
	// rounds to NOW() doesn't 400.
	if req.SendAt.Before(time.Now().Add(-60 * time.Second)) {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "send_at must be in the future",
		})
		return
	}

	ch, err := h.channelRepo.GetByID(c.Request.Context(), req.ChannelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{
			Code: "CHANNEL_NOT_FOUND", Message: "channel not found",
		})
		return
	}

	payload, err := json.Marshal(req.Payload)
	if err != nil {
		// Don't echo the json marshal error — it surfaces internal
		// struct paths to the caller. Log it for ops instead.
		slog.Warn("schedule message: payload marshal failed",
			"account_id", accountID, "error", err)
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "VALIDATION_ERROR", Message: "invalid payload",
		})
		return
	}

	row := &domain.OutboundScheduled{
		ID:             ulid.Make().String(),
		AccountID:      accountID,
		ChannelID:      req.ChannelID,
		ConversationID: req.ConversationID,
		Recipient:      req.Recipient,
		Payload:        payload,
		SendAt:         req.SendAt.Time,
		Status:         "pending",
	}
	if err := h.repo.Create(c.Request.Context(), row); err != nil {
		slog.Error("scheduled message create failed",
			"account_id", accountID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{
			Code: "INTERNAL_ERROR", Message: "failed to schedule message",
		})
		return
	}
	h.audit(c, accountID, "scheduled_message.create", row.ID, map[string]any{
		"channel_id": row.ChannelID,
		"recipient":  row.Recipient,
		"send_at":    row.SendAt.Format(time.RFC3339),
	})
	c.JSON(http.StatusCreated, row)
}

// audit fires a best-effort audit_log insert. Failures don't block
// the API call — auditing is observability, not enforcement.
func (h *ScheduledMessageHandler) audit(c *gin.Context, accountID, action, entityID string, payload map[string]any) {
	if h.auditLog == nil {
		return
	}
	actor := c.GetString("userID")
	body, _ := json.Marshal(payload)
	row := &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ActorID:    &actor,
		Action:     action,
		EntityType: "scheduled_message",
		EntityID:   entityID,
		Payload:    body,
		CreatedAt:  time.Now(),
	}
	if err := h.auditLog.Create(c.Request.Context(), row); err != nil {
		slog.Warn("scheduled audit log failed", "action", action, "error", err)
	}
}

// HandleList godoc
// @Summary      List scheduled messages for the current account
// @Tags         scheduled-messages
// @Produce      json
// @Security     BearerAuth
// @Param        limit   query  int  false  "page size (default 50, max 200)"
// @Param        offset  query  int  false  "offset (default 0)"
// @Success      200  {array}  domain.OutboundScheduled
// @Router       /scheduled-messages [get]
func (h *ScheduledMessageHandler) HandleList(c *gin.Context) {
	accountID := getAccountID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := h.repo.ListByAccount(c.Request.Context(), accountID, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// HandleCancel godoc
// @Summary      Cancel a scheduled message before it dispatches
// @Description  Idempotent: already-sent rows are unchanged. 404 when the row doesn't exist or belongs to another account.
// @Tags         scheduled-messages
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Scheduled message ID"
// @Success      200  {object}  map[string]bool
// @Failure      404  {object}  domain.AppError
// @Router       /scheduled-messages/{id} [delete]
func (h *ScheduledMessageHandler) HandleCancel(c *gin.Context) {
	accountID := getAccountID(c)
	id := c.Param("id")
	if err := h.repo.Cancel(c.Request.Context(), id, accountID); err != nil {
		respondError(c, err)
		return
	}
	h.audit(c, accountID, "scheduled_message.cancel", id, nil)
	c.JSON(http.StatusOK, gin.H{"cancelled": true})
}
