package e2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_ChannelLifecycle(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Create agent first
	w := doRequest("POST", "/v1/agents", map[string]string{
		"name":     "Bot",
		"provider": "google",
	}, token)
	require.Equal(t, 201, w.Code)
	var agent map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &agent)
	agentID := agent["id"].(string)

	// Create channel
	w = doRequest("POST", "/v1/channels", map[string]interface{}{
		"agent_id": agentID,
		"type":     "telegram",
		"name":     "Telegram Support",
		"config":   map[string]string{"bot_token": "fake-token"},
	}, token)
	assert.Equal(t, 201, w.Code, "create channel failed: %s", w.Body.String())
	var channel map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &channel)
	channelID := channel["id"].(string)
	assert.NotEmpty(t, channelID)

	// List channels
	w = doRequest("GET", "/v1/channels?agent_id="+agentID, nil, token)
	assert.Equal(t, 200, w.Code)
	channels := parseJSONArray(t, w)
	assert.Len(t, channels, 1)

	// Update channel
	w = doRequest("PUT", "/v1/channels/"+channelID, map[string]interface{}{
		"agent_id": agentID,
		"type":     "telegram",
		"name":     "Updated TG Channel",
		"config":   map[string]string{"bot_token": "new-fake-token"},
	}, token)
	assert.Equal(t, 200, w.Code, "update channel failed: %s", w.Body.String())

	// Toggle channel
	w = doRequest("PATCH", "/v1/channels/"+channelID+"/toggle?agent_id="+agentID, nil, token)
	assert.Equal(t, 200, w.Code, "toggle channel failed: %s", w.Body.String())

	// Delete channel
	w = doRequest("DELETE", "/v1/channels/"+channelID+"?agent_id="+agentID, nil, token)
	assert.Equal(t, 200, w.Code, "delete channel failed: %s", w.Body.String())

	// List should be empty
	w = doRequest("GET", "/v1/channels?agent_id="+agentID, nil, token)
	assert.Equal(t, 200, w.Code)
	channels = parseJSONArray(t, w)
	assert.Len(t, channels, 0)
}

func TestE2E_ChannelOwnership(t *testing.T) {
	cleanDB(t)

	// User A
	registerUser(t, "Corp A", "userA@test.com", "pass1234")
	verifyEmail(t, "userA@test.com")
	tokenA := login(t, "userA@test.com", "pass1234")

	// User B
	registerUser(t, "Corp B", "userB@test.com", "pass1234")
	verifyEmail(t, "userB@test.com")
	tokenB := login(t, "userB@test.com", "pass1234")

	// A creates agent + channel
	w := doRequest("POST", "/v1/agents", map[string]string{
		"name":     "A Bot",
		"provider": "google",
	}, tokenA)
	require.Equal(t, 201, w.Code)
	var agent map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &agent)
	agentID := agent["id"].(string)

	w = doRequest("POST", "/v1/channels", map[string]interface{}{
		"agent_id": agentID,
		"type":     "telegram",
		"name":     "A's Channel",
		"config":   map[string]string{"bot_token": "token-a"},
	}, tokenA)
	require.Equal(t, 201, w.Code)
	var channel map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &channel)
	channelID := channel["id"].(string)

	// B cannot delete A's channel
	w = doRequest("DELETE", "/v1/channels/"+channelID+"?agent_id="+agentID, nil, tokenB)
	assert.NotEqual(t, 200, w.Code)

	// B's listing is account-scoped; A's channel must not appear in it.
	w = doRequest("GET", "/v1/channels?agent_id="+agentID, nil, tokenB)
	assert.Equal(t, 200, w.Code)
	bChannels := parseJSONArray(t, w)
	for _, raw := range bChannels {
		ch := raw.(map[string]interface{})
		assert.NotEqual(t, channelID, ch["id"], "B should not see A's channel")
	}
}
