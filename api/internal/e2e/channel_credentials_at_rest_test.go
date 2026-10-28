package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-012: channel credentials are encrypted at rest. Fails on the old
// code, where `SELECT credentials FROM channels` returned the Twilio
// auth_token in plaintext.
func TestE2E_ChannelCredentialsEncryptedAtRest(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	const authToken = "twilio-auth-token-plain"
	w := doRequest("POST", "/v1/channels", map[string]interface{}{
		"type": "sms",
		"name": "SMS BR",
		"config": map[string]string{
			"account_sid": "AC0123456789abcdef0123456789abcdef",
			"auth_token":  authToken,
			"from_number": "+15551234567",
		},
	}, token)
	require.Equal(t, 201, w.Code, "create channel failed: %s", w.Body.String())
	var channel map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &channel))
	channelID := channel["id"].(string)

	// The COLUMN must hold ciphertext, never the credential.
	var rawCreds string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT credentials::text FROM channels WHERE id = $1", channelID).Scan(&rawCreds))
	assert.NotContains(t, rawCreds, authToken, "auth_token stored in plaintext")
	assert.Contains(t, rawCreds, "encv1:", "auth_token should carry the encryption prefix")
	assert.Contains(t, rawCreds, "AC0123456789abcdef0123456789abcdef",
		"non-sensitive keys stay readable for SQL lookups")

	// The repository must transparently decrypt — the send/webhook
	// paths depend on reading the real token back.
	repoCipher, err := secrets.NewAESGCMCipher(strings.Repeat("ab", 32))
	require.NoError(t, err)
	repo := postgres.NewChannelRepository(testPool, repoCipher)
	ch, err := repo.GetByID(context.Background(), channelID)
	require.NoError(t, err)
	cfg, ok := ch.Config.([]byte)
	require.True(t, ok, "repo returns config as raw JSONB bytes")
	var m map[string]string
	require.NoError(t, json.Unmarshal(cfg, &m))
	assert.Equal(t, authToken, m["auth_token"])

	// And the API response still masks it (US-003 behavior preserved).
	w = doRequest("GET", "/v1/channels", nil, token)
	require.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), authToken)
	assert.NotContains(t, w.Body.String(), "encv1:")
}
