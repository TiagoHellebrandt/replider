package email

import (
	"errors"
	"fmt"
	"net/smtp"
)

// XOAUTH2, the SASL mechanism Google accepts on both SMTP submission and IMAP.
//
// One implementation for both protocols because the payload is identical and
// two copies would be two places to get the separator wrong — and getting it
// wrong produces "authentication failed", indistinguishable from a bad grant.
//
// The wire format is fixed by Google:
//
//	user=<address>^Aauth=Bearer <token>^A^A
//
// where ^A is a literal 0x01. Base64 of that is the initial client response.

// xoauth2Payload builds the initial response. Exported through the two adapters
// below rather than directly, so there is exactly one place that knows the
// byte layout.
func xoauth2Payload(username, accessToken string) []byte {
	return []byte(fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", username, accessToken))
}

// smtpXOAuth2 implements smtp.Auth.
type smtpXOAuth2 struct {
	username    string
	accessToken string
	// host is checked before the token leaves the process, mirroring what
	// smtp.PlainAuth does for a password: an unencrypted or unexpected server
	// must not be handed a live credential.
	host string
}

// newSMTPXOAuth2 returns the mechanism for one connection.
func newSMTPXOAuth2(username, accessToken, host string) smtp.Auth {
	return &smtpXOAuth2{username: username, accessToken: accessToken, host: host}
}

var errXOAuth2Insecure = errors.New("smtp: refusing to send an OAuth token over an unencrypted connection")

func (a *smtpXOAuth2) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// An access token is a bearer credential: whoever reads it off the wire can
	// send mail as the customer until it expires. Same guard smtp.PlainAuth
	// applies to passwords, and for a stronger reason — a password at least
	// needs the account name to be useful.
	if !server.TLS {
		return "", nil, errXOAuth2Insecure
	}
	if server.Name != a.host {
		return "", nil, fmt.Errorf("smtp: unexpected server name %q", server.Name)
	}
	return "XOAUTH2", xoauth2Payload(a.username, a.accessToken), nil
}

// Next handles the challenge Google sends when the token is rejected.
//
// The protocol is unusual here: on failure the server does NOT fail the AUTH
// outright, it sends a base64 JSON error and waits for an empty response before
// reporting it. Returning an error at this point instead of the empty line
// leaves the connection in a state the client library cannot recover, so the
// send fails with a confusing protocol error rather than "your grant expired".
func (a *smtpXOAuth2) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return []byte(""), nil
	}
	return nil, nil
}

// imapXOAuth2 implements the go-sasl client interface that go-imap's
// Authenticate expects. Declared here rather than pulled from go-sasl so the
// dependency stays indirect and the payload stays in one file.
type imapXOAuth2 struct {
	username    string
	accessToken string
}

func newIMAPXOAuth2(username, accessToken string) *imapXOAuth2 {
	return &imapXOAuth2{username: username, accessToken: accessToken}
}

func (a *imapXOAuth2) Start() (mech string, ir []byte, err error) {
	return "XOAUTH2", xoauth2Payload(a.username, a.accessToken), nil
}

// Next answers the same failure challenge as the SMTP side: an empty response,
// so the server produces its real error instead of a protocol stall.
func (a *imapXOAuth2) Next(_ []byte) ([]byte, error) {
	return []byte(""), nil
}
