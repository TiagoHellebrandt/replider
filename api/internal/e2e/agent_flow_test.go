package e2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_AgentLifecycle(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Create agent
	w := doRequest("POST", "/v1/agents", map[string]string{
		"name":     "Support Bot",
		"provider": "google",
	}, token)
	assert.Equal(t, 201, w.Code, "create agent failed: %s", w.Body.String())
	var agent map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &agent)
	require.NoError(t, err)
	agentID := agent["id"].(string)
	assert.NotEmpty(t, agentID)

	// List agents
	w = doRequest("GET", "/v1/agents", nil, token)
	assert.Equal(t, 200, w.Code)
	agents := parseJSONArray(t, w)
	assert.Len(t, agents, 1)

	// Update agent
	w = doRequest("PUT", "/v1/agents/"+agentID, map[string]interface{}{
		"name":          "Updated Bot",
		"system_prompt": "Be helpful",
		"provider":      "google",
		"model":         "gemini-3.1-flash-live-preview",
		"timezone":      "America/Sao_Paulo",
	}, token)
	assert.Equal(t, 200, w.Code, "update agent failed: %s", w.Body.String())

	// Verify update
	w = doRequest("GET", "/v1/agents", nil, token)
	agents = parseJSONArray(t, w)
	require.Len(t, agents, 1)
	updatedAgent := agents[0].(map[string]interface{})
	assert.Equal(t, "Updated Bot", updatedAgent["name"])

	// Toggle (pause)
	w = doRequest("PATCH", "/v1/agents/"+agentID+"/toggle", map[string]interface{}{
		"is_active": false,
	}, token)
	assert.Equal(t, 200, w.Code, "toggle agent failed: %s", w.Body.String())

	// Delete
	w = doRequest("DELETE", "/v1/agents/"+agentID, nil, token)
	assert.Equal(t, 200, w.Code, "delete agent failed: %s", w.Body.String())

	// Verify deleted
	w = doRequest("GET", "/v1/agents", nil, token)
	agents = parseJSONArray(t, w)
	assert.Len(t, agents, 0)

	// Login again should show is_new=true (no agents)
	w = doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "pass1234",
	}, "")
	loginResp := parseJSON(t, w)
	assert.Equal(t, true, loginResp["is_new"])
}

func TestE2E_AgentOwnership(t *testing.T) {
	cleanDB(t)
	// User A
	registerUser(t, "Corp A", "userA@test.com", "pass1234")
	verifyEmail(t, "userA@test.com")
	tokenA := login(t, "userA@test.com", "pass1234")

	// User B
	registerUser(t, "Corp B", "userB@test.com", "pass1234")
	verifyEmail(t, "userB@test.com")
	tokenB := login(t, "userB@test.com", "pass1234")

	// A creates agent
	w := doRequest("POST", "/v1/agents", map[string]string{
		"name":     "A's Bot",
		"provider": "google",
	}, tokenA)
	assert.Equal(t, 201, w.Code)
	var agent map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &agent)
	agentID := agent["id"].(string)

	// B tries to update A's agent -> 404
	w = doRequest("PUT", "/v1/agents/"+agentID, map[string]interface{}{
		"name":     "Hacked",
		"provider": "google",
	}, tokenB)
	assert.Equal(t, 404, w.Code)

	// B tries to delete A's agent -> 404
	w = doRequest("DELETE", "/v1/agents/"+agentID, nil, tokenB)
	assert.Equal(t, 404, w.Code)

	// A can still see the agent
	w = doRequest("GET", "/v1/agents", nil, tokenA)
	agents := parseJSONArray(t, w)
	assert.Len(t, agents, 1)

	// B sees no agents
	w = doRequest("GET", "/v1/agents", nil, tokenB)
	agents = parseJSONArray(t, w)
	assert.Len(t, agents, 0)
}

func TestE2E_MultipleAgents(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Create 1 agent (limit is 1 per account)
	w := doRequest("POST", "/v1/agents", map[string]string{
		"name":     "Bot A",
		"provider": "google",
	}, token)
	assert.Equal(t, 201, w.Code)

	// Second agent should fail (limit reached)
	w = doRequest("POST", "/v1/agents", map[string]string{
		"name":     "Bot B",
		"provider": "google",
	}, token)
	assert.NotEqual(t, 201, w.Code, "should reject second agent due to limit")

	// List should return 1
	w = doRequest("GET", "/v1/agents", nil, token)
	assert.Equal(t, 200, w.Code)
	agents := parseJSONArray(t, w)
	assert.Len(t, agents, 1)
}
