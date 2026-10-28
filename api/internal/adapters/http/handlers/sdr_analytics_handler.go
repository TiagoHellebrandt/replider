package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// SDR analytics (§6.12, I1).

// The endpoints hang off the existing AnalyticsHandler and are wired by an
// optional setter, so neither NewAnalyticsHandler nor router.Setup changes
// signature (same pattern as me_support_handler.go). An unwired use case
// answers 503 rather than panicking.

// SetSDRAnalyticsUC wires the SDR report.
func (h *AnalyticsHandler) SetSDRAnalyticsUC(uc *usecases.SDRAnalyticsUseCase) {
	h.sdrUC = uc
}

// HandleGetSDRAnalytics serves GET /v1/analytics/sdr.
//
// @Summary     SDR analytics
// @Description Funnel, strategy, seller, channel and signal cuts of one period. Rates over an empty denominator are omitted rather than reported as zero.
// @Tags        Analytics
// @Produce     json
// @Param       period query string false "today | 7d | 30d | mtd (default 7d)"
// @Success     200 {object} domain.SDRAnalytics
// @Failure     401 {object} domain.AppError
// @Failure     500 {object} domain.AppError
// @Security    BearerAuth
// @Router      /analytics/sdr [get]
func (h *AnalyticsHandler) HandleGetSDRAnalytics(c *gin.Context) {
	if h.sdrUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "SDR_ANALYTICS_UNAVAILABLE", Message: "sdr analytics not configured"})
		return
	}
	report, err := h.sdrUC.Execute(c.Request.Context(), getAccountID(c), c.Query("period"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

// sdrDrillResponse names the window the list covers.
//
// The period travels back with the rows because the drill-down was reached by
// clicking a number: the list has to be able to say "estes são os 40 abordados
// entre 24/07 e 30/07", or an operator counting rows against a remembered
// number has no way to tell a filter apart from a stale screen.
type sdrDrillResponse struct {
	Period domain.SDRPeriod    `json:"period"`
	Metric string              `json:"metric"`
	Leads  []domain.SDRLeadRow `json:"leads"`
}

// HandleDrillSDRAnalytics serves GET /v1/analytics/sdr/leads.
//
// The one drill-down for the whole report: a funnel rung, optionally narrowed
// by strategy, seller, channel or signal source. One endpoint rather than one
// per section, so every number on the screen opens the same way.
//
// @Summary     Leads behind an SDR metric
// @Description Lists the leads counted by one funnel rung, narrowed by the optional cuts. `seller_id=none` selects the leads nobody owns.
// @Tags        Analytics
// @Produce     json
// @Param       metric        query string true  "entered | researched | approached | replied | qualified | won"
// @Param       period        query string false "today | 7d | 30d | mtd (default 7d)"
// @Param       strategy_id   query string false "Narrow to one strategy"
// @Param       seller_id     query string false "Narrow to one seller, or `none` for unowned leads"
// @Param       channel       query string false "Narrow to leads approached through this channel type"
// @Param       signal_source query string false "Narrow to leads originating from this signal source"
// @Param       limit         query int    false "Page size (default 50, max 200)"
// @Param       offset        query int    false "Page offset"
// @Success     200 {object} sdrDrillResponse
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Security    BearerAuth
// @Router      /analytics/sdr/leads [get]
func (h *AnalyticsHandler) HandleDrillSDRAnalytics(c *gin.Context) {
	if h.sdrUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "SDR_ANALYTICS_UNAVAILABLE", Message: "sdr analytics not configured"})
		return
	}
	limit, offset := pagination(c)
	// Trimmed here, before the guard: the guard validates the exact string it
	// is handed, so trimming afterwards would let a value through that the
	// query then fails to match.
	filter := domain.SDRDrillFilter{
		AccountID:    getAccountID(c),
		Metric:       strings.TrimSpace(c.Query("metric")),
		StrategyID:   strings.TrimSpace(c.Query("strategy_id")),
		SellerUserID: strings.TrimSpace(c.Query("seller_id")),
		Channel:      strings.TrimSpace(c.Query("channel")),
		SignalSource: strings.TrimSpace(c.Query("signal_source")),
		Limit:        int32(limit),  //nolint:gosec // G115: pagination() caps at 200
		Offset:       int32(offset), //nolint:gosec // G115: pagination() floors at 0
	}

	leads, period, err := h.sdrUC.Drill(c.Request.Context(), filter, c.Query("period"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, sdrDrillResponse{Period: period, Metric: filter.Metric, Leads: leads})
}
