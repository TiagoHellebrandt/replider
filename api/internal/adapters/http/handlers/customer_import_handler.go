package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// CustomerImportHandler brings in the customer list an account already has.
//
// Same two-endpoint shape as the price list: the preview reads the file and
// reports what it found, the import creates from rows a person confirmed. There
// is no one-shot upload-and-create — this file decides who the account will be
// writing to.
type CustomerImportHandler struct {
	uc *usecases.ImportCustomersUseCase
}

func NewCustomerImportHandler(uc *usecases.ImportCustomersUseCase) *CustomerImportHandler {
	return &CustomerImportHandler{uc: uc}
}

// HandlePreview parses an uploaded customer list and reports what it found.
//
//	@Summary		Preview a customer import
//	@Description	Reads a CSV customer list and reports every row, including the ones it could not read and the ones already in the account's base. Creates nothing.
//	@Tags			contacts
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"CSV file (comma or semicolon delimited)"
//	@Success		200		{object}	usecases.CustomerImportPreview
//	@Failure		400		{object}	domain.AppError
//	@Failure		413		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/contacts/import/preview [post]
func (h *CustomerImportHandler) HandlePreview(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	header, err := c.FormFile("file")
	if err != nil {
		respondError(c, usecases.ErrCustomerImportUnreadable)
		return
	}
	file, err := header.Open()
	if err != nil {
		respondError(c, usecases.ErrCustomerImportUnreadable)
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

// HandleImport creates a contact per confirmed row.
//
//	@Summary		Import confirmed customers
//	@Description	Creates a contact per confirmed row and indexes how to reach them, so their next message lands on this contact instead of creating a stranger. Re-checks every phone and address server-side; a customer already in the base is skipped and counted.
//	@Tags			contacts
//	@Accept			json
//	@Produce		json
//	@Param			body	body		usecases.ImportCustomersInput	true	"Confirmed rows"
//	@Success		200		{object}	usecases.ImportCustomersResult
//	@Failure		400		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/contacts/import [post]
func (h *CustomerImportHandler) HandleImport(c *gin.Context) {
	var req usecases.ImportCustomersInput
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
