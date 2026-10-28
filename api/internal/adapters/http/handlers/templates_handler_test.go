package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPromptTemplates_Success(t *testing.T) {
	handler := NewTemplatesHandler()

	router := setupTestRouter()
	router.GET("/v1/templates/prompts", handler.HandleListPromptTemplates)

	w := performRequest(router, http.MethodGet, "/v1/templates/prompts", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []PromptTemplate
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 8) // 8 predefined templates
}

func TestListPromptTemplates_HasExpectedFields(t *testing.T) {
	handler := NewTemplatesHandler()

	router := setupTestRouter()
	router.GET("/v1/templates/prompts", handler.HandleListPromptTemplates)

	w := performRequest(router, http.MethodGet, "/v1/templates/prompts", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []PromptTemplate
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify each template has the required fields
	for _, tmpl := range resp {
		assert.NotEmpty(t, tmpl.ID, "template ID should not be empty")
		assert.NotEmpty(t, tmpl.Name, "template Name should not be empty")
		assert.NotEmpty(t, tmpl.NameEN, "template NameEN should not be empty")
		assert.NotEmpty(t, tmpl.Prompt, "template Prompt should not be empty")
	}

	// Verify first and last templates by ID
	assert.Equal(t, "clinic", resp[0].ID)
	assert.Equal(t, "general", resp[len(resp)-1].ID)
}
