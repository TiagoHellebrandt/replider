package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A scoped, short-lived token that lets a BROWSER open one stored email.
//
// The original view has to render in a browser — that is where the CSP does
// its work — and a browser tab carries no Authorization header. The obvious
// shortcut is putting the session JWT in the query string, which is exactly
// what must not happen: it lands in browser history, in proxy logs and in
// the Referer of anything the page links to, and it is a key to the whole
// API rather than to one document.
//
// So: a separate token that can do exactly one thing. It names one message
// and one account, it expires in two minutes, and it is signed with its own
// purpose baked in, so it can never be replayed against another endpoint.
//
// Stateless (HMAC, no table) because single-use would need storage and buys
// little here: the window is two minutes and the capability is "read one
// email you were already allowed to read".

const (
	emailViewTokenTTL     = 2 * time.Minute
	emailViewTokenPurpose = "email-original-v1"
)

// MintEmailViewToken returns "accountID.messageID.expiry.signature", base64
// URL-encoded so it survives a query string untouched.
func MintEmailViewToken(secret, accountID, messageID string, now time.Time) string {
	exp := strconv.FormatInt(now.Add(emailViewTokenTTL).Unix(), 10)
	payload := strings.Join([]string{accountID, messageID, exp}, ".")
	return base64.RawURLEncoding.EncodeToString(
		[]byte(payload + "." + signEmailViewToken(secret, payload)))
}

// ParseEmailViewToken validates a token and returns what it authorises.
//
// Every failure returns the same error. A caller learning WHICH check failed
// learns whether a message id exists, which is the thing the account scoping
// exists to hide.
func ParseEmailViewToken(secret, token string, now time.Time) (accountID, messageID string, err error) {
	invalid := fmt.Errorf("invalid view token")

	raw, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if decodeErr != nil {
		return "", "", invalid
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 4 {
		return "", "", invalid
	}
	payload := strings.Join(parts[:3], ".")

	// Constant time: a plain == leaks how much of a forged signature was
	// right, one byte at a time.
	if !hmac.Equal([]byte(parts[3]), []byte(signEmailViewToken(secret, payload))) {
		return "", "", invalid
	}
	exp, convErr := strconv.ParseInt(parts[2], 10, 64)
	if convErr != nil || now.Unix() > exp {
		return "", "", invalid
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", invalid
	}
	return parts[0], parts[1], nil
}

// signEmailViewToken binds the signature to this purpose, so a token minted
// here can never be accepted by another feature that happens to share the
// secret.
func signEmailViewToken(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(emailViewTokenPurpose + "\x00" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// signEmailViewTokenWithPurpose exists so a test can prove the purpose is
// actually part of the signature; production always uses the constant.
func signEmailViewTokenWithPurpose(secret, purpose, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(purpose + "\x00" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
