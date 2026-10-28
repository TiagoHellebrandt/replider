package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

type ExperimentHandler struct {
	uc        *usecases.ExperimentUseCase
	suggestUC *usecases.SuggestImprovementsUseCase
}

func NewExperimentHandler(uc *usecases.ExperimentUseCase) *ExperimentHandler {
	return &ExperimentHandler{uc: uc}
}

// SetSuggester wires the optional MVP-3 dependency. Kept as a setter
// so the handler stays usable on Pro accounts (where MVP-3 isn't
// gated in) and so tests can construct the handler without a
// suggester double.
func (h *ExperimentHandler) SetSuggester(s *usecases.SuggestImprovementsUseCase) {
	h.suggestUC = s
}

type createExperimentReq struct {
	Name          string `json:"name" binding:"required"`
	VariantABody  string `json:"variant_a_body" binding:"required"`
	VariantBBody  string `json:"variant_b_body" binding:"required"`
	VariantALabel string `json:"variant_a_label"`
	VariantBLabel string `json:"variant_b_label"`
	SplitPctB     int    `json:"split_pct_b"`
}

func (h *ExperimentHandler) HandleCreate(c *gin.Context) {
	var req createExperimentReq
	if !bindJSON(c, &req) {
		return
	}
	e, err := h.uc.Create(c.Request.Context(), usecases.CreateExperimentDTO{
		AccountID:     getAccountID(c),
		AgentID:       c.Param("id"),
		Name:          req.Name,
		VariantABody:  req.VariantABody,
		VariantBBody:  req.VariantBBody,
		VariantALabel: req.VariantALabel,
		VariantBLabel: req.VariantBLabel,
		SplitPctB:     req.SplitPctB,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, e)
}

func (h *ExperimentHandler) HandleListByAgent(c *gin.Context) {
	limit, offset := pagination(c)
	out, err := h.uc.ListByAgent(c.Request.Context(), getAccountID(c), c.Param("id"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *ExperimentHandler) HandleGet(c *gin.Context) {
	e, err := h.uc.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *ExperimentHandler) HandleStart(c *gin.Context) {
	e, err := h.uc.Start(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *ExperimentHandler) HandleStop(c *gin.Context) {
	e, err := h.uc.Stop(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *ExperimentHandler) HandleResults(c *gin.Context) {
	res, err := h.uc.Results(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *ExperimentHandler) HandleSuggest(c *gin.Context) {
	if h.suggestUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "SUGGESTER_DISABLED",
			"message": "auto-suggester not configured",
		})
		return
	}
	out, err := h.suggestUC.Execute(c.Request.Context(), usecases.SuggestImprovementsDTO{
		AccountID:    getAccountID(c),
		UserID:       getUserID(c),
		ExperimentID: c.Param("id"),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
