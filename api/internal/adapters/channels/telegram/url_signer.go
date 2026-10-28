package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// URLSigner mints short-lived signed URLs that the frontend can load
// directly via <img>/NetworkImage without ever seeing the Telegram bot
// token. The API side then validates the HMAC and proxies the raw file
// from Telegram using the channel's stored credentials.
type URLSigner struct {
	APIBaseURL string
	Secret     []byte
}

// SignFileURL produces a URL of the form:
//
//	{APIBaseURL}/v1/telegram/file?c={channelID}&p={filePath}&e={exp}&s={hex(hmac)}
//
// ttl bounds how long the URL stays valid. Avatars use 7d (they rotate
// server-side on 24h refresh anyway); media messages can pick shorter.
func (s *URLSigner) SignFileURL(channelID, filePath string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	msg := channelID + "|" + filePath + "|" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf(
		"%s/v1/telegram/file?c=%s&p=%s&e=%d&s=%s",
		s.APIBaseURL,
		url.QueryEscape(channelID),
		url.QueryEscape(filePath),
		exp,
		sig,
	)
}

// Verify returns true when (channelID, filePath, exp, sig) reconstruct
// a valid, unexpired HMAC. Constant-time comparison.
func (s *URLSigner) Verify(channelID, filePath, expStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	msg := channelID + "|" + filePath + "|" + expStr
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
