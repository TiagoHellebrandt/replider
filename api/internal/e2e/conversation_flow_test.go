package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_ConversationList(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Create agent
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
		"name":     "TG",
		"config":   map[string]string{"bot_token": "fake"},
	}, token)
	require.Equal(t, 201, w.Code)
	var channel map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &channel)
	channelID := channel["id"].(string)

	// Get account ID from DB
	var accountID string
	err := testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "admin@test.com").Scan(&accountID)
	require.NoError(t, err)

	// Insert conversation + messages directly via SQL (simulating webhook)
	convID := ulid.Make().String()
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status)
		VALUES ($1, $2, $3, $4, 'customer-123', 'John Doe', 'open')
	`, convID, accountID, channelID, agentID)
	require.NoError(t, err)

	msgID := ulid.Make().String()
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction)
		VALUES ($1, $2, $3, 'customer', 'text', 'Hello!', 'inbound')
	`, msgID, channelID, convID)
	require.NoError(t, err)

	// GET /conversations
	w = doRequest("GET", "/v1/conversations", nil, token)
	assert.Equal(t, 200, w.Code, "list conversations failed: %s", w.Body.String())
	convs := parseJSONArray(t, w)
	assert.GreaterOrEqual(t, len(convs), 1)

	// GET /conversations/:id/messages
	w = doRequest("GET", "/v1/conversations/"+convID+"/messages", nil, token)
	assert.Equal(t, 200, w.Code, "list messages failed: %s", w.Body.String())
	msgs := parseJSONArray(t, w)
	assert.GreaterOrEqual(t, len(msgs), 1)

	// PATCH /conversations/:id/resume-bot
	w = doRequest("PATCH", "/v1/conversations/"+convID+"/resume-bot", nil, token)
	assert.Equal(t, 200, w.Code, "resume bot failed: %s", w.Body.String())
}

func TestE2E_ConversationCrossAccountIsolation(t *testing.T) {
	cleanDB(t)

	// User A
	registerUser(t, "Corp A", "userA@test.com", "pass1234")
	verifyEmail(t, "userA@test.com")
	tokenA := login(t, "userA@test.com", "pass1234")

	// User B
	registerUser(t, "Corp B", "userB@test.com", "pass1234")
	verifyEmail(t, "userB@test.com")
	tokenB := login(t, "userB@test.com", "pass1234")

	// A creates agent + channel + conversation
	w := doRequest("POST", "/v1/agents", map[string]string{"name": "Bot A", "provider": "google"}, tokenA)
	require.Equal(t, 201, w.Code)
	var agentA map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &agentA)
	agentIDa := agentA["id"].(string)

	w = doRequest("POST", "/v1/channels", map[string]interface{}{
		"agent_id": agentIDa, "type": "telegram", "name": "TG A", "config": map[string]string{"bot_token": "a"},
	}, tokenA)
	require.Equal(t, 201, w.Code)
	var chA map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &chA)
	channelIDa := chA["id"].(string)

	var accountIDA string
	testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "userA@test.com").Scan(&accountIDA)

	convID := ulid.Make().String()
	testPool.Exec(context.Background(), `
		INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, status)
		VALUES ($1, $2, $3, $4, 'cust-1', 'open')
	`, convID, accountIDA, channelIDa, agentIDa)

	// B should see no conversations
	w = doRequest("GET", "/v1/conversations", nil, tokenB)
	assert.Equal(t, 200, w.Code)
	convsB := parseJSONArray(t, w)
	assert.Len(t, convsB, 0)

	// A sees the conversation
	w = doRequest("GET", "/v1/conversations", nil, tokenA)
	assert.Equal(t, 200, w.Code)
	convsA := parseJSONArray(t, w)
	assert.Len(t, convsA, 1)
}
