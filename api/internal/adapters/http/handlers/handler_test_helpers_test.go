package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func performRequest(router *gin.Engine, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func setupAuthMiddleware(jwtMock *MockJWTService) gin.HandlerFunc {
	return middleware.AuthMiddleware(jwtMock)
}

func expectAuth(jwtMock *MockJWTService, token, userID, accountID, role string) {
	jwtMock.On("ValidateToken", token).Return(&domain.UserClaims{
		UserID:    userID,
		UserName:  "Test User",
		AccountID: accountID,
		Role:      role,
	}, nil)
}

// strp returns a pointer to s. Useful for constructing domain structs with
// nullable string fields in tests (Go doesn't allow &"literal").
func strp(s string) *string { return &s }
