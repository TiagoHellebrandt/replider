package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Conversational onboarding (§6.20, K5).
//
// Hangs off MeHandler by optional setter, like the other additions, so neither
// NewMeHandler nor router.Setup changes signature.

// SetOnboardingChatUC wires the conversation.
func (h *MeHandler) SetOnboardingChatUC(uc *usecases.OnboardingChatUseCase) {
	h.onboardingChatUC = uc
}

// HandleOnboardingChat serves GET /v1/me/onboarding-chat.
//
// @Summary     The onboarding conversation
// @Description The current question, what has been gathered, and the transcript so returning after lunch shows the conversation rather than an empty box.
// @Tags        Me
// @Produce     json
// @Success     200 {object} usecases.OnboardingChatView
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/onboarding-chat [get]
func (h *MeHandler) HandleOnboardingChat(c *gin.Context) {
	if h.onboardingChatUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "ONBOARDING_CHAT_UNAVAILABLE", Message: "onboarding chat not configured"})
		return
	}
	view, err := h.onboardingChatUC.Start(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

type onboardingAnswerRequest struct {
	Text    string   `json:"text,omitempty"`
	Choices []string `json:"choices,omitempty"`
	Skip    bool     `json:"skip,omitempty"`
}

// HandleOnboardingAnswer serves POST /v1/me/onboarding-chat.
//
// @Summary     Answer the current onboarding question
// @Description Records the turn and advances. Every step is skippable except the company name — everything downstream is written in that company's voice.
// @Tags        Me
// @Accept      json
// @Produce     json
// @Param       request body onboardingAnswerRequest true "The answer"
// @Success     200 {object} usecases.OnboardingChatView
// @Failure     400 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/onboarding-chat [post]
func (h *MeHandler) HandleOnboardingAnswer(c *gin.Context) {
	if h.onboardingChatUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "ONBOARDING_CHAT_UNAVAILABLE", Message: "onboarding chat not configured"})
		return
	}
	var req onboardingAnswerRequest
	if !bindJSON(c, &req) {
		return
	}
	view, err := h.onboardingChatUC.Answer(c.Request.Context(), usecases.AnswerInput{
		AccountID: getAccountID(c),
		Text:      req.Text,
		Choices:   req.Choices,
		Skip:      req.Skip,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}
