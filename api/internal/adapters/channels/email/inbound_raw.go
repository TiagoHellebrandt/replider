package email

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
)

// Parsing a message read straight off a mailbox (IMAP inbound).
//
// The webhook parser next door consumes JSON a provider already decoded for us.
// Here there is no provider: the bytes are RFC 5322 as the sending server wrote
// them, and everything the pipeline needs — the reply headers that keep a thread
// together, the attachments, the Authentication-Results the CUSTOMER's own
// server stamped on arrival — has to be lifted out of the MIME.
//
// It produces the SAME InboundEmail as the webhook path on purpose. Everything
// downstream (identity resolution, impersonation checks, triage, attachments)
// already works against that struct, and a second shape would mean a second set
// of rules that drift.
//
// A note on the auth verdict, because it is the one thing IMAP does BETTER than
// a forwarding bridge: the customer's server evaluated SPF/DKIM/DMARC against
// the real sending IP and wrote its conclusion into the message. Reading the
// mailbox preserves that. Forwarding a copy to a third party does not — the
// forward re-sends from the customer's own IP, SPF fails, and every legitimate
// reply arrives looking forged.

// maxRawTextBytes caps the text we lift out of one message. A reply quoting a
// thread with an inline image chain can carry megabytes of body; past this the
// text is truncated rather than the message dropped, because the useful part of
// a reply is always at the top.
const maxRawTextBytes = 256 << 10

// ErrRawEmailUnparseable means the bytes were not a message we could read at
// all. Distinct from an empty body: one is a broken fetch, the other is a real
// message somebody sent with no text.
var ErrRawEmailUnparseable = errors.New("email: raw message could not be parsed")

// ParseRawInbound turns a full RFC 5322 message into the normalized shape.
func ParseRawInbound(raw []byte) (InboundEmail, error) {
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		return InboundEmail{}, ErrRawEmailUnparseable
	}

	var out InboundEmail
	header := msg.Header

	out.From, out.FromName = splitAddress(decodeHeader(header.Get("From")))
	if out.From == "" {
		// Nothing to attribute the message to. The pipeline keys a contact on
		// the sender address, so a message with no readable From has no home —
		// better refused here than filed under an empty identity.
		return InboundEmail{}, ErrRawEmailUnparseable
	}
	out.To, _ = splitAddress(decodeHeader(header.Get("To")))
	out.Subject = decodeHeader(header.Get("Subject"))
	out.MessageID = strings.TrimSpace(header.Get("Message-Id"))
	out.InReplyTo = strings.TrimSpace(header.Get("In-Reply-To"))
	out.References = strings.TrimSpace(header.Get("References"))
	out.ReplyTo, _ = splitAddress(decodeHeader(header.Get("Reply-To")))
	out.ListUnsubscribe = strings.TrimSpace(header.Get("List-Unsubscribe")) != "" ||
		strings.TrimSpace(header.Get("List-Id")) != ""

	// The auto-reply markers, exactly as written. IMAP is the one path that
	// sees them: a forwarding provider strips most of them, and without them
	// an out-of-office lands in the pipeline as a reply from a person.
	out.AutoSubmitted = strings.TrimSpace(header.Get("Auto-Submitted"))
	out.Precedence = strings.TrimSpace(header.Get("Precedence"))
	out.HasAutoReplyHeader = header.Get("X-Autoreply") != "" ||
		header.Get("X-Autorespond") != "" ||
		header.Get("X-Auto-Response-Suppress") != ""

	// The verdict the receiving server reached. Vendor fields are empty here:
	// there is no vendor, only the header the customer's own MX wrote.
	out.Auth = ParseAuthResults(header.Get("Authentication-Results"), nil)

	text, htmlBody, attachments := walkParts(
		msg.Body,
		header.Get("Content-Type"),
		header.Get("Content-Transfer-Encoding"),
		header.Get("Content-Disposition"),
		header.Get("Content-Id"),
		0,
	)

	if strings.TrimSpace(text) != "" {
		out.Body = normalizeText(text)
	} else if strings.TrimSpace(htmlBody) != "" {
		out.Body = sanitizeToText(htmlBody)
		out.HadHTMLOnly = true
	}
	out.HTML = htmlBody
	out.Attachments = attachments
	return out, nil
}

// maxMIMEDepth bounds nesting. A crafted message can nest multipart parts
// thousands deep; each level costs a reader and a buffer, and no real mail goes
// past a handful (mixed → alternative → related is three).
const maxMIMEDepth = 10

// walkParts descends the MIME tree, collecting the best text, the HTML, and
// every attachment.
//
// "Best text" means the FIRST text/plain found, not the concatenation of all of
// them: a reply quoting the original produces several text parts and joining
// them yields the customer's answer glued to a copy of our own message.
func walkParts(
	body io.Reader,
	contentType, encoding, disposition, contentID string,
	depth int,
) (text, htmlBody string, attachments []InboundAttachment) {
	if depth > maxMIMEDepth {
		return "", "", nil
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// No parseable Content-Type: the RFC default, and what a plain
		// hand-written message looks like.
		mediaType = "text/plain"
		params = map[string]string{}
	}
	mediaType = strings.ToLower(mediaType)

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", "", nil
		}
		reader := multipart.NewReader(body, boundary)
		for {
			part, err := reader.NextPart()
			if err != nil {
				// io.EOF, and also a truncated message: whatever was read
				// before the break is still the customer's reply.
				break
			}
			t, h, atts := walkParts(
				part,
				part.Header.Get("Content-Type"),
				part.Header.Get("Content-Transfer-Encoding"),
				part.Header.Get("Content-Disposition"),
				part.Header.Get("Content-Id"),
				depth+1,
			)
			// First text/plain wins; first HTML wins. Later ones are quoted
			// history or an alternative rendering of what we already have.
			if text == "" {
				text = t
			}
			if htmlBody == "" {
				htmlBody = h
			}
			attachments = append(attachments, atts...)
			_ = part.Close()
		}
		return text, htmlBody, attachments
	}

	filename := attachmentFilename(disposition, params)
	dispLower := strings.ToLower(strings.TrimSpace(disposition))
	hasContentID := strings.TrimSpace(contentID) != ""
	// Inline means "furniture the HTML renders", and the flag drives the UI's
	// attachment counter — so it follows the DISPOSITION, plus the one shape
	// some MUAs write without one: a cid:-referenced image inside
	// multipart/related. A Content-ID alone must NOT make a part inline:
	// Gmail stamps one on regular attachments too, and a contract PDF
	// flagged 'inline' vanishes from the attachment count.
	isImage := strings.HasPrefix(mediaType, "image/")
	inline := strings.HasPrefix(dispLower, "inline") || (hasContentID && isImage)
	isAttachment := filename != "" || inline || hasContentID

	// A named part is a file even when its type is text/plain — that is exactly
	// how a .csv or a .txt attachment arrives, and treating it as the body would
	// replace the customer's reply with the contents of their spreadsheet.
	isBodyText := filename == "" &&
		(mediaType == "text/plain" || mediaType == "text/html") &&
		!strings.HasPrefix(dispLower, "attachment")

	// The read cap depends on what the part IS. maxRawTextBytes exists to
	// stop a quoted-thread body from ballooning — applied to attachments it
	// silently dropped every phone photo (>256KB): no row, no log, and the
	// email view had nothing to render.
	limit := maxRawTextBytes
	if !isBodyText && isAttachment {
		limit = maxAttachmentPartBytes
	}
	decoded, err := decodePart(body, encoding, limit)
	if err != nil {
		// Loud: a dropped part used to be indistinguishable from a message
		// that never had one.
		slog.Warn("email inbound: part dropped",
			"filename", filename, "content_type", mediaType, "error", err)
		return "", "", nil
	}

	if isBodyText {
		content := decodeCharset(decoded, params["charset"])
		if mediaType == "text/html" {
			return "", content, nil
		}
		return content, "", nil
	}

	if !isAttachment {
		// An unnamed non-text part with no disposition and no id: nothing to
		// show and nothing to call it.
		return "", "", nil
	}
	att := InboundAttachment{
		Filename:    filename,
		ContentType: mediaType,
		Content:     decoded,
		Size:        int64(len(decoded)),
		Inline:      inline,
	}
	if hasContentID {
		att.ContentID = strings.Trim(strings.TrimSpace(contentID), "<>")
	}
	return "", "", []InboundAttachment{att}
}

// maxAttachmentPartBytes caps one decoded attachment. Matches the storage
// layer's MaxAttachmentBytes (25MB) — the IMAP fetch already bounds the whole
// message at 15MB, so this is a consistency ceiling, not the working one.
const maxAttachmentPartBytes = 25 << 20

// attachmentFilename reads the name from either place senders put it.
func attachmentFilename(disposition string, typeParams map[string]string) string {
	if disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if name := decodeHeader(strings.TrimSpace(params["filename"])); name != "" {
				return name
			}
		}
	}
	// Legacy senders put it on Content-Type instead of Content-Disposition.
	return decodeHeader(strings.TrimSpace(typeParams["name"]))
}

// decodePart reads a part's bytes, undoing the transfer encoding.
func decodePart(body io.Reader, encoding string, limit int) ([]byte, error) {
	reader := transferDecoder(body, encoding)
	return readAtMost(reader, limit)
}

// decodeHeader undoes RFC 2047 encoded-words, so an accented Portuguese subject
// arrives as "Orçamento" rather than "=?utf-8?B?T3Jj...?=". A header we cannot
// decode is returned as-is: the raw form is ugly but readable, and dropping it
// would lose the subject entirely.
func decodeHeader(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	decoder := new(mime.WordDecoder)
	decoder.CharsetReader = charsetReader
	if decoded, err := decoder.DecodeHeader(v); err == nil {
		return strings.TrimSpace(decoded)
	}
	return v
}
