package handlers

import "strings"

// What the browser is allowed to RENDER, versus what it must download.
//
// The media proxy echoed the stored content type straight back with
// `X-Content-Type-Options: nosniff` and nothing else. nosniff stops the browser
// GUESSING a type; it does not stop it OBEYING a declared one. These bytes
// arrive from strangers on WhatsApp and e-mail, and the sender picks the type —
// so an attachment declared `text/html` was served as a page on
// api.replider.com. A phishing page on the customer's own API domain, with a
// valid certificate.
//
// The fix is an allowlist rather than a blocklist, because the blocklist is
// unbounded: text/html, image/svg+xml, application/xhtml+xml, and whatever the
// next browser decides to execute.

// inlineTypes are the exact types worth rendering in place. Anything not here
// downloads.
var inlineTypes = map[string]bool{
	"application/pdf": true, // the brochure the seller opens to check a spec
	"text/plain":      true,
	"audio/mpeg":      true,
	"audio/ogg":       true,
	"audio/wav":       true,
	"audio/webm":      true,
	"audio/mp4":       true,
	"audio/aac":       true,
	"video/mp4":       true,
	"video/webm":      true,
	"video/ogg":       true,
	"video/quicktime": true,
}

// inlineImageTypes is deliberately an explicit list and NOT `image/*`.
//
// image/svg+xml is an image to every mime table and a script host to every
// browser: an SVG can carry <script> and run it on whatever origin served it.
// A prefix match on "image/" is the single most common way this allowlist gets
// written wrong.
var inlineImageTypes = map[string]bool{
	"image/jpeg":               true,
	"image/png":                true,
	"image/gif":                true,
	"image/webp":               true,
	"image/avif":               true,
	"image/bmp":                true,
	"image/heic":               true,
	"image/heif":               true,
	"image/tiff":               true,
	"image/x-icon":             true,
	"image/vnd.microsoft.icon": true,
}

// mediaDisposition decides how one stored object may be served.
//
// It returns the content type to send and whether it may render inline. An
// unrecognised, absent or executable type collapses to
// application/octet-stream: the browser is told, in the only two ways it
// listens to, that these bytes are a file and not a document.
func mediaDisposition(stored string) (contentType string, inline bool) {
	// The stored value can carry parameters ("text/html; charset=utf-8") and
	// arbitrary case. Both are attacker-supplied, so the decision is made on
	// the normalised essence and the parameters are dropped rather than
	// echoed.
	essence := strings.ToLower(strings.TrimSpace(stored))
	if i := strings.IndexByte(essence, ';'); i >= 0 {
		essence = strings.TrimSpace(essence[:i])
	}
	if inlineTypes[essence] || inlineImageTypes[essence] {
		return essence, true
	}
	return "application/octet-stream", false
}
