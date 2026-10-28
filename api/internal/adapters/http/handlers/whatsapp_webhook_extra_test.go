package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

func nowUnix() int64 { return time.Now().Unix() }

// ─────────── outboxAsyncTracker counts down + writes correct state ───────────

type fakeOutboxRecorder struct {
	mu        sync.Mutex
	processed []string
	failed    map[string]string
}

func newFakeOutboxRecorder() *fakeOutboxRecorder {
	return &fakeOutboxRecorder{failed: map[string]string{}}
}

func (f *fakeOutboxRecorder) Insert(ctx context.Context, id string, payload []byte, sig string) error {
	return nil
}
func (f *fakeOutboxRecorder) MarkProcessed(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}
func (f *fakeOutboxRecorder) MarkSkipped(ctx context.Context, id string) error { return nil }
func (f *fakeOutboxRecorder) MarkFailed(ctx context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id] = reason
	return nil
}
func (f *fakeOutboxRecorder) MarkDead(ctx context.Context, id string) error { return nil }
func (f *fakeOutboxRecorder) ListPending(ctx context.Context, limit int) ([]*domain.WAWebhookEvent, error) {
	return nil, nil
}
func (f *fakeOutboxRecorder) ListByStatus(ctx context.Context, status string, limit int) ([]*domain.WAWebhookEventDigest, error) {
	return nil, nil
}
func (f *fakeOutboxRecorder) ListAll(ctx context.Context, limit int) ([]*domain.WAWebhookEventDigest, error) {
	return nil, nil
}
func (f *fakeOutboxRecorder) ResetForRetry(ctx context.Context, id string) error { return nil }
func (f *fakeOutboxRecorder) PruneProcessed(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}

func TestOutboxAsyncTracker_LastSignalMarksProcessed(t *testing.T) {
	rec := newFakeOutboxRecorder()
	tracker := &outboxAsyncTracker{eventID: "evt-1", outbox: rec}
	tracker.pending.Add(3)

	tracker.SignalDone(false, "")
	tracker.SignalDone(false, "")
	// First two calls should NOT mark — counter still > 0.
	rec.mu.Lock()
	if len(rec.processed) != 0 || len(rec.failed) != 0 {
		t.Fatalf("expected no mark before final SignalDone, got processed=%v failed=%v",
			rec.processed, rec.failed)
	}
	rec.mu.Unlock()

	tracker.SignalDone(false, "")
	// Last decrement should mark Processed.
	if len(rec.processed) != 1 || rec.processed[0] != "evt-1" {
		t.Errorf("expected Processed mark on evt-1, got %v", rec.processed)
	}
	if len(rec.failed) != 0 {
		t.Errorf("expected no Failed marks, got %v", rec.failed)
	}
}

func TestOutboxAsyncTracker_AnyFailureMarksFailed(t *testing.T) {
	rec := newFakeOutboxRecorder()
	tracker := &outboxAsyncTracker{eventID: "evt-2", outbox: rec}
	tracker.pending.Add(2)

	tracker.SignalDone(true, "send timeout")
	tracker.SignalDone(false, "")

	if len(rec.processed) != 0 {
		t.Errorf("expected no Processed when any goroutine failed, got %v", rec.processed)
	}
	if rec.failed["evt-2"] == "" {
		t.Errorf("expected Failed mark on evt-2, got %v", rec.failed)
	}
}

func TestOutboxAsyncTracker_NilSafe(t *testing.T) {
	var tracker *outboxAsyncTracker
	tracker.SignalDone(false, "") // should not panic
}

// ─────────── Per-channel read receipts toggle ───────────

func TestReadReceiptsEnabled_DefaultsOn(t *testing.T) {
	ch := &domain.Channel{
		Type:   "whatsapp",
		Config: []byte(`{"phone_number_id":"pn"}`),
	}
	if !readReceiptsEnabled(ch) {
		t.Error("expected read receipts ON by default")
	}
}

func TestReadReceiptsEnabled_DisabledFlags(t *testing.T) {
	for _, v := range []string{"off", "false", "no", "0", "disabled", "OFF", "False"} {
		ch := &domain.Channel{
			Type:   "whatsapp",
			Config: []byte(`{"phone_number_id":"pn","read_receipts":"` + v + `"}`),
		}
		if readReceiptsEnabled(ch) {
			t.Errorf("expected read receipts OFF for value %q", v)
		}
	}
}

func TestReadReceiptsEnabled_NilChannel(t *testing.T) {
	if !readReceiptsEnabled(nil) {
		t.Error("expected ON for nil channel (safe default)")
	}
}

// ─────────── Replay guard: entry.time older than MaxWebhookAge ───────────

func TestWhatsAppInbound_ReplayGuard_RejectsAgedPayload(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	// Build a valid envelope with entry.time stamped 24h in the past.
	// Signature verification passes; replay guard should reject 401.
	old := json.Number("0").String()
	_ = old
	payload := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []any{
			map[string]any{
				"id":      "biz",
				"time":    1, // unix epoch second 1 — clearly aged
				"changes": []any{},
			},
		},
	}
	body, _ := json.Marshal(payload)
	sig := computeHMAC(body, testAppSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"aged entry.time should be rejected by replay guard")
}

func TestWhatsAppInbound_ReplayGuard_AcceptsFreshPayload(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	// Fresh entry.time → guard passes; payload is otherwise empty so
	// the handler responds 200 ignored.
	payload := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []any{
			map[string]any{
				"id":      "biz",
				"time":    nowUnix(),
				"changes": []any{},
			},
		},
	}
	body, _ := json.Marshal(payload)
	sig := computeHMAC(body, testAppSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"fresh entry.time should pass the replay guard")
}

// ─────────── Body cap (5MB) returns 413 ───────────

func TestWhatsAppInbound_BodyOver5MB_Returns413(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	// Build a body that exceeds the 5MB cap. The signature check
	// would also reject this, but the size guard fires FIRST so we
	// confirm 413 instead of 403/200.
	big := bytes.Repeat([]byte("x"), 5<<20+1024)
	sig := computeHMAC(big, testAppSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code,
		"expected 413 when body exceeds 5MB cap")
}

func TestWhatsAppInbound_BodyAtCapAccepted(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	// Status-only payload sized to fit comfortably under the cap.
	// Wrap in valid JSON so the signature check + ParseStatuses both
	// see a real envelope. The body is allowed; we don't need to
	// match a channel because the test only verifies the size guard
	// doesn't fire.
	envelope := map[string]any{
		"object": "whatsapp_business_account",
		"entry":  []any{},
		"_pad":   strings.Repeat("p", 1024),
	}
	body, _ := json.Marshal(envelope)
	sig := computeHMAC(body, testAppSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusRequestEntityTooLarge, w.Code)
}

// Mixed status + message coverage lives in the parser-level tests
// (TestParseInboundAll_* + TestParseStatuses_*). Wiring the full
// HTTP path here would require mocking conversation + message repos
// just to verify the handler doesn't early-return after statuses,
// and the dedicated parser tests catch the same regression with a
// narrower blast radius.

// ─────────── ListByWABAID-driven cross-tenant fan-out ───────────

func TestWhatsAppFanout_TemplateEventReachesEveryAccountSharingWABA(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("ListByWABAID", mock.Anything, "waba-multi").Return([]*domain.Channel{
		{ID: "ch-A", AccountID: "acc-A", Type: "whatsapp",
			Config: []byte(`{"waba_id":"waba-multi","phone_number_id":"pn-A","display_phone_number":"+5511AAA","access_token":"t"}`)},
		{ID: "ch-B", AccountID: "acc-B", Type: "whatsapp",
			Config: []byte(`{"waba_id":"waba-multi","phone_number_id":"pn-B","display_phone_number":"+5511BBB","access_token":"t"}`)},
	}, nil)

	handler, _ := setupWhatsAppWebhookHandler(channelRepo)

	// Template-status events are NOT phone-scoped; the handler must
	// fan out to every account. We invoke fanoutAccounts directly to
	// keep the test focused on the lookup (the broadcast layer is
	// covered by other tests).
	accounts := handler.fanoutAccounts(context.Background(), "waba-multi", "")
	assert.ElementsMatch(t, []string{"acc-A", "acc-B"}, accounts,
		"template event should reach both tenants under the WABA")
}

func TestWhatsAppFanout_QualityEventScopesToMatchingPhone(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("ListByWABAID", mock.Anything, "waba-multi").Return([]*domain.Channel{
		{ID: "ch-A", AccountID: "acc-A", Type: "whatsapp",
			Config: []byte(`{"waba_id":"waba-multi","phone_number_id":"pn-A","display_phone_number":"+5511AAA","access_token":"t"}`)},
		{ID: "ch-B", AccountID: "acc-B", Type: "whatsapp",
			Config: []byte(`{"waba_id":"waba-multi","phone_number_id":"pn-B","display_phone_number":"+5511BBB","access_token":"t"}`)},
	}, nil)

	handler, _ := setupWhatsAppWebhookHandler(channelRepo)

	// Quality events DO carry display_phone_number — fanout should
	// pin to the matching channel only so unrelated tenants don't
	// see another tenant's quality drop.
	accounts := handler.fanoutAccounts(context.Background(), "waba-multi", "+5511BBB")
	assert.Equal(t, []string{"acc-B"}, accounts,
		"quality event with display_phone_number should pin to matching channel")
}

func TestWhatsAppFanout_QualityEventFallsBackToFullSetWhenNoMatch(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("ListByWABAID", mock.Anything, "waba-multi").Return([]*domain.Channel{
		{ID: "ch-A", AccountID: "acc-A", Type: "whatsapp",
			Config: []byte(`{"waba_id":"waba-multi","phone_number_id":"pn-A","access_token":"t"}`)},
	}, nil)

	handler, _ := setupWhatsAppWebhookHandler(channelRepo)

	// Channel doesn't cache display_phone_number → fallback to all.
	// Better to over-deliver than silently lose the alert.
	accounts := handler.fanoutAccounts(context.Background(), "waba-multi", "+5511UNKNOWN")
	assert.Equal(t, []string{"acc-A"}, accounts)
}

// ─────────── async drain on shutdown ───────────

func TestWhatsAppShutdown_DrainsInFlightAsyncReplies(t *testing.T) {
	// We can't run the full webhook flow here without wiring the AI
	// pipeline. Instead, exercise the WaitGroup contract directly:
	// simulate "an inbound landed and async work is in flight" by
	// adding to asyncWG, then assert Shutdown blocks until Done.
	channelRepo := new(MockChannelRepo)
	handler, _ := setupWhatsAppWebhookHandler(channelRepo)

	handler.asyncWG.Add(1)
	done := make(chan struct{})
	var mu sync.Mutex
	released := false

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*1_000_000_000) // 2s
		defer cancel()
		handler.Shutdown(ctx)
		mu.Lock()
		released = true
		mu.Unlock()
		close(done)
	}()

	// Shouldn't have completed yet — the in-flight goroutine is
	// still pretend-running.
	mu.Lock()
	if released {
		t.Fatal("Shutdown returned before Done was called")
	}
	mu.Unlock()

	handler.asyncWG.Done()
	<-done

	if !handler.shuttingDown.Load() {
		t.Error("expected shuttingDown to be set after Shutdown")
	}
}

// ─────────── helper: read first line of a response body without
// consuming it ───────────

var _ = io.EOF // anchor the io import for static checks
