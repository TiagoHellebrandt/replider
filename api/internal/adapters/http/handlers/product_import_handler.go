package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// ProductImportHandler brings in the price list a distributor already has.
//
// Two endpoints, and the split is the safeguard: the preview reads the file and
// reports what it found, the import creates from rows a person confirmed. There
// is no one-shot upload-and-create, because the row this parser most needs a
// human to look at is a price it deliberately refuses to guess.
type ProductImportHandler struct {
	uc *usecases.ImportProductsUseCase
	// pdfUC is the brochure door (optional; see product_import_pdf_handler.go).
	pdfUC       *usecases.ImportProductsPDFUseCase
	pdfMaxBytes int64
}

func NewProductImportHandler(uc *usecases.ImportProductsUseCase) *ProductImportHandler {
	return &ProductImportHandler{uc: uc}
}

// maxUploadBytes bounds what multipart will buffer before the use case's own
// limit applies. Kept slightly above it so the use case is the one that reports
// "too large" in its own words.
const maxUploadBytes = 4 << 20

// HandlePreview parses an uploaded price list and reports what it found.
//
//	@Summary		Preview a product import
//	@Description	Reads a CSV price list and reports every row, including the ones it could not read. Creates nothing.
//	@Tags			products
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"CSV file (comma or semicolon delimited)"
//	@Success		200		{object}	usecases.ProductImportPreview
//	@Failure		400		{object}	domain.AppError
//	@Failure		413		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/products/import/preview [post]
func (h *ProductImportHandler) HandlePreview(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	header, err := c.FormFile("file")
	if err != nil {
		respondError(c, usecases.ErrProductImportUnreadable)
		return
	}
	file, err := header.Open()
	if err != nil {
		respondError(c, usecases.ErrProductImportUnreadable)
		return
	}
	defer func() { _ = file.Close() }()

	preview, err := h.uc.Preview(c.Request.Context(), getAccountID(c), file)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

// HandleImport creates a product per confirmed row.
//
// The client sends back the ROWS the preview showed rather than the file: the
// person approved what they saw, and re-parsing would let a file that changed
// underneath be imported instead. Every check the preview applied runs again
// here — a row arriving with its problem cleared creates nothing.
//
//	@Summary		Import confirmed products
//	@Description	Creates a product per confirmed row. Re-checks names and prices server-side; a row that already exists is skipped and counted.
//	@Tags			products
//	@Accept			json
//	@Produce		json
//	@Param			body	body		usecases.ImportProductsInput	true	"Confirmed rows"
//	@Success		200		{object}	usecases.ImportProductsResult
//	@Failure		400		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/products/import [post]
func (h *ProductImportHandler) HandleImport(c *gin.Context) {
	var req usecases.ImportProductsInput
	if !bindJSON(c, &req) {
		return
	}

	result, err := h.uc.Import(c.Request.Context(), getAccountID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Referenced so swag resolves domain.AppError in the annotations above.
var _ = domain.AppError{}
