package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type AuthHandler struct {
	authenticateUC       *usecases.AuthenticateUserUseCase
	verifyMfaLoginUC     *usecases.VerifyMfaLoginUseCase
	forgotPasswordUC     *usecases.ForgotPasswordUseCase
	resetPasswordUC      *usecases.ResetPasswordUseCase
	googleAuthUC         *usecases.GoogleAuthUseCase
	metaAuthUC           *usecases.MetaAuthUseCase
	verifyEmailUC        *usecases.VerifyEmailUseCase
	resendVerificationUC *usecases.ResendVerificationUseCase
}

func NewAuthHandler(
	authenticateUC *usecases.AuthenticateUserUseCase,
	verifyMfaLoginUC *usecases.VerifyMfaLoginUseCase,
	forgotPasswordUC *usecases.ForgotPasswordUseCase,
	resetPasswordUC *usecases.ResetPasswordUseCase,
	googleAuthUC *usecases.GoogleAuthUseCase,
	metaAuthUC *usecases.MetaAuthUseCase,
	verifyEmailUC *usecases.VerifyEmailUseCase,
	resendVerificationUC *usecases.ResendVerificationUseCase,
) *AuthHandler {
	return &AuthHandler{
		authenticateUC:       authenticateUC,
		verifyMfaLoginUC:     verifyMfaLoginUC,
		forgotPasswordUC:     forgotPasswordUC,
		resetPasswordUC:      resetPasswordUC,
		googleAuthUC:         googleAuthUC,
		metaAuthUC:           metaAuthUC,
		verifyEmailUC:        verifyEmailUC,
		resendVerificationUC: resendVerificationUC,
	}
}

// VerifyMfaLogin godoc
// @Summary      Trade an MFA session token + 2FA code for a real JWT
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body usecases.VerifyMfaLoginDTO true "MFA session + code"
// @Success      200 {object} usecases.VerifyMfaLoginResponse
// @Failure      401 {object} domain.AppError "code: AUTH_UNAUTHORIZED or TOTP_INVALID_CODE"
// @Router       /auth/2fa/verify-login [post]
func (h *AuthHandler) VerifyMfaLogin(c *gin.Context) {
	var body usecases.VerifyMfaLoginDTO
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.verifyMfaLoginUC.Execute(c.Request.Context(), body)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": appErr.Message, "code": appErr.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// Login godoc
// @Summary      Authenticate User
// @Description  Logs in a user with email/password and returns a JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body usecases.AuthenticateUserDTO true "Login Credentials"
// @Success      200  {object}  usecases.AuthenticateUserResponse
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Failure      401  {object}  domain.AppError  "code: AUTH_INVALID_CREDENTIALS — invalid email or password"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var body usecases.AuthenticateUserDTO
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	response, err := h.authenticateUC.Execute(c.Request.Context(), body)
	if err != nil {
		slog.Error("authentication failed", "error", err)
		if err == domain.ErrEmailNotVerified {
			c.JSON(http.StatusForbidden, gin.H{"error": err.(*domain.AppError).Message, "code": err.(*domain.AppError).Code})
			return
		}
		if appErr, ok := err.(*domain.AppError); ok {
			status := http.StatusUnauthorized
			if appErr.Code == usecases.ErrAuthLocked.Code {
				// Lockout: the client must back off, not retry with the
				// next password guess.
				status = http.StatusTooManyRequests
				c.Header("Retry-After", "900")
			}
			c.JSON(status, gin.H{"error": appErr.Message, "code": appErr.Code})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ForgotPassword godoc
// @Summary      Request Password Reset
// @Description  Sends a password reset link to the user's email. Always returns 200 to prevent email enumeration.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body usecases.ForgotPasswordDTO true "Email address"
// @Success      200  {object}  map[string]string  "Reset email sent (if account exists)"
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Router       /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var body usecases.ForgotPasswordDTO
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_BODY", Message: "email is required"})
		return
	}

	_ = h.forgotPasswordUC.Execute(c.Request.Context(), body)

	// Always return 200 to prevent email enumeration attacks
	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent."})
}

// ResetPassword godoc
// @Summary      Reset Password
// @Description  Validates a reset token and updates the user's password
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body usecases.ResetPasswordDTO true "Reset token and new password"
// @Success      200  {object}  map[string]string  "Password reset successfully"
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Failure      401  {object}  domain.AppError  "code: AUTH_INVALID_RESET_TOKEN — invalid or expired reset token"
// @Router       /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var body usecases.ResetPasswordDTO
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_BODY", Message: "token and new_password are required (min 8 chars)"})
		return
	}

	if err := h.resetPasswordUC.Execute(c.Request.Context(), body); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusUnauthorized, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully."})
}

// GoogleAuth godoc
// @Summary      Google Social Login
// @Description  Verifies a Google id_token, finds or creates the user, and returns a JWT
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body usecases.SocialAuthDTO true "Google ID Token"
// @Success      200  {object}  usecases.SocialAuthResponse
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Failure      401  {object}  domain.AppError  "code: AUTH_SOCIAL_FAILED — social authentication failed"
// @Router       /auth/google [post]
func (h *AuthHandler) GoogleAuth(c *gin.Context) {
	var body usecases.SocialAuthDTO
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_BODY", Message: "id_token is required"})
		return
	}
	// Signup origin audit (US-015): server-derived, never client-bound.
	body.SignupIP = c.ClientIP()
	body.SignupCountry = clientCountry(c)

	result, err := h.googleAuthUC.Execute(c.Request.Context(), body)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusUnauthorized, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "Google authentication failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// MetaAuth godoc
// @Summary      Meta (Facebook/Instagram) Social Login
// @Description  Verifies a Meta access token, finds or creates the user, and returns a JWT
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body usecases.SocialAuthDTO true "Meta Access Token"
// @Success      200  {object}  usecases.SocialAuthResponse
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Failure      401  {object}  domain.AppError  "code: AUTH_SOCIAL_FAILED — social authentication failed"
// @Router       /auth/meta [post]
func (h *AuthHandler) MetaAuth(c *gin.Context) {
	var body usecases.SocialAuthDTO
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_BODY", Message: "id_token is required"})
		return
	}
	// Signup origin audit (US-015): server-derived, never client-bound.
	body.SignupIP = c.ClientIP()
	body.SignupCountry = clientCountry(c)

	result, err := h.metaAuthUC.Execute(c.Request.Context(), body)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusUnauthorized, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "Meta authentication failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleVerifyEmail godoc
// @Summary      Verify Email Address
// @Description  Validates a verification token and marks the user's email as verified
// @Tags         Auth
// @Produce      json
// @Param        token query string true "Verification token"
// @Success      200  {object}  map[string]string  "Email verified successfully"
// @Failure      400  {object}  domain.AppError  "Missing or invalid token"
// @Router       /auth/verify-email [get]
func (h *AuthHandler) HandleVerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "token is required"})
		return
	}

	if err := h.verifyEmailUC.Execute(c.Request.Context(), token); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusBadRequest, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to verify email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

// HandleResendVerification godoc
// @Summary      Resend email verification link
// @Description  Re-issues a fresh token and mails a verification link. Always 200 — we never reveal
//
//	whether the email exists or was already verified, to prevent account enumeration.
//
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "Email payload {\"email\":\"user@example.com\"}"
// @Success      200  {object}  map[string]string  "always ok"
// @Failure      400  {object}  domain.AppError    "code: INVALID_REQUEST"
// @Router       /auth/resend-verification [post]
func (h *AuthHandler) HandleResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "email is required"})
		return
	}
	// Rate limit is applied at the route layer; here we just run the use
	// case and always respond 200 — enumeration-safe.
	if err := h.resendVerificationUC.Execute(c.Request.Context(), req.Email); err != nil {
		slog.Error("resend verification failed", "error", err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "if the account exists and is unverified, an email has been sent"})
}
