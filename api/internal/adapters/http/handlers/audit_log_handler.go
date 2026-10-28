package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// AuditLogHandler exposes the read-only admin view of the audit log.
// Used by compliance reports and by admins investigating "who did what"
// — especially around LGPD actions (export / anonymize).
type AuditLogHandler struct {
	repo domain.AuditLogRepository
}

func NewAuditLogHandler(repo domain.AuditLogRepository) *AuditLogHandler {
	return &AuditLogHandler{repo: repo}
}

type listAuditLogsResponse struct {
	Total   int64              `json:"total"`
	Entries []*domain.AuditLog `json:"entries"`
}

// HandleList godoc
// @Summary  List audit log entries for the current account
// @Tags     audit
// @Produce  json
// @Security BearerAuth
// @Param    limit        query  int     false  "max items (default 50, max 200)"
// @Param    offset       query  int     false  "offset (default 0)"
// @Param    action       query  string  false  "exact-match action filter"
// @Param    entity_type  query  string  false  "exact-match entity_type filter"
// @Param    actor_id     query  string  false  "exact-match actor_id filter"
// @Param    from         query  string  false  "from RFC3339 timestamp"
// @Param    to           query  string  false  "to RFC3339 timestamp"
// @Success  200  {object}  listAuditLogsResponse
// @Router   /audit [get]
func (h *AuditLogHandler) HandleList(c *gin.Context) {
	accountID := c.GetString("accountID")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filter := h.filterFromQuery(c, accountID, int32(limit), int32(offset))

	entries, err := h.repo.ListFiltered(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}
	total, err := h.repo.CountFiltered(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, listAuditLogsResponse{Total: total, Entries: entries})
}

// HandleExportCSV godoc
// @Summary  Export filtered audit log as CSV
// @Description  Streams a CSV download of up to 10000 matching rows.
// @Description  Same filters as the list endpoint; pagination is ignored.
// @Tags     audit
// @Produce  text/csv
// @Security BearerAuth
// @Param    action       query  string  false  "exact-match action filter"
// @Param    entity_type  query  string  false  "exact-match entity_type filter"
// @Param    actor_id     query  string  false  "exact-match actor_id filter"
// @Param    from         query  string  false  "from RFC3339 timestamp"
// @Param    to           query  string  false  "to RFC3339 timestamp"
// @Success  200  {string}  text/csv
// @Router   /audit/export.csv [get]
func (h *AuditLogHandler) HandleExportCSV(c *gin.Context) {
	accountID := c.GetString("accountID")
	const maxRows = 10000
	filter := h.filterFromQuery(c, accountID, maxRows, 0)

	entries, err := h.repo.ListFiltered(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}

	filename := fmt.Sprintf("audit-log-%s.csv", time.Now().UTC().Format("20060102-150405"))
	c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Writer.WriteHeader(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"id", "created_at", "actor_id", "action", "entity_type", "entity_id", "payload", "ip", "user_agent"})
	for _, e := range entries {
		actor := ""
		if e.ActorID != nil {
			actor = *e.ActorID
		}
		ip := ""
		if e.IP != nil {
			ip = *e.IP
		}
		ua := ""
		if e.UserAgent != nil {
			ua = *e.UserAgent
		}
		_ = w.Write(domain.CSVSafeRow([]string{
			e.ID,
			e.CreatedAt.UTC().Format(time.RFC3339),
			actor,
			e.Action,
			e.EntityType,
			e.EntityID,
			string(e.Payload),
			ip,
			ua,
		}))
	}
	w.Flush()
}

func (h *AuditLogHandler) filterFromQuery(c *gin.Context, accountID string, limit, offset int32) domain.AuditLogFilter {
	f := domain.AuditLogFilter{AccountID: accountID, Limit: limit, Offset: offset}
	if v := c.Query("action"); v != "" {
		f.Action = &v
	}
	if v := c.Query("entity_type"); v != "" {
		f.EntityType = &v
	}
	if v := c.Query("actor_id"); v != "" {
		f.ActorID = &v
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	return f
}
