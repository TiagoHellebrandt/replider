package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The token exists so the session JWT never travels in a query string. It has
// to be narrow enough that leaking it costs one email and nothing else.

const testSecret = "secret-do-jwt-de-teste"

func TestEmailViewToken_RoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tok := MintEmailViewToken(testSecret, "acc-1", "msg-1", now)

	acc, msg, err := ParseEmailViewToken(testSecret, tok, now)

	require.NoError(t, err)
	assert.Equal(t, "acc-1", acc)
	assert.Equal(t, "msg-1", msg)
}

func TestEmailViewToken_ExpiresInTwoMinutes(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tok := MintEmailViewToken(testSecret, "acc-1", "msg-1", now)

	_, _, stillOK := ParseEmailViewToken(testSecret, tok, now.Add(119*time.Second))
	assert.NoError(t, stillOK)

	_, _, expired := ParseEmailViewToken(testSecret, tok, now.Add(2*time.Minute+time.Second))
	assert.Error(t, expired)
}

func TestEmailViewToken_RejectsAForgedSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tok := MintEmailViewToken(testSecret, "acc-1", "msg-1", now)

	_, _, err := ParseEmailViewToken("outro-segredo", tok, now)
	assert.Error(t, err)
}

// The whole point of scoping: a token for one message cannot be edited into a
// token for another.
func TestEmailViewToken_CannotBeRepointedAtAnotherMessage(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tok := MintEmailViewToken(testSecret, "acc-1", "msg-1", now)

	tampered := strings.ReplaceAll(tok, "A", "B")
	_, _, err := ParseEmailViewToken(testSecret, tampered, now)
	assert.Error(t, err)
}

func TestEmailViewToken_RejectsGarbage(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, tok := range []string{"", "   ", "not-base64!!", "YWJj", "a.b.c.d"} {
		_, _, err := ParseEmailViewToken(testSecret, tok, now)
		assert.Error(t, err, "input %q", tok)
	}
}

// Purpose binding: a signature made for this feature must not validate under
// a different purpose, so a token can never be replayed against another
// endpoint that happens to share the secret.
func TestEmailViewToken_IsBoundToItsPurpose(t *testing.T) {
	payload := "acc-1.msg-1.1800000120"

	assert.NotEqual(t,
		signEmailViewToken(testSecret, payload),
		signEmailViewTokenWithPurpose(testSecret, "some-other-purpose", payload))
}
