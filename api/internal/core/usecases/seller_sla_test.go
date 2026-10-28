package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// --- US-012: seller panel live SLA widget ---

type mockSellerSLARepo struct {
	convs  []domain.SLAConversation
	states map[string]*domain.SellerSLAState // key: seller|cycle
}

func (m *mockSellerSLARepo) ListSLAConversations(_ context.Context, _ string, _, _ time.Time) ([]domain.SLAConversation, error) {
	return m.convs, nil
}

func (m *mockSellerSLARepo) GetSLAState(_ context.Context, seller, cycle string) (*domain.SellerSLAState, error) {
	return m.states[seller+"|"+cycle], nil
}

// newSellerSLAUC builds the use case frozen at 2026-06-20 12:00 UTC
// (a Saturday would skew business minutes — the 20th is a Saturday, so
// pick a weekday: 2026-06-19 is a Friday).
func newSellerSLAUC(repo *mockSellerSLARepo) *SellerSLAUseCase {
	uc := NewSellerSLAUseCase(repo, &mockSettingsKV{}, "platform-acc")
	uc.now = func() time.Time {
		return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	}
	return uc
}

// weekdayAt returns a June/2026 weekday timestamp at the given day/hour
// UTC. Days 1-5, 8-12, 15-19 are Mon-Fri.
func weekdayAt(day, hour int) time.Time {
	return time.Date(2026, 6, day, hour, 0, 0, 0, time.UTC)
}

func TestSellerSLA_AllWithinSLA(t *testing.T) {
	answered := weekdayAt(15, 14) // Monday 14:00 UTC = 10:00 Cuiabá
	repo := &mockSellerSLARepo{convs: []domain.SLAConversation{
		// Created Monday 13:00 UTC (9:00 Cuiabá), answered 1 business hour later.
		{ConversationID: "c1", AssignedTo: "seller1", ContactName: "Ana", CreatedAt: weekdayAt(15, 13), FirstHumanResponseAt: &answered},
	}}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Cycle != "2026-06" {
		t.Fatalf("cycle = %s, want 2026-06", out.Cycle)
	}
	if out.TotalConversations != 1 || out.BreachedConversations != 0 {
		t.Fatalf("counts = %d/%d, want 1/0", out.TotalConversations, out.BreachedConversations)
	}
	if out.WithinSLAPct != 100 || out.Breach || out.Action != domain.SLAActionNone {
		t.Fatalf("pct=%v breach=%v action=%s, want 100/false/none", out.WithinSLAPct, out.Breach, out.Action)
	}
	if len(out.Breached) != 0 {
		t.Fatalf("breached list = %d, want empty", len(out.Breached))
	}
	if out.Settings == nil || out.Settings.FirstResponseMinutes != DefaultSLAFirstResponseMinutes {
		t.Fatalf("settings missing or wrong: %+v", out.Settings)
	}
}

func TestSellerSLA_BreachedConversationListedAndLadderPreviewed(t *testing.T) {
	// Answered two full business days later — way past 240 min.
	lateAnswer := weekdayAt(17, 14)
	repo := &mockSellerSLARepo{convs: []domain.SLAConversation{
		{ConversationID: "c-late", AssignedTo: "seller1", ContactName: "Bruno", CreatedAt: weekdayAt(15, 13), FirstHumanResponseAt: &lateAnswer},
		// Unanswered, created Monday — the window blew past by Friday noon.
		{ConversationID: "c-unanswered", AssignedTo: "seller1", ContactName: "Carla", CreatedAt: weekdayAt(15, 14)},
		// Another seller's breach must not leak into seller1's widget.
		{ConversationID: "c-other", AssignedTo: "seller2", CreatedAt: weekdayAt(15, 13), FirstHumanResponseAt: &lateAnswer},
	}}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalConversations != 2 || out.BreachedConversations != 2 {
		t.Fatalf("counts = %d/%d, want 2/2", out.TotalConversations, out.BreachedConversations)
	}
	if out.WithinSLAPct != 0 || !out.Breach {
		t.Fatalf("pct=%v breach=%v, want 0/true", out.WithinSLAPct, out.Breach)
	}
	// No previous-cycle state: first breach previews as warning.
	if out.ConsecutiveBreaches != 1 || out.Action != domain.SLAActionWarning {
		t.Fatalf("streak=%d action=%s, want 1/warning", out.ConsecutiveBreaches, out.Action)
	}
	if len(out.Breached) != 2 {
		t.Fatalf("breached list = %d, want 2", len(out.Breached))
	}
	// Newest first: c-unanswered (created 14:00) before c-late (13:00).
	if out.Breached[0].ConversationID != "c-unanswered" || out.Breached[1].ConversationID != "c-late" {
		t.Fatalf("order = %s,%s", out.Breached[0].ConversationID, out.Breached[1].ConversationID)
	}
	if out.Breached[0].ContactName != "Carla" || out.Breached[0].FirstHumanResponseAt != nil {
		t.Fatalf("unanswered row wrong: %+v", out.Breached[0])
	}
	if out.Breached[1].BusinessMinutes <= 240 {
		t.Fatalf("late row minutes = %d, want > 240", out.Breached[1].BusinessMinutes)
	}
}

func TestSellerSLA_FreshUnansweredNotPunished(t *testing.T) {
	repo := &mockSellerSLARepo{convs: []domain.SLAConversation{
		// Unanswered but created 1h before "now" — window not blown yet.
		{ConversationID: "c-fresh", AssignedTo: "seller1", CreatedAt: weekdayAt(19, 11)},
	}}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.BreachedConversations != 0 || out.Breach {
		t.Fatalf("fresh unanswered conversation counted as breach: %+v", out)
	}
}

func TestSellerSLA_ConsecutiveStreakFromPreviousCycle(t *testing.T) {
	repo := &mockSellerSLARepo{
		convs: []domain.SLAConversation{
			{ConversationID: "c1", AssignedTo: "seller1", CreatedAt: weekdayAt(15, 13)}, // unanswered, blown
		},
		states: map[string]*domain.SellerSLAState{
			"seller1|2026-05": {Breach: true, ConsecutiveBreaches: 1, Action: domain.SLAActionWarning},
		},
	}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.PreviousCycleAction != domain.SLAActionWarning {
		t.Fatalf("previous action = %s, want warning", out.PreviousCycleAction)
	}
	if out.ConsecutiveBreaches != 2 || out.Action != domain.SLAActionReduced {
		t.Fatalf("streak=%d action=%s, want 2/reduced", out.ConsecutiveBreaches, out.Action)
	}
}

func TestSellerSLA_NoConversationsIsAllClear(t *testing.T) {
	repo := &mockSellerSLARepo{states: map[string]*domain.SellerSLAState{
		"seller1|2026-05": {Breach: false, Action: domain.SLAActionNone},
	}}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalConversations != 0 || out.WithinSLAPct != 100 || out.Breach || out.Action != domain.SLAActionNone {
		t.Fatalf("empty cycle not all-clear: %+v", out)
	}
}

func TestSellerSLA_PlatformUnsetIsInert(t *testing.T) {
	repo := &mockSellerSLARepo{convs: []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "seller1", CreatedAt: weekdayAt(15, 13)},
	}}
	uc := NewSellerSLAUseCase(repo, nil, "")
	uc.now = func() time.Time { return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC) }
	out, err := uc.Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalConversations != 0 || out.Breach || out.Action != domain.SLAActionNone {
		t.Fatalf("unset platform not inert: %+v", out)
	}
}

// TestSellerSLA_InternalThreadsAggregatePerAccount (canal-interno
// US-009): the widget applies the same per-account aggregation as the
// payroll compute — two blown internal threads of one customer account
// count as ONE breached unit, while the Breached list still names each
// late thread individually so both stay linkable.
func TestSellerSLA_InternalThreadsAggregatePerAccount(t *testing.T) {
	fast := weekdayAt(15, 14)
	repo := &mockSellerSLARepo{convs: []domain.SLAConversation{
		// Both unanswered since Monday — both blown by Friday noon.
		{ConversationID: "t-admin", AssignedTo: "seller1", ContactName: "Admin", CustomerAccountID: "cust-a", CreatedAt: weekdayAt(15, 13)},
		{ConversationID: "t-attendant", AssignedTo: "seller1", ContactName: "Atendente", CustomerAccountID: "cust-a", CreatedAt: weekdayAt(15, 14)},
		// A clean regular conversation keeps counting as its own unit.
		{ConversationID: "c-regular", AssignedTo: "seller1", CreatedAt: weekdayAt(15, 13), FirstHumanResponseAt: &fast},
	}}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalConversations != 2 || out.BreachedConversations != 1 {
		t.Fatalf("counts = %d/%d, want 2 units / 1 breach (account counted once, not per thread)",
			out.TotalConversations, out.BreachedConversations)
	}
	if !out.Breach { // 1/2 = 50% > 20%
		t.Fatalf("breach = false, want true at 50%%")
	}
	if len(out.Breached) != 2 {
		t.Fatalf("breached list = %d, want both late threads listed", len(out.Breached))
	}
	// Newest first across the whole list, thread granularity preserved.
	if out.Breached[0].ConversationID != "t-attendant" || out.Breached[1].ConversationID != "t-admin" {
		t.Fatalf("order = %s,%s, want t-attendant,t-admin",
			out.Breached[0].ConversationID, out.Breached[1].ConversationID)
	}
}

func TestSellerSLA_ExactlyAtCutIsNotBreach(t *testing.T) {
	late := weekdayAt(18, 14)
	fast := weekdayAt(15, 14)
	convs := []domain.SLAConversation{
		// 1 of 5 breached = 20%, exactly at the default cut — strict >.
		{ConversationID: "c1", AssignedTo: "seller1", CreatedAt: weekdayAt(15, 13), FirstHumanResponseAt: &late},
	}
	for _, id := range []string{"c2", "c3", "c4", "c5"} {
		convs = append(convs, domain.SLAConversation{
			ConversationID: id, AssignedTo: "seller1",
			CreatedAt: weekdayAt(15, 13), FirstHumanResponseAt: &fast,
		})
	}
	repo := &mockSellerSLARepo{convs: convs}
	out, err := newSellerSLAUC(repo).Execute(context.Background(), "seller1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.BreachedConversations != 1 || out.Breach {
		t.Fatalf("20%% exact flagged as breach: %+v", out)
	}
	if len(out.Breached) != 1 {
		t.Fatalf("breached list should still name the blown conversation: %d", len(out.Breached))
	}
	if out.WithinSLAPct != 80 {
		t.Fatalf("pct = %v, want 80", out.WithinSLAPct)
	}
}
