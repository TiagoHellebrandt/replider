package whatsapp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// stubOptOut is a hand-rolled OptOutChecker: returns optedOut for any
// (accountID, externalID) and optionally an error to exercise fail-open.
type stubOptOut struct {
	optedOut bool
	err      error
	called   bool
}

func (s *stubOptOut) IsOptedOut(_ context.Context, _, _ string) (bool, error) {
	s.called = true
	return s.optedOut, s.err
}

func newGuardTestChannel() *domain.Channel {
	return &domain.Channel{
		ID:        "ch-test",
		AccountID: "acc-1",
		Type:      "whatsapp",
		Config:    []byte(`{"phone_number_id":"pn-1","access_token":"tok"}`),
	}
}

// failServer panics on any request — a send that reaches it means the guard
// failed to abort before the network call.
func failServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected Meta call for opted-out contact: %s", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.X"}]}`))
	}))
	t.Cleanup(srv.Close)
	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })
	return srv
}

func TestSendMessage_BlockedWhenOptedOut(t *testing.T) {
	failServer(t)
	adapter := NewWhatsAppAdapter("")
	checker := &stubOptOut{optedOut: true}
	adapter.SetOptOutChecker(checker)

	body := "olá"
	_, err := adapter.SendMessageAndGetID(context.Background(), newGuardTestChannel(),
		"+55 11 99999-9999", &domain.Message{Type: "text", Content: &body})
	if !errors.Is(err, domain.ErrContactOptedOut) {
		t.Fatalf("expected ErrContactOptedOut, got %v", err)
	}
	if !checker.called {
		t.Error("expected opt-out checker to be consulted")
	}
}

func TestSendTemplate_BlockedWhenOptedOut(t *testing.T) {
	failServer(t)
	adapter := NewWhatsAppAdapter("")
	adapter.SetOptOutChecker(&stubOptOut{optedOut: true})

	_, err := adapter.SendTemplateAndGetID(context.Background(), newGuardTestChannel(),
		"+55 11 99999-9999", "promo", "pt_BR", nil)
	if !errors.Is(err, domain.ErrContactOptedOut) {
		t.Fatalf("expected ErrContactOptedOut for template send, got %v", err)
	}
}

func TestSendMessage_CheckerErrorFailsOpen(t *testing.T) {
	// A checker error must NOT block legitimate sends — the durable
	// enforcement is the inbound opt-out keyword, this is belt-and-suspenders.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.OK"}]}`))
	}))
	t.Cleanup(srv.Close)
	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })

	adapter := NewWhatsAppAdapter("")
	adapter.SetOptOutChecker(&stubOptOut{err: errors.New("db down")})

	body := "olá"
	id, err := adapter.SendMessageAndGetID(context.Background(), newGuardTestChannel(),
		"+55 11 99999-9999", &domain.Message{Type: "text", Content: &body})
	if err != nil {
		t.Fatalf("expected send to proceed on checker error, got %v", err)
	}
	if id != "wamid.OK" {
		t.Errorf("expected wamid.OK, got %q", id)
	}
}
