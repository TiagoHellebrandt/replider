package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// LGPDHandler serves the two titular-rights endpoints:
//   - Export (Art. 18, I/II/V: access + portability)
//   - Anonymize (Art. 18, IV/VI: anonymization / erasure)
//
// Both are admin-only and operate in the caller's account scope. The
// titular's identifier is the external ID (phone / chat_id) since that is
// the stable handle across channels and is what the customer types on
// their end. We explicitly avoid using the Contact's internal UUID
// because that isn't meaningful to a support operator.
type LGPDHandler struct {
	exportUC    *usecases.ExportContactDataUseCase
	anonymizeUC *usecases.AnonymizeContactUseCase
}

func NewLGPDHandler(
	exportUC *usecases.ExportContactDataUseCase,
	anonymizeUC *usecases.AnonymizeContactUseCase,
) *LGPDHandler {
	return &LGPDHandler{exportUC: exportUC, anonymizeUC: anonymizeUC}
}

// lgpdSubjectRequest carries the data subject's identifier in the request
// BODY rather than the query string.
//
// The identifier is a phone number — personal data of the very person whose
// privacy rights the endpoint exists to serve. In a query string it is written
// to the nginx access log, to the edge provider's log, to the browser history,
// and to any Referer sent onward. Putting the LGPD screen itself in the
// business of scattering phone numbers across logs is the one place that
// cannot be hand-waved.
type lgpdSubjectRequest struct {
	ExternalID string `json:"external_id" binding:"required"`
}

// subjectID reads the identifier from the JSON body.
func subjectID(c *gin.Context) (string, bool) {
	var req lgpdSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ExternalID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "EXTERNAL_ID_REQUIRED", Message: "external_id is required in the request body",
		})
		return "", false
	}
	return req.ExternalID, true
}

// HandleExport godoc
// @Summary  Export all data tied to a contact (LGPD Art. 18 right of access)
// @Description The subject identifier travels in the body, never the query
//
//	string: it is a phone number, and a query string ends up in access logs
//	and browser history.
//
// @Tags     lgpd
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body  body  lgpdSubjectRequest  true  "Contact phone or channel ID"
// @Success  200  {object}  domain.LGPDExportBundle
// @Failure  400  {object}  domain.AppError  "external_id required"
// @Failure  404  {object}  domain.AppError  "no data"
// @Router   /lgpd/export [post]
func (h *LGPDHandler) HandleExport(c *gin.Context) {
	accountID := c.GetString("accountID")
	actorID := c.GetString("userID")
	externalID, ok := subjectID(c)
	if !ok {
		return
	}

	bundle, err := h.exportUC.Execute(c.Request.Context(), accountID, actorID, externalID)
	if err != nil {
		respondError(c, err)
		return
	}
	// Content-Disposition suggests a filename when the admin hits the URL
	// directly — useful for simple ops flows without a UI.
	c.Header("Content-Disposition",
		`attachment; filename="lgpd-export-`+externalID+`.json"`)
	c.JSON(http.StatusOK, bundle)
}

// HandleAnonymize godoc
// @Summary  Anonymize all personal data tied to a contact (LGPD Art. 18 right to erasure)
// @Description Replaces PII fields on the contact, its messages, and the
//
//	denormalized contact_name on conversations. Preserves aggregate counters.
//	The external_id is replaced with an anonymous token so future messages
//	from the same phone create a fresh relationship.
//
// @Tags     lgpd
// @Accept   json
// @Security BearerAuth
// @Param    body  body  lgpdSubjectRequest  true  "Contact phone or channel ID"
// @Success  204
// @Failure  400  {object}  domain.AppError
// @Failure  404  {object}  domain.AppError
// @Router   /lgpd/anonymize [post]
func (h *LGPDHandler) HandleAnonymize(c *gin.Context) {
	accountID := c.GetString("accountID")
	actorID := c.GetString("userID")
	externalID, ok := subjectID(c)
	if !ok {
		return
	}

	if err := h.anonymizeUC.Execute(c.Request.Context(), accountID, actorID, externalID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
