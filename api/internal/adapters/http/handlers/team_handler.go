package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type TeamHandler struct {
	listMembersUC  *usecases.ListTeamMembersUseCase
	inviteMemberUC *usecases.InviteTeamMemberUseCase
	updateRoleUC   *usecases.UpdateMemberRoleUseCase
	removeMemberUC *usecases.RemoveMemberUseCase
	resendInviteUC *usecases.ResendInviteUseCase
	autoAssignUC   *usecases.AutoAssignSettingsUseCase // optional, via SetAutoAssignUC
}

// SetAutoAssignUC wires the round-robin config use case (US-016).
// Setter keeps the constructor stable for existing call sites.
func (h *TeamHandler) SetAutoAssignUC(uc *usecases.AutoAssignSettingsUseCase) {
	h.autoAssignUC = uc
}

func NewTeamHandler(
	listMembersUC *usecases.ListTeamMembersUseCase,
	inviteMemberUC *usecases.InviteTeamMemberUseCase,
	updateRoleUC *usecases.UpdateMemberRoleUseCase,
	removeMemberUC *usecases.RemoveMemberUseCase,
	resendInviteUC *usecases.ResendInviteUseCase,
) *TeamHandler {
	return &TeamHandler{
		listMembersUC:  listMembersUC,
		inviteMemberUC: inviteMemberUC,
		updateRoleUC:   updateRoleUC,
		removeMemberUC: removeMemberUC,
		resendInviteUC: resendInviteUC,
	}
}

// HandleListMembers godoc
// @Summary      List team members
// @Description  Returns all users in the authenticated account.
// @Tags         Team
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   usecases.TeamMember
// @Failure      401  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /team [get]
func (h *TeamHandler) HandleListMembers(c *gin.Context) {
	accountID := c.GetString("accountID")

	members, err := h.listMembersUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list team members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

type inviteTeamMemberRequest struct {
	Name  string `json:"name"  binding:"required"`
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"  binding:"required"`
}

// HandleInviteMember godoc
// @Summary      Invite team member
// @Description  Creates a new user in the account and sends an invite email.
// @Tags         Team
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      inviteTeamMemberRequest  true  "Invite payload"
// @Success      201  {object}  usecases.TeamMember
// @Failure      400  {object}  domain.AppError
// @Failure      409  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /team [post]
func (h *TeamHandler) HandleInviteMember(c *gin.Context) {
	accountID := c.GetString("accountID")
	userName := c.GetString("userName")

	var req inviteTeamMemberRequest
	if !bindJSON(c, &req) {
		return
	}

	member, err := h.inviteMemberUC.Execute(c.Request.Context(), usecases.InviteTeamMemberDTO{
		AccountID:   accountID,
		InviterName: userName,
		Name:        req.Name,
		Email:       req.Email,
		Role:        req.Role,
	})
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			switch appErr {
			case domain.ErrEmailAlreadyExists:
				c.JSON(http.StatusConflict, appErr)
			case domain.ErrInvalidRole:
				c.JSON(http.StatusBadRequest, appErr)
			default:
				c.JSON(http.StatusBadRequest, appErr)
			}
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to invite team member"})
		return
	}

	c.JSON(http.StatusCreated, member)
}

type updateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// HandleUpdateRole godoc
// @Summary      Update member role
// @Description  Changes a team member's role.
// @Tags         Team
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                   true  "Member ID"
// @Param        body  body      updateMemberRoleRequest  true  "Role payload"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /team/{id}/role [patch]
func (h *TeamHandler) HandleUpdateRole(c *gin.Context) {
	accountID := c.GetString("accountID")
	memberID := c.Param("id")

	var req updateMemberRoleRequest
	if !bindJSON(c, &req) {
		return
	}

	err := h.updateRoleUC.Execute(c.Request.Context(), usecases.UpdateMemberRoleDTO{
		AccountID: accountID,
		UserID:    memberID,
		Role:      req.Role,
	})
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			switch appErr {
			case domain.ErrMemberNotFound:
				c.JSON(http.StatusNotFound, appErr)
			case domain.ErrInvalidRole:
				c.JSON(http.StatusBadRequest, appErr)
			default:
				c.JSON(http.StatusBadRequest, appErr)
			}
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

// HandleRemoveMember godoc
// @Summary      Remove team member
// @Description  Deactivates a team member. Cannot remove yourself.
// @Tags         Team
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Member ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /team/{id} [delete]
func (h *TeamHandler) HandleRemoveMember(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	memberID := c.Param("id")

	err := h.removeMemberUC.Execute(c.Request.Context(), usecases.RemoveMemberDTO{
		AccountID:    accountID,
		UserID:       memberID,
		RequestingID: userID,
	})
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			switch appErr {
			case domain.ErrMemberNotFound:
				c.JSON(http.StatusNotFound, appErr)
			case domain.ErrCannotRemoveSelf:
				c.JSON(http.StatusBadRequest, appErr)
			default:
				c.JSON(http.StatusBadRequest, appErr)
			}
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

// HandleResendInvite godoc
// @Summary      Resend team invite
// @Description  Regenerates the password-reset token and e-mails the invite again. Only works while the user hasn't completed signup (is_email_verified = false).
// @Tags         Team
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  domain.AppError  "code: AUTH_USER_NOT_FOUND"
// @Failure      409  {object}  domain.AppError  "code: TEAM_INVITE_ALREADY_ACCEPTED"
// @Router       /team/{id}/resend-invite [post]
func (h *TeamHandler) HandleResendInvite(c *gin.Context) {
	accountID := c.GetString("accountID")
	inviterName := c.GetString("userName")
	memberID := c.Param("id")

	err := h.resendInviteUC.Execute(c.Request.Context(), usecases.ResendInviteDTO{
		AccountID:   accountID,
		InviterName: inviterName,
		TargetID:    memberID,
	})
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			switch appErr {
			case domain.ErrUserNotFound:
				c.JSON(http.StatusNotFound, appErr)
			case domain.ErrInviteAlreadyAccepted:
				c.JSON(http.StatusConflict, appErr)
			default:
				c.JSON(http.StatusBadRequest, appErr)
			}
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to resend invite"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invite resent"})
}

// HandleGetAutoAssign godoc
// @Summary      Get auto-assign settings
// @Description  Returns the account's round-robin auto-assignment config. Empty participant list means every active attendant participates.
// @Tags         Team
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.AutoAssignSettingsView
// @Failure      401  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /team/auto-assign [get]
func (h *TeamHandler) HandleGetAutoAssign(c *gin.Context) {
	accountID := c.GetString("accountID")

	view, err := h.autoAssignUC.Get(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

type updateAutoAssignRequest struct {
	Enabled            *bool    `json:"enabled" binding:"required"`
	ParticipantUserIDs []string `json:"participant_user_ids"`
}

// HandleUpdateAutoAssign godoc
// @Summary      Update auto-assign settings
// @Description  Toggles round-robin auto-assignment and restricts which attendants participate. Empty participant list means every active attendant participates.
// @Tags         Team
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      updateAutoAssignRequest  true  "Settings payload"
// @Success      200  {object}  usecases.AutoAssignSettingsView
// @Failure      400  {object}  domain.AppError  "code: TEAM_INVALID_PARTICIPANT"
// @Failure      401  {object}  domain.AppError
// @Failure      500  {object}  domain.AppError
// @Router       /team/auto-assign [put]
func (h *TeamHandler) HandleUpdateAutoAssign(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req updateAutoAssignRequest
	if !bindJSON(c, &req) {
		return
	}

	view, err := h.autoAssignUC.Update(c.Request.Context(), accountID, *req.Enabled, req.ParticipantUserIDs)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}
