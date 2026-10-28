package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// ScheduledDispatchWorker drains rows from outbound_scheduled whose
// send_at has fallen due, sends them via the channel adapter, and
// marks the row processed. Sits side-by-side with the campaign and
// outbox workers so each layer owns its own staging table:
// - campaigns_recipients: bulk broadcast (templates)
// - outbound_scheduled:   one-off "send tomorrow at 10am" intents
// - wa_webhook_events:     replay safety for inbound + AI reply
type ScheduledDispatchWorker struct {
	repo         domain.OutboundScheduledRepository
	channels     ScheduledChannelLookup
	messages     ScheduledMessageWriter
	providers    map[string]domain.MessageProvider
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	running      atomic.Bool
}

// ScheduledChannelLookup is the slim contract we need to load a
// channel before sending. Mirrors the existing CSATAskerChannelLookup.
type ScheduledChannelLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Channel, error)
}

// ScheduledMessageWriter persists the message row into messages once
// the send succeeded — keeps the conversation transcript accurate.
type ScheduledMessageWriter interface {
	Create(ctx context.Context, msg *domain.Message) (bool, error)
}

func NewScheduledDispatchWorker(
	repo domain.OutboundScheduledRepository,
	channels ScheduledChannelLookup,
	messages ScheduledMessageWriter,
	providers map[string]domain.MessageProvider,
) *ScheduledDispatchWorker {
	return &ScheduledDispatchWorker{
		repo:         repo,
		channels:     channels,
		messages:     messages,
		providers:    providers,
		pollInterval: 30 * time.Second,
		batchSize:    50,
		maxAttempts:  5,
	}
}

func (w *ScheduledDispatchWorker) WithPollInterval(d time.Duration) *ScheduledDispatchWorker {
	w.pollInterval = d
	return w
}

func (w *ScheduledDispatchWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("scheduled dispatch worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("scheduled dispatch worker started",
		"poll_interval", w.pollInterval, "batch_size", w.batchSize)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	recover := time.NewTicker(5 * time.Minute)
	defer recover.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduled dispatch worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		case <-recover.C:
			// Crash recovery: any row a replica claimed but never
			// finished (status='processing') gets flipped back to
			// 'pending' after 10 minutes. Threshold is far past the
			// 30s send timeout so a mid-flight worker isn't
			// hijacked.
			recovered, err := w.repo.RecoverStale(ctx, 10)
			if err != nil {
				slog.Warn("scheduled dispatch: stale recovery failed", "error", err)
			} else if recovered > 0 {
				slog.Info("scheduled dispatch: recovered stale processing rows", "count", recovered)
			}
		}
	}
}

func (w *ScheduledDispatchWorker) tick(ctx context.Context) {
	rows, err := w.repo.ListDue(ctx, w.batchSize)
	if err != nil {
		slog.Warn("scheduled dispatch: list due failed", "error", err)
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		w.dispatch(ctx, row)
	}
}

func (w *ScheduledDispatchWorker) dispatch(ctx context.Context, row *domain.OutboundScheduled) {
	channel, err := w.channels.GetByID(ctx, row.ChannelID)
	if err != nil || channel == nil {
		slog.Warn("scheduled dispatch: channel not found",
			"id", row.ID, "channel_id", row.ChannelID, "error", err)
		_ = w.repo.MarkFailed(ctx, row.ID, "channel not found")
		return
	}
	provider, ok := w.providers[channel.Type]
	if !ok {
		slog.Warn("scheduled dispatch: no provider for channel type",
			"id", row.ID, "channel_type", channel.Type)
		_ = w.repo.MarkFailed(ctx, row.ID, "no provider for channel type")
		return
	}
	var msg domain.Message
	if err := json.Unmarshal(row.Payload, &msg); err != nil {
		_ = w.repo.MarkFailed(ctx, row.ID, "payload decode: "+err.Error())
		return
	}
	// Generate the message ID up-front so the row + provider call
	// share it. Keeps audit trail consistent.
	msg.ID = ulid.Make().String()
	msg.ChannelID = channel.ID
	msg.ConversationID = row.ConversationID
	msg.Direction = "outbound"
	msg.From = "scheduler"
	msg.Timestamp = time.Now()

	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := provider.SendMessage(sendCtx, channel, row.Recipient, &msg); err != nil {
		// Retry on transient errors up to maxAttempts; mark final
		// after the cap. The dispatcher never reschedules send_at —
		// failed-final means ops or the user must re-enqueue.
		if row.Attempts+1 >= w.maxAttempts {
			_ = w.repo.MarkFailed(ctx, row.ID, err.Error())
			return
		}
		_ = w.repo.TouchAttempt(ctx, row.ID, err.Error())
		return
	}
	if w.messages != nil {
		if _, err := w.messages.Create(ctx, &msg); err != nil {
			slog.Warn("scheduled dispatch: persist message failed",
				"id", row.ID, "error", err)
		}
	}
	_ = w.repo.MarkSent(ctx, row.ID, msg.ID)
}
