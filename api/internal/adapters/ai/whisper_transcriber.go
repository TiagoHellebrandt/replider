package ai

import (
	"bytes"
	"context"
	"fmt"
	"io"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

// WhisperTranscriber uses OpenAI's whisper-1 model for speech-to-text.
// Chosen over Google STT because we already wire up OpenAI for chat and
// Whisper is cheap (~$0.006/min) and dominant for Portuguese accuracy on
// noisy WhatsApp voice notes.
type WhisperTranscriber struct {
	client *openai.Client
	model  openai.AudioModel
}

func NewWhisperTranscriber(apiKey string) *WhisperTranscriber {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &WhisperTranscriber{
		client: &client,
		model:  openai.AudioModelWhisper1,
	}
}

// namedReader satisfies the openai-go multipart encoder's optional
// Filename()/ContentType() interfaces. Without these the SDK ships the
// upload as `filename="anonymous_file"` with `application/octet-stream`,
// and Whisper rejects it ("Invalid file format. Supported formats are
// mp3, mp4, m4a, ogg, wav, ..."), surfacing as a 400 from the SDK and
// our handler wrapping it into a 502.
type namedReader struct {
	*bytes.Reader
	name string
	ct   string
}

func (n *namedReader) Filename() string    { return n.name }
func (n *namedReader) ContentType() string { return n.ct }

// Transcribe uploads the audio to OpenAI and returns the transcript. The
// hint speeds up recognition and avoids false-positive detection of a
// different language on short or low-quality clips.
func (w *WhisperTranscriber) Transcribe(ctx context.Context, r io.Reader, languageHint string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read audio: %w", err)
	}
	filename, contentType := detectAudioFormat(data)
	params := openai.AudioTranscriptionNewParams{
		File:  &namedReader{Reader: bytes.NewReader(data), name: filename, ct: contentType},
		Model: w.model,
	}
	if languageHint != "" {
		params.Language = param.NewOpt(languageHint)
	}
	resp, err := w.client.Audio.Transcriptions.New(ctx, params)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// detectAudioFormat sniffs the magic bytes so Whisper sees the right
// filename extension. Browser MediaRecorder ships AAC-in-MP4 (.m4a),
// WhatsApp ships Opus-in-Ogg (.ogg), and Telegram ships Opus-in-Ogg
// too — without the right extension on the multipart upload, Whisper
// 400s on otherwise-valid audio.
func detectAudioFormat(b []byte) (filename, contentType string) {
	switch {
	case len(b) >= 4 && string(b[:4]) == "OggS":
		return "audio.ogg", "audio/ogg"
	case len(b) >= 8 && string(b[4:8]) == "ftyp":
		return "audio.m4a", "audio/mp4"
	case len(b) >= 4 && string(b[:4]) == "RIFF":
		return "audio.wav", "audio/wav"
	case len(b) >= 4 && string(b[:4]) == "fLaC":
		return "audio.flac", "audio/flac"
	case len(b) >= 4 && b[0] == 0x1A && b[1] == 0x45 && b[2] == 0xDF && b[3] == 0xA3:
		return "audio.webm", "audio/webm"
	case len(b) >= 3 && string(b[:3]) == "ID3":
		return "audio.mp3", "audio/mpeg"
	case len(b) >= 2 && b[0] == 0xFF && (b[1]&0xE0) == 0xE0:
		return "audio.mp3", "audio/mpeg"
	default:
		return "audio.m4a", "audio/mp4"
	}
}
