package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// US-009 fixtures. All wall-clock literals below are UTC; the default
// window is 8h-18h America/Cuiaba (UTC-4), so 14:00 UTC = 10:00 local.

const slaPlatformAccount = "acc-platform"

func newSLACommissionUC(repo *mockCommissionRepo) *ComputeCommissionsUseCase {
	uc := newCommissionUC(repo, nil)
	uc.SetPlatformAccountID(slaPlatformAccount)
	// Fixed "now" far past every test cycle so unanswered conversations
	// are measured to the cycle's end, deterministically.
	uc.now = func() time.Time { return time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) }
	return uc
}

func defaultSLATestConfig(t *testing.T) *SLASettings {
	t.Helper()
	loc, err := time.LoadLocation(DefaultSLATimezone)
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	return &SLASettings{
		FirstResponseMinutes: DefaultSLAFirstResponseMinutes,
		BreachPct:            DefaultSLABreachPct,
		BusinessStartHour:    DefaultSLABusinessStartHour,
		BusinessEndHour:      DefaultSLABusinessEndHour,
		Timezone:             DefaultSLATimezone,
		loc:                  loc,
	}
}

func TestBusinessMinutesBetween(t *testing.T) {
	cfg := defaultSLATestConfig(t)
	local := func(y int, m time.Month, d, hh, mm int) time.Time {
		return time.Date(y, m, d, hh, mm, 0, 0, cfg.loc)
	}
	cases := []struct {
		name     string
		from, to time.Time
		want     int64
	}{
		// 2026-03-02 is a Monday.
		{"same day inside window", local(2026, 3, 2, 10, 0), local(2026, 3, 2, 11, 30), 90},
		{"after hours to next morning", local(2026, 3, 2, 19, 0), local(2026, 3, 3, 8, 30), 30},
		{"friday evening to monday morning", local(2026, 3, 6, 17, 0), local(2026, 3, 9, 9, 0), 120},
		{"whole weekend", local(2026, 3, 7, 9, 0), local(2026, 3, 8, 17, 0), 0},
		{"before window clamps to start", local(2026, 3, 2, 6, 0), local(2026, 3, 2, 9, 0), 60},
		{"to before from", local(2026, 3, 2, 11, 0), local(2026, 3, 2, 10, 0), 0},
		{"full business day", local(2026, 3, 2, 0, 0), local(2026, 3, 3, 0, 0), 600},
	}
	for _, c := range cases {
		if got := businessMinutesBetween(c.from, c.to, cfg); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

// seedSLABase links account -> seller with a positive net profit for
// the cycle so the ledger side of the ladder is observable.
func seedSLABase(repo *mockCommissionRepo, cycle, seller, account string) {
	repo.bases[cycle] = []domain.CommissionBase{{
		SellerUserID:   seller,
		AccountID:      account,
		LeadSource:     strPointer(domain.LeadSourceSellerRef),
		NetProfitCents: 10000,
	}}
}

// TestComputeCommissions_SLALadderFull walks the complete ladder:
// breach 1 -> warning (ledger untouched at the 50% track), breach 2 ->
// reduced (sla_reduced at 20%), breach 3 -> flag_reallocation (the 20%
// reduction stays, nothing else automatic), then a clean cycle resets
// the counter and the track rate returns.
func TestComputeCommissions_SLALadderFull(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	ctx := context.Background()

	// One unanswered conversation per breach cycle (100% > 20%).
	// 2026-01-05, 2026-02-02 and 2026-03-02 are Mondays.
	repo.slaConvs = []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "s1", CreatedAt: time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC)},
		{ConversationID: "c2", AssignedTo: "s1", CreatedAt: time.Date(2026, 2, 2, 14, 0, 0, 0, time.UTC)},
		{ConversationID: "c3", AssignedTo: "s1", CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)},
		// April: answered after 30 business minutes — inside the SLA.
		{ConversationID: "c4", AssignedTo: "s1", CreatedAt: time.Date(2026, 4, 6, 14, 0, 0, 0, time.UTC),
			FirstHumanResponseAt: timePtr(time.Date(2026, 4, 6, 14, 30, 0, 0, time.UTC))},
	}

	// Cycle 1: warning, ledger untouched.
	seedSLABase(repo, "2026-01", "s1", "a1")
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-01"); err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	st := repo.slaStates["s1|2026-01"]
	if st == nil || !st.Breach || st.ConsecutiveBreaches != 1 || st.Action != domain.SLAActionWarning {
		t.Fatalf("cycle 1 state = %+v, want breach consecutive=1 warning", st)
	}
	if e := repo.get(t, "s1", "a1", "2026-01", domain.CommissionReasonStandard); e.RateApplied != 50 || e.AmountCents != 5000 {
		t.Fatalf("cycle 1 entry = rate %v amount %d, want 50/5000 (warning never touches the ledger)", e.RateApplied, e.AmountCents)
	}

	// Cycle 2: reduced — the profit share drops to 20% under
	// 'sla_reduced' and the stale standard row is gone.
	seedSLABase(repo, "2026-02", "s1", "a1")
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-02"); err != nil {
		t.Fatalf("cycle 2: %v", err)
	}
	st = repo.slaStates["s1|2026-02"]
	if st == nil || st.ConsecutiveBreaches != 2 || st.Action != domain.SLAActionReduced {
		t.Fatalf("cycle 2 state = %+v, want consecutive=2 reduced", st)
	}
	if e := repo.get(t, "s1", "a1", "2026-02", domain.CommissionReasonSLAReduced); e.RateApplied != SLAReducedCommissionRate || e.AmountCents != 2000 {
		t.Fatalf("cycle 2 entry = rate %v amount %d, want 20/2000", e.RateApplied, e.AmountCents)
	}
	if _, ok := repo.entries[entryKey("s1", "a1", "2026-02", domain.CommissionReasonStandard)]; ok {
		t.Fatal("cycle 2 should not keep a standard entry alongside sla_reduced")
	}

	// Recompute of cycle 2 is idempotent: same step, same money.
	firstID := repo.get(t, "s1", "a1", "2026-02", domain.CommissionReasonSLAReduced).ID
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-02"); err != nil {
		t.Fatalf("cycle 2 recompute: %v", err)
	}
	st = repo.slaStates["s1|2026-02"]
	if st.ConsecutiveBreaches != 2 || st.Action != domain.SLAActionReduced {
		t.Fatalf("cycle 2 recompute state = %+v, want unchanged consecutive=2 reduced", st)
	}
	if e := repo.get(t, "s1", "a1", "2026-02", domain.CommissionReasonSLAReduced); e.ID != firstID || e.AmountCents != 2000 {
		t.Fatalf("cycle 2 recompute entry = id %s amount %d, want same id %s amount 2000", e.ID, e.AmountCents, firstID)
	}

	// Cycle 3: flag_reallocation — owner acts manually; the reduction
	// itself stays while the streak lasts.
	seedSLABase(repo, "2026-03", "s1", "a1")
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("cycle 3: %v", err)
	}
	st = repo.slaStates["s1|2026-03"]
	if st == nil || st.ConsecutiveBreaches != 3 || st.Action != domain.SLAActionFlagReallocation {
		t.Fatalf("cycle 3 state = %+v, want consecutive=3 flag_reallocation", st)
	}
	if e := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonSLAReduced); e.RateApplied != SLAReducedCommissionRate {
		t.Fatalf("cycle 3 entry rate = %v, want the reduction to persist at 20", e.RateApplied)
	}

	// Cycle 4: no breach — counter resets and the track rate returns.
	seedSLABase(repo, "2026-04", "s1", "a1")
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-04"); err != nil {
		t.Fatalf("cycle 4: %v", err)
	}
	st = repo.slaStates["s1|2026-04"]
	if st == nil || st.Breach || st.ConsecutiveBreaches != 0 || st.Action != domain.SLAActionNone {
		t.Fatalf("cycle 4 state = %+v, want reset (no breach, consecutive=0, none)", st)
	}
	if e := repo.get(t, "s1", "a1", "2026-04", domain.CommissionReasonStandard); e.RateApplied != 50 {
		t.Fatalf("cycle 4 entry rate = %v, want the 50 track back", e.RateApplied)
	}
	if _, ok := repo.entries[entryKey("s1", "a1", "2026-04", domain.CommissionReasonSLAReduced)]; ok {
		t.Fatal("cycle 4 should not keep an sla_reduced entry")
	}
}

// TestComputeCommissions_SLABreachThresholdStrict: the cut is strict —
// exactly 20% of the conversations blown is NOT a breach; more is.
func TestComputeCommissions_SLABreachThresholdStrict(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)

	answered := timePtr(time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC))
	mkConv := func(id, seller string, breached bool) domain.SLAConversation {
		c := domain.SLAConversation{ConversationID: id, AssignedTo: seller,
			CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)}
		if !breached {
			c.FirstHumanResponseAt = answered
		}
		return c
	}
	// s1: 1 of 5 breached = exactly 20% -> no breach.
	// s2: 2 of 5 breached = 40% -> breach.
	repo.slaConvs = []domain.SLAConversation{
		mkConv("a", "s1", true), mkConv("b", "s1", false), mkConv("c", "s1", false),
		mkConv("d", "s1", false), mkConv("e", "s1", false),
		mkConv("f", "s2", true), mkConv("g", "s2", true), mkConv("h", "s2", false),
		mkConv("i", "s2", false), mkConv("j", "s2", false),
	}

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	s1 := repo.slaStates["s1|2026-03"]
	if s1 == nil || s1.Breach || s1.Action != domain.SLAActionNone || s1.BreachedConversations != 1 || s1.TotalConversations != 5 {
		t.Fatalf("s1 state = %+v, want 1/5 measured and NO breach at exactly 20%%", s1)
	}
	s2 := repo.slaStates["s2|2026-03"]
	if s2 == nil || !s2.Breach || s2.Action != domain.SLAActionWarning || s2.BreachedConversations != 2 {
		t.Fatalf("s2 state = %+v, want 2/5 breached -> warning", s2)
	}
}

// TestComputeCommissions_SLAInternalThreadsAggregatePerAccount
// (canal-interno US-009): a customer account's internal support threads
// (thread-per-user) collapse into ONE SLA unit per account — two
// threads of the same account with one (or both) blown count as one
// breached unit, never two; threads of different accounts stay separate
// units; regular conversations (no customer_account_id) keep counting
// one each.
func TestComputeCommissions_SLAInternalThreadsAggregatePerAccount(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	ctx := context.Background()

	answered := timePtr(time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC))
	created := time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC) // Monday 10:00 Cuiabá
	mk := func(id, custAcc string, fast bool) domain.SLAConversation {
		c := domain.SLAConversation{ConversationID: id, AssignedTo: "s1",
			CustomerAccountID: custAcc, CreatedAt: created}
		if fast {
			c.FirstHumanResponseAt = answered
		}
		return c
	}

	// cust-a: 2 threads, 1 blown -> 1 unit, breached.
	// cust-b: 2 threads, BOTH blown -> still 1 unit, 1 breach (not 2).
	// cust-c: 1 thread, fast -> 1 unit, clean.
	// r1: regular conversation, fast -> its own clean unit.
	repo.slaConvs = []domain.SLAConversation{
		mk("a1", "cust-a", true), mk("a2", "cust-a", false),
		mk("b1", "cust-b", false), mk("b2", "cust-b", false),
		mk("c1", "cust-c", true),
		mk("r1", "", true),
	}

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	st := repo.slaStates["s1|2026-03"]
	if st == nil {
		t.Fatal("no SLA state written for s1")
	}
	// Per-thread counting would say 6 total / 3 breached; per-account
	// units say 4 total / 2 breached (cust-a + cust-b, once each).
	if st.TotalConversations != 4 || st.BreachedConversations != 2 {
		t.Fatalf("counts = %d/%d, want 4 units with 2 breached (one per account, never per thread)",
			st.TotalConversations, st.BreachedConversations)
	}
	if !st.Breach { // 2/4 = 50% > 20%
		t.Fatalf("state = %+v, want breach at 50%%", st)
	}
}

// TestComputeCommissions_SLAReducedBeatsUpgrade: precedence is
// activity > SLA > upgrade > track — a plan upgrade in a reduced cycle
// still pays the reduced 20, and a track rate below the cap is kept
// (the SLA penalty never raises pay).
func TestComputeCommissions_SLAReducedBeatsUpgrade(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	ctx := context.Background()

	// Previous cycle already breached once.
	repo.slaStates["s1|2026-02"] = &domain.SellerSLAState{
		SellerUserID: "s1", Cycle: "2026-02", Breach: true, ConsecutiveBreaches: 1,
		Action: domain.SLAActionWarning,
	}
	repo.slaConvs = []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "s1", CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)},
	}
	repo.bases["2026-03"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceSellerRef),
			NetProfitCents: 10000, HasPlanUpgrade: true},
	}

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	e := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonSLAReduced)
	if e.RateApplied != SLAReducedCommissionRate || e.AmountCents != 2000 {
		t.Fatalf("entry = rate %v amount %d, want the SLA cap 20/2000 to beat bonus_upgrade", e.RateApplied, e.AmountCents)
	}
	if _, ok := repo.entries[entryKey("s1", "a1", "2026-03", domain.CommissionReasonBonusUpgrade)]; ok {
		t.Fatal("no bonus_upgrade entry should coexist with sla_reduced")
	}

	// Track rate below the cap is kept: custom inbound rate 10 stays 10.
	repo.settings["s1"] = &domain.SellerSettings{
		UserID: "s1", CommissionRateOwn: 50, CommissionRateInbound: 10, IsActiveAM: true,
	}
	repo.bases["2026-03"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 10000},
	}
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle (low track): %v", err)
	}
	if e := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonSLAReduced); e.RateApplied != 10 || e.AmountCents != 1000 {
		t.Fatalf("entry = rate %v amount %d, want 10/1000 (cap never raises pay)", e.RateApplied, e.AmountCents)
	}
}

// TestComputeCommissions_SLAResidualUnaffected: the activity clause
// outranks the SLA — an inactive seller inside the residual window
// stays on the 15% residual even while their SLA streak says reduced.
func TestComputeCommissions_SLAResidualUnaffected(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)

	until := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	repo.settings["s1"] = &domain.SellerSettings{
		UserID: "s1", CommissionRateOwn: 50, CommissionRateInbound: 20,
		IsActiveAM: false, ResidualUntil: &until,
	}
	repo.slaStates["s1|2026-02"] = &domain.SellerSLAState{
		SellerUserID: "s1", Cycle: "2026-02", Breach: true, ConsecutiveBreaches: 1,
		Action: domain.SLAActionWarning,
	}
	repo.slaConvs = []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "s1", CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)},
	}
	repo.bases["2026-03"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceSellerRef), NetProfitCents: 10000},
	}

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	if st := repo.slaStates["s1|2026-03"]; st == nil || st.Action != domain.SLAActionReduced {
		t.Fatalf("state = %+v, want the ladder still measured (reduced)", st)
	}
	e := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonResidual)
	if e.RateApplied != ResidualCommissionRate || e.AmountCents != 1500 {
		t.Fatalf("entry = rate %v amount %d, want the residual 15/1500 untouched by SLA", e.RateApplied, e.AmountCents)
	}
}

// TestComputeCommissions_SLAUnansweredOpenCycle: in an open cycle
// (prévia) an unanswered conversation only breaches once "now" is past
// the threshold — a fresh conversation isn't punished early.
func TestComputeCommissions_SLAUnansweredOpenCycle(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	// "now" 30 business minutes after the conversation arrived, inside
	// the cycle.
	uc.now = func() time.Time { return time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC) }
	repo.slaConvs = []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "s1", CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)},
	}
	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	if st := repo.slaStates["s1|2026-03"]; st == nil || st.Breach {
		t.Fatalf("state = %+v, want measured but not breached (only 30 business min elapsed)", st)
	}
}

// TestComputeCommissions_DetractorFlagInformationalOnly: detractor NPS
// surfaces as a flag on the listing and NEVER changes a rate.
func TestComputeCommissions_DetractorFlagInformationalOnly(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	ctx := context.Background()

	repo.detractors = []domain.SellerDetractorFlag{
		{SellerUserID: "s1", SellerEmail: "s1@x.com", DetractorResponses: 2},
	}
	seedSLABase(repo, "2026-03", "s1", "a1")

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	if e := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonStandard); e.RateApplied != 50 {
		t.Fatalf("entry rate = %v, want 50 — detractor NPS never touches a rate", e.RateApplied)
	}

	listing, err := uc.List(ctx, "2026-03")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.DetractorFlags) != 1 || listing.DetractorFlags[0].DetractorResponses != 2 {
		t.Fatalf("listing detractor flags = %+v, want the informational flag surfaced", listing.DetractorFlags)
	}
	if listing.SLASettings == nil || listing.SLASettings.FirstResponseMinutes != DefaultSLAFirstResponseMinutes {
		t.Fatalf("listing sla settings = %+v, want the defaults bundled", listing.SLASettings)
	}
}

// TestComputeCommissions_SLAPlatformUnset: without PLATFORM_ACCOUNT_ID
// there is no metric source — no states written, ledger untouched.
func TestComputeCommissions_SLAPlatformUnset(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newCommissionUC(repo, nil) // platform NOT set
	repo.slaConvs = []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "s1", CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)},
	}
	seedSLABase(repo, "2026-03", "s1", "a1")

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("ComputeCycle: %v", err)
	}
	if len(repo.slaStates) != 0 {
		t.Fatalf("sla states = %d, want none when the platform account is unset", len(repo.slaStates))
	}
	if e := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonStandard); e.RateApplied != 50 {
		t.Fatalf("entry rate = %v, want the plain 50 track", e.RateApplied)
	}
}

func TestSetSLASettings(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	ctx := context.Background()

	// Unwired settings repo.
	if err := uc.SetSLASettings(ctx, "o@x.com", SLASettings{FirstResponseMinutes: 240, BreachPct: 20, BusinessStartHour: 8, BusinessEndHour: 18, Timezone: "America/Cuiaba"}); err == nil {
		t.Fatal("expected COMMISSION_SETTINGS_UNAVAILABLE without a settings repo")
	}

	store := newMockProviderCostRepo()
	uc.SetPlatformSettings(store)
	invalid := []SLASettings{
		{FirstResponseMinutes: 0, BreachPct: 20, BusinessStartHour: 8, BusinessEndHour: 18, Timezone: "America/Cuiaba"},
		{FirstResponseMinutes: 240, BreachPct: 101, BusinessStartHour: 8, BusinessEndHour: 18, Timezone: "America/Cuiaba"},
		{FirstResponseMinutes: 240, BreachPct: 20, BusinessStartHour: 18, BusinessEndHour: 8, Timezone: "America/Cuiaba"},
		{FirstResponseMinutes: 240, BreachPct: 20, BusinessStartHour: 8, BusinessEndHour: 18, Timezone: "Mars/OlympusMons"},
	}
	for i, in := range invalid {
		err := uc.SetSLASettings(ctx, "o@x.com", in)
		appErr, ok := err.(*domain.AppError)
		if !ok || appErr.Code != "COMMISSION_INVALID_SLA" {
			t.Fatalf("case %d: err = %v, want COMMISSION_INVALID_SLA", i, err)
		}
	}

	if err := uc.SetSLASettings(ctx, "o@x.com", SLASettings{FirstResponseMinutes: 120, BreachPct: 25, BusinessStartHour: 9, BusinessEndHour: 17, Timezone: "America/Sao_Paulo"}); err != nil {
		t.Fatalf("valid settings: %v", err)
	}
	cfg, err := uc.loadSLAConfig(ctx)
	if err != nil {
		t.Fatalf("loadSLAConfig: %v", err)
	}
	if cfg.FirstResponseMinutes != 120 || cfg.BreachPct != 25 || cfg.BusinessStartHour != 9 || cfg.BusinessEndHour != 17 || cfg.Timezone != "America/Sao_Paulo" {
		t.Fatalf("reloaded settings = %+v, want the persisted values", cfg)
	}
}
