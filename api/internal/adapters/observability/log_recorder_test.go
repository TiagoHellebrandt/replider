package observability

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type collectingRepo struct {
	mu   sync.Mutex
	logs []*domain.AppLog
}

func (c *collectingRepo) InsertBatch(_ context.Context, l []*domain.AppLog) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, l...)
	return nil
}
func (c *collectingRepo) Query(context.Context, domain.AppLogFilter) ([]*domain.AppLog, error) {
	return nil, nil
}
func (c *collectingRepo) Summarize(context.Context, time.Time, time.Time, int) ([]*domain.AppLogGroup, error) {
	return nil, nil
}
func (c *collectingRepo) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (c *collectingRepo) snapshot() []*domain.AppLog {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*domain.AppLog, len(c.logs))
	copy(out, c.logs)
	return out
}

func discardHandler() slog.Handler {
	return slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: RedactPII,
	})
}

// What gets stored is what serves a diagnosis. INFO at production volume would
// drown the table — and is not what anyone looks for when something broke.
func TestRecordingHandler_KeepsOnlyWhatDiagnoses(t *testing.T) {
	h := NewRecordingHandler(discardHandler(), slog.LevelWarn, 16)
	log := slog.New(h)

	log.Info("rotina normal")
	log.Warn("algo estranho")
	log.Error("quebrou")

	repo := &collectingRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	NewLogWriter(repo, h, 5*time.Millisecond, 10).Start(ctx)
	time.Sleep(60 * time.Millisecond)
	cancel()

	got := repo.snapshot()
	require.Len(t, got, 2, "INFO não entra")
	assert.Equal(t, "WARN", got[0].Level)
	assert.Equal(t, "ERROR", got[1].Level)
}

// The fields an investigation goes through are promoted to columns: that is how
// the error the customer saw gets tied to the server's record.
func TestRecordingHandler_PromotesTheFieldsYouSearchBy(t *testing.T) {
	h := NewRecordingHandler(discardHandler(), slog.LevelWarn, 16)
	slog.New(h).Error("falhou",
		"request_id", "req-123", "account_id", "acc-1",
		"path", "/v1/leads", "code", "LEAD_NOT_FOUND")

	repo := &collectingRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	NewLogWriter(repo, h, 5*time.Millisecond, 10).Start(ctx)
	time.Sleep(60 * time.Millisecond)
	cancel()

	got := repo.snapshot()
	require.Len(t, got, 1)
	assert.Equal(t, "req-123", got[0].RequestID)
	assert.Equal(t, "acc-1", got[0].AccountID)
	assert.Equal(t, "/v1/leads", got[0].Path)
	assert.Equal(t, "LEAD_NOT_FOUND", got[0].Code)
}

// The table must not become the back door through which the personal data the
// stdout log refuses to write gets in.
func TestRecordingHandler_RedactsTheSameThingsStdoutDoes(t *testing.T) {
	h := NewRecordingHandler(discardHandler(), slog.LevelWarn, 16)
	slog.New(h).Error("envio falhou", "phone", "+5511999998888", "code", "X")

	repo := &collectingRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	NewLogWriter(repo, h, 5*time.Millisecond, 10).Start(ctx)
	time.Sleep(60 * time.Millisecond)
	cancel()

	got := repo.snapshot()
	require.Len(t, got, 1)
	var attrs map[string]any
	require.NoError(t, json.Unmarshal(got[0].Attrs, &attrs))
	assert.NotContains(t, string(got[0].Attrs), "5511999998888")
	assert.Equal(t, "[REDACTED]", attrs["phone"])
}

// A full buffer drops the line rather than holding the request — and the drop is
// counted. A table with holes nobody knows about is worse than an empty one:
// absence becomes proof that "nothing happened".
func TestRecordingHandler_DropsInsteadOfBlocking(t *testing.T) {
	h := NewRecordingHandler(discardHandler(), slog.LevelWarn, 2)
	log := slog.New(h)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			log.Error("enchendo")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("o log travou o chamador — é exatamente o que não pode acontecer")
	}
}
