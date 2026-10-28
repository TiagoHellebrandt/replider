package observability

import (
	"context"
	"io"
	"log/slog"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

// counterValue reads the current value of log_lines_total for a given level
// without pulling in prometheus/testutil (which drags an un-go.sum'd dep —
// see docs/metrics.md).
func counterValue(t *testing.T, level string) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := logLinesTotal.WithLabelValues(level).Write(m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestMeteringHandler_CountsEmittedRecords(t *testing.T) {
	// Discard output — we only care about the metric side effect.
	inner := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(NewMeteringHandler(inner))

	before := counterValue(t, "INFO")
	logger.Info("hello", "account_id", "acc_123")
	logger.Info("world")
	if got := counterValue(t, "INFO") - before; got != 2 {
		t.Fatalf("expected 2 INFO records counted, got %v", got)
	}

	beforeErr := counterValue(t, "ERROR")
	logger.Error("boom")
	if got := counterValue(t, "ERROR") - beforeErr; got != 1 {
		t.Fatalf("expected 1 ERROR record counted, got %v", got)
	}
}

func TestMeteringHandler_SkipsFilteredRecords(t *testing.T) {
	// Inner handler drops anything below WARN; DEBUG records must NOT be
	// metered because the wrapper honors Enabled() — slog never calls Handle.
	inner := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(NewMeteringHandler(inner))

	before := counterValue(t, "DEBUG")
	logger.Debug("verbose noise")
	if got := counterValue(t, "DEBUG") - before; got != 0 {
		t.Fatalf("expected filtered DEBUG record NOT counted, got %v", got)
	}
}

func TestMeteringHandler_WithAttrsAndGroupStillMeter(t *testing.T) {
	inner := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(NewMeteringHandler(inner)).
		With("service", "api").
		WithGroup("req")

	before := counterValue(t, "INFO")
	logger.InfoContext(context.Background(), "scoped", "id", "x")
	if got := counterValue(t, "INFO") - before; got != 1 {
		t.Fatalf("expected derived (WithAttrs/WithGroup) logger to still meter, got %v", got)
	}
}
