package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupAccountRoutes(v1 *gin.RouterGroup, accountHandler *handlers.AccountHandler, usageHandler *handlers.UsageHandler, authMw gin.HandlerFunc, rl func(string) gin.HandlerFunc, signupCaptcha gin.HandlerFunc) {
	accounts := v1.Group("/accounts")
	{
		// Public signup. Two layers of abuse control:
		//   1. US-053: 5/hour per IP hard cap (registration is expensive —
		//      Account + admin User + trial UsageRecord + SMTP mail).
		//   2. US-054: escalating captcha — after 3 attempts/hour from the
		//      same IP a valid Turnstile token is required. Captcha runs
		//      first so a token-verified human is checked before consuming
		//      the rate-limit budget; bots that can't solve it never reach
		//      the limiter and abusive IPs still hit the 5/hour ceiling.
		accounts.POST("", signupCaptcha, rl("5-H"), accountHandler.HandleCreateAccount)

		// Invite probe (sales-led signup): public because the register
		// screen calls it before any session exists. Rate-limited hard —
		// its whole design is anti-enumeration (one uniform answer), and
		// the limiter keeps even that answer expensive to farm.
		v1.GET("/invites/probe", rl("30-H"), accountHandler.HandleInviteProbe)

		// Protected Routes (admin only)
		protected := accounts.Group("")
		protected.Use(authMw, middleware.RequireRole("admin"))
		{
			protected.GET("/usage", usageHandler.HandleGetUsage)
			protected.PATCH("", accountHandler.HandleUpdateAccount)
			protected.DELETE("", accountHandler.HandleDeleteAccount)
		}
	}
}
