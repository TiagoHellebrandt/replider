package email

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime/quotedprintable"
	"strings"
	"unicode/utf8"
)

// Transfer and charset decoding for messages read off a mailbox.
//
// Kept small and separate because every function here has the same job: turn
// bytes a stranger's mail client produced into something we can store without
// corrupting it, and NEVER fail the whole message over a part we could not
// decode. A reply that arrives mangled is a support conversation; a reply that
// never arrives is a lost deal.

// ErrTooLarge means the reader carried more than the caller allowed.
var ErrTooLarge = errors.New("email: content exceeds the allowed size")

// readAtMost reads up to limit bytes, refusing anything larger.
//
// Reads limit+1 so "exactly at the limit" is a success rather than a guess. The
// cap exists because both callers read from a network socket a stranger controls:
// a message can claim any size, and a poller that reads whatever arrives is one
// oversized attachment away from taking the process down.
func readAtMost(r io.Reader, limit int) ([]byte, error) {
	if limit <= 0 {
		return nil, ErrTooLarge
	}
	buf, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(buf) > limit {
		return nil, ErrTooLarge
	}
	return buf, nil
}

// transferDecoder wraps a part's body in the decoder its encoding calls for.
//
// An unknown encoding passes through undecoded rather than erroring: 7bit, 8bit
// and binary are all "already decoded", and a sender who invents a value is
// more likely to have sent plain text than something we should discard.
func transferDecoder(body io.Reader, encoding string) io.Reader {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		// Mail folds base64 at 76 columns, and some senders use CRLF while
		// others use bare LF; the standard decoder rejects both as invalid
		// characters, so whitespace is stripped before decoding.
		return base64.NewDecoder(base64.StdEncoding, &whitespaceStripper{r: body})
	case "quoted-printable":
		return quotedprintable.NewReader(body)
	default:
		return body
	}
}

// whitespaceStripper drops CR, LF, spaces and tabs, which is what separates a
// wrapped base64 body from a decodable one.
type whitespaceStripper struct{ r io.Reader }

func (w *whitespaceStripper) Read(p []byte) (int, error) {
	n, err := w.r.Read(p)
	if n == 0 {
		return 0, err
	}
	out := p[:0]
	for _, b := range p[:n] {
		switch b {
		case '\r', '\n', ' ', '\t':
			continue
		default:
			out = append(out, b)
		}
	}
	// A chunk that was ALL whitespace returns (0, nil), which io.Reader callers
	// are allowed to see but some treat as EOF. Recurse so the next chunk is
	// read instead, unless the underlying reader is finished.
	if len(out) == 0 && err == nil {
		return w.Read(p)
	}
	return len(out), err
}

// decodeCharset turns a part's bytes into a UTF-8 string.
//
// Bodies that are already valid UTF-8 pass straight through, which covers
// essentially all modern mail. The one legacy encoding worth handling is
// ISO-8859-1 / Windows-1252, still emitted by older Brazilian mail clients and
// exactly where accented Portuguese lives — decoded byte-by-byte because in
// Latin-1 every byte IS its own code point, so no table is needed.
func decodeCharset(content []byte, charset string) string {
	switch normalizeCharset(charset) {
	case "", "utf-8", "us-ascii", "ascii":
		if utf8.Valid(content) {
			return string(content)
		}
		// Declared UTF-8 and is not: fall through to Latin-1, which at least
		// yields readable text instead of replacement characters.
		return latin1ToUTF8(content)
	case "iso-8859-1", "iso8859-1", "latin1", "windows-1252", "cp1252":
		return latin1ToUTF8(content)
	default:
		// An encoding we do not know. Valid UTF-8 is kept; anything else gets
		// the Latin-1 reading, which is wrong for CJK but right for the
		// single-byte encodings this product actually meets.
		if utf8.Valid(content) {
			return string(content)
		}
		return latin1ToUTF8(content)
	}
}

func normalizeCharset(charset string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(charset), `"`))
}

func latin1ToUTF8(content []byte) string {
	var b strings.Builder
	b.Grow(len(content))
	for _, c := range content {
		b.WriteRune(rune(c))
	}
	return b.String()
}

// charsetReader lets the header decoder handle non-UTF-8 encoded-words, which is
// how a subject line written in Windows-1252 survives.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	raw, err := io.ReadAll(io.LimitReader(input, 64<<10))
	if err != nil {
		return nil, err
	}
	return bytes.NewReader([]byte(decodeCharset(raw, charset))), nil
}
