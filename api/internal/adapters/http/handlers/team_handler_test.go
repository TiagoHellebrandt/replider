package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ═══════════════════════════ GET /team ═══════════════════════════

func TestListTeamMembers_Success_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	role := "admin"
	userRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.User{
		{ID: "u1", AccountID: testAccountID, Name: "Alice", Email: "alice@test.com", Role: &role, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}, nil)

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/team", authMw, middleware.RequireRole("admin"), handler.HandleListMembers)

	w := performRequest(router, http.MethodGet, "/v1/team", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []usecases.TeamMember
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Alice", resp[0].Name)
	assert.Equal(t, "admin", resp[0].Role)
	userRepo.AssertExpectations(t)
}

func TestListTeamMembers_Error_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	userRepo.On("ListByAccountID", mock.Anything, testAccountID).Return(([]*domain.User)(nil), errors.New("db error"))

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/team", authMw, middleware.RequireRole("admin"), handler.HandleListMembers)

	w := performRequest(router, http.MethodGet, "/v1/team", nil, testToken)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INTERNAL_ERROR", resp["code"])
	userRepo.AssertExpectations(t)
}

// ═══════════════════════════ POST /team ═══════════════════════════

func TestInviteTeamMember_Success_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	userRepo.On("GetByEmail", mock.Anything, "new@test.com").Return(nil, nil)
	userRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	userRepo.On("SetResetToken", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	userRepo.On("MarkInviteSent", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	emailSvc.On("SendTeamInviteEmail", "new@test.com", "Test User", mock.AnythingOfType("string"), "https://app.replider.com", mock.AnythingOfType("string")).Return(nil)

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/team", authMw, middleware.RequireRole("admin"), handler.HandleInviteMember)

	body := map[string]string{
		"name":  "New User",
		"email": "new@test.com",
		"role":  "attendant",
	}

	w := performRequest(router, http.MethodPost, "/v1/team", body, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp usecases.TeamMember
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "New User", resp.Name)
	assert.Equal(t, "new@test.com", resp.Email)
	assert.Equal(t, "attendant", resp.Role)
	userRepo.AssertExpectations(t)
}

func TestInviteTeamMember_ValidationError_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/team", authMw, middleware.RequireRole("admin"), handler.HandleInviteMember)

	// Missing required fields
	body := map[string]string{
		"name": "User Without Email",
	}

	w := performRequest(router, http.MethodPost, "/v1/team", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp["code"])
}

func TestInviteTeamMember_DuplicateEmail_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	role := "attendant"
	userRepo.On("GetByEmail", mock.Anything, "existing@test.com").Return(&domain.User{
		ID: "u1", Email: "existing@test.com", Role: &role,
	}, nil)

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/team", authMw, middleware.RequireRole("admin"), handler.HandleInviteMember)

	body := map[string]string{
		"name":  "Existing",
		"email": "existing@test.com",
		"role":  "attendant",
	}

	w := performRequest(router, http.MethodPost, "/v1/team", body, testToken)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AUTH_EMAIL_ALREADY_EXISTS", resp["code"])
	userRepo.AssertExpectations(t)
}

// ═══════════════════════════ PATCH /team/:id/role ═══════════════════════════

func TestUpdateMemberRole_Success_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	role := "attendant"
	userRepo.On("GetByID", mock.Anything, "u2").Return(&domain.User{
		ID: "u2", AccountID: testAccountID, Role: &role,
	}, nil)
	userRepo.On("UpdateRole", mock.Anything, "u2", "admin").Return(nil)

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.PATCH("/v1/team/:id/role", authMw, middleware.RequireRole("admin"), handler.HandleUpdateRole)

	body := map[string]string{"role": "admin"}
	w := performRequest(router, http.MethodPatch, "/v1/team/u2/role", body, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "role updated", resp["message"])
	userRepo.AssertExpectations(t)
}

func TestUpdateMemberRole_InvalidRole_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.PATCH("/v1/team/:id/role", authMw, middleware.RequireRole("admin"), handler.HandleUpdateRole)

	body := map[string]string{"role": "superadmin"}
	w := performRequest(router, http.MethodPatch, "/v1/team/u2/role", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "TEAM_INVALID_ROLE", resp["code"])
}

// ═══════════════════════════ DELETE /team/:id ═══════════════════════════

func TestRemoveMember_Success_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	userRepo.On("GetByID", mock.Anything, "u2").Return(&domain.User{
		ID: "u2", AccountID: testAccountID,
	}, nil)
	userRepo.On("Deactivate", mock.Anything, "u2").Return(nil)

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/team/:id", authMw, middleware.RequireRole("admin"), handler.HandleRemoveMember)

	w := performRequest(router, http.MethodDelete, "/v1/team/u2", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "member removed", resp["message"])
	userRepo.AssertExpectations(t)
}

func TestRemoveMember_CannotRemoveSelf_Handler(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	listUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteUC := usecases.NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	updateUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeUC := usecases.NewRemoveMemberUseCase(userRepo)
	handler := NewTeamHandler(listUC, inviteUC, updateUC, removeUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/team/:id", authMw, middleware.RequireRole("admin"), handler.HandleRemoveMember)

	// testUserID == "user-1", so deleting "user-1" should fail
	w := performRequest(router, http.MethodDelete, "/v1/team/"+testUserID, nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "TEAM_CANNOT_REMOVE_SELF", resp["code"])
}
