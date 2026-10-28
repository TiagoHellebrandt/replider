package services

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

type fakeWAStore struct {
	channels []*domain.Channel
	merged   map[string]map[string]string // channelID -> fields
}

func (f *fakeWAStore) ListAllByProvider(ctx context.Context, provider string) ([]*domain.Channel, error) {
	return f.channels, nil
}

func (f *fakeWAStore) MergeCredentials(ctx context.Context, channelID string, fields map[string]string) error {
	if f.merged == nil {
		f.merged = map[string]map[string]string{}
	}
	f.merged[channelID] = fields
	return nil
}

type fakeProber struct {
	rating string
	tier   string
	err    error
	calls  int
}

func (p *fakeProber) PhoneStatus(ctx context.Context, phoneNumberID, accessToken string) (string, string, error) {
	p.calls++
	return p.rating, p.tier, p.err
}

func chWA(id string, active bool, creds string) *domain.Channel {
	return &domain.Channel{ID: id, Type: "whatsapp", IsActive: active, Config: []byte(creds)}
}

func TestWAQualityWorker_PersistsRatingForActiveChannel(t *testing.T) {
	store := &fakeWAStore{channels: []*domain.Channel{
		chWA("ch-1", true, `{"phone_number_id":"p1","access_token":"t1"}`),
	}}
	prober := &fakeProber{rating: "YELLOW", tier: "TIER_1K"}
	w := NewWhatsAppQualityWorker(store, prober)

	w.tick(context.Background())

	if prober.calls != 1 {
		t.Fatalf("expected 1 probe, got %d", prober.calls)
	}
	got := store.merged["ch-1"]
	if got["quality_rating"] != "YELLOW" || got["messaging_limit_tier"] != "TIER_1K" {
		t.Fatalf("unexpected merged fields: %#v", got)
	}
	if got["quality_checked_at"] == "" {
		t.Fatalf("expected quality_checked_at to be stamped")
	}
	if _, ok := got["quality_degraded_at"]; ok {
		t.Fatalf("YELLOW must not stamp quality_degraded_at")
	}
}

func TestWAQualityWorker_StampsDegradedOnFreshRed(t *testing.T) {
	store := &fakeWAStore{channels: []*domain.Channel{
		// Previously GREEN, now turning RED → degradation must be stamped.
		chWA("ch-1", true, `{"phone_number_id":"p1","quality_rating":"GREEN"}`),
	}}
	w := NewWhatsAppQualityWorker(store, &fakeProber{rating: "RED", tier: "TIER_1K"})

	w.tick(context.Background())

	if store.merged["ch-1"]["quality_degraded_at"] == "" {
		t.Fatalf("a fresh RED must stamp quality_degraded_at")
	}
}

func TestWAQualityWorker_DoesNotRestampChronicRed(t *testing.T) {
	store := &fakeWAStore{channels: []*domain.Channel{
		chWA("ch-1", true, `{"phone_number_id":"p1","quality_rating":"RED"}`),
	}}
	w := NewWhatsAppQualityWorker(store, &fakeProber{rating: "RED", tier: "TIER_1K"})

	w.tick(context.Background())

	if _, ok := store.merged["ch-1"]["quality_degraded_at"]; ok {
		t.Fatalf("an already-RED number must not re-stamp quality_degraded_at")
	}
}

func TestWAQualityWorker_SkipsInactiveAndNoPhone(t *testing.T) {
	store := &fakeWAStore{channels: []*domain.Channel{
		chWA("inactive", false, `{"phone_number_id":"p1"}`),
		chWA("no-phone", true, `{"access_token":"t"}`),
	}}
	prober := &fakeProber{rating: "GREEN"}
	w := NewWhatsAppQualityWorker(store, prober)

	w.tick(context.Background())

	if prober.calls != 0 {
		t.Fatalf("expected no probes, got %d", prober.calls)
	}
	if len(store.merged) != 0 {
		t.Fatalf("expected no merges, got %#v", store.merged)
	}
}
