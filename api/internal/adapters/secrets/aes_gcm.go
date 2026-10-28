// Package secrets holds at-rest encryption utilities. Today: AES-256-GCM
// for per-account secrets the platform stores on behalf of customers
// (BYOK API keys for OpenAI/Gemini). Cipher choice rationale:
//
//   - AES-256-GCM is FIPS-140-validated, authenticated, and shipping in
//     Go's std library — no extra deps, no crypto-misuse footguns.
//   - Each ciphertext carries its own random 96-bit nonce so re-encrypting
//     the same plaintext yields different ciphertext (no equality leaks).
//   - The wire format is `nonce | ciphertext | tag` base64-encoded so
//     rotation and pipeline tools (psql copy, audit dumps) stay text-safe.
//
// Key management: the encryption key is loaded from
// BYOK_ENCRYPTION_KEY (32 random bytes, hex-encoded). Rotation requires
// re-encrypting every stored row under the new key; we keep that hook
// out of v1 — when needed, add a key version prefix to the wire format
// and walk the table.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// AESGCMCipher seals and opens secrets with AES-256-GCM. Holds the
// 32-byte master key in memory; instantiated once at boot from config.
type AESGCMCipher struct {
	aead cipher.AEAD
}

// NewAESGCMCipher decodes hexKey (must be exactly 64 hex characters →
// 32 bytes / 256 bits) and prepares the AEAD primitive. Returns a clear
// error so misconfiguration fails the boot sequence loudly instead of
// silently falling through to plaintext.
func NewAESGCMCipher(hexKey string) (*AESGCMCipher, error) {
	if hexKey == "" {
		return nil, fmt.Errorf("secrets: encryption key is empty")
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("secrets: encryption key must be hex-encoded: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("secrets: encryption key must decode to 32 bytes (got %d)", len(keyBytes))
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes block: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: gcm: %w", err)
	}
	return &AESGCMCipher{aead: aead}, nil
}

// Seal encrypts plaintext and returns base64(nonce || ciphertext || tag).
// Empty plaintext returns "" so callers can persist NULL for "no key
// configured" without an extra branch upstream.
func (c *AESGCMCipher) Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open decodes and decrypts a sealed value produced by Seal. Empty
// input returns ""; callers expect NULL → empty plaintext for the BYOK
// "no key set" state.
func (c *AESGCMCipher) Open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("secrets: base64 decode: %w", err)
	}
	if len(raw) < c.aead.NonceSize() {
		return "", fmt.Errorf("secrets: ciphertext too short")
	}
	nonce := raw[:c.aead.NonceSize()]
	body := raw[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("secrets: open: %w", err)
	}
	return string(plain), nil
}

// SealBytes encrypts plaintext and returns the raw sealed bytes
// (nonce || ciphertext || tag), suitable for a Postgres `bytea` column.
// It is the binary sibling of Seal: same wire format, no base64 wrapper.
// Empty plaintext returns a nil slice so callers can persist SQL NULL for
// the "no secret staged" state without an extra branch.
func (c *AESGCMCipher) SealBytes(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secrets: nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// OpenBytes decrypts raw sealed bytes produced by SealBytes. Empty input
// returns "" (NULL → empty plaintext, matching SealBytes).
func (c *AESGCMCipher) OpenBytes(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	if len(sealed) < c.aead.NonceSize() {
		return "", fmt.Errorf("secrets: ciphertext too short")
	}
	nonce := sealed[:c.aead.NonceSize()]
	body := sealed[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("secrets: open: %w", err)
	}
	return string(plain), nil
}

// MaskKey returns a safe-to-display preview of an API key (first 4 +
// last 4 characters, middle masked). Used by the dashboard so admins
// can confirm "the right key is stored" without the platform logging
// the secret. Returns "" for empty input so the UI can render an empty
// state instead of "•••".
func MaskKey(key string) string {
	if len(key) <= 8 {
		// Defensive — real OpenAI/Gemini keys are well over 30 chars.
		// If a tiny string slipped through, mask the whole thing.
		return "••••••••"
	}
	return key[:4] + "••••••••" + key[len(key)-4:]
}
