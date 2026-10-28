package twilio

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func inboundForm() url.Values {
	return url.Values{
		"MessageSid": {"SM0123456789abcdef0123456789abcdef"},
		"AccountSid": {"ACabc"},
		"From":       {"+15558675310"},
		"To":         {"+15550001111"},
		"Body":       {"Hi there"},
		"NumMedia":   {"0"},
		"SmsStatus":  {"received"},
	}
}

func TestParseInboundForm_Text(t *testing.T) {
	msg, ok := ParseInboundForm(inboundForm())
	assert.True(t, ok)
	assert.Equal(t, "+15558675310", msg.From)
	assert.Equal(t, "+15550001111", msg.To)
	assert.Equal(t, "Hi there", msg.Body)
	assert.Equal(t, "SM0123456789abcdef0123456789abcdef", msg.MessageSID)
	assert.Empty(t, msg.Media)
}

func TestParseInboundForm_MMS_AllAttachments(t *testing.T) {
	form := inboundForm()
	form.Set("NumMedia", "3")
	form.Set("MediaUrl0", "https://api.twilio.com/media/ME0")
	form.Set("MediaContentType0", "image/png")
	form.Set("MediaUrl1", "https://api.twilio.com/media/ME1")
	form.Set("MediaContentType1", "video/mp4")
	form.Set("MediaUrl2", "https://api.twilio.com/media/ME2")
	form.Set("MediaContentType2", "application/pdf")

	msg, ok := ParseInboundForm(form)
	assert.True(t, ok)
	// EVERY attachment must be read — MediaUrl0-only parsing silently
	// dropped the rest of a multi-photo MMS.
	assert.Equal(t, []InboundMedia{
		{URL: "https://api.twilio.com/media/ME0", Type: "image"},
		{URL: "https://api.twilio.com/media/ME1", Type: "video"},
		{URL: "https://api.twilio.com/media/ME2", Type: "document"},
	}, msg.Media)
}

func TestParseInboundForm_MMS_SkipsEmptySlots(t *testing.T) {
	form := inboundForm()
	form.Set("NumMedia", "2")
	form.Set("MediaUrl0", "https://api.twilio.com/media/ME0")
	form.Set("MediaContentType0", "image/png")
	// MediaUrl1 absent despite NumMedia=2 — malformed, must not panic
	// nor produce an empty-URL attachment.

	msg, ok := ParseInboundForm(form)
	assert.True(t, ok)
	assert.Equal(t, []InboundMedia{{URL: "https://api.twilio.com/media/ME0", Type: "image"}}, msg.Media)
}

func TestParseInboundForm_MediaTypeBuckets(t *testing.T) {
	cases := map[string]string{
		"image/jpeg":      "image",
		"video/mp4":       "video",
		"audio/ogg":       "audio",
		"application/pdf": "document",
		"text/vcard":      "document",
	}
	for contentType, want := range cases {
		assert.Equal(t, want, mediaTypeFromContentType(contentType), contentType)
	}
}

func TestParseInboundForm_StatusCallbackRejected(t *testing.T) {
	form := inboundForm()
	form.Set("MessageStatus", "delivered")
	form.Set("SmsStatus", "delivered")
	_, ok := ParseInboundForm(form)
	assert.False(t, ok)
}

func TestParseInboundForm_MissingFromRejected(t *testing.T) {
	form := inboundForm()
	form.Del("From")
	_, ok := ParseInboundForm(form)
	assert.False(t, ok)
}

func TestParseInboundForm_MissingMessageSidRejected(t *testing.T) {
	form := inboundForm()
	form.Del("MessageSid")
	_, ok := ParseInboundForm(form)
	assert.False(t, ok)
}

// signTwilio mirrors Twilio's documented algorithm so ValidateSignature
// is checked against an independent implementation, not itself.
func signTwilio(authToken, fullURL string, form url.Values) string {
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	payload := fullURL
	for _, k := range keys {
		payload += k + form.Get(k)
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestValidateSignature_Valid(t *testing.T) {
	form := inboundForm()
	const fullURL = "https://api.replider.com/v1/webhooks/tok123"
	sig := signTwilio("secret-token", fullURL, form)
	assert.True(t, ValidateSignature("secret-token", fullURL, form, sig))
}

func TestValidateSignature_Rejections(t *testing.T) {
	form := inboundForm()
	const fullURL = "https://api.replider.com/v1/webhooks/tok123"
	sig := signTwilio("secret-token", fullURL, form)

	assert.False(t, ValidateSignature("wrong-token", fullURL, form, sig), "wrong auth token")
	assert.False(t, ValidateSignature("secret-token", fullURL, form, ""), "missing signature")
	assert.False(t, ValidateSignature("", fullURL, form, sig), "empty auth token")
	assert.False(t, ValidateSignature("secret-token", "https://evil.example/v1/webhooks/tok123", form, sig), "different URL")

	tampered := url.Values{}
	for k, v := range form {
		tampered[k] = v
	}
	tampered.Set("Body", "changed")
	assert.False(t, ValidateSignature("secret-token", fullURL, tampered, sig), "tampered params")
}
