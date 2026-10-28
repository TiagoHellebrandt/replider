package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Sales strategies (§4, E2/E3).
//
// The written-down version of "como a gente vende", and the thing every
// approach is attributed to. Without the attribution nothing downstream
// measures anything.
type StrategyHandler struct {
	repo domain.StrategyRepository
}

func NewStrategyHandler(repo domain.StrategyRepository) *StrategyHandler {
	return &StrategyHandler{repo: repo}
}

type strategyDetail struct {
	*domain.Strategy
	// Stats is per VERSION: an approach sent under version 2 must not count
	// toward version 3's numbers, which is the whole reason versions exist.
	Stats []domain.StrategyStat `json:"stats,omitempty"`
}

// HandleList serves GET /v1/strategies.
//
// @Summary  List sales strategies
// @Tags     strategies
// @Produce  json
// @Success  200 {array} strategyDetail
// @Router   /strategies [get]
func (h *StrategyHandler) HandleList(c *gin.Context) {
	accountID := getAccountID(c)
	list, err := h.repo.List(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}

	// One stats query for the whole list rather than one per strategy: the
	// numbers come from a single GROUP BY, and N round trips for a screen
	// that renders N cards is a shape that only gets worse.
	byStrategy := map[string][]domain.StrategyStat{}
	if stats, err := h.repo.Stats(c.Request.Context(), accountID); err == nil {
		for _, s := range stats {
			byStrategy[s.StrategyID] = append(byStrategy[s.StrategyID], s)
		}
	}

	out := make([]strategyDetail, 0, len(list))
	for _, s := range list {
		out = append(out, strategyDetail{Strategy: s, Stats: byStrategy[s.ID]})
	}
	c.JSON(http.StatusOK, out)
}

// HandleGet serves GET /v1/strategies/:id.
//
// @Summary  Get a sales strategy
// @Tags     strategies
// @Produce  json
// @Param    id path string true "Strategy ID"
// @Success  200 {object} strategyDetail
// @Failure  404 {object} domain.AppError
// @Router   /strategies/{id} [get]
func (h *StrategyHandler) HandleGet(c *gin.Context) {
	accountID := getAccountID(c)
	s, err := h.repo.Get(c.Request.Context(), c.Param("id"), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	if s == nil {
		respondError(c, &domain.AppError{
			Code: "STRATEGY_NOT_FOUND", Message: "estratégia não encontrada"})
		return
	}
	detail := strategyDetail{Strategy: s}
	if stats, err := h.repo.Stats(c.Request.Context(), accountID); err == nil {
		for _, stat := range stats {
			if stat.StrategyID == s.ID {
				detail.Stats = append(detail.Stats, stat)
			}
		}
	}
	c.JSON(http.StatusOK, detail)
}

type strategyRequest struct {
	Name       string                     `json:"name"`
	Status     string                     `json:"status"`
	ICP        string                     `json:"icp"`
	ChannelMix string                     `json:"channel_mix"`
	SequenceID string                     `json:"sequence_id"`
	Tone       string                     `json:"tone"`
	Arguments  []string                   `json:"arguments"`
	Objections []domain.StrategyObjection `json:"objections"`
	Triggers   []string                   `json:"triggers"`
}

// HandleUpsert serves PUT /v1/strategies/:id ("new" creates).
//
// @Summary  Create or update a sales strategy
// @Tags     strategies
// @Accept   json
// @Produce  json
// @Param    id path string true "Strategy ID, or 'new'"
// @Success  200 {object} domain.Strategy
// @Failure  400 {object} domain.AppError
// @Router   /strategies/{id} [put]
func (h *StrategyHandler) HandleUpsert(c *gin.Context) {
	var req strategyRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "dê um nome à estratégia"})
		return
	}
	status := domain.StrategyStatus(strings.ToLower(strings.TrimSpace(req.Status)))
	if req.Status == "" {
		// New strategies start as drafts. Half a strategy applied to a real
		// prospect is worse than none.
		status = domain.StrategyDraft
	} else if !domain.ValidStrategyStatus(req.Status) {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "situação inválida"})
		return
	}

	id := c.Param("id")
	if id == "" || id == "new" {
		id = ulid.Make().String()
	}

	s := &domain.Strategy{
		ID: id, AccountID: getAccountID(c), Name: req.Name, Status: status,
		ICP: req.ICP, ChannelMix: req.ChannelMix, SequenceID: req.SequenceID,
		Tone: req.Tone, Arguments: req.Arguments, Objections: req.Objections,
		Triggers: req.Triggers,
		// Authorship is recognition, and recognition is what makes a
		// salesperson contribute one. Taken from the token rather than the
		// body: an author field a client can set is a byline anyone can
		// forge.
		AuthorUserID: getUserID(c),
		AuthorName:   c.GetString("userName"),
	}
	if err := h.repo.Upsert(c.Request.Context(), s); err != nil {
		respondError(c, err)
		return
	}
	saved, err := h.repo.Get(c.Request.Context(), id, s.AccountID)
	if err != nil || saved == nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

type strategyStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// HandleSetStatus serves PUT /v1/strategies/:id/status.
//
// Separate from the upsert because promoting or retiring a strategy is a
// decision about the team's method, not an edit — and it must NOT bump the
// version, or promoting one would orphan the numbers that justified it.
//
// @Summary  Promote, retire or move a strategy into testing
// @Tags     strategies
// @Accept   json
// @Param    id path string true "Strategy ID"
// @Success  204
// @Failure  400 {object} domain.AppError
// @Router   /strategies/{id}/status [put]
func (h *StrategyHandler) HandleSetStatus(c *gin.Context) {
	var req strategyStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	if !domain.ValidStrategyStatus(req.Status) {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "situação inválida"})
		return
	}
	err := h.repo.SetStatus(c.Request.Context(), c.Param("id"), getAccountID(c),
		domain.StrategyStatus(strings.ToLower(strings.TrimSpace(req.Status))))
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
