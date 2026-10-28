package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type ContactHandler struct {
	listUC      *usecases.ListContactsUseCase
	getUC       *usecases.GetContactUseCase
	updateUC    *usecases.UpdateContactUseCase
	contactRepo domain.ContactRepository // used directly for opt-in (simple enough to skip a use case)
}

func NewContactHandler(
	listUC *usecases.ListContactsUseCase,
	getUC *usecases.GetContactUseCase,
	updateUC *usecases.UpdateContactUseCase,
	contactRepo domain.ContactRepository,
) *ContactHandler {
	return &ContactHandler{listUC: listUC, getUC: getUC, updateUC: updateUC, contactRepo: contactRepo}
}

func (h *ContactHandler) HandleListContacts(c *gin.Context) {
	accountID := c.GetString("accountID")
	query := c.Query("q")
	limit, offset := pagination(c)

	contacts, err := h.listUC.Execute(c.Request.Context(), accountID, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list contacts"})
		return
	}
	if contacts == nil {
		contacts = []*domain.Contact{}
	}
	c.JSON(http.StatusOK, contacts)
}

func (h *ContactHandler) HandleGetContact(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	contact, err := h.getUC.Execute(c.Request.Context(), id, accountID)
	if err != nil {
		if err == domain.ErrContactNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to get contact"})
		return
	}
	c.JSON(http.StatusOK, contact)
}

// HandleOptInContact reverses an opt-out for the given contact. Admin-only
// — LGPD treats opt-out as an explicit consent revocation, so restoring
// it must be a deliberate action from the account owner who presumably
// has new written/verbal consent from the customer. Idempotent: calling
// this on an already-opted-in contact returns 200 without doing anything.
func (h *ContactHandler) HandleOptInContact(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	contact, err := h.getUC.Execute(c.Request.Context(), id, accountID)
	if err != nil {
		if err == domain.ErrContactNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to fetch contact"})
		return
	}

	if _, err := h.contactRepo.ClearOptedOut(c.Request.Context(), accountID, contact.ExternalID); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to opt in contact"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "opted_in"})
}

func (h *ContactHandler) HandleUpdateContact(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	var dto usecases.UpdateContactDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: err.Error()})
		return
	}

	contact, err := h.updateUC.Execute(c.Request.Context(), id, accountID, dto)
	if err != nil {
		if err == domain.ErrContactNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to update contact"})
		return
	}
	c.JSON(http.StatusOK, contact)
}
