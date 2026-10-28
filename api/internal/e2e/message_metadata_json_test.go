package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A message re-read from the list must serialize its metadata as the JSON
// OBJECT it is — not as a base64 string.
//
// The generated row types carry the JSONB column as []byte, and a []byte
// inside an interface{} marshals as base64. Consequence in production: the
// email sender-auth verdict (email_auth.trustworthy) was unreadable to the
// dashboard after any refresh, and every legitimate, DKIM-passed message
// rendered "remetente não verificado". Live WebSocket delivery carried a map
// and serialized fine, which is why the bug only existed after a reload.
func TestListMessages_MetadataSerializesAsObject(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)

	channelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, name, provider, credentials, is_active, webhook_token, created_at, updated_at)
		 VALUES ($1, $2, 'Email', 'email', '{}', true, $3, NOW(), NOW())`,
		channelID, accountID, ulid.Make().String())
	require.NoError(t, err)

	convID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'tiago@gmail.com', 'open', NOW(), NOW())`,
		convID, accountID, channelID)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx,
		`INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, metadata, created_at)
		 VALUES ($1, $2, $3, 'tiago@gmail.com', 'text', 'salve', 'inbound',
		         '{"subject":"teste","email_auth":{"trustworthy":true,"suspicious":false,"dkim":"pass"}}'::jsonb,
		         NOW())`,
		ulid.Make().String(), channelID, convID)
	require.NoError(t, err)

	repo := postgres.NewMessageRepository(testPool)
	msgs, err := repo.ListByConversationID(ctx, convID, 50, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	// What the HTTP handler does with the message: marshal it. This is where
	// []byte silently became base64.
	payload, err := json.Marshal(msgs[0])
	require.NoError(t, err)
	body := string(payload)
	assert.Contains(t, body, `"email_auth"`,
		"metadata must reach the client as an object the dashboard can read")
	assert.Contains(t, body, `"trustworthy":true`)
	assert.False(t, strings.Contains(body, `"metadata":"`),
		"metadata must never serialize as a (base64) string")
}
