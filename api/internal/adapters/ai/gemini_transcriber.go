package ai

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiTranscriber uses Gemini multimodal audio input to produce a
// transcript. Exists as a fallback for WhisperTranscriber: when the
// OpenAI account hits its quota, Whisper returns 429 insufficient_quota
// and every voice note in the UI fails with a generic error. Gemini has
// native audio support on gemini-2.5-flash, so the bot can keep
// transcribing while we top up.
type GeminiTranscriber struct {
	client *genai.Client
	model  string
}

func NewGeminiTranscriber(ctx context.Context, apiKey string, modelName string) (*GeminiTranscriber, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}
	return &GeminiTranscriber{client: client, model: modelName}, nil
}

// Transcribe uploads the audio as inline data and asks Gemini for the
// transcript. Prompt is deliberately strict ("transcript only, no
// commentary") so the output drops straight into the chat log without
// a "Here's the transcript:" preamble.
func (g *GeminiTranscriber) Transcribe(ctx context.Context, r io.Reader, languageHint string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read audio: %w", err)
	}
	_, mimeType := detectAudioFormat(data)

	model := g.client.GenerativeModel(g.model)

	prompt := "Transcribe this audio verbatim. Return ONLY the transcript text, with no preamble, no quotes, and no commentary."
	if languageHint != "" {
		prompt += fmt.Sprintf(" The spoken language is %q.", languageHint)
	}

	resp, err := model.GenerateContent(ctx,
		genai.Text(prompt),
		genai.Blob{MIMEType: mimeType, Data: data},
	)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", nil
	}

	var b strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			b.WriteString(string(t))
		}
	}
	return strings.TrimSpace(b.String()), nil
}
