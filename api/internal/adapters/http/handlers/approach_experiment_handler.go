package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// A/B of approach writing, judged by approval WITHOUT editing (US-F216).
type ApproachExperimentHandler struct {
	uc *usecases.ApproachExperimentUseCase
}

func NewApproachExperimentHandler(uc *usecases.ApproachExperimentUseCase) *ApproachExperimentHandler {
	return &ApproachExperimentHandler{uc: uc}
}

type createApproachExperimentRequest struct {
	Name           string `json:"name"`
	VariantASuffix string `json:"variant_a_suffix"`
	VariantBSuffix string `json:"variant_b_suffix"`
	VariantALabel  string `json:"variant_a_label"`
	VariantBLabel  string `json:"variant_b_label"`
	VariantAModel  string `json:"variant_a_model"`
	VariantBModel  string `json:"variant_b_model"`
	SplitPctB      int    `json:"split_pct_b"`
}

// HandleCreate serves POST /v1/funnels/:id/approach-experiments.
func (h *ApproachExperimentHandler) HandleCreate(c *gin.Context) {
	var req createApproachExperimentRequest
	if !bindJSON(c, &req) {
		return
	}
	e, err := h.uc.Create(c.Request.Context(), usecases.CreateApproachExperimentDTO{
		AccountID: getAccountID(c), FunnelID: c.Param("id"), Name: req.Name,
		VariantASuffix: req.VariantASuffix, VariantBSuffix: req.VariantBSuffix,
		VariantALabel: req.VariantALabel, VariantBLabel: req.VariantBLabel,
		VariantAModel: req.VariantAModel, VariantBModel: req.VariantBModel,
		SplitPctB: req.SplitPctB,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, e)
}

// HandleReport serves GET /v1/approach-experiments/:id — both arms plus a
// deliberately conservative verdict.
func (h *ApproachExperimentHandler) HandleReport(c *gin.Context) {
	rep, err := h.uc.Report(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rep)
}

// HandleList serves GET /v1/approach-experiments.
func (h *ApproachExperimentHandler) HandleList(c *gin.Context) {
	limit, _ := pagination(c)
	out, err := h.uc.List(c.Request.Context(), getAccountID(c), int32(limit)) //nolint:gosec // G115: pagination() clamps
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type stopApproachExperimentRequest struct {
	Winner string `json:"winner"`
}

// HandleStop serves POST /v1/approach-experiments/:id/stop.
func (h *ApproachExperimentHandler) HandleStop(c *gin.Context) {
	var req stopApproachExperimentRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.Stop(c.Request.Context(), getAccountID(c), c.Param("id"), req.Winner); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
