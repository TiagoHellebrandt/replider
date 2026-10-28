package usecases

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// mockCommissionRepo implements domain.CommissionRepository in memory,
// mirroring the ON CONFLICT semantics: one row per (seller, account,
// cycle, reason), paid rows never rewritten, stale pending
// profit-share rows deleted.
type mockCommissionRepo struct {
	settings   map[string]*domain.SellerSettings
	bases      map[string][]domain.CommissionBase // key: cycle
	entries    map[string]*domain.CommissionEntry // key: seller|account|cycle|reason
	slaConvs   []domain.SLAConversation           // US-009 metric source
	slaStates  map[string]*domain.SellerSLAState  // key: seller|cycle
	detractors []domain.SellerDetractorFlag
}

func newMockCommissionRepo() *mockCommissionRepo {
	return &mockCommissionRepo{
		settings:  map[string]*domain.SellerSettings{},
		bases:     map[string][]domain.CommissionBase{},
		entries:   map[string]*domain.CommissionEntry{},
		slaStates: map[string]*domain.SellerSLAState{},
	}
}

func (m *mockCommissionRepo) ListSLAConversations(_ context.Context, _ string, from, to time.Time) ([]domain.SLAConversation, error) {
	out := []domain.SLAConversation{}
	for _, c := range m.slaConvs {
		if !c.CreatedAt.Before(from) && c.CreatedAt.Before(to) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockCommissionRepo) GetSLAState(_ context.Context, seller, cycle string) (*domain.SellerSLAState, error) {
	return m.slaStates[seller+"|"+cycle], nil
}

func (m *mockCommissionRepo) UpsertSLAState(_ context.Context, s *domain.SellerSLAState) error {
	key := s.SellerUserID + "|" + s.Cycle
	if existing, ok := m.slaStates[key]; ok {
		s.ID = existing.ID // mirror the ON CONFLICT id-preserving upsert
	}
	cp := *s
	m.slaStates[key] = &cp
	return nil
}

func (m *mockCommissionRepo) ListSLAStatesByCycle(_ context.Context, cycle string) ([]*domain.SellerSLAState, error) {
	out := []*domain.SellerSLAState{}
	for _, s := range m.slaStates {
		if s.Cycle == cycle {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *mockCommissionRepo) ListDetractorFlags(_ context.Context, _, _ time.Time) ([]domain.SellerDetractorFlag, error) {
	return m.detractors, nil
}

func entryKey(seller, account, cycle, reason string) string {
	return seller + "|" + account + "|" + cycle + "|" + reason
}

func (m *mockCommissionRepo) GetSellerSettings(_ context.Context, userID string) (*domain.SellerSettings, error) {
	return m.settings[userID], nil
}

func (m *mockCommissionRepo) UpsertSellerActivity(_ context.Context, userID string, isActive bool, deactivatedAt, residualUntil *time.Time) error {
	s, ok := m.settings[userID]
	if !ok {
		s = &domain.SellerSettings{
			UserID:                userID,
			CommissionRateOwn:     DefaultCommissionRateOwn,
			CommissionRateInbound: DefaultCommissionRateInbound,
		}
		m.settings[userID] = s
	}
	s.IsActiveAM = isActive
	s.DeactivatedAt = deactivatedAt
	s.ResidualUntil = residualUntil
	s.UpdatedAt = time.Now()
	return nil
}

func (m *mockCommissionRepo) SetSellerFlag(_ context.Context, userID string, isSeller bool) error {
	s, ok := m.settings[userID]
	if !ok {
		s = &domain.SellerSettings{
			UserID:                userID,
			CommissionRateOwn:     DefaultCommissionRateOwn,
			CommissionRateInbound: DefaultCommissionRateInbound,
			IsActiveAM:            true,
		}
		m.settings[userID] = s
	}
	s.IsSeller = isSeller
	s.UpdatedAt = time.Now()
	return nil
}

func (m *mockCommissionRepo) ListCommissionBases(_ context.Context, cycle string, _, _ time.Time) ([]domain.CommissionBase, error) {
	return m.bases[cycle], nil
}

func (m *mockCommissionRepo) HasBonusEntry(_ context.Context, accountID, reason string, cycles []string) (bool, error) {
	inWindow := func(cycle string) bool {
		if cycles == nil {
			return true
		}
		for _, c := range cycles {
			if c == cycle {
				return true
			}
		}
		return false
	}
	for _, e := range m.entries {
		if e.AccountID == accountID && e.Reason == reason && inWindow(e.Cycle) {
			return true, nil
		}
	}
	return false, nil
}

// Seller-statement methods (US-011) — unused by the compute tests,
// present so the mock satisfies domain.CommissionRepository.
func (m *mockCommissionRepo) ListSellerStatement(context.Context, string, string) ([]domain.SellerStatementLine, error) {
	return nil, nil
}

func (m *mockCommissionRepo) ListSellerCycles(context.Context, string) ([]string, error) {
	return nil, nil
}

func (m *mockCommissionRepo) UpsertSellerPixKey(context.Context, string, *string) error {
	return nil
}

func (m *mockCommissionRepo) UpsertEntry(_ context.Context, e *domain.CommissionEntry) error {
	key := entryKey(e.SellerUserID, e.AccountID, e.Cycle, e.Reason)
	if existing, ok := m.entries[key]; ok {
		if existing.Status == domain.CommissionStatusPaid {
			// The 000135 partial index arbitrates PENDING rows only: a
			// paid row is frozen history and a fresh pending row inserts
			// NEXT TO it (US-005). Mirror that by parking the paid row
			// under a unique archive key and taking over the canonical
			// slot (the pending one) below.
			m.entries[key+"#"+existing.ID] = existing
		} else {
			existing.BaseNetProfitCents = e.BaseNetProfitCents
			existing.RateApplied = e.RateApplied
			existing.AmountCents = e.AmountCents
			existing.UpdatedAt = time.Now()
			return nil
		}
	}
	cp := *e
	m.entries[key] = &cp
	return nil
}

var profitShareReasons = map[string]bool{
	domain.CommissionReasonStandard:     true,
	domain.CommissionReasonResidual:     true,
	domain.CommissionReasonSLAReduced:   true,
	domain.CommissionReasonBonusUpgrade: true,
	domain.CommissionReasonAdjustment:   true,
}

func (m *mockCommissionRepo) SumPaidProfitShare(_ context.Context, accountID, cycle string) (int64, string, bool, error) {
	var total int64
	found := false
	// paidSeller mirrors the SQL: seller of the most recently settled
	// row (paid_at DESC), the one a clawback debits (US-023).
	var paidSeller string
	var latest time.Time
	for _, e := range m.entries {
		if e.AccountID == accountID && e.Cycle == cycle &&
			e.Status == domain.CommissionStatusPaid && profitShareReasons[e.Reason] {
			total += e.AmountCents
			found = true
			at := time.Time{}
			if e.PaidAt != nil {
				at = *e.PaidAt
			}
			if paidSeller == "" || at.After(latest) {
				paidSeller, latest = e.SellerUserID, at
			}
		}
	}
	return total, paidSeller, found, nil
}

func (m *mockCommissionRepo) DeleteStaleProfitShare(_ context.Context, accountID, cycle, keepSeller, keepReason string) error {
	for key, e := range m.entries {
		if e.AccountID != accountID || e.Cycle != cycle {
			continue
		}
		if e.Status != domain.CommissionStatusPending || !profitShareReasons[e.Reason] {
			continue
		}
		if e.SellerUserID == keepSeller && e.Reason == keepReason {
			continue
		}
		delete(m.entries, key)
	}
	return nil
}

func (m *mockCommissionRepo) ListEntriesByCycle(_ context.Context, cycle string) ([]*domain.CommissionEntry, error) {
	out := []*domain.CommissionEntry{}
	for _, e := range m.entries {
		if e.Cycle == cycle {
			out = append(out, e)
		}
	}
	return out, nil
}

// Payout methods (US-013) — in-memory versions of the SQL grouping so
// the payout tests exercise real settle/adjustment behavior.
func (m *mockCommissionRepo) ListPayoutSummaries(_ context.Context, cycle string) ([]domain.SellerPayoutSummary, error) {
	bySeller := map[string]*domain.SellerPayoutSummary{}
	for _, e := range m.entries {
		current := e.Cycle == cycle
		adjustment := e.Cycle < cycle && e.Status == domain.CommissionStatusPending
		if !current && !adjustment {
			continue
		}
		s, ok := bySeller[e.SellerUserID]
		if !ok {
			s = &domain.SellerPayoutSummary{SellerUserID: e.SellerUserID}
			if set := m.settings[e.SellerUserID]; set != nil {
				s.PixKey = set.PixKey
			}
			bySeller[e.SellerUserID] = s
		}
		switch {
		case adjustment:
			s.AdjustmentCents += e.AmountCents
			s.AdjustmentEntries++
		case e.Status == domain.CommissionStatusPending:
			s.PendingCents += e.AmountCents
			s.PendingEntries++
		default:
			s.PaidCents += e.AmountCents
		}
	}
	out := []domain.SellerPayoutSummary{}
	for _, s := range bySeller {
		s.TotalDueCents = s.PendingCents + s.AdjustmentCents
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SellerUserID < out[j].SellerUserID })
	return out, nil
}

func (m *mockCommissionRepo) MarkPayoutPaid(_ context.Context, sellerUserID, cycle string, paidAt time.Time, note *string) (int64, int64, error) {
	var updated, totalCents int64
	for _, e := range m.entries {
		if e.SellerUserID != sellerUserID || e.Cycle > cycle || e.Status != domain.CommissionStatusPending {
			continue
		}
		e.Status = domain.CommissionStatusPaid
		at := paidAt
		e.PaidAt = &at
		if note != nil {
			n := *note
			e.Note = &n
		}
		updated++
		totalCents += e.AmountCents
	}
	return updated, totalCents, nil
}

// InCycleTx has no transaction to model in memory — the mock just runs
// fn against itself (the postgres repo owns the lock semantics; the
// e2e concurrency tests exercise those).
func (m *mockCommissionRepo) InCycleTx(_ context.Context, _ []string, fn func(domain.CommissionRepository) error) error {
	return fn(m)
}

type mockCommissionUsers struct {
	users map[string]*domain.User
}

func (m *mockCommissionUsers) GetByID(_ context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}

func newCommissionUC(repo *mockCommissionRepo, users *mockCommissionUsers) *ComputeCommissionsUseCase {
	if users == nil {
		users = &mockCommissionUsers{users: map[string]*domain.User{}}
	}
	return NewComputeCommissionsUseCase(repo, users, nil)
}

func strPointer(s string) *string { return &s }

func (m *mockCommissionRepo) get(t *testing.T, seller, account, cycle, reason string) *domain.CommissionEntry {
	t.Helper()
	e, ok := m.entries[entryKey(seller, account, cycle, reason)]
	if !ok {
		t.Fatalf("expected entry %s not found (have %d entries)", entryKey(seller, account, cycle, reason), len(m.entries))
	}
	return e
}

func TestComputeCommissions_BothTracks(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc-own", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 100000},
		{SellerUserID: "seller1", AccountID: "acc-inb", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	own := repo.get(t, "seller1", "acc-own", "2026-06", domain.CommissionReasonStandard)
	if own.RateApplied != 50 || own.AmountCents != 50000 {
		t.Errorf("own track: expected rate 50 amount 50000, got %v / %d", own.RateApplied, own.AmountCents)
	}
	inb := repo.get(t, "seller1", "acc-inb", "2026-06", domain.CommissionReasonStandard)
	if inb.RateApplied != 20 || inb.AmountCents != 20000 {
		t.Errorf("inbound track: expected rate 20 amount 20000, got %v / %d", inb.RateApplied, inb.AmountCents)
	}
	if own.Status != domain.CommissionStatusPending || inb.Status != domain.CommissionStatusPending {
		t.Errorf("new entries must be pending")
	}
}

func TestComputeCommissions_NilLeadSourceUsesInboundRate(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: nil, NetProfitCents: 50000},
	}
	uc := newCommissionUC(repo, nil)

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard)
	if e.RateApplied != 20 || e.AmountCents != 10000 {
		t.Errorf("nil lead_source must fall on the inbound rate: got %v / %d", e.RateApplied, e.AmountCents)
	}
}

func TestComputeCommissions_CustomRates(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.settings["seller1"] = &domain.SellerSettings{
		UserID: "seller1", CommissionRateOwn: 40, CommissionRateInbound: 25.5, IsActiveAM: true,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 10000},
		{SellerUserID: "seller1", AccountID: "acc2", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 10000},
	}
	uc := newCommissionUC(repo, nil)

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard); e.AmountCents != 4000 {
		t.Errorf("custom own rate 40%%: expected 4000, got %d", e.AmountCents)
	}
	// 25.5% of 10000 = 2550 — fractional percents stay exact via bps.
	if e := repo.get(t, "seller1", "acc2", "2026-06", domain.CommissionReasonStandard); e.AmountCents != 2550 {
		t.Errorf("custom inbound rate 25.5%%: expected 2550, got %d", e.AmountCents)
	}
}

func TestComputeCommissions_NegativeNetPaysZero(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: -4200},
	}
	uc := newCommissionUC(repo, nil)

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard)
	if e.AmountCents != 0 {
		t.Errorf("negative net must pay 0, got %d", e.AmountCents)
	}
	if e.BaseNetProfitCents != -4200 || e.RateApplied != 50 {
		t.Errorf("entry must still record base and rate: got %d / %v", e.BaseNetProfitCents, e.RateApplied)
	}
}

func TestComputeCommissions_ResidualInsideWindow(t *testing.T) {
	repo := newMockCommissionRepo()
	until := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	repo.settings["seller1"] = &domain.SellerSettings{
		UserID: "seller1", CommissionRateOwn: 50, CommissionRateInbound: 20,
		IsActiveAM: false, ResidualUntil: &until,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 100000},
	}
	// Stale pending 'standard' from a run before the deactivation must
	// be replaced by the residual entry, not coexist with it.
	repo.entries[entryKey("seller1", "acc1", "2026-06", domain.CommissionReasonStandard)] = &domain.CommissionEntry{
		SellerUserID: "seller1", AccountID: "acc1", Cycle: "2026-06",
		Reason: domain.CommissionReasonStandard, Status: domain.CommissionStatusPending, AmountCents: 50000,
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 entry (residual replaced standard), got %d", len(entries))
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonResidual)
	if e.RateApplied != 15 || e.AmountCents != 15000 {
		t.Errorf("residual: expected rate 15 amount 15000, got %v / %d", e.RateApplied, e.AmountCents)
	}
}

func TestComputeCommissions_PastResidualEarnsNothing(t *testing.T) {
	repo := newMockCommissionRepo()
	until := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) // before the 2026-06 cycle start
	repo.settings["seller1"] = &domain.SellerSettings{
		UserID: "seller1", CommissionRateOwn: 50, CommissionRateInbound: 20,
		IsActiveAM: false, ResidualUntil: &until,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 100000},
	}
	repo.entries[entryKey("seller1", "acc1", "2026-06", domain.CommissionReasonResidual)] = &domain.CommissionEntry{
		SellerUserID: "seller1", AccountID: "acc1", Cycle: "2026-06",
		Reason: domain.CommissionReasonResidual, Status: domain.CommissionStatusPending, AmountCents: 15000,
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("past residual_until must produce no entries and delete stale pending, got %d", len(entries))
	}
}

func TestComputeCommissions_RecomputeIdempotent(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstID := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard).ID

	// Profit was recomputed upward; rerun updates the pending entry in
	// place — same unique row, refreshed amount.
	repo.bases["2026-06"][0].NetProfitCents = 200000
	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("recompute must not duplicate: got %d entries", len(entries))
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard)
	if e.ID != firstID || e.AmountCents != 100000 {
		t.Errorf("expected same row updated to 100000, got id match %v amount %d", e.ID == firstID, e.AmountCents)
	}
}

func TestComputeCommissions_PaidEntryNeverRewritten(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard)
	e.Status = domain.CommissionStatusPaid // owner settled the payout

	repo.bases["2026-06"][0].NetProfitCents = 999999
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("second run: %v", err)
	}
	e = repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonStandard)
	if e.AmountCents != 50000 || e.Status != domain.CommissionStatusPaid {
		t.Errorf("paid entry must survive recompute untouched, got amount %d status %s", e.AmountCents, e.Status)
	}
}

func TestComputeCommissions_SellerReallocationDropsStalePending(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller2", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000},
	}
	// Entry from when seller1 still owned the account.
	repo.entries[entryKey("seller1", "acc1", "2026-06", domain.CommissionReasonStandard)] = &domain.CommissionEntry{
		SellerUserID: "seller1", AccountID: "acc1", Cycle: "2026-06",
		Reason: domain.CommissionReasonStandard, Status: domain.CommissionStatusPending, AmountCents: 50000,
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].SellerUserID != "seller2" {
		t.Fatalf("reallocation must leave only the new seller's entry, got %d", len(entries))
	}
}

func TestComputeCommissions_InvalidCycle(t *testing.T) {
	uc := newCommissionUC(newMockCommissionRepo(), nil)
	for _, cycle := range []string{"", "2026", "06-2026", "2026-13"} {
		if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", cycle); err == nil {
			t.Errorf("cycle %q must be rejected", cycle)
		}
	}
	if _, err := uc.List(context.Background(), "nope"); err == nil {
		t.Error("List must reject an invalid cycle")
	}
}

func TestSetSellerActivity_DeactivateStampsResidualWindow(t *testing.T) {
	repo := newMockCommissionRepo()
	users := &mockCommissionUsers{users: map[string]*domain.User{
		"seller1": {ID: "seller1", AccountID: "platform", IsActive: true},
	}}
	uc := NewComputeCommissionsUseCase(repo, users, nil)
	uc.SetPlatformAccountID("platform")
	frozen := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return frozen }

	s, err := uc.SetSellerActivity(context.Background(), "owner@x.com", "seller1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.IsActiveAM {
		t.Error("seller must be inactive")
	}
	if s.DeactivatedAt == nil || !s.DeactivatedAt.Equal(frozen) {
		t.Errorf("deactivated_at must be now, got %v", s.DeactivatedAt)
	}
	wantUntil := frozen.AddDate(0, 12, 0)
	if s.ResidualUntil == nil || !s.ResidualUntil.Equal(wantUntil) {
		t.Errorf("residual_until must be +12 months (%v), got %v", wantUntil, s.ResidualUntil)
	}

	// Reactivating clears the clause.
	s, err = uc.SetSellerActivity(context.Background(), "owner@x.com", "seller1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.IsActiveAM || s.DeactivatedAt != nil || s.ResidualUntil != nil {
		t.Errorf("reactivation must clear the activity clause, got %+v", s)
	}
}

func TestSetSellerActivity_NotFoundAndWrongAccount(t *testing.T) {
	repo := newMockCommissionRepo()
	users := &mockCommissionUsers{users: map[string]*domain.User{
		"outsider": {ID: "outsider", AccountID: "customer-acc", IsActive: true},
	}}
	uc := NewComputeCommissionsUseCase(repo, users, nil)
	uc.SetPlatformAccountID("platform")

	for _, id := range []string{"ghost", "outsider"} {
		_, err := uc.SetSellerActivity(context.Background(), "owner@x.com", id, false)
		appErr, ok := err.(*domain.AppError)
		if !ok || appErr.Code != "COMMISSION_SELLER_NOT_FOUND" {
			t.Errorf("seller %q: expected COMMISSION_SELLER_NOT_FOUND, got %v", id, err)
		}
	}
}

// US-014: the explicit seller grant/revoke (only flagged users reach the
// /seller panel or become payable).
func TestSetSellerFlag_GrantAndRevoke(t *testing.T) {
	repo := newMockCommissionRepo()
	users := &mockCommissionUsers{users: map[string]*domain.User{
		"seller1": {ID: "seller1", AccountID: "platform", IsActive: true},
	}}
	uc := NewComputeCommissionsUseCase(repo, users, nil)
	uc.SetPlatformAccountID("platform")

	s, err := uc.SetSellerFlag(context.Background(), "owner@x.com", "seller1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.IsSeller {
		t.Error("flag must be granted")
	}

	s, err = uc.SetSellerFlag(context.Background(), "owner@x.com", "seller1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.IsSeller {
		t.Error("flag must be revoked")
	}
}

func TestSetSellerFlag_NotFoundAndWrongAccount(t *testing.T) {
	repo := newMockCommissionRepo()
	users := &mockCommissionUsers{users: map[string]*domain.User{
		"outsider": {ID: "outsider", AccountID: "customer-acc", IsActive: true},
	}}
	uc := NewComputeCommissionsUseCase(repo, users, nil)
	uc.SetPlatformAccountID("platform")

	for _, id := range []string{"ghost", "outsider"} {
		_, err := uc.SetSellerFlag(context.Background(), "owner@x.com", id, true)
		appErr, ok := err.(*domain.AppError)
		if !ok || appErr.Code != "COMMISSION_SELLER_NOT_FOUND" {
			t.Errorf("seller %q: expected COMMISSION_SELLER_NOT_FOUND, got %v", id, err)
		}
	}
}

func TestCommissionList_Totals(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.entries[entryKey("s1", "a1", "2026-06", domain.CommissionReasonStandard)] = &domain.CommissionEntry{
		SellerUserID: "s1", AccountID: "a1", Cycle: "2026-06",
		Reason: domain.CommissionReasonStandard, Status: domain.CommissionStatusPending, AmountCents: 30000,
	}
	repo.entries[entryKey("s1", "a2", "2026-06", domain.CommissionReasonStandard)] = &domain.CommissionEntry{
		SellerUserID: "s1", AccountID: "a2", Cycle: "2026-06",
		Reason: domain.CommissionReasonStandard, Status: domain.CommissionStatusPaid, AmountCents: 20000,
	}
	uc := newCommissionUC(repo, nil)

	out, err := uc.List(context.Background(), "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalPendingCents != 30000 || out.TotalPaidCents != 20000 {
		t.Errorf("totals wrong: pending %d paid %d", out.TotalPendingCents, out.TotalPaidCents)
	}
}

// --- US-008: objective-event bonuses ---

// mockSettingsKV is a tiny in-memory platform_settings for the bonus
// amounts.
type mockSettingsKV struct{ vals map[string]string }

func (m *mockSettingsKV) GetSetting(_ context.Context, key string) (string, error) {
	return m.vals[key], nil
}

func (m *mockSettingsKV) SetSetting(_ context.Context, key, value, _ string) error {
	if m.vals == nil {
		m.vals = map[string]string{}
	}
	m.vals[key] = value
	return nil
}

func TestComputeCommissions_UpgradeBonusReplacesStandard(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		// INBOUND account (20% track) that upgraded its plan this cycle:
		// the month pays the own-lead rate under reason bonus_upgrade.
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000, HasPlanUpgrade: true},
	}
	// Stale pending 'standard' from a recompute run before the upgrade
	// event landed — must be replaced, not duplicated.
	repo.entries[entryKey("seller1", "acc1", "2026-06", domain.CommissionReasonStandard)] = &domain.CommissionEntry{
		SellerUserID: "seller1", AccountID: "acc1", Cycle: "2026-06",
		Reason: domain.CommissionReasonStandard, Status: domain.CommissionStatusPending, AmountCents: 20000,
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (bonus_upgrade replaced standard), got %d", len(entries))
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonBonusUpgrade)
	if e.RateApplied != 50 || e.AmountCents != 50000 {
		t.Errorf("upgrade cycle must pay the own rate: expected 50/50000, got %v/%d", e.RateApplied, e.AmountCents)
	}

	// Idempotent under recompute: still one row.
	entries, err = uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("recompute must not duplicate, got %d entries", len(entries))
	}
}

func TestComputeCommissions_UpgradeActivityClauseWins(t *testing.T) {
	repo := newMockCommissionRepo()
	until := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	repo.settings["seller1"] = &domain.SellerSettings{
		UserID: "seller1", CommissionRateOwn: 50, CommissionRateInbound: 20,
		IsActiveAM: false, ResidualUntil: &until,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000, HasPlanUpgrade: true},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonResidual)
	if e.RateApplied != 15 {
		t.Errorf("inactive seller stays on residual even with an upgrade, got rate %v", e.RateApplied)
	}
}

func TestComputeCommissions_NPSBonus(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000, HasPromoterNPS: true},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected standard + bonus_nps, got %d entries", len(entries))
	}
	bonus := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonBonusNPS)
	if bonus.AmountCents != DefaultBonusNPSCents || bonus.RateApplied != 0 {
		t.Errorf("nps bonus: expected fixed %d at rate 0, got %d/%v", int64(DefaultBonusNPSCents), bonus.AmountCents, bonus.RateApplied)
	}

	// Recompute: same two rows, no duplicate bonus.
	entries, _ = uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if len(entries) != 2 {
		t.Fatalf("recompute must stay at 2 entries, got %d", len(entries))
	}
}

func TestComputeCommissions_NPSBonusMaxOncePerQuarter(t *testing.T) {
	repo := newMockCommissionRepo()
	// Bonus already granted in 2026-05 — same calendar quarter (Q2) as
	// 2026-06, so the new promoter response earns nothing extra.
	repo.entries[entryKey("seller1", "acc1", "2026-05", domain.CommissionReasonBonusNPS)] = &domain.CommissionEntry{
		SellerUserID: "seller1", AccountID: "acc1", Cycle: "2026-05",
		Reason: domain.CommissionReasonBonusNPS, Status: domain.CommissionStatusPaid, AmountCents: 10000,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000, HasPromoterNPS: true},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("same-quarter promoter must not double the bonus: got %d entries", len(entries))
	}

	// A grant in ANOTHER quarter doesn't block: move the old bonus to
	// 2026-03 (Q1) and recompute.
	old := repo.entries[entryKey("seller1", "acc1", "2026-05", domain.CommissionReasonBonusNPS)]
	delete(repo.entries, entryKey("seller1", "acc1", "2026-05", domain.CommissionReasonBonusNPS))
	old.Cycle = "2026-03"
	repo.entries[entryKey("seller1", "acc1", "2026-03", domain.CommissionReasonBonusNPS)] = old

	entries, err = uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("new quarter must grant the bonus again: got %d entries", len(entries))
	}
}

func TestComputeCommissions_RetentionBonusOneTime(t *testing.T) {
	repo := newMockCommissionRepo()
	created := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC) // 12 months land inside 2026-06
	base := domain.CommissionBase{
		SellerUserID: "seller1", AccountID: "acc1",
		LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000,
		AccountCreatedAt: created, SubscriptionStatus: domain.SubscriptionStatusActive,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{base}
	repo.bases["2026-07"] = []domain.CommissionBase{base}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected standard + bonus_retention, got %d entries", len(entries))
	}
	bonus := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonBonusRetention)
	if bonus.AmountCents != DefaultBonusRetentionCents {
		t.Errorf("retention bonus: expected %d, got %d", int64(DefaultBonusRetentionCents), bonus.AmountCents)
	}

	// Recompute of the same cycle: idempotent.
	if entries, _ = uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06"); len(entries) != 2 {
		t.Fatalf("recompute must stay at 2 entries, got %d", len(entries))
	}
	// The NEXT cycle must NOT grant it again — one-time, ever.
	entries, err = uc.ComputeCycle(context.Background(), "owner@x.com", "2026-07")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("retention bonus is one-time: 2026-07 must only have standard, got %d entries", len(entries))
	}
}

func TestComputeCommissions_RetentionBonusRequiresActiveAnd12Months(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		// Old enough but churned.
		{SellerUserID: "s1", AccountID: "acc-churned", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 1000,
			AccountCreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), SubscriptionStatus: domain.SubscriptionStatusCanceled},
		// Active but too young (anniversary after the cycle end).
		{SellerUserID: "s1", AccountID: "acc-young", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 1000,
			AccountCreatedAt: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC), SubscriptionStatus: domain.SubscriptionStatusActive},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entries {
		if e.Reason == domain.CommissionReasonBonusRetention {
			t.Errorf("account %s must not earn the retention bonus", e.AccountID)
		}
	}
}

// TestComputeCommissions_ReferralNeedsNoExtraEntry documents the AC:
// an account prospected via the seller's ?ref= link (US-003) is stamped
// lead_source='seller_ref' at signup and ALREADY earns the own-lead 50%
// on the standard entry — the "indicação" bonus needs no extra ledger
// row.
func TestComputeCommissions_ReferralNeedsNoExtraEntry(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc-ref", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("referral account must have exactly the standard entry, got %d", len(entries))
	}
	e := repo.get(t, "seller1", "acc-ref", "2026-06", domain.CommissionReasonStandard)
	if e.RateApplied != 50 {
		t.Errorf("referral (?ref=) account already pays 50%% via lead_source, got %v", e.RateApplied)
	}
}

func TestComputeCommissions_BonusAmountsFromSettings(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "seller1", AccountID: "acc1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000, HasPromoterNPS: true},
	}
	uc := newCommissionUC(repo, nil)
	uc.SetPlatformSettings(&mockSettingsKV{vals: map[string]string{
		domain.SettingBonusNPSCents: "2500",
	}})

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e := repo.get(t, "seller1", "acc1", "2026-06", domain.CommissionReasonBonusNPS); e.AmountCents != 2500 {
		t.Errorf("bonus must come from settings: expected 2500, got %d", e.AmountCents)
	}

	// 0 disables the bonus entirely (fresh repo, no earlier grant).
	repo2 := newMockCommissionRepo()
	repo2.bases["2026-06"] = repo.bases["2026-06"]
	uc2 := newCommissionUC(repo2, nil)
	uc2.SetPlatformSettings(&mockSettingsKV{vals: map[string]string{
		domain.SettingBonusNPSCents: "0",
	}})
	entries, err := uc2.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("bonus 0 must disable the grant, got %d entries", len(entries))
	}
}

func TestSetBonusSettings_BoundsAndPersistence(t *testing.T) {
	kv := &mockSettingsKV{vals: map[string]string{}}
	uc := newCommissionUC(newMockCommissionRepo(), nil)
	uc.SetPlatformSettings(kv)
	ctx := context.Background()

	if err := uc.SetBonusSettings(ctx, "owner@x.com", 5000, 20000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kv.vals[domain.SettingBonusNPSCents] != "5000" || kv.vals[domain.SettingBonusRetentionCents] != "20000" {
		t.Errorf("settings not persisted: %v", kv.vals)
	}

	for _, bad := range [][2]int64{{-1, 0}, {0, -1}, {2000000, 0}, {0, 2000000}} {
		if err := uc.SetBonusSettings(ctx, "owner@x.com", bad[0], bad[1]); err == nil {
			t.Errorf("bounds must reject %v", bad)
		}
	}

	// Unwired settings → explicit error, not a silent no-op.
	if err := newCommissionUC(newMockCommissionRepo(), nil).SetBonusSettings(ctx, "o@x.com", 1, 1); err == nil {
		t.Error("nil settings must error")
	}

	// List surfaces the persisted amounts.
	out, err := uc.List(ctx, "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.BonusNPSCents != 5000 || out.BonusRetentionCents != 20000 {
		t.Errorf("List must bundle the settings, got %d/%d", out.BonusNPSCents, out.BonusRetentionCents)
	}
}
