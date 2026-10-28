package usecases

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// --- fakes -----------------------------------------------------------------

type fakeOnboardingNudgeStore struct {
	due     []domain.OnboardingNudgeAccount
	listErr error

	mu     sync.Mutex
	marked map[string]int // accountID -> stage
}

func (f *fakeOnboardingNudgeStore) ListDueForNudge(_ context.Context, _ time.Time) ([]domain.OnboardingNudgeAccount, error) {
	return f.due, f.listErr
}

func (f *fakeOnboardingNudgeStore) MarkOnboardingNudgeSent(_ context.Context, accountID string, stage int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.marked == nil {
		f.marked = map[string]int{}
	}
	f.marked[accountID] = stage
	return nil
}

type nudgeSent struct {
	milestone string
	url       string
}

type fakeOnboardingNudgeNotifier struct {
	mu   sync.Mutex
	sent map[string]nudgeSent // recipient -> what was asked for
	err  error
}

func (f *fakeOnboardingNudgeNotifier) SendOnboardingNudgeEmail(to, _ string, milestone string, ctaURL, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sent == nil {
		f.sent = map[string]nudgeSent{}
	}
	f.sent[to] = nudgeSent{milestone: milestone, url: ctaURL}
	return nil
}

// fakeNudgeProgress answers the six milestones per account, the way
// GetOnboardingProgressUseCase would (percentage included).
type fakeNudgeProgress struct {
	byAccount map[string]*domain.OnboardingProgress
	err       error
}

func (f *fakeNudgeProgress) Execute(_ context.Context, accountID string) (*domain.OnboardingProgress, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byAccount[accountID], nil
}

// progressWith builds a progress row with the named milestones done and the
// percentage the real use case would compute.
func progressWith(done ...string) *domain.OnboardingProgress {
	p := &domain.OnboardingProgress{}
	for _, d := range done {
		switch d {
		case OnboardingMilestoneChannel:
			p.ChannelConnected = true
		case OnboardingMilestoneProducts:
			p.ProductsAdded = true
		case OnboardingMilestoneCustomers:
			p.CustomersAdded = true
		case OnboardingMilestoneProposal:
			p.ProposalMade = true
		case OnboardingMilestoneFirstInbound:
			p.FirstInboundReceived = true
		case OnboardingMilestoneTeamInvited:
			p.TeamInvited = true
		}
	}
	p.CompletionPercentage = computeOnboardingPercentage(p)
	return p
}

// fakeAdminResolver and fixedClock are reused from the sibling lifecycle
// email tests (same package).

func daysAgo(now time.Time, d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }

func newNudgeUC(store *fakeOnboardingNudgeStore, admins *fakeAdminResolver, notifier *fakeOnboardingNudgeNotifier, progress *fakeNudgeProgress, now time.Time) *SendOnboardingNudgesUseCase {
	return NewSendOnboardingNudgesUseCase(store, admins, notifier, progress, "https://app.replider.com").WithClock(fixedClock(now))
}

// --- tests -----------------------------------------------------------------

// Each stage asks for the FIRST pending sales milestone — the email follows
// the person, not a fixed channel/knowledge/share script.
func TestSendOnboardingNudges_AsksForTheFirstPendingMilestone(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{
		due: []domain.OnboardingNudgeAccount{
			// 1 day old, nothing done -> D1 asks for the mailbox.
			{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 1), StageSent: 0},
			// 3 days old, mailbox connected -> D3 asks for products.
			{AccountID: "acc2", Name: "B", CreatedAt: daysAgo(now, 3), StageSent: 1},
			// 7 days old, everything but the seller's seat -> D7 asks for the team.
			{AccountID: "acc3", Name: "C", CreatedAt: daysAgo(now, 7), StageSent: 3},
		},
	}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{
		"acc1": progressWith(),
		"acc2": progressWith(OnboardingMilestoneChannel),
		"acc3": progressWith(OnboardingMilestoneChannel, OnboardingMilestoneProducts, OnboardingMilestoneCustomers,
			OnboardingMilestoneProposal, OnboardingMilestoneFirstInbound),
	}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
		"acc2": {Email: "b@x.com", Name: "Bob"},
		"acc3": {Email: "c@x.com", Name: "Cris"},
	}}
	notifier := &fakeOnboardingNudgeNotifier{}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 3 {
		t.Fatalf("expected 3 sent, got %d", sent)
	}
	if got := notifier.sent["a@x.com"]; got.milestone != OnboardingMilestoneChannel ||
		got.url != "https://app.replider.com/settings/channels?utm_source=lifecycle&utm_medium=email&utm_campaign=onboarding_nudge&utm_content=d1_channel" {
		t.Errorf("acc1 should be asked for the channel with a tracked link, got %+v", got)
	}
	if got := notifier.sent["b@x.com"]; got.milestone != OnboardingMilestoneProducts ||
		got.url != "https://app.replider.com/sales/products/import?utm_source=lifecycle&utm_medium=email&utm_campaign=onboarding_nudge&utm_content=d3_products" {
		t.Errorf("acc2 should be asked for products, got %+v", got)
	}
	if got := notifier.sent["c@x.com"]; got.milestone != OnboardingMilestoneTeamInvited ||
		got.url != "https://app.replider.com/settings/team?utm_source=lifecycle&utm_medium=email&utm_campaign=onboarding_nudge&utm_content=d7_team_invited" {
		t.Errorf("acc3 should be asked to invite the team, got %+v", got)
	}
	if store.marked["acc1"] != domain.OnboardingNudgeStageChannel ||
		store.marked["acc2"] != domain.OnboardingNudgeStageKnowledge ||
		store.marked["acc3"] != domain.OnboardingNudgeStageShare {
		t.Errorf("stamps should advance to the stage sent, got %v", store.marked)
	}
}

// The offers queue already carries a query string: the UTMs append to it.
func TestSendOnboardingNudges_QueueLinkKeepsItsQuery(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{due: []domain.OnboardingNudgeAccount{
		{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 3), StageSent: 1},
	}}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{
		"acc1": progressWith(OnboardingMilestoneChannel, OnboardingMilestoneProducts, OnboardingMilestoneCustomers),
	}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{"acc1": {Email: "a@x.com", Name: "Ana"}}}
	notifier := &fakeOnboardingNudgeNotifier{}

	if _, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := notifier.sent["a@x.com"]
	if got.milestone != OnboardingMilestoneProposal {
		t.Fatalf("expected the proposal nudge, got %+v", got)
	}
	want := "https://app.replider.com/prospect?tab=queue&utm_source=lifecycle&utm_medium=email&utm_campaign=onboarding_nudge&utm_content=d3_proposal"
	if got.url != want {
		t.Errorf("url = %s, want %s", got.url, want)
	}
}

// Fully onboarded: no email, but the stamp still advances so the drip ends.
func TestSendOnboardingNudges_SkipsAccountsAt100Percent(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{due: []domain.OnboardingNudgeAccount{
		{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 1), StageSent: 0},
	}}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{
		"acc1": progressWith(onboardingMilestoneOrder...),
	}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{"acc1": {Email: "a@x.com", Name: "Ana"}}}
	notifier := &fakeOnboardingNudgeNotifier{}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || len(notifier.sent) != 0 {
		t.Fatalf("nothing should be sent to a finished account, got %d / %v", sent, notifier.sent)
	}
	if store.marked["acc1"] != domain.OnboardingNudgeStageChannel {
		t.Errorf("stamp should advance on skip, got %d", store.marked["acc1"])
	}
}

func TestSendOnboardingNudges_SkipsWhenNoMilestoneDue(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{
		due: []domain.OnboardingNudgeAccount{
			// 0 days old — no milestone elapsed yet.
			{AccountID: "acc1", Name: "A", CreatedAt: now, StageSent: 0},
			// 5 days old but D3 already sent; D7 not yet reached.
			{AccountID: "acc2", Name: "B", CreatedAt: daysAgo(now, 5), StageSent: 3},
		},
	}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{
		"acc1": progressWith(), "acc2": progressWith(),
	}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
		"acc2": {Email: "b@x.com", Name: "Bob"},
	}}
	notifier := &fakeOnboardingNudgeNotifier{}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent, got %d", sent)
	}
	if len(store.marked) != 0 {
		t.Errorf("nothing should be marked, got %v", store.marked)
	}
}

func TestSendOnboardingNudges_BacklogCollapsesToLatest(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{
		due: []domain.OnboardingNudgeAccount{
			// 8 days old, nothing sent: a worker-down backlog collapses to
			// the single latest elapsed stage (D7), not a burst of D1+D3+D7.
			{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 8), StageSent: 0},
		},
	}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{"acc1": progressWith()}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{"acc1": {Email: "a@x.com", Name: "Ana"}}}
	notifier := &fakeOnboardingNudgeNotifier{}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 1 {
		t.Fatalf("expected 1 sent, got %d", sent)
	}
	if store.marked["acc1"] != domain.OnboardingNudgeStageShare {
		t.Errorf("backlog should stamp D7, got %d", store.marked["acc1"])
	}
	if got := notifier.sent["a@x.com"]; got.milestone != OnboardingMilestoneChannel {
		t.Errorf("a D7 nudge to an account that did nothing still asks for the first step, got %+v", got)
	}
}

func TestSendOnboardingNudges_NoAdminNotMarked(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{due: []domain.OnboardingNudgeAccount{
		{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 1), StageSent: 0},
	}}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{"acc1": progressWith()}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{}} // no admin
	notifier := &fakeOnboardingNudgeNotifier{}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || len(store.marked) != 0 {
		t.Errorf("account with no admin must not be marked (retry next tick), got %d / %v", sent, store.marked)
	}
}

// A progress read that fails is retried next tick, never stamped: stamping
// would silently end the drip for an account we know nothing about.
func TestSendOnboardingNudges_ProgressErrorNotMarked(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{due: []domain.OnboardingNudgeAccount{
		{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 1), StageSent: 0},
	}}
	progress := &fakeNudgeProgress{err: errors.New("db down")}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{"acc1": {Email: "a@x.com", Name: "Ana"}}}
	notifier := &fakeOnboardingNudgeNotifier{}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || len(store.marked) != 0 || len(notifier.sent) != 0 {
		t.Errorf("progress failure must send and mark nothing, got %d / %v / %v", sent, store.marked, notifier.sent)
	}
}

func TestSendOnboardingNudges_SendErrorNotMarked(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeOnboardingNudgeStore{due: []domain.OnboardingNudgeAccount{
		{AccountID: "acc1", Name: "A", CreatedAt: daysAgo(now, 1), StageSent: 0},
	}}
	progress := &fakeNudgeProgress{byAccount: map[string]*domain.OnboardingProgress{"acc1": progressWith()}}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{"acc1": {Email: "a@x.com", Name: "Ana"}}}
	notifier := &fakeOnboardingNudgeNotifier{err: errors.New("smtp down")}

	sent, err := newNudgeUC(store, admins, notifier, progress, now).Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || len(store.marked) != 0 {
		t.Errorf("failed send must not be marked, got %d / %v", sent, store.marked)
	}
}

func TestSendOnboardingNudges_ListErrorPropagates(t *testing.T) {
	store := &fakeOnboardingNudgeStore{listErr: errors.New("db down")}
	uc := NewSendOnboardingNudgesUseCase(store, &fakeAdminResolver{}, &fakeOnboardingNudgeNotifier{}, &fakeNudgeProgress{}, "https://app.replider.com")
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected list error to propagate")
	}
}

func TestNextOnboardingMilestone(t *testing.T) {
	cases := []struct {
		name string
		done []string
		want string
	}{
		{"nothing done", nil, OnboardingMilestoneChannel},
		{"channel only", []string{OnboardingMilestoneChannel}, OnboardingMilestoneProducts},
		{"setup done", []string{OnboardingMilestoneChannel, OnboardingMilestoneProducts, OnboardingMilestoneCustomers}, OnboardingMilestoneProposal},
		{"out of order: products before channel still asks for the channel", []string{OnboardingMilestoneProducts}, OnboardingMilestoneChannel},
		{"all done", onboardingMilestoneOrder, ""},
	}
	for _, c := range cases {
		if got := NextOnboardingMilestone(progressWith(c.done...)); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
	if got := NextOnboardingMilestone(nil); got != OnboardingMilestoneChannel {
		t.Errorf("nil progress should ask for the first step, got %q", got)
	}
}

func TestNextOnboardingNudgeStage(t *testing.T) {
	cases := []struct {
		daysSince, stageSent, want int
	}{
		{0, 0, 0}, // nothing elapsed
		{1, 0, 1}, // exactly D1
		{2, 1, 0}, // D1 sent, D3 not reached
		{3, 1, 3}, // D3 due
		{5, 0, 3}, // backlog collapses to latest elapsed (3), not 1
		{7, 3, 7}, // D7 due
		{9, 7, 0}, // all sent
	}
	for _, c := range cases {
		if got := domain.NextOnboardingNudgeStage(c.daysSince, c.stageSent); got != c.want {
			t.Errorf("NextOnboardingNudgeStage(%d,%d)=%d want %d", c.daysSince, c.stageSent, got, c.want)
		}
	}
}
