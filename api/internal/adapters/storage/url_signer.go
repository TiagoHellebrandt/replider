package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// URLSigner mints URLs that point at OUR media proxy instead of at Google.
//
// A GCS V4 signed URL is correct and secure, and the browser still could not
// use it: Flutter Web decodes an image by FETCHING it, and a GCS bucket sends
// no CORS headers, so every attachment failed in the dashboard while opening
// fine in a browser tab. Serving the bytes from the API instead puts them on an
// origin whose CORS policy we already control (CORS_ALLOWED_ORIGINS), and it
// takes the bucket's own configuration out of the render path entirely.
//
// Same shape as telegram.URLSigner, and deliberately so — that proxy already
// solved the identical problem for the bot token, and two different signing
// schemes for two media proxies is one more thing to get subtly wrong.
type URLSigner struct {
	APIBaseURL string
	Secret     []byte
}

// mediaObjectPrefix is where every account's media lives. Signing is what
// authorises a fetch, but the prefix check is what keeps a bug in a future
// caller from minting a URL for something outside an account's own tree.
const mediaObjectPrefix = "accounts/"

// SignObjectURL produces a URL of the form:
//
//	{APIBaseURL}/v1/media/file?a={accountID}&p={objectPath}&e={exp}&s={hex(hmac)}
//
// The account id is inside the signed message, not just alongside it: without
// it a signature minted for one tenant's object would verify for any caller who
// replayed it under another account id.
func (s *URLSigner) SignObjectURL(accountID, objectPath string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	sig := s.sign(accountID, objectPath, strconv.FormatInt(exp, 10))
	return fmt.Sprintf(
		"%s/v1/media/file?a=%s&p=%s&e=%d&s=%s",
		s.APIBaseURL,
		url.QueryEscape(accountID),
		url.QueryEscape(objectPath),
		exp,
		sig,
	)
}

// Verify returns true when (accountID, objectPath, exp, sig) reconstruct a
// valid, unexpired HMAC. Constant-time comparison.
func (s *URLSigner) Verify(accountID, objectPath, expStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(s.sign(accountID, objectPath, expStr)), []byte(sig))
}

func (s *URLSigner) sign(accountID, objectPath, expStr string) string {
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(accountID + "|" + objectPath + "|" + expStr))
	return hex.EncodeToString(mac.Sum(nil))
}

// ObjectBelongsToAccount reports whether objectPath sits inside this account's
// own tree, and contains no traversal.
//
// Redundant with the signature by design: the signature proves WE minted the
// URL, this proves we minted a sane one. A path built from a future caller's
// input — an export, an admin tool — must not be able to walk out of the
// account prefix just because it was signed on the way past.
func ObjectBelongsToAccount(accountID, objectPath string) bool {
	if accountID == "" || objectPath == "" {
		return false
	}
	if strings.Contains(objectPath, "..") || strings.HasPrefix(objectPath, "/") {
		return false
	}
	return strings.HasPrefix(objectPath, mediaObjectPrefix+accountID+"/")
}
