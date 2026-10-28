package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateCalendar_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	calRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Calendar")).Return(nil)

	createUC := usecases.NewCreateCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(createUC, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/agents/:id/calendars", authMw, middleware.RequireRole("admin"), handler.HandleCreateCalendar)

	body := map[string]interface{}{
		"name":           "Business Hours",
		"available_days": []int{1, 2, 3, 4, 5},
	}
	w := performRequest(router, http.MethodPost, "/v1/agents/"+testAgentID+"/calendars", body, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.Calendar
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Business Hours", resp.Name)
	assert.Len(t, resp.ID, 26) // ULID length

	agentRepo.AssertExpectations(t)
	calRepo.AssertExpectations(t)
}

func TestCreateCalendar_MissingName(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler := NewCalendarHandler(nil, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/agents/:id/calendars", authMw, middleware.RequireRole("admin"), handler.HandleCreateCalendar)

	body := map[string]interface{}{
		"available_days": []int{1, 2, 3},
	}
	w := performRequest(router, http.MethodPost, "/v1/agents/"+testAgentID+"/calendars", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp["code"])
}

func TestCreateCalendar_AgentNotFound(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, "nonexistent-agent").Return(nil, nil)

	createUC := usecases.NewCreateCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(createUC, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/agents/:id/calendars", authMw, middleware.RequireRole("admin"), handler.HandleCreateCalendar)

	body := map[string]interface{}{
		"name": "Test Calendar",
	}
	w := performRequest(router, http.MethodPost, "/v1/agents/nonexistent-agent/calendars", body, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AGENT_NOT_FOUND", resp["code"])

	agentRepo.AssertExpectations(t)
}

func TestListCalendars_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	calendars := []*domain.Calendar{
		{
			ID:                  "cal-1",
			AgentID:             testAgentID,
			Name:                "Business Hours",
			AvailableDays:       []int32{1, 2, 3, 4, 5},
			SlotDurationMinutes: 30,
			IsActive:            true,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		},
	}
	calRepo.On("ListByAgentID", mock.Anything, testAgentID).Return(calendars, nil)

	listUC := usecases.NewListCalendarsUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, nil, nil, listUC, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/agents/:id/calendars", authMw, middleware.RequireRole("admin"), handler.HandleListCalendars)

	w := performRequest(router, http.MethodGet, "/v1/agents/"+testAgentID+"/calendars", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Calendar
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Business Hours", resp[0].Name)

	agentRepo.AssertExpectations(t)
	calRepo.AssertExpectations(t)
}

func TestListCalendars_AgentNotFound(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, "nonexistent-agent").Return(nil, nil)

	listUC := usecases.NewListCalendarsUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, nil, nil, listUC, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/agents/:id/calendars", authMw, middleware.RequireRole("admin"), handler.HandleListCalendars)

	w := performRequest(router, http.MethodGet, "/v1/agents/nonexistent-agent/calendars", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AGENT_NOT_FOUND", resp["code"])
}

func TestUpdateCalendar_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	calRepo.On("GetByID", mock.Anything, "cal-1").Return(&domain.Calendar{
		ID:                  "cal-1",
		AgentID:             testAgentID,
		Name:                "Old Name",
		SlotDurationMinutes: 30,
		IsActive:            true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil)

	calRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Calendar")).Return(nil)

	updateUC := usecases.NewUpdateCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, updateUC, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.PUT("/v1/agents/:id/calendars/:calendarId", authMw, middleware.RequireRole("admin"), handler.HandleUpdateCalendar)

	newName := "Updated Name"
	body := map[string]interface{}{
		"name": newName,
	}
	w := performRequest(router, http.MethodPut, "/v1/agents/"+testAgentID+"/calendars/cal-1", body, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.Calendar
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.Name)

	agentRepo.AssertExpectations(t)
	calRepo.AssertExpectations(t)
}

func TestUpdateCalendar_CalendarNotFound(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	calRepo.On("GetByID", mock.Anything, "nonexistent-cal").Return(nil, nil)

	updateUC := usecases.NewUpdateCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, updateUC, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.PUT("/v1/agents/:id/calendars/:calendarId", authMw, middleware.RequireRole("admin"), handler.HandleUpdateCalendar)

	body := map[string]interface{}{
		"name": "New Name",
	}
	w := performRequest(router, http.MethodPut, "/v1/agents/"+testAgentID+"/calendars/nonexistent-cal", body, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "CALENDAR_NOT_FOUND", resp["code"])
}

func TestDeleteCalendar_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	calRepo.On("GetByID", mock.Anything, "cal-1").Return(&domain.Calendar{
		ID:      "cal-1",
		AgentID: testAgentID,
	}, nil)

	calRepo.On("Delete", mock.Anything, "cal-1").Return(nil)

	deleteUC := usecases.NewDeleteCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, nil, deleteUC, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/agents/:id/calendars/:calendarId", authMw, middleware.RequireRole("admin"), handler.HandleDeleteCalendar)

	w := performRequest(router, http.MethodDelete, "/v1/agents/"+testAgentID+"/calendars/cal-1", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "calendar deleted", resp["message"])

	agentRepo.AssertExpectations(t)
	calRepo.AssertExpectations(t)
}

func TestDeleteCalendar_AgentNotFound(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, "nonexistent-agent").Return(nil, nil)

	deleteUC := usecases.NewDeleteCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, nil, deleteUC, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/agents/:id/calendars/:calendarId", authMw, middleware.RequireRole("admin"), handler.HandleDeleteCalendar)

	w := performRequest(router, http.MethodDelete, "/v1/agents/nonexistent-agent/calendars/cal-1", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AGENT_NOT_FOUND", resp["code"])
}

func TestDeleteCalendar_CalendarNotFound(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	calRepo := new(MockCalendarRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	calRepo.On("GetByID", mock.Anything, "nonexistent-cal").Return(nil, nil)

	deleteUC := usecases.NewDeleteCalendarUseCase(agentRepo, calRepo)
	handler := NewCalendarHandler(nil, nil, deleteUC, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/agents/:id/calendars/:calendarId", authMw, middleware.RequireRole("admin"), handler.HandleDeleteCalendar)

	w := performRequest(router, http.MethodDelete, "/v1/agents/"+testAgentID+"/calendars/nonexistent-cal", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "CALENDAR_NOT_FOUND", resp["code"])
}
