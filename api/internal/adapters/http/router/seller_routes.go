package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupSellerRoutes wires the seller (vendedores) panel endpoints. The
// whole group is gated by RequirePlatformAccount: outside the Replider
// platform account (or with PLATFORM_ACCOUNT_ID unset) every route is a
// 404 — not 403 — so the feature's existence never leaks (US-001/US-003).
// Inside it, RequireSeller (US-014) additionally demands the explicit
// owner-granted seller flag — a regular platform attendant opening the
// panel gets 403 and never mints a ref code / becomes payable.
func setupSellerRoutes(v1 *gin.RouterGroup, h *handlers.SellerHandler, authMw gin.HandlerFunc, platformAccountID string, sellerFlags middleware.SellerFlagChecker) {
	seller := v1.Group("/seller")
	seller.Use(authMw, middleware.RequirePlatformAccount(platformAccountID), middleware.RequireSeller(sellerFlags))
	{
		seller.POST("/ref-code", h.HandleEnsureRefCode)
		// Seller panel (US-010): carteira with header totals.
		seller.GET("/carteira", h.HandleGetCarteira)
		// Seller panel (US-011): commission statement + own PIX key.
		seller.GET("/commissions", h.HandleGetStatement)
		seller.PUT("/pix-key", h.HandleUpdatePixKey)
		// Seller panel (US-012): live SLA widget for the current cycle.
		seller.GET("/sla", h.HandleGetSLA)
		// Co-pilot (US-014): on-demand suggestion feed + dismissals.
		seller.GET("/suggestions", h.HandleGetSuggestions)
		seller.POST("/suggestions/state", h.HandleSetSuggestionState)
		// Convert-lead flow (US-004): search targets + explicit conversion.
		seller.GET("/lead-accounts", h.HandleSearchLeadAccounts)
		seller.POST("/conversations/:id/convert", h.HandleConvertLead)
		// Canal interno (US-007): internal CS threads grouped by carteira
		// account. Seller-only — the carteira is the grouping key.
		seller.GET("/support-threads", h.HandleGetSupportThreads)
		// Canal interno (US-008): outbound-first thread opener for the
		// co-pilot's 1-click action (target account's admin).
		seller.POST("/support-threads/open", h.HandleOpenSupportThread)
	}

	// Account note (US-007): deliberately WITHOUT RequireSeller — the AC
	// grants edit to "vendedor/admin da plataforma", and an unflagged
	// platform admin must still reach it. The use case enforces
	// admin-or-flagged-seller (SUPPORT_NOTE_FORBIDDEN otherwise).
	note := v1.Group("/seller/conversations/:id/account-note")
	note.Use(authMw, middleware.RequirePlatformAccount(platformAccountID))
	{
		note.GET("", h.HandleGetAccountNote)
		note.PUT("", h.HandleSetAccountNote)
	}
}
