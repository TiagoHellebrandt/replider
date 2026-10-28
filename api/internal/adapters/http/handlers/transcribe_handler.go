package handlers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// TranscribeHandler exposes POST /messages/:id/transcribe. On first call
// it downloads the audio (GCS when the media_url is an object path,
// direct HTTP otherwise), runs the configured Transcriber (Whisper by
// default), caches the result on the messages row, and returns the text.
// Subsequent calls short-circuit from the cached column — so a page
// refresh doesn't re-bill OpenAI.
type TranscribeHandler struct {
	msgRepo     domain.MessageRepository
	gcsAdapter  *storage.GCSAdapter
	transcriber domain.Transcriber
	usageRepo   domain.UsageRepository
}

func NewTranscribeHandler(
	msgRepo domain.MessageRepository,
	gcsAdapter *storage.GCSAdapter,
	transcriber domain.Transcriber,
	usageRepo domain.UsageRepository,
) *TranscribeHandler {
	return &TranscribeHandler{
		msgRepo:     msgRepo,
		gcsAdapter:  gcsAdapter,
		transcriber: transcriber,
		usageRepo:   usageRepo,
	}
}

type transcribeResponse struct {
	MessageID  string `json:"message_id"`
	Transcript string `json:"transcript"`
	Cached     bool   `json:"cached"`
}

// HandleTranscribe godoc
// @Summary      Transcribe an audio/voice message
// @Description  Runs the cached transcriber over the message's media and
// @Description  returns the text. Subsequent calls hit a cached result.
// @Tags         messages
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Message ID"
// @Success      200  {object}  transcribeResponse
// @Failure      404  {object}  domain.AppError
// @Failure      422  {object}  domain.AppError  "non-audio message"
// @Failure      503  {object}  domain.AppError  "transcriber not configured"
// @Router       /messages/{id}/transcribe [post]
func (h *TranscribeHandler) HandleTranscribe(c *gin.Context) {
	if h.transcriber == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "TRANSCRIBER_DISABLED",
			Message: "audio transcription not configured on this server",
		})
		return
	}

	accountID := getAccountID(c)
	messageID := c.Param("id")

	msg, err := h.msgRepo.GetByID(c.Request.Context(), messageID, accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	if msg == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "MESSAGE_NOT_FOUND", Message: "message not found"})
		return
	}
	if msg.Type != "audio" && msg.Type != "voice" {
		c.JSON(http.StatusUnprocessableEntity, domain.AppError{
			Code:    "NOT_AUDIO",
			Message: "only audio/voice messages can be transcribed",
		})
		return
	}
	if msg.MediaURL == nil || *msg.MediaURL == "" {
		c.JSON(http.StatusUnprocessableEntity, domain.AppError{
			Code:    "NO_MEDIA_URL",
			Message: "message has no media URL",
		})
		return
	}

	// Cache short-circuit. A transcriber that legitimately returned ""
	// (silent clip) is still a valid answer — we won't re-run it.
	if msg.Transcript != nil {
		c.JSON(http.StatusOK, transcribeResponse{
			MessageID:  msg.ID,
			Transcript: *msg.Transcript,
			Cached:     true,
		})
		return
	}

	audioBytes, err := h.fetchAudio(c.Request.Context(), *msg.MediaURL)
	if err != nil {
		slog.Error("transcribe: failed to fetch audio", "message_id", messageID, "error", err)
		c.JSON(http.StatusBadGateway, domain.AppError{
			Code:    "MEDIA_FETCH_FAILED",
			Message: "failed to fetch audio bytes: " + err.Error(),
		})
		return
	}

	// The account rides the context so the BYOK transcriber picks the
	// customer's own key.
	transcribeCtx, cancel := context.WithTimeout(services.WithAIAccount(c.Request.Context(), accountID), 25*time.Second)
	defer cancel()
	text, err := h.transcriber.Transcribe(transcribeCtx, bytes.NewReader(audioBytes), "pt")
	if err != nil {
		slog.Error("transcribe: provider failed", "message_id", messageID, "error", err)
		c.JSON(http.StatusBadGateway, domain.AppError{
			Code:    "TRANSCRIBE_FAILED",
			Message: "transcription provider failed: " + err.Error(),
		})
		return
	}

	// Cache on best-effort: a failure to cache must not break the response.
	if err := h.msgRepo.SetTranscript(c.Request.Context(), msg.ID, text); err != nil {
		slog.Warn("transcribe: failed to cache transcript", "message_id", messageID, "error", err)
	}

	// Bill the account. Transcription providers don't all return token
	// counts (Whisper has none, Gemini audio does but we hide it behind
	// the common interface), so approximate: audio bytes → input
	// tokens via a fixed bps→tokens ratio, transcript text → output
	// tokens at the standard chars/4 rate. Total flows into
	// ai_tokens_used same as chat replies.
	go func(acctID string, audioLen int, transcript string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// ~80 kbps AAC → ~10KB/s; 1 token per ~500 bytes gives a
		// conservative proxy (roughly 20 tokens per second of audio).
		inputTokens := int64(audioLen) / 500
		outputTokens := int64(len([]rune(transcript))) / 4
		total := inputTokens + outputTokens
		if err := h.usageRepo.AddUsage(ctx, acctID, total, inputTokens, outputTokens, 0); err != nil {
			slog.Error("transcribe: failed to increment usage",
				"account_id", acctID, "tokens", total, "error", err)
		}
	}(accountID, len(audioBytes), text)

	c.JSON(http.StatusOK, transcribeResponse{
		MessageID:  msg.ID,
		Transcript: text,
		Cached:     false,
	})
}

// fetchAudio resolves the message's media_url to bytes. GCS object paths
// go through the GCS SDK directly (no network hop to our own signed URL),
// external URLs go through an HTTP GET with a size cap.
//
// The GET uses httpsafe.Client, never a bare http.Client: a stored media_url
// is caller-influenced (an attendant's reply, a CDN link a channel reported)
// and the bytes come back to the caller in the transcript, which makes this
// an SSRF read primitive without the guard. The guard refuses loopback,
// private, link-local and metadata addresses at dial time, after DNS.
func (h *TranscribeHandler) fetchAudio(ctx context.Context, mediaURL string) ([]byte, error) {
	const maxBytes int64 = 25 * 1024 * 1024 // OpenAI Whisper cap is 25MB
	if storage.IsGCSPath(mediaURL) && h.gcsAdapter != nil {
		return h.gcsAdapter.DownloadObject(ctx, mediaURL, maxBytes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpsafe.Client(20 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, httpFetchError(resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string { return http.StatusText(e.code) }

func httpFetchError(code int) error { return &httpStatusError{code: code} }
