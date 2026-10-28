package postgres

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

// channelConfigEncPrefix marks a config value encrypted at rest.
// Sensitive channel-config values (Twilio auth_token, IG access_token,
// Telegram bot_token, …) are sealed with the same AES-256-GCM cipher as
// BYOK keys and stored as "encv1:" + base64(nonce||ciphertext||tag)
// inside the credentials JSONB. Values without the prefix are legacy
// plaintext rows — they read through unchanged and get re-encrypted on
// their next write (lazy migration). The version tag leaves room for a
// key-rotation format without a table walk flag.
const channelConfigEncPrefix = "encv1:"

// errChannelCipherUnconfigured follows the *_CIPHER_UNCONFIGURED
// convention (see user_repository.SetTOTPSecret): writes of sensitive
// values FAIL when no key is configured — the platform never persists a
// customer credential in plaintext.
func errChannelCipherUnconfigured() error {
	return &domain.AppError{
		Code:    "CHANNEL_CIPHER_UNCONFIGURED",
		Message: "channel credentials cannot be stored: encryption key not configured on this server",
	}
}

// encryptSensitiveConfig seals every sensitive key of a raw config
// JSONB blob. Non-object blobs and already-sealed values pass through
// (idempotent). A nil cipher errors only when there is actually a
// sensitive plaintext value to protect, so credential-free writes
// (webchat settings, WhatsApp status fields) keep working on a
// degraded deployment.
func encryptSensitiveConfig(cipher *secrets.AESGCMCipher, raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return raw, nil
	}
	changed := false
	for key, val := range m {
		s, isStr := val.(string)
		if !isStr || s == "" || !domain.SensitiveConfigKey(key) ||
			strings.HasPrefix(s, channelConfigEncPrefix) {
			continue
		}
		sealed, err := encryptCredentialValue(cipher, s)
		if err != nil {
			return nil, err
		}
		m[key] = sealed
		changed = true
	}
	if !changed {
		return raw, nil
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// encryptCredentialValue seals a single credential string (the
// single-key write paths: UpdateCredentialField, MergeCredentials).
func encryptCredentialValue(cipher *secrets.AESGCMCipher, value string) (string, error) {
	if cipher == nil {
		return "", errChannelCipherUnconfigured()
	}
	sealed, err := cipher.Seal(value)
	if err != nil {
		return "", err
	}
	return channelConfigEncPrefix + sealed, nil
}

// decryptSensitiveConfig opens every sealed value of a stored config
// blob so everything above the repository keeps seeing plaintext. Per
// the encrypted-column convention, an unopenable value (nil cipher,
// rotated key, corrupt ciphertext) is logged and treated as ABSENT —
// dropping one credential degrades that channel, never every read.
func decryptSensitiveConfig(cipher *secrets.AESGCMCipher, raw []byte, channelID string) []byte {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(channelConfigEncPrefix)) {
		return raw
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return raw
	}
	changed := false
	for key, val := range m {
		s, isStr := val.(string)
		if !isStr || !strings.HasPrefix(s, channelConfigEncPrefix) {
			continue
		}
		if cipher == nil {
			slog.Error("channel config: encrypted credential but no cipher configured — treating as absent",
				"channel_id", channelID, "key", key)
			delete(m, key)
			changed = true
			continue
		}
		plain, err := cipher.Open(strings.TrimPrefix(s, channelConfigEncPrefix))
		if err != nil {
			slog.Error("channel config: credential decrypt failed — treating as absent",
				"channel_id", channelID, "key", key, "error", err)
			delete(m, key)
			changed = true
			continue
		}
		m[key] = plain
		changed = true
	}
	if !changed {
		return raw
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
