package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// CSATAskerIdleFinder is the minimal repo port the worker uses to
// pull idle conversations. Matches AnalyticsRepository.ListIdleForCSAT.
type CSATAskerIdleFinder interface {
	ListIdleForCSAT(ctx context.Context, idleMinutes, limit int) ([]*domain.IdleConversationForCSAT, error)
}

// CSATAskerChannelLookup is how the worker resolves the provider
// adapter for each idle conversation. Matches ChannelRepository.GetByID.
type CSATAskerChannelLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Channel, error)
}

// CSATAskerMessageWriter persists the outbound survey message so the
// dashboard shows "bot asked for rating" in the transcript.
type CSATAskerMessageWriter interface {
	Create(ctx context.Context, msg *domain.Message) (bool, error)
}

// CSATAskerRepo persists the pending csat_ratings row that the
// customer's numeric reply closes out.
type CSATAskerRepo interface {
	CreatePending(ctx context.Context, r *domain.CSATRating) error
}

// CSATAskerWorker is a single-goroutine ticker that, every 5 minutes,
// drains a small batch of idle conversations and fires the CSAT
// prompt on each. Scope: one account-agnostic worker; per-account
// rate limits can be layered on later without touching this shape.
type CSATAskerWorker struct {
	finder    CSATAskerIdleFinder
	channels  CSATAskerChannelLookup
	messages  CSATAskerMessageWriter
	csat      CSATAskerRepo
	providers map[string]domain.MessageProvider

	idleMinutes  int
	batchSize    int
	pollInterval time.Duration

	running atomic.Bool
}

// csatCopyByLanguage holds the CSAT prompt per supported language.
// Fallback is pt_BR — detected languages outside this map (or empty
// on the conversation) read the default entry. Add entries when we
// add lingua-go language coverage in lang_detect.go.
var csatCopyByLanguage = map[string]string{
	"pt": "De 1 a 5, como foi o atendimento? Responda com um número.",
	"en": "On a scale of 1 to 5, how was the service? Reply with a number.",
	"es": "Del 1 al 5, ¿cómo fue la atención? Responde con un número.",
}

func csatCopy(lang string) string {
	if c, ok := csatCopyByLanguage[lang]; ok {
		return c
	}
	return csatCopyByLanguage["pt"]
}

// DefaultCSATAskerConfig matches the Mission 6 spec: 30 min idle
// threshold, 5 min poll cadence, 100-row batch cap. askCopy is
// portuguese by default; if we ever add per-account branding the
// copy moves into domain.Account.
func NewCSATAskerWorker(
	finder CSATAskerIdleFinder,
	channels CSATAskerChannelLookup,
	messages CSATAskerMessageWriter,
	csat CSATAskerRepo,
	providers map[string]domain.MessageProvider,
) *CSATAskerWorker {
	return &CSATAskerWorker{
		finder:       finder,
		channels:     channels,
		messages:     messages,
		csat:         csat,
		providers:    providers,
		idleMinutes:  30,
		batchSize:    100,
		pollInterval: 5 * time.Minute,
	}
}

// WithIdleMinutes lets tests run the worker with a 1-minute idle
// window so they can verify the happy path in seconds, not minutes.
func (w *CSATAskerWorker) WithIdleMinutes(m int) *CSATAskerWorker {
	w.idleMinutes = m
	return w
}

// WithPollInterval lets tests skip the 5-min ticker by setting a
// millisecond-scale interval. Production leaves the default.
func (w *CSATAskerWorker) WithPollInterval(d time.Duration) *CSATAskerWorker {
	w.pollInterval = d
	return w
}

// Start runs the poll loop until ctx is cancelled. Safe to call once;
// subsequent calls are no-ops to prevent accidental double-start.
func (w *CSATAskerWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("csat asker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)

	slog.Info("csat asker started",
		"poll_interval", w.pollInterval,
		"idle_minutes", w.idleMinutes,
		"batch_size", w.batchSize,
	)
	// One tick immediately — deploys that landed while the idle
	// window filled up don't have to wait for the first interval.
	w.tick(ctx)

	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("csat asker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *CSATAskerWorker) tick(ctx context.Context) {
	rows, err := w.finder.ListIdleForCSAT(ctx, w.idleMinutes, w.batchSize)
	if err != nil {
		slog.Error("csat asker: list idle failed", "error", err)
		return
	}
	for _, r := range rows {
		if ctx.Err() != nil {
			return
		}
		if err := w.askOne(ctx, r); err != nil {
			// Per-row failures are logged, never fatal. A hung Meta
			// upstream shouldn't lock out the next conversation.
			slog.Warn("csat asker: ask failed",
				"conversation_id", r.ConversationID, "error", err)
		}
	}
}

// askOne creates the pending csat row, persists the prompt message,
// and dispatches it through the conversation's channel provider.
// Order matters: the csat_ratings row is written FIRST so a later
// inbound "5" is intercepted as the answer even if the provider
// send is slow or retried.
func (w *CSATAskerWorker) askOne(ctx context.Context, r *domain.IdleConversationForCSAT) error {
	now := time.Now()
	pending := &domain.CSATRating{
		ID:             ulid.Make().String(),
		AccountID:      r.AccountID,
		ConversationID: r.ConversationID,
		ContactID:      r.ContactID,
		Rating:         0,
		SentAt:         &now,
		CreatedAt:      now,
	}
	if err := w.csat.CreatePending(ctx, pending); err != nil {
		return err
	}

	msgID := ulid.Make().String()
	content := csatCopy(r.Language)
	botName := "Replider"
	msg := &domain.Message{
		ID:             msgID,
		ChannelID:      r.ChannelID,
		ConversationID: &r.ConversationID,
		From:           "bot",
		SenderName:     &botName,
		Type:           "text",
		Content:        &content,
		Direction:      "outbound",
		Timestamp:      now,
	}
	if _, err := w.messages.Create(ctx, msg); err != nil {
		// Message persist failure is still recoverable — the csat row
		// is there and the next poll will retry the send. Return the
		// error so the caller logs it.
		return err
	}

	channel, err := w.channels.GetByID(ctx, r.ChannelID)
	if err != nil || channel == nil {
		return err
	}
	provider, ok := w.providers[channel.Type]
	if !ok {
		slog.Warn("csat asker: no provider for channel type",
			"channel_type", channel.Type)
		return nil
	}
	return provider.SendMessage(ctx, channel, r.ContactID, msg)
}
