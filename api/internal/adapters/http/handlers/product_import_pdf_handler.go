package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Referenced so swag resolves domain.AppError in the annotations below.
var _ = domain.AppError{}

// The brochure door of the product import (Bloco C).
//
// Its own file rather than another block in product_import_handler.go — same
// 400-line reasoning as the lead contacts import. The endpoint EXTRACTS and
// previews only; confirmation is the existing POST /products/import, so a PDF
// can never put a row in the catalogue that a person did not review.

// SetPDFUseCase wires the PDF extraction. Optional setter so the constructor
// and its call sites stay untouched; unwired, the route answers 503.
func (h *ProductImportHandler) SetPDFUseCase(uc *usecases.ImportProductsPDFUseCase, maxBytes int64) {
	h.pdfUC = uc
	h.pdfMaxBytes = maxBytes
}

// HandlePDF serves POST /v1/products/import/pdf.
//
//	@Summary		Extract products from a brochure PDF
//	@Description	Sends a manufacturer brochure PDF through the account's AI provider and returns the SAME preview the CSV import returns — including rows with problems (ambiguous price, missing name) and the brochure page each row was read from. Creates nothing; confirmation is POST /products/import.
//	@Tags			products
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"Brochure PDF"
//	@Success		200		{object}	usecases.ProductImportPreview
//	@Failure		400		{object}	domain.AppError
//	@Failure		413		{object}	domain.AppError
//	@Failure		503		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/products/import/pdf [post]
func (h *ProductImportHandler) HandlePDF(c *gin.Context) {
	if h.pdfUC == nil {
		respondError(c, usecases.ErrProductPDFImportUnavailable)
		return
	}
	// One byte of slack over the use case's limit so the use case is the one
	// that says "too large" in its own words instead of a bare 413.
	limit := h.pdfMaxBytes
	if limit <= 0 {
		limit = maxUploadBytes
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit+(1<<20))

	header, err := c.FormFile("file")
	if err != nil {
		respondError(c, usecases.ErrProductPDFUnreadable)
		return
	}
	file, err := header.Open()
	if err != nil {
		respondError(c, usecases.ErrProductPDFUnreadable)
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		respondError(c, usecases.ErrProductPDFUnreadable)
		return
	}

	preview, err := h.pdfUC.Extract(c.Request.Context(), getAccountID(c), data, header.Filename)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}
