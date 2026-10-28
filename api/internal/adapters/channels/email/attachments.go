package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/replider/api/internal/adapters/httpsafe"
)

// Sending the picture somebody asked for.
//
// domain.Message already carries a MediaURL, and WhatsApp, Telegram and
// Instagram all send it. Email did not: the adapter read Content and nothing
// else, so "manda uma foto do equipamento" on an email thread had no answer —
// the picture existed, was one click from the product screen, and could not
// travel.
//
// It attaches on REPLIES, which is where that question gets asked. Cold
// outbound stays plain text and unattached on purpose (see buildMIME): a
// first-touch message to a stranger carrying a two-megabyte file is a spam
// filter's easiest decision, and the deliverability of the first message is
// the whole funnel.

const (
	// A brochure photograph is a few hundred KB. This is the ceiling for what
	// we will pull down and re-send; above it the message goes without the
	// attachment rather than timing out mid-send.
	maxAttachmentBytes = 10 << 20
	// Fetching is a network call inside a send that already has an SMTP
	// dialogue ahead of it.
	attachmentFetchTimeout = 20 * time.Second
)

// attachment is a file to hang off the message.
type attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// fetchAttachment downloads the media a message points at.
//
// Through the guarded client, not http.DefaultClient: the URL can come from a
// customer-supplied field, and an adapter that fetches an arbitrary address
// from inside the network is an SSRF. Any failure returns nil — the message
// still goes, without the picture, because a reply that never arrives is worse
// than one missing its attachment.
func fetchAttachment(ctx context.Context, rawURL string) *attachment {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	client := httpsafe.Client(attachmentFetchTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAttachmentBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxAttachmentBytes {
		return nil
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if i := strings.Index(ct, ";"); i > 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	return &attachment{Filename: attachmentName(rawURL, ct), ContentType: ct, Data: data}
}

// attachmentName is what the recipient sees in their mail client. Taken from
// the URL path when it looks like a filename, because "p004-9f2c1a.jpg" at
// least says which page it came from; otherwise a plain name with the right
// extension, never the signed URL's query string.
func attachmentName(rawURL, contentType string) string {
	base := rawURL
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	base = path.Base(base)
	if base != "" && base != "." && base != "/" && strings.Contains(base, ".") {
		return base
	}
	ext := ".bin"
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	return "anexo" + ext
}

// writeAttachment appends one MIME part.
func writeAttachment(b *strings.Builder, boundary string, a attachment) {
	b.WriteString("\r\n--" + boundary + "\r\n")
	b.WriteString("Content-Type: " + a.ContentType + "\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%q\r\n\r\n", a.Filename))

	// Wrapped at 76 columns: RFC 2045 caps a line at 78, and mail servers do
	// enforce it — an unwrapped base64 blob comes back as a bounce.
	enc := base64.StdEncoding.EncodeToString(a.Data)
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	if enc != "" {
		b.WriteString(enc + "\r\n")
	}
}
