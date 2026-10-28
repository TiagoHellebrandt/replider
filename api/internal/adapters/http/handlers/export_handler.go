package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type ExportHandler struct {
	convRepo domain.ConversationRepository
	msgRepo  domain.MessageRepository
	// exportUC is the universal pipeline (I3). Optional — see
	// SetUniversalExportUC.
	exportUC *usecases.ExportUseCase
}

func NewExportHandler(convRepo domain.ConversationRepository, msgRepo domain.MessageRepository) *ExportHandler {
	return &ExportHandler{convRepo: convRepo, msgRepo: msgRepo}
}

// HandleExportConversations exports conversations as CSV or JSON.
func (h *ExportHandler) HandleExportConversations(c *gin.Context) {
	accountID := c.GetString("accountID")
	format := c.DefaultQuery("format", "csv")
	status := c.DefaultQuery("status", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "1000"))
	if limit > 5000 {
		limit = 5000
	}

	conversations, err := h.convRepo.ListByAccountID(c.Request.Context(), accountID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to export"})
		return
	}

	// Limit results
	if len(conversations) > limit {
		conversations = conversations[:limit]
	}

	timestamp := time.Now().Format("2006-01-02")

	switch format {
	case "json":
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=conversations_%s.json", timestamp))
		c.JSON(http.StatusOK, conversations)

	default: // csv
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=conversations_%s.csv", timestamp))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)

		w := csv.NewWriter(c.Writer)
		// Header
		w.Write([]string{"id", "contact_name", "contact_id", "channel_type", "status", "priority", "assigned_to", "created_at", "updated_at"})

		for _, conv := range conversations {
			assignedTo := ""
			if conv.AssignedTo != nil {
				assignedTo = *conv.AssignedTo
			}
			contactName := conv.ContactID
			if conv.ContactName != nil {
				contactName = *conv.ContactName
			}
			_ = w.Write(domain.CSVSafeRow([]string{
				conv.ID,
				contactName,
				conv.ContactID,
				conv.ChannelID,
				string(conv.Status),
				conv.Priority,
				assignedTo,
				conv.CreatedAt.Format(time.RFC3339),
				conv.UpdatedAt.Format(time.RFC3339),
			}))
		}
		w.Flush()
	}
}

// HandleExportMessages exports messages for a specific conversation.
func (h *ExportHandler) HandleExportMessages(c *gin.Context) {
	accountID := c.GetString("accountID")
	conversationID := c.Query("conversation_id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "conversation_id is required"})
		return
	}

	// Verify ownership
	conv, err := h.convRepo.GetByID(c.Request.Context(), conversationID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	messages, err := h.msgRepo.ListByConversationID(c.Request.Context(), conversationID, 1000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to export messages"})
		return
	}

	timestamp := time.Now().Format("2006-01-02")
	format := c.DefaultQuery("format", "json")

	switch format {
	case "csv":
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=messages_%s_%s.csv", conversationID[:8], timestamp))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)

		w := csv.NewWriter(c.Writer)
		w.Write([]string{"id", "direction", "sender", "content", "type", "created_at"})
		for _, msg := range messages {
			content := ""
			if msg.Content != nil {
				content = *msg.Content
			}
			_ = w.Write(domain.CSVSafeRow([]string{msg.ID, msg.Direction, msg.From, content, msg.Type, msg.Timestamp.Format(time.RFC3339)}))
		}
		w.Flush()

	default: // json
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=messages_%s_%s.json", conversationID[:8], timestamp))
		data, _ := json.Marshal(messages)
		c.Data(http.StatusOK, "application/json", data)
	}
}

// ── Universal export (§6.18, I3) ─────────────────────────────────────────────

// SetUniversalExportUC wires the generic export pipeline. Optional: nil answers
// 503 and leaves the two legacy endpoints working.
func (h *ExportHandler) SetUniversalExportUC(uc *usecases.ExportUseCase) {
	h.exportUC = uc
}

// exportReservedParams are the query keys the export itself consumes. Everything
// else on the query string is treated as a FILTER and passed through — which is
// what makes "whatever the screen is showing" work without the export learning
// about each new filter.
var exportReservedParams = map[string]struct{}{
	"format": {}, "limit": {}, "from": {}, "to": {},
}

// HandleUniversalExport serves GET /v1/export/:entity.
//
// @Summary     Export a list
// @Description Exports leads, contacts, sales, conversations or signals as CSV or XLSX, applying the same filters the screen is showing. Every download is recorded (who, what, which filters, how many rows).
// @Tags        Export
// @Produce     text/csv
// @Param       entity path string true "leads | contacts | sales | conversations | signals"
// @Param       format query string false "csv (default) or xlsx"
// @Param       from   query string false "Start of the period (YYYY-MM-DD or RFC3339)"
// @Param       to     query string false "End of the period, exclusive"
// @Param       limit  query int    false "Row ceiling (default 5000, max 50000)"
// @Success     200 {file} file
// @Failure     400 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Security    BearerAuth
// @Router      /export/{entity} [get]
func (h *ExportHandler) HandleUniversalExport(c *gin.Context) {
	if h.exportUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "EXPORT_UNAVAILABLE", Message: "export not configured"})
		return
	}

	req := domain.ExportRequest{
		AccountID: getAccountID(c),
		UserID:    getUserID(c),
		Entity:    domain.ExportEntity(c.Param("entity")),
		Format:    domain.ExportFormat(c.Query("format")),
		Filters:   map[string]string{},
	}
	for key, values := range c.Request.URL.Query() {
		if _, reserved := exportReservedParams[key]; reserved || len(values) == 0 {
			continue
		}
		req.Filters[key] = values[0]
	}
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil {
		req.Limit = limit
	}
	if from, ok := parseExportDate(c, "from"); ok && from != nil {
		req.From = from
	} else if !ok {
		return
	}
	if to, ok := parseExportDate(c, "to"); ok && to != nil {
		req.To = to
	} else if !ok {
		return
	}

	result, err := h.exportUC.Execute(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}

	// Announced in a header rather than in the body: the body is a file the
	// browser is already downloading, so a truncation notice has nowhere else
	// to go — and somebody who asked for "all leads" needs to know this is not
	// all of them.
	if result.Truncated {
		c.Header("X-Export-Truncated", "true")
	}
	c.Header("X-Export-Rows", strconv.Itoa(result.Rows))
	c.Header("Content-Disposition", "attachment; filename="+result.Filename)
	c.Data(http.StatusOK, result.ContentType, result.Body)
}

// parseExportDate reads an optional date. Returns (nil, true) when absent and
// (nil, false) after responding with a 400 — an unparseable date is refused
// rather than ignored, because an export silently covering a period nobody asked
// for is a file that gets forwarded to somebody.
func parseExportDate(c *gin.Context, param string) (*time.Time, bool) {
	raw := strings.TrimSpace(c.Query(param))
	if raw == "" {
		return nil, true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, true
		}
	}
	respondError(c, &domain.AppError{
		Code:    "EXPORT_INVALID_PERIOD",
		Message: "data inválida em '" + param + "': use YYYY-MM-DD ou RFC3339",
	})
	return nil, false
}

// HandleListExportAudits serves GET /v1/export/audits.
//
// @Summary     Who exported what
// @Description The export trail: who downloaded which list, with which filters, how many rows and from where. Admin-only — it is the record of who took the customer list.
// @Tags        Export
// @Produce     json
// @Param       limit  query int false "Page size (default 50, max 200)"
// @Param       offset query int false "Page offset"
// @Success     200 {array} domain.ExportAudit
// @Failure     403 {object} domain.AppError
// @Security    BearerAuth
// @Router      /export/audits [get]
func (h *ExportHandler) HandleListExportAudits(c *gin.Context) {
	if h.exportUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "EXPORT_UNAVAILABLE", Message: "export not configured"})
		return
	}
	limit, offset := pagination(c)
	audits, err := h.exportUC.ListAudits(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, audits)
}
