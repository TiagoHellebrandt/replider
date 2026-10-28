package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupChannelRoutes(
	v1 *gin.RouterGroup,
	h *handlers.ChannelHandler,
	waSignupHandler *handlers.WhatsAppSignupHandler,
	waConfigHandler *handlers.WhatsAppConfigHandler,
	authMw gin.HandlerFunc,
) {
	channels := v1.Group("/channels", authMw, middleware.RequireRole("admin"))
	{
		channels.GET("", h.HandleListChannels)
		channels.POST("", h.HandleCreateChannel)
		channels.PUT("/:id", h.HandleUpdateChannel)
		channels.DELETE("/:id", h.HandleDeleteChannel)
		channels.PATCH("/:id/toggle", h.HandleToggleChannel)
		channels.POST("/:id/test", h.HandleTestChannel)
		channels.GET("/:id/webhook-status", h.HandleWebhookStatus)
		// Email authentication setup + live verification (US-M006).
		channels.GET("/:id/email-dns", h.HandleEmailDNS)
		// One-click publish. Admin-only via the group above, which is the
		// right level: the token this takes can redirect a company's mail.
		channels.POST("/:id/email-dns/apply", h.HandleApplyEmailDNS)
		// Volume sending via the platform's bulk provider (hybrid model):
		// registers the channel's domain and reports verification state.
		channels.GET("/:id/bulk-sending", h.HandleBulkSendingStatus)
		channels.POST("/:id/bulk-sending/enable", h.HandleEnableBulkSending)
		channels.POST("/:id/webhook-register", h.HandleWebhookRegister)
		channels.POST("/:id/whatsapp/register", h.HandleWhatsAppRegisterPhone)
		channels.POST("/:id/whatsapp/request-code", h.HandleWhatsAppRequestCode)
		channels.POST("/:id/whatsapp/verify-code", h.HandleWhatsAppVerifyCode)
		channels.POST("/:id/whatsapp/deregister", h.HandleWhatsAppDeregister)

		// WhatsApp Embedded Signup
		if waSignupHandler != nil {
			channels.POST("/whatsapp/signup", waSignupHandler.HandleWhatsAppSignup)
			channels.GET("/:id/templates", waSignupHandler.HandleListTemplates)
			channels.POST("/:id/templates/send", waSignupHandler.HandleSendTemplate)
			channels.GET("/:id/phone-numbers", waSignupHandler.HandleListPhoneNumbers)
		}

		// WhatsApp business profile + template CRUD
		if waConfigHandler != nil {
			channels.GET("/:id/whatsapp/profile", waConfigHandler.HandleGetBusinessProfile)
			channels.PUT("/:id/whatsapp/profile", waConfigHandler.HandleUpdateBusinessProfile)
			channels.POST("/:id/whatsapp/profile/picture", waConfigHandler.HandleUploadProfilePicture)
			channels.DELETE("/:id/whatsapp/profile/picture", waConfigHandler.HandleDeleteProfilePicture)
			channels.POST("/:id/templates", waConfigHandler.HandleCreateTemplate)
			channels.POST("/:id/templates/sync", waConfigHandler.HandleSyncTemplates)
			channels.DELETE("/:id/templates/:name", waConfigHandler.HandleDeleteTemplate)
			channels.POST("/:id/whatsapp/templates/upload-media", waConfigHandler.HandleUploadTemplateMedia)
		}
	}
}

// setupGoogleMailboxRoutes mounts "conecte o e-mail que você já tem" (pillar 1).
//
// Its own function, and the two routes deliberately sit in DIFFERENT groups:
// starting the flow is an admin action on a logged-in account, while the
// callback is an unauthenticated browser redirect from Google. Mounting the
// callback under the auth middleware would make it 401 every time — the user's
// session cookie is not on a cross-site redirect, and the bearer token never was.
//
// The callback is not unprotected: it authenticates on an HMAC-signed state that
// carries the account, so a forged one is unconstructable rather than merely
// rejected. It is rate-limited because it is a public path that performs a
// token exchange.
func setupGoogleMailboxRoutes(
	v1 *gin.RouterGroup,
	h *handlers.GoogleMailboxHandler,
	authMw gin.HandlerFunc,
	rl func(string) gin.HandlerFunc,
) {
	if h == nil {
		return
	}
	v1.POST("/channels/email/google/connect",
		authMw, middleware.RequireRole("admin"), h.HandleConnect)
	v1.GET("/oauth/google-mailbox/callback", rl("30-M"), h.HandleCallback)
}
