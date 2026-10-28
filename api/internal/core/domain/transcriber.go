package domain

import (
	"context"
	"io"
)

// Transcriber converts speech audio into text. WhatsApp in Brazil is
// ~70-80% voice notes, so a bot that silently ignores audio reads as
// broken. Implementations should be best-effort: a failure returns an
// empty string + error, and the caller falls back to a polite "não
// consegui entender o áudio" message so the conversation keeps moving.
type Transcriber interface {
	// Transcribe reads the entire audio from r and returns the recognized
	// text. languageHint is an ISO-639-1 code (e.g. "pt", "en"); pass ""
	// when unknown and the provider will auto-detect (slower).
	Transcribe(ctx context.Context, r io.Reader, languageHint string) (string, error)
}
