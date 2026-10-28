package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
)

// The two small-wins reads: the shell badge count and the command-center
// waiting_reply source. Both defined by conversations.waiting_since — set
// by the inbound, cleared by a human reply — on an open conversation.

func seedWaitingConversation(t *testing.T, accountID, name string, waiting bool) string {
	t.Helper()
	channelID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO channels (id, account_id, name, provider, credentials, is_active, webhook_token, created_at, updated_at)
		 VALUES ($1, $2, 'c', 'whatsapp', '{}', TRUE, $3, NOW(), NOW())`,
		channelID, accountID, ulid.Make().String())
	require.NoError(t, err)
	convID := ulid.Make().String()
	ws := "NOW()"
	if !waiting {
		ws = "NULL"
	}
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_name, status, is_bot_paused, waiting_since, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'open', FALSE, `+ws+`, NOW(), NOW())`,
		convID, accountID, channelID, ulid.Make().String()[10:], name)
	require.NoError(t, err)
	return convID
}

func TestWaitingReplies_CountAndCommandCenterSource(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	registerUser(t, "Corp", "wait@test.com", "pass1234")
	verifyEmail(t, "wait@test.com")
	token := login(t, "wait@test.com", "pass1234")

	var accountID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT account_id FROM users WHERE email = 'wait@test.com'`).Scan(&accountID))

	seedWaitingConversation(t, accountID, "Hospital A", true)
	seedWaitingConversation(t, accountID, "Hospital B", true)
	seedWaitingConversation(t, accountID, "Hospital C", false) // answered: not counted

	// The badge endpoint.
	w := doRequest("GET", "/v1/conversations/waiting-count", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := parseJSON(t, w)
	assert.Equal(t, float64(2), resp["count"])

	// The command-center source, straight at the repo (the HTTP surface is
	// covered by the command-center endpoint's own tests).
	repo := postgres.NewCommandCenterRepository(testPool)
	actions, err := repo.WaitingReplies(ctx, accountID, "", 10)
	require.NoError(t, err)
	require.Len(t, actions, 2)
	assert.Equal(t, "Hospital A", actions[0].CustomerName)
	assert.Equal(t, "waiting_reply", string(actions[0].Kind))
	assert.NotNil(t, actions[0].SinceAt)
	assert.Equal(t, "/home", actions[0].DeepLink)
}
