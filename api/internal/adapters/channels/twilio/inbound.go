package twilio

import (
	"net/url"
	"strconv"
	"strings"

	twclient "github.com/twilio/twilio-go/client"
)

// InboundSMS is the normalized subset of Twilio's inbound Message
// webhook parameters (application/x-www-form-urlencoded POST) that the
// pipeline consumes. From/To are E.164 phone numbers.
type InboundSMS struct {
	From       string
	To         string
	Body       string
	MessageSID string
	// Media holds EVERY MMS attachment (MediaUrl0..MediaUrl{N-1},
	// NumMedia caps at 10 on Twilio's side). Reading only MediaUrl0
	// silently dropped the rest of a multi-photo MMS. Twilio media URLs
	// are fetchable without the account's basic auth unless HTTP auth
	// is enabled on the account.
	Media []InboundMedia
}

// InboundMedia is one MMS attachment.
type InboundMedia struct {
	URL  string
	Type string
}

// ParseInboundForm normalizes a Twilio webhook form into an InboundSMS.
// Returns ok=false for deliveries that are not an inbound message:
// status callbacks (MessageStatus/SmsStatus other than "received")
// pointed at the wrong URL, or payloads missing From/MessageSid.
func ParseInboundForm(form url.Values) (InboundSMS, bool) {
	// Status callbacks carry MessageStatus (queued/sent/delivered/...);
	// an actual inbound message either omits SmsStatus or sets it to
	// "received". Anything else is not a message to ingest.
	if s := form.Get("MessageStatus"); s != "" && !strings.EqualFold(s, "received") {
		return InboundSMS{}, false
	}
	if s := form.Get("SmsStatus"); s != "" && !strings.EqualFold(s, "received") {
		return InboundSMS{}, false
	}

	msg := InboundSMS{
		From:       form.Get("From"),
		To:         form.Get("To"),
		Body:       form.Get("Body"),
		MessageSID: form.Get("MessageSid"),
	}
	if msg.From == "" || msg.MessageSID == "" {
		return InboundSMS{}, false
	}

	if n, err := strconv.Atoi(form.Get("NumMedia")); err == nil && n > 0 {
		for i := 0; i < n; i++ {
			u := form.Get("MediaUrl" + strconv.Itoa(i))
			if u == "" {
				continue
			}
			msg.Media = append(msg.Media, InboundMedia{
				URL:  u,
				Type: mediaTypeFromContentType(form.Get("MediaContentType" + strconv.Itoa(i))),
			})
		}
	}
	return msg, true
}

// mediaTypeFromContentType maps an MMS attachment MIME type onto the
// pipeline's message-type vocabulary (same buckets SendMessage sends).
func mediaTypeFromContentType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	default:
		return "document"
	}
}

// ValidateSignature checks the X-Twilio-Signature header of a
// form-encoded webhook POST: HMAC-SHA1 over the full request URL plus
// the form params concatenated key+value in sorted key order, base64,
// keyed by the channel's own auth token (BYO Twilio — every channel
// validates against its own credentials). Thin wrapper over the SDK's
// RequestValidator so the algorithm never drifts from Twilio's.
func ValidateSignature(authToken, fullURL string, form url.Values, signature string) bool {
	if authToken == "" || signature == "" {
		return false
	}
	params := make(map[string]string, len(form))
	for k, v := range form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	validator := twclient.NewRequestValidator(authToken)
	return validator.Validate(fullURL, params, signature)
}
