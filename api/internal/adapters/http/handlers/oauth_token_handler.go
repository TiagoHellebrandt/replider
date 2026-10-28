package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// OAuthTokenHandler serves the machine-facing OAuth endpoints: token
// exchange, revocation and dynamic client registration. These speak the
// RFC 6749 error envelope ({"error": ..., "error_description": ...}) —
// deliberately NOT respondError/AppError.
type OAuthTokenHandler struct {
	tokenUC    *usecases.OAuthTokenUseCase
	revokeUC   *usecases.OAuthRevokeUseCase
	registerUC *usecases.OAuthRegisterClientUseCase
}

func NewOAuthTokenHandler(
	tokenUC *usecases.OAuthTokenUseCase,
	revokeUC *usecases.OAuthRevokeUseCase,
	registerUC *usecases.OAuthRegisterClientUseCase,
) *OAuthTokenHandler {
	return &OAuthTokenHandler{tokenUC: tokenUC, revokeUC: revokeUC, registerUC: registerUC}
}

// oauthError writes an RFC 6749-shaped error response.
func oauthError(c *gin.Context, err error) {
	c.Header("Cache-Control", "no-store")
	var oe *usecases.OAuthProtocolError
	if !errors.As(err, &oe) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	status := http.StatusBadRequest
	switch oe.Code {
	case "invalid_client":
		status = http.StatusUnauthorized
	case "server_error":
		status = http.StatusInternalServerError
	}
	c.JSON(status, gin.H{"error": oe.Code, "error_description": oe.Description})
}

// clientCredentialsFrom supports both client_secret_basic (Authorization
// header) and client_secret_post / public clients (form fields).
func clientCredentialsFrom(c *gin.Context) (clientID, clientSecret string) {
	if id, secret, ok := c.Request.BasicAuth(); ok {
		return id, secret
	}
	return c.PostForm("client_id"), c.PostForm("client_secret")
}

// HandleToken implements POST /oauth/token (form-encoded).
func (h *OAuthTokenHandler) HandleToken(c *gin.Context) {
	clientID, clientSecret := clientCredentialsFrom(c)
	resp, err := h.tokenUC.Execute(c.Request.Context(), usecases.TokenRequestDTO{
		GrantType:    c.PostForm("grant_type"),
		Code:         c.PostForm("code"),
		CodeVerifier: c.PostForm("code_verifier"),
		RedirectURI:  c.PostForm("redirect_uri"),
		RefreshToken: c.PostForm("refresh_token"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		oauthError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, resp)
}

// HandleRevoke implements RFC 7009. Always 200 for unknown tokens.
func (h *OAuthTokenHandler) HandleRevoke(c *gin.Context) {
	token := c.PostForm("token")
	if err := h.revokeUC.Execute(c.Request.Context(), token); err != nil {
		oauthError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
}

// HandleRegister implements RFC 7591 dynamic client registration (JSON).
func (h *OAuthTokenHandler) HandleRegister(c *gin.Context) {
	var dto usecases.RegisterClientDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		oauthError(c, &usecases.OAuthProtocolError{Code: "invalid_client_metadata", Description: "malformed registration request"})
		return
	}
	resp, err := h.registerUC.Execute(c.Request.Context(), dto)
	if err != nil {
		oauthError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, resp)
}
