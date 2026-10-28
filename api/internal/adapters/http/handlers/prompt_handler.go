package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// PromptHandler covers the "Meus Prompts" library + synthesis +
// apply/rollback to an agent. Synthesis and apply paths are admin-only;
// listing/reading is admin-only too because prompts may carry business
// instructions.
type PromptHandler struct {
	promptUC      *usecases.PromptUseCase
	synthUC       *usecases.SynthesizePromptUseCase
	applyUC       *usecases.ApplyPromptToAgentUseCase
	refineUC      *usecases.RefinePromptUseCase
	scenariosUC   *usecases.TestPromptScenariosUseCase
	suggestionsUC *usecases.WizardSuggestionsUseCase
}

func NewPromptHandler(
	promptUC *usecases.PromptUseCase,
	synthUC *usecases.SynthesizePromptUseCase,
	applyUC *usecases.ApplyPromptToAgentUseCase,
) *PromptHandler {
	return &PromptHandler{promptUC: promptUC, synthUC: synthUC, applyUC: applyUC}
}

// SetRefiner wires the optional Pro+ refinement use case. When nil the
// /prompts/refine route returns 503 SERVICE_UNAVAILABLE so the route
// can be registered unconditionally.
func (h *PromptHandler) SetRefiner(uc *usecases.RefinePromptUseCase) {
	h.refineUC = uc
}

// SetScenarioTester wires the optional Business+ scenario-test runner.
// Same fallback as SetRefiner: 503 if the route is hit before wiring.
func (h *PromptHandler) SetScenarioTester(uc *usecases.TestPromptScenariosUseCase) {
	h.scenariosUC = uc
}

// SetWizardSuggester wires the RAG-driven wizard suggestion endpoint.
// Returns 503 if the route is hit before wiring.
func (h *PromptHandler) SetWizardSuggester(uc *usecases.WizardSuggestionsUseCase) {
	h.suggestionsUC = uc
}

// ── Library CRUD ────────────────────────────────────────────────────────────

type createPromptReq struct {
	Name          string                `json:"name" binding:"required"`
	Description   *string               `json:"description"`
	Body          string                `json:"body" binding:"required"`
	Source        string                `json:"source"`
	WizardAnswers *domain.WizardAnswers `json:"wizard_answers"`
}

func (h *PromptHandler) HandleCreate(c *gin.Context) {
	var req createPromptReq
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.promptUC.Create(c.Request.Context(), usecases.CreatePromptDTO{
		AccountID:     getAccountID(c),
		UserID:        getUserID(c),
		Name:          req.Name,
		Description:   req.Description,
		Body:          req.Body,
		Source:        req.Source,
		WizardAnswers: req.WizardAnswers,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *PromptHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	out, err := h.promptUC.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *PromptHandler) HandleGet(c *gin.Context) {
	p, err := h.promptUC.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

type updatePromptReq struct {
	Name          string                `json:"name" binding:"required"`
	Description   *string               `json:"description"`
	Body          string                `json:"body" binding:"required"`
	WizardAnswers *domain.WizardAnswers `json:"wizard_answers"`
}

func (h *PromptHandler) HandleUpdate(c *gin.Context) {
	var req updatePromptReq
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.promptUC.Update(c.Request.Context(), usecases.UpdatePromptDTO{
		AccountID:     getAccountID(c),
		ID:            c.Param("id"),
		Name:          req.Name,
		Description:   req.Description,
		Body:          req.Body,
		WizardAnswers: req.WizardAnswers,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *PromptHandler) HandleDelete(c *gin.Context) {
	if err := h.promptUC.Delete(c.Request.Context(), getAccountID(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "prompt deleted"})
}

func (h *PromptHandler) HandleDuplicate(c *gin.Context) {
	p, err := h.promptUC.Duplicate(c.Request.Context(), getAccountID(c), getUserID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

// ── Synthesis ───────────────────────────────────────────────────────────────

type synthesizeReq struct {
	Answers domain.WizardAnswers `json:"answers" binding:"required"`
	SaveAs  string               `json:"save_as"`
}

func (h *PromptHandler) HandleSynthesize(c *gin.Context) {
	var req synthesizeReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.synthUC.Execute(c.Request.Context(), usecases.SynthesizePromptDTO{
		AccountID: getAccountID(c),
		UserID:    getUserID(c),
		Answers:   req.Answers,
		SaveAs:    req.SaveAs,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ── Refinement (Pro+) ──────────────────────────────────────────────────────

type refineReq struct {
	CurrentBody      string               `json:"current_body" binding:"required"`
	Answers          domain.WizardAnswers `json:"answers"`
	Section          string               `json:"section"`
	DeltaInstruction string               `json:"delta_instruction" binding:"required"`
}

func (h *PromptHandler) HandleRefine(c *gin.Context) {
	if h.refineUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "REFINER_DISABLED", "message": "refinement is not configured"},
		})
		return
	}
	var req refineReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.refineUC.Execute(c.Request.Context(), usecases.RefinePromptDTO{
		AccountID:        getAccountID(c),
		UserID:           getUserID(c),
		CurrentBody:      req.CurrentBody,
		Answers:          req.Answers,
		Section:          req.Section,
		DeltaInstruction: req.DeltaInstruction,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ── Test scenarios (Business+) ─────────────────────────────────────────────

type testScenariosReq struct {
	Body     string               `json:"body" binding:"required"`
	Vertical string               `json:"vertical"`
	Answers  domain.WizardAnswers `json:"answers"`
}

func (h *PromptHandler) HandleTestScenarios(c *gin.Context) {
	if h.scenariosUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "SCENARIOS_DISABLED", "message": "test scenarios are not configured"},
		})
		return
	}
	var req testScenariosReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.scenariosUC.Execute(c.Request.Context(), usecases.TestPromptScenariosDTO{
		AccountID: getAccountID(c),
		UserID:    getUserID(c),
		Vertical:  req.Vertical,
		Body:      req.Body,
		Answers:   req.Answers,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ── Wizard suggestions from RAG ────────────────────────────────────────────

func (h *PromptHandler) HandleWizardSuggestions(c *gin.Context) {
	if h.suggestionsUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "SUGGESTIONS_DISABLED", "message": "wizard suggestions are not configured"},
		})
		return
	}
	out, err := h.suggestionsUC.Execute(c.Request.Context(), usecases.WizardSuggestionsDTO{
		AccountID: getAccountID(c),
		UserID:    getUserID(c),
		AgentID:   c.Param("id"),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ── Apply / Rollback / Versions ────────────────────────────────────────────

type applyPromptReq struct {
	PromptID *string `json:"prompt_id"`
	Body     *string `json:"body"`
	Note     *string `json:"note"`
}

func (h *PromptHandler) HandleApply(c *gin.Context) {
	var req applyPromptReq
	if !bindJSON(c, &req) {
		return
	}
	v, err := h.applyUC.Execute(c.Request.Context(), usecases.ApplyPromptToAgentDTO{
		AccountID: getAccountID(c),
		UserID:    getUserID(c),
		AgentID:   c.Param("id"),
		PromptID:  req.PromptID,
		Body:      req.Body,
		Note:      req.Note,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *PromptHandler) HandleListVersions(c *gin.Context) {
	limit, offset := pagination(c)
	out, err := h.applyUC.ListVersions(c.Request.Context(), getAccountID(c), c.Param("id"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *PromptHandler) HandleRollback(c *gin.Context) {
	v, err := h.applyUC.Rollback(c.Request.Context(),
		getAccountID(c), getUserID(c), c.Param("id"), c.Param("vid"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
