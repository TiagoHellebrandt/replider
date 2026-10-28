package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateSource_Success(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	knowledgeRepo.On("CreateSource", mock.Anything, mock.AnythingOfType("*domain.KnowledgeSource")).Return(nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources", authMw, middleware.RequireRole("admin"), handler.HandleCreateSource)

	body := map[string]interface{}{
		"agent_id": testAgentID,
		"name":     "FAQ Knowledge",
	}
	w := performRequest(router, http.MethodPost, "/v1/knowledge/sources", body, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.KnowledgeSource
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "FAQ Knowledge", resp.Name)
	assert.Equal(t, testAgentID, resp.AgentID)
	assert.Len(t, resp.ID, 26) // ULID

	knowledgeRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}

func TestCreateSource_MissingRequiredFields(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler := NewKnowledgeHandler(nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources", authMw, middleware.RequireRole("admin"), handler.HandleCreateSource)

	// Missing agent_id and name
	body := map[string]interface{}{}
	w := performRequest(router, http.MethodPost, "/v1/knowledge/sources", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp["code"])
}

func TestCreateSource_MissingName(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler := NewKnowledgeHandler(nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources", authMw, middleware.RequireRole("admin"), handler.HandleCreateSource)

	body := map[string]interface{}{
		"agent_id": testAgentID,
	}
	w := performRequest(router, http.MethodPost, "/v1/knowledge/sources", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListSources_Success(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	sources := []*domain.KnowledgeSource{
		{
			ID:        "src-1",
			AgentID:   testAgentID,
			Name:      "FAQ",
			CreatedAt: time.Now(),
		},
	}
	knowledgeRepo.On("ListSources", mock.Anything, testAgentID).Return(sources, nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/knowledge/sources", authMw, middleware.RequireRole("admin"), handler.HandleListSources)

	w := performRequest(router, http.MethodGet, "/v1/knowledge/sources?agent_id="+testAgentID, nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.KnowledgeSource
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "FAQ", resp[0].Name)

	knowledgeRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}

func TestListSources_MissingAgentID(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler := NewKnowledgeHandler(nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/knowledge/sources", authMw, middleware.RequireRole("admin"), handler.HandleListSources)

	w := performRequest(router, http.MethodGet, "/v1/knowledge/sources", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp["code"])
}

func TestIngestKnowledge_Success(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	knowledgeRepo.On("GetSourceByID", mock.Anything, "src-1").Return(&domain.KnowledgeSource{
		ID:      "src-1",
		AgentID: testAgentID,
		Name:    "FAQ",
	}, nil)

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	embedding := []float32{0.1, 0.2, 0.3}
	aiProvider.On("GetEmbedding", mock.Anything, "What is Replider?").Return(embedding, nil)

	knowledgeRepo.On("CreateChunk", mock.Anything, mock.AnythingOfType("*domain.KnowledgeChunk")).Return(nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources/:id/chunks", authMw, middleware.RequireRole("admin"), handler.HandleIngestKnowledge)

	body := map[string]interface{}{
		"content":     "What is Replider?",
		"source_name": "FAQ Section",
	}
	w := performRequest(router, http.MethodPost, "/v1/knowledge/sources/src-1/chunks", body, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.KnowledgeChunk
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "What is Replider?", resp.Content)
	assert.Equal(t, "src-1", resp.SourceID)
	assert.Len(t, resp.ID, 26)

	knowledgeRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
	aiProvider.AssertExpectations(t)
}

func TestIngestKnowledge_MissingContent(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler := NewKnowledgeHandler(nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources/:id/chunks", authMw, middleware.RequireRole("admin"), handler.HandleIngestKnowledge)

	body := map[string]interface{}{
		"source_name": "FAQ",
	}
	w := performRequest(router, http.MethodPost, "/v1/knowledge/sources/src-1/chunks", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp["code"])
}

func TestListChunks_Success(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	knowledgeRepo.On("GetSourceByID", mock.Anything, "src-1").Return(&domain.KnowledgeSource{
		ID:      "src-1",
		AgentID: testAgentID,
	}, nil)

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	sourceName := "FAQ"
	chunks := []*domain.KnowledgeChunk{
		{
			ID:         "chunk-1",
			SourceID:   "src-1",
			Content:    "Hello world",
			SourceName: &sourceName,
			CreatedAt:  time.Now(),
		},
	}
	knowledgeRepo.On("ListChunksBySource", mock.Anything, "src-1").Return(chunks, nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/knowledge/sources/:id/chunks", authMw, middleware.RequireRole("admin"), handler.HandleListChunks)

	w := performRequest(router, http.MethodGet, "/v1/knowledge/sources/src-1/chunks", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.KnowledgeChunk
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Hello world", resp[0].Content)

	knowledgeRepo.AssertExpectations(t)
}

func TestDeleteChunk_Success(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	knowledgeRepo.On("GetChunkByID", mock.Anything, "chunk-1").Return(&domain.KnowledgeChunk{
		ID:       "chunk-1",
		SourceID: "src-1",
		Content:  "Some content",
	}, nil)

	knowledgeRepo.On("GetSourceByID", mock.Anything, "src-1").Return(&domain.KnowledgeSource{
		ID:      "src-1",
		AgentID: testAgentID,
	}, nil)

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)

	knowledgeRepo.On("DeleteChunk", mock.Anything, "chunk-1").Return(nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/knowledge/chunks/:id", authMw, middleware.RequireRole("admin"), handler.HandleDeleteKnowledge)

	w := performRequest(router, http.MethodDelete, "/v1/knowledge/chunks/chunk-1", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "knowledge chunk deleted", resp["message"])

	knowledgeRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}

func TestDeleteChunk_NotFound(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	knowledgeRepo.On("GetChunkByID", mock.Anything, "nonexistent").Return(nil, nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/knowledge/chunks/:id", authMw, middleware.RequireRole("admin"), handler.HandleDeleteKnowledge)

	w := performRequest(router, http.MethodDelete, "/v1/knowledge/chunks/nonexistent", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "KNOWLEDGE_NOT_FOUND", resp["code"])
}

// buildMultipart writes a single "file" part plus any extra form
// fields. Used by the upload handler tests below — the shared
// performRequest helper only does JSON.
func buildMultipart(t *testing.T, fields map[string]string, fileField, fileName string, fileBody []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	part, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(fileBody); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestUploadFile_Success(t *testing.T) {
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)
	knowledgeRepo.On("CreateSource", mock.Anything, mock.AnythingOfType("*domain.KnowledgeSource")).Return(nil)
	aiProvider.On("GetEmbedding", mock.Anything, mock.Anything).
		Return([]float32{0.1}, nil)
	knowledgeRepo.On("CreateChunk", mock.Anything, mock.AnythingOfType("*domain.KnowledgeChunk")).
		Return(nil)

	uc := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources/upload", authMw,
		middleware.RequireRole("admin"), handler.HandleUploadFile)

	body, ct := buildMultipart(t,
		map[string]string{"agent_id": testAgentID, "name": "FAQ.txt"},
		"file", "FAQ.txt",
		[]byte("Clinica aberta de segunda a sexta."),
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/sources/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["source_id"])
	assert.Equal(t, float64(1), resp["chunks_created"])
}

func TestUploadFile_InvalidType(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	uc := usecases.NewKnowledgeUseCase(new(MockKnowledgeRepo), new(MockAgentRepo), new(MockAIProvider))
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources/upload", authMw,
		middleware.RequireRole("admin"), handler.HandleUploadFile)

	// JPEG magic (0xFF 0xD8 0xFF) — invalid UTF-8, so the sniff falls
	// through to "" and the handler rejects with INVALID_FILE_TYPE
	// before even touching the usecase.
	body, ct := buildMultipart(t,
		map[string]string{"agent_id": testAgentID},
		"file", "bad.jpg",
		[]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/sources/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_FILE_TYPE", resp["code"])
}

func TestCrawlURL_RejectsHTTP(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo := new(MockAgentRepo)
	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID: testAgentID, AccountID: testAccountID,
	}, nil)

	uc := usecases.NewKnowledgeUseCase(new(MockKnowledgeRepo), agentRepo, new(MockAIProvider))
	handler := NewKnowledgeHandler(uc)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.POST("/v1/knowledge/sources/url", authMw,
		middleware.RequireRole("admin"), handler.HandleCrawlURL)

	body := map[string]interface{}{
		"agent_id":  testAgentID,
		"url":       "http://example.com",
		"max_pages": 3,
	}
	w := performRequest(router, http.MethodPost, "/v1/knowledge/sources/url", body, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "KNOWLEDGE_URL_CRAWL_FAILED", resp["code"])
}
