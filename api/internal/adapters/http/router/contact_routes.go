package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupDiscoveredContactRoutes mounts "traz os clientes que você já tem".
//
// Admin-only, and deliberately: the scan reads a mailbox's Sent history and the
// import writes into the CRM. Both are decisions about the account's customer
// list, not day-to-day attendant work.
func setupDiscoveredContactRoutes(rg *gin.RouterGroup, h *handlers.DiscoveredContactHandler, authMW gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := rg.Group("", authMW, middleware.RequireRole("admin"))
	{
		g.POST("/channels/:id/discover-contacts", h.HandleScan)
		g.GET("/discovered-contacts", h.HandleList)
		// Registered before nothing ambiguous, but kept explicit: "import" and
		// "dismiss" are actions, not ids.
		g.POST("/discovered-contacts/import", h.HandleImport)
		g.POST("/discovered-contacts/dismiss", h.HandleDismiss)
	}
}

func setupContactRoutes(rg *gin.RouterGroup, contactHandler *handlers.ContactHandler, authMW gin.HandlerFunc) {
	contacts := rg.Group("/contacts", authMW, middleware.RequireRole("admin", "attendant"))
	{
		contacts.GET("", contactHandler.HandleListContacts)
		contacts.GET("/:id", contactHandler.HandleGetContact)
		contacts.PUT("/:id", contactHandler.HandleUpdateContact)
		// Opt-in requires admin (undoes an LGPD consent revocation).
		contacts.POST("/:id/opt-in", middleware.RequireRole("admin"), contactHandler.HandleOptInContact)
	}
}

// setupCustomerImportRoutes mounts the customer-list import (pillar 2).
//
// A group of its OWN rather than rows inside setupContactRoutes, for the same
// reason the price-list import has one: that function runs unconditionally with
// the contact CRUD handler, and nesting these there would tie one feature's
// availability to another's — a 404 at runtime instead of a compile error.
//
// Admin-only, narrower than the contact CRUD above: this writes the account's
// customer base in bulk, which is the same decision the Sent-folder import makes
// and is not day-to-day attendant work.
//
// "/contacts/import" is unambiguous against "/contacts/:id" because gin's tree
// matches a static segment ahead of a param — "import" can never be read as an id.
func setupCustomerImportRoutes(
	rg *gin.RouterGroup, h *handlers.CustomerImportHandler, authMW gin.HandlerFunc,
) {
	if h == nil {
		return
	}
	g := rg.Group("/contacts", authMW, middleware.RequireRole("admin"))
	{
		g.POST("/import/preview", h.HandlePreview)
		g.POST("/import", h.HandleImport)
	}
}

// setupProposalRoutes mounts pillar 3 — "gere propostas de valor".
//
// admin+attendant, unlike the discovery routes: composing an offer for a customer
// IS day-to-day sales work, and a salesperson who cannot price one has nothing to
// send. What neither role can do here is deliver it — there is no send route, only
// the path into the approval queue.
func setupProposalRoutes(rg *gin.RouterGroup, h *handlers.ProposalHandler, authMW gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := rg.Group("", authMW, middleware.RequireRole("admin", "attendant"))
	{
		g.POST("/proposals", h.HandleCreate)
		g.GET("/proposals/:id", h.HandleGet)
		g.PUT("/proposals/:id/lines", h.HandleReplaceLines)
		g.PUT("/proposals/:id/body", h.HandleSetBody)
		g.POST("/proposals/:id/send-for-approval", h.HandleSendForApproval)
		g.GET("/leads/:id/proposals", h.HandleListForLead)
		// The file the seller forwards. Not a send route — it hands over a
		// document, and the only way a proposal reaches a customer is still the
		// approval queue.
		g.GET("/proposals/:id/pdf", h.HandleDownloadPDF)
	}
}
