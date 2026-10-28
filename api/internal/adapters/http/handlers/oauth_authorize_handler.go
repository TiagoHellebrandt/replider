package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers/oauthpage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// OAuthAuthorizeHandler renders the login+consent page (GET) and runs
// the credential + consent step (POST), redirecting back to the client
// with an authorization code.
type OAuthAuthorizeHandler struct {
	authorizeUC *usecases.OAuthAuthorizeUseCase
}

func NewOAuthAuthorizeHandler(authorizeUC *usecases.OAuthAuthorizeUseCase) *OAuthAuthorizeHandler {
	return &OAuthAuthorizeHandler{authorizeUC: authorizeUC}
}

func authorizeRequestFrom(c *gin.Context) usecases.AuthorizeRequest {
	get := func(key string) string {
		if v := c.Query(key); v != "" {
			return v
		}
		return c.PostForm(key)
	}
	return usecases.AuthorizeRequest{
		ClientID:            get("client_id"),
		RedirectURI:         get("redirect_uri"),
		State:               get("state"),
		Scope:               get("scope"),
		CodeChallenge:       get("code_challenge"),
		CodeChallengeMethod: get("code_challenge_method"),
		ResponseType:        get("response_type"),
	}
}

func renderAuthorizePage(c *gin.Context, status int, view oauthpage.View) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	_ = oauthpage.Render(c.Writer, view)
}

func renderAuthorizeError(c *gin.Context, message string) {
	renderAuthorizePage(c, http.StatusBadRequest, oauthpage.View{Mode: "error", Error: message})
}

// HandleAuthorizeGet validates the request and renders the login page.
// Client/redirect errors NEVER redirect — they render on our page.
func (h *OAuthAuthorizeHandler) HandleAuthorizeGet(c *gin.Context) {
	req := authorizeRequestFrom(c)
	validated, err := h.authorizeUC.ValidateAuthorizeRequest(c.Request.Context(), req)
	if err != nil {
		renderAuthorizeError(c, oauthErrorMessagePTBR(err))
		return
	}
	renderAuthorizePage(c, http.StatusOK, oauthpage.View{
		Mode:        "login",
		ClientName:  validated.Client.ClientName,
		FlowToken:   validated.FlowToken,
		ClientID:    req.ClientID,
		RedirectURI: req.RedirectURI,
		State:       req.State,
		Scope:       req.Scope,
		Challenge:   req.CodeChallenge,
		Method:      req.CodeChallengeMethod,
	})
}

// HandleAuthorizePost runs login (+2FA) and consent, then 302s back to
// the client's redirect_uri with the authorization code.
func (h *OAuthAuthorizeHandler) HandleAuthorizePost(c *gin.Context) {
	req := authorizeRequestFrom(c)
	dto := usecases.AuthorizeDTO{
		Request:    req,
		FlowToken:  c.PostForm("flow_token"),
		Email:      c.PostForm("email"),
		Password:   c.PostForm("password"),
		MfaSession: c.PostForm("mfa_session"),
		MfaCode:    c.PostForm("code"),
	}

	redirect, err := h.authorizeUC.Authorize(c.Request.Context(), dto)
	if err == nil {
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusFound, redirect)
		return
	}

	var mfaErr *usecases.MfaRequiredError
	if errors.As(err, &mfaErr) {
		h.rerender(c, req, dto, oauthpage.View{
			Mode:       "mfa",
			MfaSession: mfaErr.MfaSession,
		})
		return
	}

	// Credential failures re-render the form; protocol failures render
	// the error page (the flow can't continue).
	var oe *usecases.OAuthProtocolError
	if errors.As(err, &oe) {
		renderAuthorizeError(c, oauthErrorMessagePTBR(err))
		return
	}

	mode := "login"
	if dto.MfaSession != "" {
		mode = "mfa"
	}
	h.rerender(c, req, dto, oauthpage.View{
		Mode:       mode,
		MfaSession: dto.MfaSession,
		Error:      credentialErrorMessagePTBR(err),
	})
}

func (h *OAuthAuthorizeHandler) rerender(c *gin.Context, req usecases.AuthorizeRequest, dto usecases.AuthorizeDTO, view oauthpage.View) {
	// Re-validate to refresh the flow token and client name; on failure
	// fall back to the error page.
	validated, err := h.authorizeUC.ValidateAuthorizeRequest(c.Request.Context(), req)
	if err != nil {
		renderAuthorizeError(c, oauthErrorMessagePTBR(err))
		return
	}
	view.ClientName = validated.Client.ClientName
	view.FlowToken = validated.FlowToken
	view.Email = dto.Email
	view.ClientID = req.ClientID
	view.RedirectURI = req.RedirectURI
	view.State = req.State
	view.Scope = req.Scope
	view.Challenge = req.CodeChallenge
	view.Method = req.CodeChallengeMethod
	status := http.StatusOK
	if view.Error != "" {
		status = http.StatusUnauthorized
	}
	renderAuthorizePage(c, status, view)
}

func oauthErrorMessagePTBR(err error) string {
	var oe *usecases.OAuthProtocolError
	if errors.As(err, &oe) {
		switch oe.Code {
		case "server_error":
			return "Erro temporário no servidor. Tente novamente em instantes."
		default:
			return "Solicitação de autorização inválida (" + oe.Description + ")."
		}
	}
	return "Não foi possível processar a autorização."
}

func credentialErrorMessagePTBR(err error) string {
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return "E-mail ou senha incorretos."
	}
	if errors.Is(err, domain.ErrUnauthorized) {
		return "Sessão de verificação expirada — entre novamente."
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case "TOTP_INVALID_CODE", "TOTP_NOT_ENABLED":
			return "Código de verificação inválido."
		}
	}
	return "Não foi possível autenticar. Verifique os dados e tente novamente."
}
