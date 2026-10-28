package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
)

// fakeChannelRepo is the bare-minimum repo for the refresh worker. It
// records Update calls so tests can assert the rotated token actually
// got persisted.
type fakeChannelRepo struct {
	mu       sync.Mutex
	channels []*domain.Channel
	updates  []*domain.Channel
	// credentialUpdates records partial JSONB writes via
	// UpdateCredentialField so tests can assert the worker rotated
	// the token using the partial-update path (avoiding the
	// read-modify-write race against the admin handler).
	credentialUpdates []credentialUpdate
}

type credentialUpdate struct {
	ChannelID string
	Key       string
	Value     string
}

func (f *fakeChannelRepo) ListAllByProvider(_ context.Context, _ string) ([]*domain.Channel, error) {
	return f.channels, nil
}

func (f *fakeChannelRepo) Update(_ context.Context, ch *domain.Channel) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, ch)
	return nil
}

// Stubs that round out the ChannelRepository interface but aren't
// touched by the worker. Returning zero values is fine.
func (f *fakeChannelRepo) Create(context.Context, *domain.Channel) error { return nil }
func (f *fakeChannelRepo) GetByID(context.Context, string) (*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) GetByPhoneNumberID(context.Context, string) (*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) GetByWABAID(context.Context, string) (*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) ListByWABAID(context.Context, string) ([]*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) UpdateCredentialField(_ context.Context, channelID, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.credentialUpdates = append(f.credentialUpdates, credentialUpdate{
		ChannelID: channelID, Key: key, Value: value,
	})
	return nil
}
func (f *fakeChannelRepo) MergeCredentials(context.Context, string, map[string]string) error {
	return nil
}
func (f *fakeChannelRepo) ListByAccountID(context.Context, string) ([]*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) ListByAgentID(context.Context, string) ([]*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) GetByWebhookToken(context.Context, string) (*domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) Delete(context.Context, string) error { return nil }
func (f *fakeChannelRepo) Toggle(context.Context, string, bool) (*domain.Channel, error) {
	return nil, nil
}

type captureAlerter struct {
	mu    sync.Mutex
	calls []string
}

func (c *captureAlerter) Record(_ string, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, msg)
}

func TestWATokenRefreshWorker_RotatesNearExpiry(t *testing.T) {
	// Token expires in 5 days — well inside the 14d threshold, so the
	// worker should call /debug_token, then fb_exchange_token, then
	// persist the new token on the channel.
	expiresAt := time.Now().Add(5 * 24 * time.Hour).Unix()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v25.0/debug_token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"is_valid":true,"expires_at":` + intStr(expiresAt) + `,"app_id":"app","user_id":"u"}}`))
		case r.URL.Path == "/v25.0/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"ROTATED-TOKEN","token_type":"bearer"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	prev := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	defer waadapter.SetBaseURLForTesting(prev)

	repo := &fakeChannelRepo{
		channels: []*domain.Channel{{
			ID:     "ch-1",
			Type:   "whatsapp",
			Config: jsonBytes(map[string]string{"access_token": "OLD-TOKEN", "phone_number_id": "1"}),
		}},
	}

	w := NewWATokenRefreshWorker(repo, "app", "secret").
		WithRefreshThreshold(14 * 24 * time.Hour)
	w.tick(context.Background())

	if len(repo.credentialUpdates) != 1 {
		t.Fatalf("expected 1 credential update, got %d", len(repo.credentialUpdates))
	}
	upd := repo.credentialUpdates[0]
	if upd.ChannelID != "ch-1" || upd.Key != "access_token" || upd.Value != "ROTATED-TOKEN" {
		t.Errorf("unexpected credential update %+v", upd)
	}
}

func TestWATokenRefreshWorker_SkipsTokenWithLongRunway(t *testing.T) {
	// Token expires in 50 days — outside the 14d threshold. The worker
	// should call /debug_token but NOT exchange or persist anything.
	expiresAt := time.Now().Add(50 * 24 * time.Hour).Unix()
	exchangeCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v25.0/oauth/access_token" {
			exchangeCalled = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"is_valid":true,"expires_at":` + intStr(expiresAt) + `}}`))
	}))
	defer srv.Close()
	prev := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	defer waadapter.SetBaseURLForTesting(prev)

	repo := &fakeChannelRepo{
		channels: []*domain.Channel{{
			ID:     "ch-1",
			Type:   "whatsapp",
			Config: jsonBytes(map[string]string{"access_token": "FRESH-TOKEN"}),
		}},
	}
	w := NewWATokenRefreshWorker(repo, "app", "secret")
	w.tick(context.Background())

	if exchangeCalled {
		t.Error("should not have called fb_exchange_token for a token with 50d runway")
	}
	if len(repo.updates) != 0 {
		t.Errorf("expected zero updates for a token with long runway, got %d", len(repo.updates))
	}
}

func TestWATokenRefreshWorker_AlertsOnRevokedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"is_valid":false,"app_id":"app"}}`))
	}))
	defer srv.Close()
	prev := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	defer waadapter.SetBaseURLForTesting(prev)

	repo := &fakeChannelRepo{
		channels: []*domain.Channel{{
			ID:     "ch-1",
			Type:   "whatsapp",
			Config: jsonBytes(map[string]string{"access_token": "REVOKED-TOKEN"}),
		}},
	}
	alerter := &captureAlerter{}
	w := NewWATokenRefreshWorker(repo, "app", "secret")
	w.SetAlerter(alerter)
	w.tick(context.Background())

	if len(alerter.calls) == 0 {
		t.Fatal("expected alerter to fire when token is invalid")
	}
	if len(repo.updates) != 0 {
		t.Errorf("must not persist a rotation when token is invalid; got %d updates", len(repo.updates))
	}
}

// jsonBytes is a shorthand for tests that mostly construct map literals.
func jsonBytes(m map[string]string) []byte {
	b, _ := json.Marshal(m)
	return b
}

func intStr(i int64) string {
	return jsonNumber(i)
}

func jsonNumber(i int64) string {
	b, _ := json.Marshal(i)
	return string(b)
}
