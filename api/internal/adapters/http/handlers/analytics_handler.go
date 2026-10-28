package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type AnalyticsHandler struct {
	getAnalyticsUC    *usecases.GetAnalyticsUseCase
	attendantPerfUC   *usecases.GetAttendantPerformanceUseCase
	liveSupervisionUC *usecases.GetLiveSupervisionUseCase
	overviewUC        *usecases.GetAnalyticsOverviewUseCase
	expensiveConvUC   *usecases.GetExpensiveConversationsUseCase
	// sdrUC is optional (see sdr_analytics_handler.go).
	sdrUC *usecases.SDRAnalyticsUseCase
}

func NewAnalyticsHandler(
	getAnalyticsUC *usecases.GetAnalyticsUseCase,
	attendantPerfUC *usecases.GetAttendantPerformanceUseCase,
	liveSupervisionUC *usecases.GetLiveSupervisionUseCase,
	overviewUC *usecases.GetAnalyticsOverviewUseCase,
	expensiveConvUC *usecases.GetExpensiveConversationsUseCase,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		getAnalyticsUC:    getAnalyticsUC,
		attendantPerfUC:   attendantPerfUC,
		liveSupervisionUC: liveSupervisionUC,
		overviewUC:        overviewUC,
		expensiveConvUC:   expensiveConvUC,
	}
}

// HandleListExpensiveConversations godoc
// @Summary      Top 10 most-expensive conversations (US-030)
// @Description  Returns the account's conversations ranked by accumulated
// @Description  estimated AI provider cost (USD). Powers the cost-
// @Description  attribution dashboard. Conversations with zero cost are
// @Description  omitted.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}  domain.ExpensiveConversation
// @Router       /analytics/expensive-conversations [get]
func (h *AnalyticsHandler) HandleListExpensiveConversations(c *gin.Context) {
	accountID := c.GetString("accountID")
	rows, err := h.expensiveConvUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// HandleGetOverview godoc
// @Summary      Operational analytics overview (Mission 6 MVP)
// @Description  Returns total conversations, % resolved by AI, avg
// @Description  first-response time, per-channel volume, top 10
// @Description  initial messages, and CSAT distribution for the
// @Description  selected window. `days` must be 7, 30 or 90; any
// @Description  other value is silently clamped to 30.
// @Description  Numbers start from the feature rollout date —
// @Description  there is no backfill of pre-existing conversations.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Param        days query int false "Window in days (7|30|90, default 30)"
// @Success      200  {object}  domain.AnalyticsOverview
// @Router       /analytics/overview [get]
func (h *AnalyticsHandler) HandleGetOverview(c *gin.Context) {
	accountID := c.GetString("accountID")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	overview, err := h.overviewUC.Execute(c.Request.Context(), accountID, days)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}

// HandleGetAnalytics godoc
// @Summary      Get Analytics
// @Description  Returns conversation, message, and usage statistics for the account.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.AnalyticsResponse
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /analytics [get]
func (h *AnalyticsHandler) HandleGetAnalytics(c *gin.Context) {
	accountID := c.GetString("accountID")

	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	// Optional date range filter (ISO 8601: YYYY-MM-DD)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	_ = startDate // TODO: pass to use case when analytics repo supports date filtering
	_ = endDate

	response, err := h.getAnalyticsUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleListAttendantPerformance godoc
// @Summary      Per-attendant performance metrics
// @Description  Returns one row per user in the account with metrics
// @Description  aggregated over the chosen fixed period. Attendants
// @Description  can only read their own row; admins see everyone.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Param        period   query  string  false  "today|7d|30d|mtd (default 30d)"
// @Param        user_id  query  string  false  "scope to one user"
// @Success      200  {array}  domain.AttendantPerformance
// @Failure      401  {object}  domain.AppError
// @Router       /analytics/attendants [get]
func (h *AnalyticsHandler) HandleListAttendantPerformance(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	rows, err := h.attendantPerfUC.Execute(c.Request.Context(), accountID, userID, attendantDTOFromQuery(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// HandleExportAttendantPerformanceCSV godoc
// @Summary      Export attendant metrics as CSV
// @Description  Same filters as the list endpoint; streams a CSV with
// @Description  header row. Used by gestores to close monthly payroll
// @Description  / bonus numbers outside the app.
// @Tags         analytics
// @Produce      text/csv
// @Security     BearerAuth
// @Param        period   query  string  false  "today|7d|30d|mtd"
// @Param        user_id  query  string  false  "scope to one user"
// @Success      200  {string}  text/csv
// @Router       /analytics/attendants/export.csv [get]
func (h *AnalyticsHandler) HandleExportAttendantPerformanceCSV(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	rows, err := h.attendantPerfUC.Execute(c.Request.Context(), accountID, userID, attendantDTOFromQuery(c))
	if err != nil {
		respondError(c, err)
		return
	}

	filename := fmt.Sprintf("attendant-metrics-%s.csv", time.Now().UTC().Format("20060102-150405"))
	c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Writer.WriteHeader(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{
		"user_id", "name", "email", "role", "last_seen_at",
		"conversations_assigned", "conversations_resolved", "messages_sent",
		"avg_first_response_seconds", "avg_resolution_seconds",
		"csat_avg", "csat_count",
	})
	for _, r := range rows {
		lastSeen := ""
		if r.LastSeenAt != nil {
			lastSeen = r.LastSeenAt.UTC().Format(time.RFC3339)
		}
		_ = w.Write(domain.CSVSafeRow([]string{
			r.UserID,
			r.Name,
			r.Email,
			r.Role,
			lastSeen,
			strconv.FormatInt(r.ConversationsAssigned, 10),
			strconv.FormatInt(r.ConversationsResolved, 10),
			strconv.FormatInt(r.MessagesSent, 10),
			strconv.Itoa(r.AvgFirstResponseSeconds),
			strconv.Itoa(r.AvgResolutionSeconds),
			strconv.FormatFloat(float64(r.CSATAvg), 'f', 2, 32),
			strconv.FormatInt(r.CSATCount, 10),
		}))
	}
	w.Flush()
}

// HandleLiveSupervision godoc
// @Summary      Real-time supervisor snapshot
// @Description  Admin-only right-now view: per-attendant open-
// @Description  conversation load + unassigned queue summary in one
// @Description  round-trip. Frontend polls at ~10s while the panel is
// @Description  open. Returns 403 for non-admins.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  domain.LiveSupervisionSnapshot
// @Failure      401  {object}  domain.AppError
// @Failure      403  {object}  domain.AppError
// @Router       /analytics/attendants/live [get]
func (h *AnalyticsHandler) HandleLiveSupervision(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	snapshot, err := h.liveSupervisionUC.Execute(c.Request.Context(), accountID, userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func attendantDTOFromQuery(c *gin.Context) usecases.GetAttendantPerformanceDTO {
	dto := usecases.GetAttendantPerformanceDTO{
		Period: usecases.AttendantPerformancePeriod(c.DefaultQuery("period", "30d")),
	}
	if v := c.Query("user_id"); v != "" {
		dto.UserID = &v
	}
	return dto
}
