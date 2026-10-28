package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Cadences (§1.5, E1).
type SequenceHandler struct {
	uc   *usecases.SequenceUseCase
	repo domain.SequenceRepository
	// enrollmentEnabled mirrors config.LegacyWorkers: with no cadence
	// runner, enrolling promises follow-ups that never go out.
	enrollmentEnabled bool
}

func NewSequenceHandler(uc *usecases.SequenceUseCase, repo domain.SequenceRepository) *SequenceHandler {
	return &SequenceHandler{uc: uc, repo: repo, enrollmentEnabled: true}
}

// SetEnrollmentEnabled gates POST /sequences/:id/enroll (LEGACY_WORKERS).
func (h *SequenceHandler) SetEnrollmentEnabled(on bool) { h.enrollmentEnabled = on }

const sequencesUnavailableMessage = "cadências automáticas desativadas neste plano"

// HandleList serves GET /v1/sequences.
//
// @Summary  List cadences
// @Tags     sequences
// @Produce  json
// @Success  200 {array} domain.Sequence
// @Router   /sequences [get]
func (h *SequenceHandler) HandleList(c *gin.Context) {
	list, err := h.uc.List(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// HandleGet serves GET /v1/sequences/:id, with its per-step funnel.
//
// @Summary  Get a cadence and its step performance
// @Tags     sequences
// @Produce  json
// @Param    id path string true "Sequence ID"
// @Success  200 {object} sequenceDetail
// @Failure  404 {object} domain.AppError
// @Router   /sequences/{id} [get]
func (h *SequenceHandler) HandleGet(c *gin.Context) {
	seq, err := h.uc.Get(c.Request.Context(), c.Param("id"), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	detail := sequenceDetail{Sequence: seq}
	if h.repo != nil {
		// Best effort: a cadence whose stats will not load is still a
		// cadence somebody needs to edit.
		if stats, err := h.repo.StepStats(c.Request.Context(), seq.ID); err == nil {
			detail.Steps = stats
		}
	}
	c.JSON(http.StatusOK, detail)
}

type sequenceDetail struct {
	*domain.Sequence
	// Steps is the funnel: how many reached each step and how many replied
	// after it. Named separately from Sequence.Steps, which is the plan.
	Steps []domain.SequenceStepStat `json:"step_stats,omitempty"`
}

type sequenceRequest struct {
	Name     string `json:"name"`
	FunnelID string `json:"funnel_id"`
	// BusinessHoursOnly holds sends until the next working hour. A cold
	// email at 03:14 reads as a machine, which is what it must not.
	BusinessHoursOnly *bool                 `json:"business_hours_only"`
	IsActive          bool                  `json:"is_active"`
	Steps             []domain.SequenceStep `json:"steps"`
}

// HandleUpsert serves PUT /v1/sequences/:id ("new" creates).
//
// Validation runs on SAVE, and every violation comes back at once — an editor
// that reports one problem per save turns a five-step cadence into five round
// trips.
//
// @Summary  Create or update a cadence
// @Tags     sequences
// @Accept   json
// @Produce  json
// @Param    id path string true "Sequence ID, or 'new'"
// @Success  200 {object} domain.Sequence
// @Failure  422 {object} sequenceInvalidResponse
// @Router   /sequences/{id} [put]
func (h *SequenceHandler) HandleUpsert(c *gin.Context) {
	var req sequenceRequest
	if !bindJSON(c, &req) {
		return
	}
	id := c.Param("id")
	if id == "new" {
		id = ""
	}
	businessHours := true
	if req.BusinessHoursOnly != nil {
		businessHours = *req.BusinessHoursOnly
	}

	saved, err := h.uc.Save(c.Request.Context(), getAccountID(c), &domain.Sequence{
		ID: id, Name: req.Name, FunnelID: req.FunnelID,
		Steps: req.Steps, BusinessHoursOnly: businessHours,
		IsActive: req.IsActive,
	})
	if err != nil {
		var invalid *usecases.ErrSequenceInvalid
		if errors.As(err, &invalid) {
			// 422, not 400: the request is well-formed and the cadence is
			// not, and the client renders the violations inline against the
			// steps that caused them.
			c.JSON(http.StatusUnprocessableEntity, sequenceInvalidResponse{
				Code:       "SEQUENCE_INVALID",
				Violations: invalid.Violations,
			})
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

type sequenceInvalidResponse struct {
	Code       string                     `json:"code"`
	Violations []domain.SequenceViolation `json:"violations"`
}

// HandleDelete serves DELETE /v1/sequences/:id.
//
// @Summary  Delete a cadence
// @Tags     sequences
// @Param    id path string true "Sequence ID"
// @Success  204
// @Router   /sequences/{id} [delete]
func (h *SequenceHandler) HandleDelete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id"), getAccountID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type enrollRequest struct {
	LeadID string `json:"lead_id" binding:"required"`
}

// HandleEnroll serves POST /v1/sequences/:id/enroll.
//
// @Summary  Put a lead on a cadence
// @Tags     sequences
// @Accept   json
// @Produce  json
// @Param    id path string true "Sequence ID"
// @Success  200 {object} domain.SequenceEnrollment
// @Failure  409 {object} domain.AppError
// @Router   /sequences/{id}/enroll [post]
func (h *SequenceHandler) HandleEnroll(c *gin.Context) {
	if !h.enrollmentEnabled {
		respondFeatureUnavailable(c, "SEQUENCES_UNAVAILABLE", sequencesUnavailableMessage)
		return
	}
	var req enrollRequest
	if !bindJSON(c, &req) {
		return
	}
	enrollment, err := h.uc.Enroll(c.Request.Context(), getAccountID(c),
		c.Param("id"), req.LeadID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, enrollment)
}

// HandlePause serves POST /v1/leads/:id/sequence/pause.
//
// Stops a lead's cadence without marking it a success — distinct from the
// automatic stop on reply, which IS the success.
//
// @Summary  Pause a lead's cadence
// @Tags     sequences
// @Param    id path string true "Lead ID"
// @Success  204
// @Router   /leads/{id}/sequence/pause [post]
func (h *SequenceHandler) HandlePause(c *gin.Context) {
	if err := h.uc.Pause(c.Request.Context(), getAccountID(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
