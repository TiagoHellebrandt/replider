package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// AccountHandler handles HTTP requests for accounts
type AccountHandler struct {
	createAccountUseCase *usecases.CreateAccountUseCase
	deleteAccountUC      *usecases.DeleteAccountUseCase
	updateAccountUC      *usecases.UpdateAccountUseCase
	// inviteUC answers the public invite probe (optional setter; nil → the
	// probe answers "invalid" for everything, which is the closed default).
	inviteUC *usecases.InviteUseCase
}

// SetInviteUC wires the invite probe.
func (h *AccountHandler) SetInviteUC(uc *usecases.InviteUseCase) { h.inviteUC = uc }

// HandleInviteProbe serves GET /v1/invites/probe?token=…
//
// The register screen asks "is this link any good?" before the person types
// a password. Every bad token — nonexistent, expired, used, revoked — gets
// the SAME {"valid":false}, so the endpoint cannot be used to enumerate.
// The e-mail is returned only against the secret itself, and the screen
// needs it to lock the field.
//
// @Summary      Probe a signup invite
// @Tags         accounts
// @Produce      json
// @Param        token query string true "Invite token from the register link"
// @Success      200 {object} usecases.InviteProbe
// @Router       /invites/probe [get]
func (h *AccountHandler) HandleInviteProbe(c *gin.Context) {
	if h.inviteUC == nil {
		c.JSON(http.StatusOK, usecases.InviteProbe{})
		return
	}
	c.JSON(http.StatusOK, h.inviteUC.Probe(c.Request.Context(), c.Query("token")))
}

// HandleCreateAccountResponse defines the HTTP response for successful account creation
type HandleCreateAccountResponse struct {
	Message string `json:"message"`
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(
	createUC *usecases.CreateAccountUseCase,
	deleteUC *usecases.DeleteAccountUseCase,
	updateUC *usecases.UpdateAccountUseCase,
) *AccountHandler {
	return &AccountHandler{
		createAccountUseCase: createUC,
		deleteAccountUC:      deleteUC,
		updateAccountUC:      updateUC,
	}
}

// HandleCreateAccount acts as the HTTP entry point to create an account
// @Summary      Create a new Account
// @Description  Creates a new SaaS account, an admin user, and initializes usage tracking.
// @Tags         accounts
// @Accept       json
// @Produce      json
// @Param        request body usecases.CreateAccountDTO true "Account Creation Payload"
// @Success      201  {object}  handlers.HandleCreateAccountResponse "Account created successfully"
// @Failure      400  {object}  domain.AppError "code: AUTH_EMAIL_ALREADY_EXISTS — email already exists"
// @Failure      500  {object}  domain.AppError "Internal server error"
// @Router       /accounts [post]
func (h *AccountHandler) HandleCreateAccount(c *gin.Context) {
	var dto usecases.CreateAccountDTO
	if !bindJSON(c, &dto) {
		return
	}
	// Signup origin audit (US-015): server-derived, never client-bound.
	dto.SignupIP = c.ClientIP()
	dto.SignupCountry = clientCountry(c)

	err := h.createAccountUseCase.Execute(c.Request.Context(), dto)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, HandleCreateAccountResponse{Message: "account created successfully"})
}

// HandleUpdateAccount acts as the HTTP entry point to update mutable
// account fields (currently only `name`).
// @Summary      Update account
// @Description  Updates mutable account fields. Requires admin role.
// @Tags         accounts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body usecases.UpdateAccountDTO true "Fields to update"
// @Success      200  {object}  map[string]string "Account updated"
// @Failure      400  {object}  domain.AppError "Invalid request"
// @Failure      401  {object}  domain.AppError "Unauthorized"
// @Failure      500  {object}  domain.AppError "Internal server error"
// @Router       /accounts [patch]
func (h *AccountHandler) HandleUpdateAccount(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")

	var dto usecases.UpdateAccountDTO
	if !bindJSON(c, &dto) {
		return
	}

	if err := h.updateAccountUC.Execute(c.Request.Context(), accountID, userID, dto); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			status := http.StatusBadRequest
			if appErr.Code == "AUTH_UNAUTHORIZED" {
				status = http.StatusUnauthorized
			}
			c.JSON(status, appErr)
			return
		}
		slog.Error("failed to update account", "account_id", accountID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account updated"})
}

// HandleDeleteAccount acts as the HTTP entry point to delete an account (GDPR)
// @Summary      Delete account
// @Description  Permanently deletes the account and all associated data. Requires admin role.
// @Tags         accounts
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]string "Account deleted"
// @Failure      401  {object}  domain.AppError "Unauthorized"
// @Failure      500  {object}  domain.AppError "Internal server error"
// @Router       /accounts [delete]
func (h *AccountHandler) HandleDeleteAccount(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")

	if err := h.deleteAccountUC.Execute(c.Request.Context(), accountID, userID); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusUnauthorized, appErr)
			return
		}
		slog.Error("failed to delete account", "account_id", accountID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account deleted"})
}
