package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// mockProviderCostRepo implements domain.ProviderCostRepository +
// domain.PlatformSettingRepository in memory.
type mockProviderCostRepo struct {
	rows     []*domain.ProviderCost
	settings map[string]string
	updated  []string
	deleted  []string
}

func newMockProviderCostRepo() *mockProviderCostRepo {
	return &mockProviderCostRepo{settings: map[string]string{}}
}

func (m *mockProviderCostRepo) List(_ context.Context) ([]*domain.ProviderCost, error) {
	return m.rows, nil
}

func (m *mockProviderCostRepo) Insert(_ context.Context, c *domain.ProviderCost) error {
	for _, r := range m.rows {
		if r.Provider == c.Provider && r.Model == c.Model && r.EffectiveFrom.Equal(c.EffectiveFrom) {
			return &domain.AppError{Code: "PROVIDER_COST_DUPLICATE", Message: "dup"}
		}
	}
	c.CreatedAt = time.Now()
	m.rows = append(m.rows, c)
	return nil
}

func (m *mockProviderCostRepo) Update(_ context.Context, id string, in, out int64, from time.Time) (bool, error) {
	for _, r := range m.rows {
		if r.ID == id {
			r.InputCostPer1MCentsUSD = in
			r.OutputCostPer1MCentsUSD = out
			r.EffectiveFrom = from
			m.updated = append(m.updated, id)
			return true, nil
		}
	}
	return false, nil
}

func (m *mockProviderCostRepo) Delete(_ context.Context, id string) (bool, error) {
	for i, r := range m.rows {
		if r.ID == id {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			m.deleted = append(m.deleted, id)
			return true, nil
		}
	}
	return false, nil
}

func (m *mockProviderCostRepo) GetEffective(_ context.Context, provider, model string, at time.Time) (*domain.ProviderCost, error) {
	var best *domain.ProviderCost
	for _, r := range m.rows {
		if r.Provider != provider || r.Model != model || r.EffectiveFrom.After(at) {
			continue
		}
		if best == nil || r.EffectiveFrom.After(best.EffectiveFrom) {
			best = r
		}
	}
	return best, nil
}

func (m *mockProviderCostRepo) GetSetting(_ context.Context, key string) (string, error) {
	return m.settings[key], nil
}

func (m *mockProviderCostRepo) SetSetting(_ context.Context, key, value, _ string) error {
	m.settings[key] = value
	return nil
}

func newProviderCostUC(repo *mockProviderCostRepo) *ProviderCostAdminUseCase {
	return NewProviderCostAdminUseCase(repo, repo, nil)
}

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("expected *domain.AppError, got %T: %v", err, err)
	}
	return appErr.Code
}

func TestProviderCostCreate_Success(t *testing.T) {
	repo := newMockProviderCostRepo()
	uc := newProviderCostUC(repo)

	c, err := uc.Create(context.Background(), "owner@x.com", ProviderCostDTO{
		Provider:                " Google ",
		Model:                   "gemini-2.5-flash",
		InputCostPer1MCentsUSD:  30,
		OutputCostPer1MCentsUSD: 250,
		EffectiveFrom:           "2026-07-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Provider != "google" {
		t.Errorf("provider not normalized: %q", c.Provider)
	}
	if c.ID == "" || len(c.ID) != 26 {
		t.Errorf("expected ULID id, got %q", c.ID)
	}
	if !c.EffectiveFrom.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("bad effective_from: %v", c.EffectiveFrom)
	}
}

func TestProviderCostCreate_Validation(t *testing.T) {
	uc := newProviderCostUC(newMockProviderCostRepo())
	cases := []struct {
		name string
		dto  ProviderCostDTO
		code string
	}{
		{"empty provider", ProviderCostDTO{Model: "m", EffectiveFrom: "2026-07-01"}, "PROVIDER_COST_INVALID_PROVIDER"},
		{"empty model", ProviderCostDTO{Provider: "openai", EffectiveFrom: "2026-07-01"}, "PROVIDER_COST_INVALID_MODEL"},
		{"negative cost", ProviderCostDTO{Provider: "openai", Model: "m", InputCostPer1MCentsUSD: -1, EffectiveFrom: "2026-07-01"}, "PROVIDER_COST_INVALID_VALUES"},
		{"bad date", ProviderCostDTO{Provider: "openai", Model: "m", EffectiveFrom: "07/01/2026"}, "PROVIDER_COST_INVALID_DATE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Create(context.Background(), "owner@x.com", tc.dto)
			if got := appErrCode(t, err); got != tc.code {
				t.Errorf("expected %s, got %s", tc.code, got)
			}
		})
	}
}

func TestProviderCostCreate_Duplicate(t *testing.T) {
	uc := newProviderCostUC(newMockProviderCostRepo())
	dto := ProviderCostDTO{Provider: "openai", Model: "gpt-5.4-mini", InputCostPer1MCentsUSD: 75, OutputCostPer1MCentsUSD: 450, EffectiveFrom: "2026-07-01"}
	if _, err := uc.Create(context.Background(), "owner@x.com", dto); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := uc.Create(context.Background(), "owner@x.com", dto)
	if got := appErrCode(t, err); got != "PROVIDER_COST_DUPLICATE" {
		t.Errorf("expected PROVIDER_COST_DUPLICATE, got %s", got)
	}
}

func TestProviderCostUpdate(t *testing.T) {
	repo := newMockProviderCostRepo()
	uc := newProviderCostUC(repo)
	c, _ := uc.Create(context.Background(), "owner@x.com", ProviderCostDTO{
		Provider: "google", Model: "gemini-2.5-flash",
		InputCostPer1MCentsUSD: 30, OutputCostPer1MCentsUSD: 250, EffectiveFrom: "2026-07-01",
	})

	err := uc.Update(context.Background(), "owner@x.com", c.ID, ProviderCostDTO{
		InputCostPer1MCentsUSD: 35, OutputCostPer1MCentsUSD: 260, EffectiveFrom: "2026-07-15",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.rows[0].InputCostPer1MCentsUSD != 35 || repo.rows[0].OutputCostPer1MCentsUSD != 260 {
		t.Errorf("row not updated: %+v", repo.rows[0])
	}

	err = uc.Update(context.Background(), "owner@x.com", "missing", ProviderCostDTO{EffectiveFrom: "2026-07-15"})
	if got := appErrCode(t, err); got != "PROVIDER_COST_NOT_FOUND" {
		t.Errorf("expected PROVIDER_COST_NOT_FOUND, got %s", got)
	}
}

func TestProviderCostDelete(t *testing.T) {
	repo := newMockProviderCostRepo()
	uc := newProviderCostUC(repo)
	c, _ := uc.Create(context.Background(), "owner@x.com", ProviderCostDTO{
		Provider: "openai", Model: "gpt-5.4-mini",
		InputCostPer1MCentsUSD: 75, OutputCostPer1MCentsUSD: 450, EffectiveFrom: "2026-07-01",
	})

	if err := uc.Delete(context.Background(), "owner@x.com", c.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.rows) != 0 {
		t.Errorf("row not deleted")
	}
	err := uc.Delete(context.Background(), "owner@x.com", c.ID)
	if got := appErrCode(t, err); got != "PROVIDER_COST_NOT_FOUND" {
		t.Errorf("expected PROVIDER_COST_NOT_FOUND, got %s", got)
	}
}

func TestUSDBRLRate(t *testing.T) {
	repo := newMockProviderCostRepo()
	uc := newProviderCostUC(repo)
	ctx := context.Background()

	// Absent setting falls back to the seeded default.
	rate, err := uc.GetUSDBRLRateCents(ctx)
	if err != nil || rate != DefaultUSDBRLRateCents {
		t.Errorf("expected default %d, got %d (%v)", DefaultUSDBRLRateCents, rate, err)
	}

	if err := uc.SetUSDBRLRateCents(ctx, "owner@x.com", 560); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rate, _ = uc.GetUSDBRLRateCents(ctx)
	if rate != 560 {
		t.Errorf("expected 560, got %d", rate)
	}

	for _, bad := range []int{0, -5, 99, 10001} {
		err := uc.SetUSDBRLRateCents(ctx, "owner@x.com", bad)
		if got := appErrCode(t, err); got != "PROVIDER_COST_INVALID_RATE" {
			t.Errorf("rate %d: expected PROVIDER_COST_INVALID_RATE, got %s", bad, got)
		}
	}

	// Corrupt stored value falls back to the default instead of erroring.
	repo.settings[domain.SettingUSDBRLRateCents] = "not-a-number"
	rate, err = uc.GetUSDBRLRateCents(ctx)
	if err != nil || rate != DefaultUSDBRLRateCents {
		t.Errorf("expected default on corrupt value, got %d (%v)", rate, err)
	}
}

func TestProviderCostList_BundlesRate(t *testing.T) {
	repo := newMockProviderCostRepo()
	uc := newProviderCostUC(repo)
	ctx := context.Background()
	_, _ = uc.Create(ctx, "owner@x.com", ProviderCostDTO{
		Provider: "google", Model: "gemini-2.5-flash",
		InputCostPer1MCentsUSD: 30, OutputCostPer1MCentsUSD: 250, EffectiveFrom: "2026-07-01",
	})
	_ = uc.SetUSDBRLRateCents(ctx, "owner@x.com", 545)

	out, err := uc.List(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Items) != 1 || out.USDBRLRateCents != 545 {
		t.Errorf("unexpected listing: %d items, rate %d", len(out.Items), out.USDBRLRateCents)
	}
}
