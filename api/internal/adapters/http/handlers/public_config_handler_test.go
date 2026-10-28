package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// US-F001: with REGISTRATION_ENABLED=false the signup endpoint answers 503
// REGISTRATION_PAUSED and never touches the repos, and the public config
// endpoint tells the client the door is closed.
func TestRegister_PausedReturns503(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTransactionManager)
	emailService := new(MockEmailService)

	createUC := usecases.NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailService, "http://test")
	createUC.SetRegistrationPaused(true)
	handler := NewAccountHandler(createUC, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/accounts", handler.HandleCreateAccount)

	body := map[string]interface{}{
		"account_name":   "Test Co",
		"admin_name":     "Jane",
		"admin_email":    "jane@example.com",
		"password":       "securepass",
		"terms_accepted": true,
	}
	w := performRequest(router, http.MethodPost, "/v1/accounts", body, "")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "REGISTRATION_PAUSED", resp["code"])

	userRepo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
	accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPublicConfig_ExposesRegistrationFlag(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		router := setupTestRouter()
		router.GET("/v1/config", NewPublicConfigHandler(enabled).HandleGetPublicConfig)

		w := performRequest(router, http.MethodGet, "/v1/config", nil, "")

		assert.Equal(t, http.StatusOK, w.Code)
		var resp PublicConfigResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, enabled, resp.RegistrationEnabled)
	}
}
