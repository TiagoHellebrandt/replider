package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/replider/api/internal/core/domain"
)

var logRecordsDropped = promauto.NewCounter(prometheus.CounterOpts{
	Name: "log_records_dropped_total",
	Help: "Log records dropped by the recorder because its buffer was full.",
})

// RecordingHandler keeps a copy of severe records in the database, so production
// can be investigated without shelling into the machine.
//
// Three rules define the design:
//
//  1. NEVER on the request path. Writing a log inside the handler would turn
//     database slowness into product slowness — and a log that delays the
//     response is worse than no log. Records go onto a channel and a separate
//     writer batches them.
//
//  2. Drops are counted, never silent. If the channel fills, the line is thrown
//     away and log_records_dropped_total goes up. A log table with holes nobody
//     knows about is worse than an empty one: absence becomes proof that
//     "nothing happened".
//
//  3. It is a transparent decorator. PII redaction, formatting and stdout stay
//     with the handler underneath — this one only observes.
type RecordingHandler struct {
	inner slog.Handler
	// minLevel is the threshold worth storing from. INFO in production would
	// drown the table, and is not what anyone looks for when something broke.
	minLevel slog.Level
	ch       chan *domain.AppLog
	attrs    []slog.Attr
}

// NewRecordingHandler wraps inner and starts copying records of minLevel and
// above. `buffer` is how many lines fit waiting to be written.
func NewRecordingHandler(inner slog.Handler, minLevel slog.Level, buffer int) *RecordingHandler {
	if buffer <= 0 {
		buffer = 1024
	}
	return &RecordingHandler{
		inner:    inner,
		minLevel: minLevel,
		ch:       make(chan *domain.AppLog, buffer),
	}
}

func (h *RecordingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *RecordingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.minLevel {
		select {
		case h.ch <- h.toAppLog(r):
		default:
			// Full: the line is lost, and that shows up in the metric. What must
			// not happen is the request waiting on a log.
			logRecordsDropped.Inc()
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *RecordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	next = append(next, h.attrs...)
	next = append(next, attrs...)
	return &RecordingHandler{
		inner:    h.inner.WithAttrs(attrs),
		minLevel: h.minLevel,
		ch:       h.ch,
		attrs:    next,
	}
}

func (h *RecordingHandler) WithGroup(name string) slog.Handler {
	return &RecordingHandler{
		inner:    h.inner.WithGroup(name),
		minLevel: h.minLevel,
		ch:       h.ch,
		attrs:    h.attrs,
	}
}

// toAppLog flattens the record. Attributes go through the SAME PII redaction as
// the stdout handler — the table must not become the back door through which the
// personal data the log does not write ends up written.
func (h *RecordingHandler) toAppLog(r slog.Record) *domain.AppLog {
	out := &domain.AppLog{
		At:    r.Time,
		Level: r.Level.String(),
		Msg:   r.Message,
	}
	if out.At.IsZero() {
		out.At = time.Now()
	}

	fields := map[string]any{}
	take := func(a slog.Attr) {
		red := RedactPII(nil, a)
		key := red.Key
		val := red.Value.Any()
		switch key {
		case "request_id":
			out.RequestID = asString(val)
		case "account_id":
			out.AccountID = asString(val)
		case "path":
			out.Path = asString(val)
		case "code":
			out.Code = asString(val)
		}
		fields[key] = val
	}
	for _, a := range h.attrs {
		take(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		take(a)
		return true
	})

	if len(fields) > 0 {
		if b, err := json.Marshal(fields); err == nil {
			out.Attrs = b
		}
	}
	return out
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// LogWriter drena o canal e grava em lote.
type LogWriter struct {
	repo     domain.AppLogRepository
	h        *RecordingHandler
	interval time.Duration
	batch    int
	once     sync.Once
}

// NewLogWriter liga o coletor ao banco. `interval` é de quanto em quanto tempo
// the partial batch is written, so an isolated failure does not sit waiting for
// the batch to fill — the line that matters is usually the only one.
func NewLogWriter(repo domain.AppLogRepository, h *RecordingHandler,
	interval time.Duration, batch int) *LogWriter {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	if batch <= 0 {
		batch = 100
	}
	return &LogWriter{repo: repo, h: h, interval: interval, batch: batch}
}

// Start runs until the context ends. Writes whatever is left before leaving.
func (w *LogWriter) Start(ctx context.Context) {
	w.once.Do(func() { go w.loop(ctx) })
}

func (w *LogWriter) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	pending := make([]*domain.AppLog, 0, w.batch)
	flush := func() {
		if len(pending) == 0 {
			return
		}
		// Its own context: the server's may already be cancelled during shutdown,
		// and that is exactly when the last errors matter.
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := w.repo.InsertBatch(writeCtx, pending); err != nil {
			// No slog here: reporting a log-write failure through the log itself
			// is a loop. It goes to the drop metric, which is where anyone looks
			// to learn the table is incomplete.
			logRecordsDropped.Add(float64(len(pending)))
		}
		pending = pending[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case rec := <-w.h.ch:
			pending = append(pending, rec)
			if len(pending) >= w.batch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
