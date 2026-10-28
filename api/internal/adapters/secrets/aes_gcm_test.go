package secrets

import (
	"encoding/base64"
	"strings"
	"testing"
)

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestAESGCMCipher_RoundTrip(t *testing.T) {
	c, err := NewAESGCMCipher(testKey)
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}
	plain := "sk-proj-XYZ_supersecret_token_value"
	sealed, err := c.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if sealed == plain {
		t.Errorf("ciphertext leaks plaintext")
	}
	got, err := c.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != plain {
		t.Errorf("round-trip mismatch: %q != %q", got, plain)
	}
}

// TestAESGCMCipher_SealBytesRoundTrip covers the bytea path used by the
// at-rest TOTP secret column (US-076): a Base32 TOTP seed sealed to raw
// bytes and recovered intact, with no plaintext leak in the ciphertext.
func TestAESGCMCipher_SealBytesRoundTrip(t *testing.T) {
	c, err := NewAESGCMCipher(testKey)
	if err != nil {
		t.Fatalf("NewAESGCMCipher: %v", err)
	}
	secret := "JBSWY3DPEHPK3PXP" // canonical RFC 6238 sample Base32 seed
	sealed, err := c.SealBytes(secret)
	if err != nil {
		t.Fatalf("SealBytes: %v", err)
	}
	if len(sealed) == 0 {
		t.Fatalf("SealBytes returned empty for non-empty input")
	}
	if strings.Contains(string(sealed), secret) {
		t.Errorf("ciphertext leaks plaintext secret")
	}
	got, err := c.OpenBytes(sealed)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	if got != secret {
		t.Errorf("round-trip mismatch: %q != %q", got, secret)
	}
}

func TestAESGCMCipher_SealBytesEmpty(t *testing.T) {
	c, _ := NewAESGCMCipher(testKey)
	sealed, err := c.SealBytes("")
	if err != nil {
		t.Fatalf("SealBytes(\"\"): %v", err)
	}
	if sealed != nil {
		t.Errorf("empty plaintext should seal to nil bytes, got %v", sealed)
	}
	plain, err := c.OpenBytes(nil)
	if err != nil {
		t.Fatalf("OpenBytes(nil): %v", err)
	}
	if plain != "" {
		t.Errorf("nil sealed should open to empty plaintext, got %q", plain)
	}
}

func TestAESGCMCipher_OpenBytesRejectsTampering(t *testing.T) {
	c, _ := NewAESGCMCipher(testKey)
	sealed, _ := c.SealBytes("payload")
	sealed[len(sealed)-1] ^= 0x01 // corrupt the GCM tag
	if _, err := c.OpenBytes(sealed); err == nil {
		t.Errorf("tampered ciphertext must fail authentication")
	}
}

func TestAESGCMCipher_NondeterministicCiphertext(t *testing.T) {
	c, _ := NewAESGCMCipher(testKey)
	a, _ := c.Seal("identical-input")
	b, _ := c.Seal("identical-input")
	if a == b {
		t.Errorf("expected unique ciphertext per call (nonce reuse)")
	}
}

func TestAESGCMCipher_EmptyRoundTrip(t *testing.T) {
	c, _ := NewAESGCMCipher(testKey)
	sealed, _ := c.Seal("")
	if sealed != "" {
		t.Errorf("empty plaintext should seal to empty string")
	}
	plain, _ := c.Open("")
	if plain != "" {
		t.Errorf("empty sealed should open to empty plaintext")
	}
}

func TestAESGCMCipher_RejectsTampering(t *testing.T) {
	c, _ := NewAESGCMCipher(testKey)
	sealed, _ := c.Seal("payload")
	raw, _ := base64.StdEncoding.DecodeString(sealed)
	// Flip a byte inside the GCM tag region (last 16 bytes) so the
	// AEAD authenticator rejects the open. Just flipping a base64 char
	// is unreliable because base64 alphabet folding can map back to
	// the same plaintext byte.
	raw[len(raw)-1] ^= 0x01
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := c.Open(tampered); err == nil {
		t.Errorf("tampered ciphertext must fail authentication")
	}
}

func TestAESGCMCipher_RejectsBadKeyLength(t *testing.T) {
	if _, err := NewAESGCMCipher("deadbeef"); err == nil {
		t.Errorf("expected error for short key")
	}
	if _, err := NewAESGCMCipher("not-hex-😀-not-hex"); err == nil {
		t.Errorf("expected error for non-hex key")
	}
	if _, err := NewAESGCMCipher(""); err == nil {
		t.Errorf("expected error for empty key")
	}
}

func TestMaskKey(t *testing.T) {
	got := MaskKey("sk-proj-1234567890abcdef")
	if !strings.HasPrefix(got, "sk-p") || !strings.HasSuffix(got, "cdef") {
		t.Errorf("MaskKey kept wrong edges: %q", got)
	}
	if !strings.Contains(got, "••••••••") {
		t.Errorf("MaskKey did not mask middle: %q", got)
	}
	if MaskKey("short") != "••••••••" {
		t.Errorf("MaskKey should fully mask short inputs")
	}
}
