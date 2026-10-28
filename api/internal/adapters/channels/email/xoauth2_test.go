package email

import (
	"encoding/base64"
	"net/smtp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The payload Google accepts is exact. A wrong separator or a missing trailing
// byte produces "authentication failed", which is indistinguishable from a bad
// grant — so it is worth pinning byte for byte rather than trusting the format
// string.
func TestXOAuth2Payload_IsTheExactWireFormat(t *testing.T) {
	got := xoauth2Payload("vendas@clinicaaurora.com", "ya29.token")

	assert.Equal(t,
		"user=vendas@clinicaaurora.com\x01auth=Bearer ya29.token\x01\x01",
		string(got))
	// Two control bytes at the end, not one: the first closes the auth
	// parameter, the second closes the list.
	assert.True(t, strings.HasSuffix(string(got), "\x01\x01"))
}

func TestSMTPXOAuth2_SendsTheTokenOnlyOverTLS(t *testing.T) {
	auth := newSMTPXOAuth2("a@b.com", "ya29.token", "smtp.gmail.com")

	// An access token is a bearer credential: whoever reads it off the wire can
	// send as the customer until it expires. Same refusal smtp.PlainAuth makes
	// for a password, and for a stronger reason.
	_, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.gmail.com", TLS: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unencrypted")

	mech, ir, err := auth.Start(&smtp.ServerInfo{Name: "smtp.gmail.com", TLS: true})
	require.NoError(t, err)
	assert.Equal(t, "XOAUTH2", mech)
	assert.Contains(t, string(ir), "auth=Bearer ya29.token")
}

// A server that is not the one we meant to reach does not get the token either.
func TestSMTPXOAuth2_RefusesAnUnexpectedServer(t *testing.T) {
	auth := newSMTPXOAuth2("a@b.com", "ya29.token", "smtp.gmail.com")

	_, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.evil.example", TLS: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected server name")
}

// The failure handshake is the part that is easy to get wrong.
//
// On a rejected token Google does not fail the AUTH command — it sends a
// base64 JSON error and waits for an empty line before reporting it. Returning
// an error here instead of that empty response leaves the connection in a state
// the client cannot finish, so the send fails with a protocol error rather than
// with "your grant expired".
func TestSMTPXOAuth2_AnswersTheFailureChallengeWithAnEmptyLine(t *testing.T) {
	auth := newSMTPXOAuth2("a@b.com", "ya29.token", "smtp.gmail.com")

	resp, err := auth.Next([]byte(`{"status":"400"}`), true)
	require.NoError(t, err)
	assert.Empty(t, resp)

	resp, err = auth.Next(nil, false)
	require.NoError(t, err)
	assert.Nil(t, resp)
}

func TestIMAPXOAuth2_UsesTheSamePayload(t *testing.T) {
	mech, ir, err := newIMAPXOAuth2("a@b.com", "ya29.token").Start()
	require.NoError(t, err)
	assert.Equal(t, "XOAUTH2", mech)
	// Base64 of the initial response is what actually goes on the wire; this
	// asserts the bytes handed to the library are the un-encoded form it
	// expects, not a double-encoded one.
	assert.Equal(t, "user=a@b.com\x01auth=Bearer ya29.token\x01\x01", string(ir))
	_, err = base64.StdEncoding.DecodeString(string(ir))
	assert.Error(t, err, "the client must hand raw bytes, not base64")
}

func TestIMAPXOAuth2_AnswersTheFailureChallenge(t *testing.T) {
	resp, err := newIMAPXOAuth2("a@b.com", "t").Next([]byte(`{"status":"400"}`))
	require.NoError(t, err)
	assert.Empty(t, resp)
}
