package ai

import (
	"bytes"
	"context"
	"io"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// FallbackTranscriber tries the primary transcriber first and, on error,
// retries the same audio against a secondary. Motivation: Whisper is our
// default (best quality for PT-BR WhatsApp notes) but its OpenAI quota
// can be exhausted mid-day, and we'd rather hit Gemini than ghost the
// user's voice note. Secondary failures surface the ORIGINAL error so
// ops see the primary cause (quota/auth) instead of the fallback's
// generic "no audio support" noise.
type FallbackTranscriber struct {
	primary   domain.Transcriber
	secondary domain.Transcriber
}

func NewFallbackTranscriber(primary, secondary domain.Transcriber) *FallbackTranscriber {
	return &FallbackTranscriber{primary: primary, secondary: secondary}
}

func (f *FallbackTranscriber) Transcribe(ctx context.Context, r io.Reader, languageHint string) (string, error) {
	// Buffer once so we can hand the same bytes to the secondary on
	// failure — the caller's reader is single-use (GCS SDK stream,
	// http body, etc.).
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	text, err := f.primary.Transcribe(ctx, bytes.NewReader(data), languageHint)
	if err == nil {
		return text, nil
	}

	slog.Warn("transcribe: primary failed, trying secondary",
		"primary_error", err.Error())

	text2, err2 := f.secondary.Transcribe(ctx, bytes.NewReader(data), languageHint)
	if err2 != nil {
		// Return the PRIMARY error — it's the signal that matters
		// (quota, auth) even though the secondary tried too.
		return "", err
	}
	return text2, nil
}
