package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupProductRoutes(v1 *gin.RouterGroup, h *handlers.ProductHandler, authPaidMw gin.HandlerFunc) {
	g := v1.Group("/products", authPaidMw, middleware.RequireRole("admin"))
	{
		g.POST("", h.HandleCreate)
		g.GET("", h.HandleList)
		g.GET("/:id", h.HandleGet)
		// The brochure this product was read from. A static segment under
		// "/:id" is fine: gin matches "source" as a literal child.
		g.GET("/:id/source", h.HandleGetSource)
		g.GET("/:id/images", h.HandleGetImages)
		g.PUT("/:id", h.HandleUpdate)
		g.DELETE("/:id", h.HandleDelete)
	}
}

// setupProductImportRoutes mounts the price-list import (pillar 2).
//
// A group of its OWN rather than rows inside setupProductRoutes: that function is
// only called when the product CRUD handler exists, so nesting these there made
// the import silently absent wherever that handler is nil — a 404 at runtime
// instead of a compile error. One feature's availability must not hang off
// another's.
//
// The paths still sort before "/products/:id" because gin's tree matches a static
// segment ahead of a param, so "import" can never be read as a product id.
func setupProductImportRoutes(
	v1 *gin.RouterGroup,
	h *handlers.ProductImportHandler,
	authPaidMw gin.HandlerFunc,
) {
	if h == nil {
		return
	}
	g := v1.Group("/products", authPaidMw, middleware.RequireRole("admin"))
	{
		g.POST("/import/preview", h.HandlePreview)
		g.POST("/import", h.HandleImport)
		// The brochure door: extract-and-preview only. Confirmation is the
		// same POST /import above — a PDF cannot skip the human review.
		g.POST("/import/pdf", h.HandlePDF)
	}
}
