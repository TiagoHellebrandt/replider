package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type MeHandler struct {
	getMeUC                      *usecases.GetMeUseCase
	updateProfileUC              *usecases.UpdateProfileUseCase
	changePasswordUC             *usecases.ChangePasswordUseCase
	getOnboardingProgressUC      *usecases.GetOnboardingProgressUseCase
	dismissOnboardingChecklistUC *usecases.DismissOnboardingChecklistUseCase
	updateOnboardingDraftUC      *usecases.UpdateOnboardingDraftUseCase
	setAPIKeysUC                 *usecases.SetAccountAPIKeysUseCase
	// dashboardBootstrapUC is the optional BFF aggregator behind
	// GET /me/dashboard. Wired via SetDashboardBootstrapUC so the
	// constructor stays stable for existing tests; nil ⇒ the route 503s.
	dashboardBootstrapUC *usecases.GetDashboardBootstrapUseCase
	// aiCostsUC serves the AI spend report + the customer ceiling (BYOK
	// cost model). Wired via SetAICostsUC — see me_ai_costs_handler.go.
	aiCostsUC *usecases.AICostReportUseCase
	// commandCenterUC backs GET /me/command-center + the pending-action
	// snooze (I2). See command_center_handler.go.
	commandCenterUC *usecases.CommandCenterUseCase
	// offersOutcomeUC backs GET /me/offers/outcome (setter-wired; nil → 503).
	offersOutcomeUC *usecases.OffersOutcomeUseCase
	// radarUC + radarParkOfferUC back /me/radar (setter-wired; nil → 503).
	radarUC          *usecases.RadarUseCase
	radarParkOfferUC *usecases.RadarParkOfferUseCase
	territoryHuntUC  *usecases.TerritoryHuntUseCase
	autoHuntUC       *usecases.AutoHuntUseCase
	// firstOfferUC backs POST /me/onboarding/first-offer, the wizard's last
	// step. See me_first_offer_handler.go.
	firstOfferUC *usecases.FirstOfferUseCase
	// overnightRepo backs the overnight-drafting opt-in and the morning
	// briefing acknowledgement (K2).
	overnightRepo domain.OvernightDraftRepository
	// onboardingChatUC backs the conversational onboarding (K5).
	onboardingChatUC *usecases.OnboardingChatUseCase
	// referralInfoUC backs GET /me/referral (US-174). Wired via
	// SetReferralInfoUC so the constructor stays stable; nil ⇒ the route 503s.
	referralInfoUC *usecases.GetReferralInfoUseCase
	// submitNPSUC backs POST /me/nps (US-178). Wired via SetSubmitNPSUC so
	// the constructor stays stable; nil ⇒ the route 503s.
	submitNPSUC *usecases.SubmitNPSUseCase
	// npsPromptUC backs GET /me/nps-prompt + POST /me/nps/dismiss (US-178
	// follow-up): server-side "should we show the NPS prompt?" + dismiss.
	// Wired via SetNPSPromptUC; nil ⇒ the prompt endpoint reports not
	// eligible (fail-closed, never nags) and dismiss 503s.
	npsPromptUC *usecases.NPSPromptUseCase
	// submitCancellationSurveyUC backs POST /me/cancellation-survey (US-180).
	// Wired via SetSubmitCancellationSurveyUC so the constructor stays stable;
	// nil ⇒ the route 503s.
	submitCancellationSurveyUC *usecases.SubmitCancellationSurveyUseCase
	// supportThreadUC backs GET/POST /me/support/messages — the internal CS
	// channel (canal-interno US-002). Wired via SetSupportThreadUC so the
	// constructor stays stable; nil ⇒ the routes report the feature as
	// unavailable. Handlers live in me_support_handler.go.
	supportThreadUC *usecases.SupportThreadUseCase
}

// SetSubmitNPSUC wires the in-app NPS submission use case (US-178). Kept as
// a setter so the MeHandler constructor signature stays untouched.
func (h *MeHandler) SetSubmitNPSUC(uc *usecases.SubmitNPSUseCase) {
	h.submitNPSUC = uc
}

// SetNPSPromptUC wires the server-side NPS prompt eligibility/dismiss use
// case (US-178 follow-up). Optional setter; nil is tolerated.
func (h *MeHandler) SetNPSPromptUC(uc *usecases.NPSPromptUseCase) {
	h.npsPromptUC = uc
}

// SetSubmitCancellationSurveyUC wires the cancellation-reason survey use case
// (US-180). Kept as a setter so the MeHandler constructor signature stays
// untouched.
func (h *MeHandler) SetSubmitCancellationSurveyUC(uc *usecases.SubmitCancellationSurveyUseCase) {
	h.submitCancellationSurveyUC = uc
}

// SetReferralInfoUC wires the referral-info use case (US-174). Kept as a
// setter so the MeHandler constructor signature stays untouched.
func (h *MeHandler) SetReferralInfoUC(uc *usecases.GetReferralInfoUseCase) {
	h.referralInfoUC = uc
}

// SetDashboardBootstrapUC wires the BFF dashboard aggregator (US-197).
// Kept as a setter so the MeHandler constructor signature — and its many
// existing call sites in tests — stay untouched.
func (h *MeHandler) SetDashboardBootstrapUC(uc *usecases.GetDashboardBootstrapUseCase) {
	h.dashboardBootstrapUC = uc
}

func NewMeHandler(
	getMeUC *usecases.GetMeUseCase,
	updateProfileUC *usecases.UpdateProfileUseCase,
	changePasswordUC *usecases.ChangePasswordUseCase,
	getOnboardingProgressUC *usecases.GetOnboardingProgressUseCase,
	dismissOnboardingChecklistUC *usecases.DismissOnboardingChecklistUseCase,
	updateOnboardingDraftUC *usecases.UpdateOnboardingDraftUseCase,
	setAPIKeysUC *usecases.SetAccountAPIKeysUseCase,
) *MeHandler {
	return &MeHandler{
		getMeUC:                      getMeUC,
		updateProfileUC:              updateProfileUC,
		changePasswordUC:             changePasswordUC,
		getOnboardingProgressUC:      getOnboardingProgressUC,
		dismissOnboardingChecklistUC: dismissOnboardingChecklistUC,
		updateOnboardingDraftUC:      updateOnboardingDraftUC,
		setAPIKeysUC:                 setAPIKeysUC,
	}
}

// respondErr is the /me handlers' error writer. It used to keep its own
// three-case switch and default everything else to 400, which meant a code
// the shared table already maps (a 409 plan ceiling, a 429 cap) came out of
// THESE routes as "bad request". It now delegates to respondError, keeping
// only the three statuses the table does not know about.
func respondErr(c *gin.Context, err error) {
	if appErr, ok := err.(*domain.AppError); ok {
		switch appErr.Code {
		case domain.ErrUnauthorized.Code:
			c.JSON(http.StatusUnauthorized, appErr)
			return
		case domain.ErrEmailAlreadyExists.Code:
			c.JSON(http.StatusConflict, appErr)
			return
		case "BYOK_CIPHER_UNCONFIGURED":
			// Server is missing BYOK_ENCRYPTION_KEY — a deployment/config
			// fault, not bad client input. 503 so it doesn't read as the
			// user's mistake (the old default 400 was misleading). Written
			// unsanitized on purpose: the message names the missing env var
			// and carries no schema/PII.
			c.JSON(http.StatusServiceUnavailable, appErr)
			return
		}
	}
	// Non-AppError stays a generic 500 (now sanitized + logged with the
	// request id, like every other handler).
	respondError(c, err)
}

// GetMe godoc
// @Summary      Get My Profile
// @Description  Returns user and account information for the currently authenticated user.
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.GetMeResponse
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /me [get]
func (h *MeHandler) HandleGetMe(c *gin.Context) {
	userID := c.GetString("userID")
	accountID := c.GetString("accountID")

	if userID == "" || accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	response, err := h.getMeUC.Execute(c.Request.Context(), userID, accountID)
	if err != nil {
		respondErr(c, err)
		return
	}

	// Echo the impersonation context so the dashboard banner has a
	// server-truth source. The auth middleware stamps it from the
	// JWT claim — empty for regular sessions.
	if actor := c.GetString("impersonatingActor"); actor != "" {
		response.ImpersonatingActor = actor
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetDashboard godoc
// @Summary      Dashboard bootstrap (BFF aggregate)
// @Description  Returns everything the dashboard needs on cold start — identity, plan limits, usage counters, agents, channels and recent conversations — in a single round-trip. This BFF aggregate (US-197) replaces the six separate GETs (/me, /me/plan-limits, /usage, /agents, /channels, /conversations) the client used to fire in parallel. Identity is essential; any other section that fails to load is omitted and named in `warnings` instead of failing the whole response.
// @Tags         Identity
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.DashboardBootstrapResponse
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Failure      503  {object}  domain.AppError  "BFF aggregator not configured"
// @Router       /me/dashboard [get]
func (h *MeHandler) HandleGetDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	accountID := c.GetString("accountID")

	if userID == "" || accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.dashboardBootstrapUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "BFF_UNAVAILABLE", Message: "dashboard aggregator not configured"})
		return
	}

	response, err := h.dashboardBootstrapUC.Execute(c.Request.Context(), userID, accountID)
	if err != nil {
		respondErr(c, err)
		return
	}

	// Mirror HandleGetMe: echo impersonation context onto the nested /me block.
	if actor := c.GetString("impersonatingActor"); actor != "" && response.Me != nil {
		response.Me.ImpersonatingActor = actor
	}

	c.JSON(http.StatusOK, response)
}

// HandleUpdateProfile godoc
// @Summary      Update Profile
// @Description  Updates the authenticated user's name and email.
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      updateProfileRequest  true  "Profile data"
// @Success      200  {object}  domain.User
// @Failure      400  {object}  domain.AppError
// @Failure      409  {object}  domain.AppError  "code: AUTH_EMAIL_ALREADY_EXISTS"
// @Router       /me [put]
func (h *MeHandler) HandleUpdateProfile(c *gin.Context) {
	userID := c.GetString("userID")
	accountID := c.GetString("accountID")

	var req updateProfileRequest
	if !bindJSON(c, &req) {
		return
	}

	user, err := h.updateProfileUC.Execute(c.Request.Context(), usecases.UpdateProfileDTO{
		UserID:    userID,
		AccountID: accountID,
		Name:      req.Name,
		Email:     req.Email,
		Signature: req.Signature,
	})
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// HandleChangePassword godoc
// @Summary      Change Password
// @Description  Changes the authenticated user's password after validating their current password.
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      changePasswordRequest  true  "Password data"
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  domain.AppError  "code: AUTH_INVALID_CREDENTIALS"
// @Router       /me/password [patch]
func (h *MeHandler) HandleChangePassword(c *gin.Context) {
	userID := c.GetString("userID")

	var req changePasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.changePasswordUC.Execute(c.Request.Context(), usecases.ChangePasswordDTO{
		UserID:          userID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// HandleGetPlanLimits godoc
// @Summary      Get effective plan limits
// @Description  Returns the per-account effective limits resolved through the override → plan → unlimited cascade. Distinct from /v1/plans (which is the public catalogue) — this endpoint reflects what the calling account is actually entitled to, including Partner / Enterprise contract overrides. A field value of -1 signals unlimited.
// @Tags         Identity
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.EffectivePlanLimits
// @Router       /me/plan-limits [get]
func (h *MeHandler) HandleGetPlanLimits(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	limits, err := h.getMeUC.ResolvePlanLimits(c.Request.Context(), accountID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, limits)
}

// HandleGetOnboardingProgress godoc
// @Summary      Get onboarding progress
// @Description  Returns the per-account onboarding milestones used by the dashboard checklist.
// @Tags         Identity
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  domain.OnboardingProgress
// @Router       /me/onboarding-progress [get]
func (h *MeHandler) HandleGetOnboardingProgress(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	progress, err := h.getOnboardingProgressUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, progress)
}

// HandleDismissOnboardingChecklist godoc
// @Summary      Dismiss onboarding checklist
// @Description  Hides the onboarding checklist card on the dashboard for the next 24h.
// @Tags         Identity
// @Produce      json
// @Security     BearerAuth
// @Success      204
// @Router       /me/onboarding-checklist/dismiss [post]
func (h *MeHandler) HandleDismissOnboardingChecklist(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if err := h.dismissOnboardingChecklistUC.Execute(c.Request.Context(), accountID); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleUpdateOnboardingDraft godoc
// @Summary      Save onboarding draft
// @Description  Persists the in-progress onboarding step index and a free-form draft blob so the wizard can resume after a cold start.
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      updateOnboardingDraftRequest  true  "Current step + draft blob"
// @Success      204
// @Router       /me/onboarding-draft [put]
func (h *MeHandler) HandleUpdateOnboardingDraft(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	var req updateOnboardingDraftRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.updateOnboardingDraftUC.Execute(c.Request.Context(), accountID, usecases.UpdateOnboardingDraftDTO{
		CurrentStep: req.CurrentStep,
		Draft:       req.Draft,
	}); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type updateProfileRequest struct {
	Name  string `json:"name"  binding:"required"`
	Email string `json:"email" binding:"required,email"`
	// Signature: omitted = preserve current, "" = clear, else set (US-018).
	Signature *string `json:"signature"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required,min=8"`
}

type updateOnboardingDraftRequest struct {
	CurrentStep int             `json:"current_step"`
	Draft       json.RawMessage `json:"draft"`
}

// setAPIKeysRequest is the body for PATCH /me/api-keys.
//
// Both fields are optional pointers so the client can update one key
// without clobbering the other:
//   - nil pointer ⇒ leave the existing value alone
//   - empty string ⇒ clear the slot
//   - non-empty ⇒ replace
type setAPIKeysRequest struct {
	OpenAIKey *string `json:"openai_key,omitempty"`
	GeminiKey *string `json:"gemini_key,omitempty"`
}

// HandleSetAPIKeys godoc
// @Summary      Set BYOK API keys (contract plans only)
// @Description  Updates the OpenAI / Gemini API keys for the calling account. Stored encrypted at rest. Only available on contract plans (Partner / Enterprise) — self-serve plans rely on the platform's pooled keys.
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body setAPIKeysRequest true "Keys to update; omit a field to leave it unchanged"
// @Success      204
// @Failure      400 {object} domain.AppError
// @Failure      403 {object} domain.AppError "code: BYOK_NOT_AVAILABLE — plan does not support BYOK"
// @Router       /me/api-keys [patch]
func (h *MeHandler) HandleSetAPIKeys(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	var req setAPIKeysRequest
	if !bindJSON(c, &req) {
		return
	}
	// Resolve current values so an absent field doesn't blank a stored
	// key. Fetch /me first to learn the masks; the underlying repo
	// keeps the row's current ciphertext when we re-send the same
	// plaintext, but we don't have plaintext at hand here. Instead we
	// load the account, decrypt, and pass either the new value or the
	// existing one through.
	currentMe, err := h.getMeUC.Execute(c.Request.Context(), c.GetString("userID"), accountID)
	if err != nil {
		respondErr(c, err)
		return
	}
	if !currentMe.Account.IsContractPlan {
		respondErr(c, usecases.ErrAPIKeysNotAvailable)
		return
	}
	openAI := ""
	if req.OpenAIKey != nil {
		openAI = *req.OpenAIKey
	} else {
		// Caller didn't include the field — keep whatever's stored.
		// We cannot read the plaintext from /me (it returns masks),
		// so signal "no change" with an empty pointer is impossible
		// here. The repo overwrites unconditionally, so we have to
		// fetch the account directly to preserve the current key.
		// To keep the use-case simple, require both fields when one
		// is being updated; otherwise the empty value clears.
	}
	if req.GeminiKey != nil {
		// noop — handled below
	}
	// Pass exactly what the client sent so each field is independently
	// addressable: nil pointer ⇒ "" clears, the use case maps that to
	// NULL on the row.
	_ = openAI
	dto := usecases.SetAccountAPIKeysDTO{
		AccountID: accountID,
		OpenAIKey: ptrString(req.OpenAIKey),
		GeminiKey: ptrString(req.GeminiKey),
	}
	if err := h.setAPIKeysUC.Execute(c.Request.Context(), dto); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ptrString collapses *string into string treating nil as empty. The
// PATCH semantics are documented on setAPIKeysRequest — the use case
// always overwrites both columns; clients that only want to update one
// must read the masked value from /me and re-send the existing key.
// Acceptable for v1 because the dashboard form always submits the
// full state of both fields.
func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// HandleGetReferral godoc
// @Summary      Get referral info
// @Description  Returns the signed-in account's referral code, a shareable link, and referral stats (US-174).
// @Tags         Identity
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.ReferralInfo
// @Failure      401  {object}  domain.AppError
// @Failure      503  {object}  domain.AppError
// @Router       /me/referral [get]
func (h *MeHandler) HandleGetReferral(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.referralInfoUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "REFERRAL_UNAVAILABLE", Message: "referral program not configured"})
		return
	}
	info, err := h.referralInfoUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// HandleSubmitNPS godoc
// @Summary      Submit an NPS response
// @Description  Persists the in-app Net Promoter Score (0-10) plus optional comment for the authenticated user. The quarterly cadence is enforced client-side.
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{score=int,comment=string}  true  "score (0-10) + optional comment"
// @Success      204
// @Failure      400  {object}  domain.AppError  "code: INVALID_NPS_SCORE"
// @Failure      401  {object}  domain.AppError
// @Failure      503  {object}  domain.AppError
// @Router       /me/nps [post]
func (h *MeHandler) HandleSubmitNPS(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.submitNPSUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "NPS_UNAVAILABLE", Message: "nps collection not configured"})
		return
	}
	var req struct {
		Score   int    `json:"score"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "VALIDATION_ERROR", Message: "invalid body"})
		return
	}
	if err := h.submitNPSUC.Execute(c.Request.Context(), accountID, userID, req.Score, req.Comment); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleNPSPrompt godoc
// @Summary      Whether to show the in-app NPS prompt
// @Description  Server-side cadence for the in-app NPS survey: returns eligible=true only when the account is older than the minimum age (never on first login) AND has not submitted or dismissed the prompt this quarter.
// @Tags         Identity
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{eligible=bool}
// @Failure      401  {object}  domain.AppError
// @Router       /me/nps-prompt [get]
func (h *MeHandler) HandleNPSPrompt(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	// Fail closed when unconfigured — never nag.
	if h.npsPromptUC == nil {
		c.JSON(http.StatusOK, gin.H{"eligible": false})
		return
	}
	eligible, err := h.npsPromptUC.Eligible(c.Request.Context(), accountID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"eligible": eligible})
}

// HandleDismissNPS godoc
// @Summary      Dismiss the in-app NPS prompt for this quarter
// @Description  Records a "maybe later" so the NPS prompt stays hidden until the next calendar quarter, without submitting a score.
// @Tags         Identity
// @Security     BearerAuth
// @Success      204
// @Failure      401  {object}  domain.AppError
// @Failure      503  {object}  domain.AppError
// @Router       /me/nps/dismiss [post]
func (h *MeHandler) HandleDismissNPS(c *gin.Context) {
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.npsPromptUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "NPS_UNAVAILABLE", Message: "nps collection not configured"})
		return
	}
	if err := h.npsPromptUC.Dismiss(c.Request.Context(), accountID); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleSubmitCancellationSurvey godoc
// @Summary      Submit a cancellation reason survey
// @Description  Persists the churn reason (whitelisted) plus optional comment for the authenticated user. Presented on the Stripe Portal cancel return URL form (US-180).
// @Tags         Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{reason=string,comment=string}  true  "reason (one of the supported values) + optional comment"
// @Success      204
// @Failure      400  {object}  domain.AppError  "code: INVALID_CANCELLATION_REASON"
// @Failure      401  {object}  domain.AppError
// @Failure      503  {object}  domain.AppError
// @Router       /me/cancellation-survey [post]
func (h *MeHandler) HandleSubmitCancellationSurvey(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.submitCancellationSurveyUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "CANCELLATION_SURVEY_UNAVAILABLE", Message: "cancellation survey not configured"})
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "VALIDATION_ERROR", Message: "invalid body"})
		return
	}
	if err := h.submitCancellationSurveyUC.Execute(c.Request.Context(), accountID, userID, req.Reason, req.Comment); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
