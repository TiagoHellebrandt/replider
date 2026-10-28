package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
)

func setupAuthRoutes(v1 *gin.RouterGroup, authHandler *handlers.AuthHandler, rl func(string) gin.HandlerFunc, loginRL gin.HandlerFunc) {
	authGroup := v1.Group("/auth")
	{
		// Two limits, two threat models (US-F012): rl("10-M") caps the raw
		// per-IP flood; loginRL caps attempts per (IP, email) so neither one
		// IP walking many accounts nor many IPs hammering one account gets a
		// bigger budget by fanning out.
		authGroup.POST("/login", rl("10-M"), loginRL, authHandler.Login)
		// MFA second leg of the password-login flow. Same IP limit as
		// /login — a brute-force attacker who already cleared step 1
		// has 5 minutes against a 6-digit code; matching the cap stops
		// them from using this endpoint as a code-guess oracle.
		authGroup.POST("/2fa/verify-login", rl("10-M"), authHandler.VerifyMfaLogin)
		authGroup.POST("/forgot-password", rl("3-M"), authHandler.ForgotPassword)
		authGroup.POST("/reset-password", rl("5-M"), authHandler.ResetPassword)
		authGroup.POST("/google", rl("10-M"), authHandler.GoogleAuth)
		authGroup.POST("/meta", rl("10-M"), authHandler.MetaAuth)
		authGroup.GET("/verify-email", authHandler.HandleVerifyEmail)
		// Aggressive IP limit: re-sending verification is cheap for an
		// attacker to abuse (SMTP cost + reputation damage) so we cap
		// hard. 3 per minute per IP is plenty for a real user fighting
		// a stuck email + SMTP.
		authGroup.POST("/resend-verification", rl("3-M"), authHandler.HandleResendVerification)
	}
}
