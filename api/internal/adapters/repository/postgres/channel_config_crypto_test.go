package postgres

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCipher(t *testing.T) *secrets.AESGCMCipher {
	t.Helper()
	c, err := secrets.NewAESGCMCipher(strings.Repeat("ab", 32))
	require.NoError(t, err)
	return c
}

// US-012: sensitive channel-config values are encrypted at rest. These
// fail on the old code, where the credentials JSONB stored whatever
// plaintext the use case handed down.
func TestChannelConfigCrypto_US012(t *testing.T) {
	cipher := testCipher(t)
	raw := []byte(`{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"supersecret","from_number":"+15551234567"}`)

	t.Run("seals only sensitive keys and round-trips", func(t *testing.T) {
		sealed, err := encryptSensitiveConfig(cipher, raw)
		require.NoError(t, err)

		var m map[string]string
		require.NoError(t, json.Unmarshal(sealed, &m))
		assert.Equal(t, "AC0123456789abcdef0123456789abcdef", m["account_sid"], "non-sensitive keys stay readable")
		assert.Equal(t, "+15551234567", m["from_number"])
		assert.True(t, strings.HasPrefix(m["auth_token"], channelConfigEncPrefix))
		assert.NotContains(t, string(sealed), "supersecret", "plaintext credential must not hit the column")

		opened := decryptSensitiveConfig(cipher, sealed, "ch1")
		var back map[string]string
		require.NoError(t, json.Unmarshal(opened, &back))
		assert.Equal(t, "supersecret", back["auth_token"])
	})

	t.Run("encrypt is idempotent on already-sealed values", func(t *testing.T) {
		sealed, err := encryptSensitiveConfig(cipher, raw)
		require.NoError(t, err)
		again, err := encryptSensitiveConfig(cipher, sealed)
		require.NoError(t, err)

		var m1, m2 map[string]string
		require.NoError(t, json.Unmarshal(sealed, &m1))
		require.NoError(t, json.Unmarshal(again, &m2))
		assert.Equal(t, m1["auth_token"], m2["auth_token"], "double-encrypt would make the value unopenable")
	})

	t.Run("legacy plaintext rows pass through on read", func(t *testing.T) {
		assert.Equal(t, raw, decryptSensitiveConfig(cipher, raw, "ch1"))
	})

	t.Run("nil cipher fails closed on sensitive write", func(t *testing.T) {
		_, err := encryptSensitiveConfig(nil, raw)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "CHANNEL_CIPHER_UNCONFIGURED", appErr.Code)
	})

	t.Run("nil cipher allows credential-free writes", func(t *testing.T) {
		cfg := []byte(`{"phone_number_id":"123","registration_status":"registered"}`)
		out, err := encryptSensitiveConfig(nil, cfg)
		require.NoError(t, err)
		assert.Equal(t, cfg, out)
	})

	t.Run("undecryptable value is dropped, siblings survive", func(t *testing.T) {
		corrupt := []byte(`{"auth_token":"` + channelConfigEncPrefix + `bm90LXJlYWwtY2lwaGVydGV4dA==","from_number":"+15551234567"}`)
		out := decryptSensitiveConfig(cipher, corrupt, "ch1")
		var m map[string]string
		require.NoError(t, json.Unmarshal(out, &m))
		_, hasToken := m["auth_token"]
		assert.False(t, hasToken, "corrupt ciphertext reads as absent")
		assert.Equal(t, "+15551234567", m["from_number"])
	})

	t.Run("nil cipher on read treats encrypted value as absent", func(t *testing.T) {
		sealed, err := encryptSensitiveConfig(cipher, raw)
		require.NoError(t, err)
		out := decryptSensitiveConfig(nil, sealed, "ch1")
		var m map[string]string
		require.NoError(t, json.Unmarshal(out, &m))
		_, hasToken := m["auth_token"]
		assert.False(t, hasToken)
		assert.NotContains(t, string(out), "supersecret")
	})

	t.Run("empty and non-object configs pass through", func(t *testing.T) {
		for _, cfg := range [][]byte{nil, {}, []byte(`null`), []byte(`"str"`)} {
			out, err := encryptSensitiveConfig(cipher, cfg)
			require.NoError(t, err)
			assert.Equal(t, cfg, out)
		}
	})
}
