package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Referenced so swag resolves domain.AppError in the annotations below.
var _ = domain.AppError{}

// SaleImportHandler brings in the sales history an account already has.
//
// Same two-endpoint shape as the customer list: the preview reads the file and
// reports what it found, the import records from rows a person confirmed.
// Mounted by setupSaleRoutes through SaleHandler.SetImport, so neither the
// router signature nor main.go's handler list changes.
type SaleImportHandler struct {
	uc *usecases.ImportSalesUseCase
}

func NewSaleImportHandler(uc *usecases.ImportSalesUseCase) *SaleImportHandler {
	return &SaleImportHandler{uc: uc}
}

// HandlePreview parses an uploaded sales history and reports what it found.
//
//	@Summary		Preview a sales import
//	@Description	Reads a CSV of past sales (cliente, produto, data, valor, quantidade) and reports every row: the customer it resolved to, the catalogue product it matched, and the ones it could not read or that are already recorded. Creates nothing.
//	@Tags			sales
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"CSV file (comma or semicolon delimited)"
//	@Success		200		{object}	usecases.SaleImportPreview
//	@Failure		400		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/sales/import/preview [post]
func (h *SaleImportHandler) HandlePreview(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	header, err := c.FormFile("file")
	if err != nil {
		respondError(c, usecases.ErrSaleImportUnreadable)
		return
	}
	file, err := header.Open()
	if err != nil {
		respondError(c, usecases.ErrSaleImportUnreadable)
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

// HandleImport records a sale per confirmed row.
//
//	@Summary		Import confirmed sales
//	@Description	Records a sale (source=imported) per confirmed row, linked to the customer and the catalogue product it resolved to. Re-resolves every row server-side; a sale already recorded for the same customer, day and total is skipped and counted.
//	@Tags			sales
//	@Accept			json
//	@Produce		json
//	@Param			body	body		usecases.ImportSalesInput	true	"Confirmed rows"
//	@Success		200		{object}	usecases.ImportSalesResult
//	@Failure		400		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/sales/import [post]
func (h *SaleImportHandler) HandleImport(c *gin.Context) {
	var req usecases.ImportSalesInput
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
