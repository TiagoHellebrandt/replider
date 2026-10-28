package usecases

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mocks (narrow interfaces only) ---

type mockSuggestionCarteira struct {
	accounts []domain.SellerCarteiraAccount
	calls    int
}

func (m *mockSuggestionCarteira) ListCarteiraAccounts(_ context.Context, _, _ string) ([]domain.SellerCarteiraAccount, error) {
	m.calls++
	return m.accounts, nil
}

type mockSuggestionSignalRepo struct {
	convStats []domain.SellerConvStats
	risks     []domain.SellerChannelRisk
	nps       []domain.SellerNPSSignal
	states    []domain.SellerSuggestionState
	upserts   []domain.SellerSuggestionState
	// onListSuggestionStates runs AFTER the states result is snapshotted
	// — lets a test interleave a concurrent dismissal with an in-flight
	// compute that already read pre-dismissal state.
	onListSuggestionStates func()
}

func (m *mockSuggestionSignalRepo) ListConversationStats(_ context.Context, _ string, _, _, _, _ time.Time) ([]domain.SellerConvStats, error) {
	return m.convStats, nil
}
func (m *mockSuggestionSignalRepo) ListWhatsAppRisk(_ context.Context, _ string) ([]domain.SellerChannelRisk, error) {
	return m.risks, nil
}
func (m *mockSuggestionSignalRepo) ListLatestNPS(_ context.Context, _ string, _, _ time.Time) ([]domain.SellerNPSSignal, error) {
	return m.nps, nil
}
func (m *mockSuggestionSignalRepo) ListSuggestionStates(_ context.Context, _, cycle string) ([]domain.SellerSuggestionState, error) {
	out := []domain.SellerSuggestionState{}
	for _, s := range append(append([]domain.SellerSuggestionState{}, m.states...), m.upserts...) {
		if s.Cycle == cycle {
			out = append(out, s)
		}
	}
	if m.onListSuggestionStates != nil {
		hook := m.onListSuggestionStates
		m.onListSuggestionStates = nil // one-shot; SetState re-enters via ListCarteiraAccounts only
		hook()
	}
	return out, nil
}
func (m *mockSuggestionSignalRepo) UpsertSuggestionState(_ context.Context, _, accountID, suggestionType, channelID, cycle, status string) error {
	m.upserts = append(m.upserts, domain.SellerSuggestionState{
		AccountID: accountID, SuggestionType: suggestionType, ChannelID: channelID, Cycle: cycle, Status: status,
	})
	return nil
}

type mockSuggestionOnboarding struct {
	progress map[string]*domain.OnboardingProgress
}

func (m *mockSuggestionOnboarding) GetOnboardingProgress(_ context.Context, accountID string) (*domain.OnboardingProgress, error) {
	return m.progress[accountID], nil
}

type mockSuggestionPlans struct {
	plans map[string]*domain.Plan
	calls int
}

func (m *mockSuggestionPlans) GetByID(_ context.Context, id string) (*domain.Plan, error) {
	m.calls++
	return m.plans[id], nil
}

// --- fixture helpers ---

// suggestionNow: July 15th → cycle 2026-07, 15 of 31 days elapsed.
var suggestionNow = time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

func i32p(v int32) *int32 { return &v }
func i64p(v int64) *int64 { return &v }

type suggestionFixture struct {
	uc       *SellerSuggestionsUseCase
	carteira *mockSuggestionCarteira
	repo     *mockSuggestionSignalRepo
	accounts *mockSuggestionOnboarding
	plans    *mockSuggestionPlans
}

// newSuggestionFixture builds one healthy carteira account ("acc-1",
// named "Padaria") that triggers NO signal: old enough, onboarding
// complete, low usage, quiet conversations. Tests then flip one signal.
// Note the account/usage data rides ON the carteira row (US-022 N+1
// fix) and the onboarding progress carries only the raw milestone
// booleans — CompletionPercentage stays 0 exactly like the repo returns
// it; the engine must derive it.
func newSuggestionFixture() *suggestionFixture {
	priceID := "price_x"
	planID := "plan-1"
	carteira := &mockSuggestionCarteira{accounts: []domain.SellerCarteiraAccount{
		{
			AccountID: "acc-1", Name: "Padaria", SubscriptionStatus: "active",
			CreatedAt: suggestionNow.AddDate(0, 0, -60), PlanID: &planID,
			UsageConversations: i32p(40), UsageAITokens: i64p(1000), UsageAddonTokens: i64p(0),
		},
	}}
	repo := &mockSuggestionSignalRepo{
		convStats: []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 2, PrevTotal: 60}},
	}
	accounts := &mockSuggestionOnboarding{
		progress: map[string]*domain.OnboardingProgress{
			// Fully onboarded means the SALES six are done.
			"acc-1": {
				ChannelConnected: true, ProductsAdded: true, CustomersAdded: true,
				ProposalMade: true, FirstInboundReceived: true, TeamInvited: true,
			},
		},
	}
	plans := &mockSuggestionPlans{plans: map[string]*domain.Plan{
		planID: {ID: planID, StripePriceID: &priceID, MaxConversationsPerMonth: 100, MaxAITokensPerMonth: 100000},
	}}
	uc := NewSellerSuggestionsUseCase(carteira, repo, accounts, plans)
	uc.now = func() time.Time { return suggestionNow }
	return &suggestionFixture{uc: uc, carteira: carteira, repo: repo, accounts: accounts, plans: plans}
}

func suggestionTypes(feed *SellerSuggestionsFeed) []string {
	out := []string{}
	for _, s := range feed.Suggestions {
		out = append(out, s.Type)
	}
	return out
}

// --- tests ---

func TestSuggestions_HealthyAccountHasNoSignals(t *testing.T) {
	f := newSuggestionFixture()
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Equal(t, "2026-07", feed.Cycle)
	assert.Empty(t, feed.Suggestions)
}

// Regression (US-022): the repo's GetOnboardingProgress returns raw
// milestone booleans with CompletionPercentage zero — only the
// onboarding endpoint's use case derived the percentage. Trusting the
// zero flagged EVERY account older than 7 days as onboarding_stuck,
// fully onboarded ones included. The engine must derive the percentage
// from the booleans itself.
func TestSuggestions_FullyOnboardedAccountIsNotStuck(t *testing.T) {
	f := newSuggestionFixture()
	// All six SALES milestones done, CompletionPercentage deliberately left 0
	// (what the repo actually returns — it is derived, not stored).
	f.accounts.progress["acc-1"] = &domain.OnboardingProgress{
		ChannelConnected: true, ProductsAdded: true, CustomersAdded: true,
		ProposalMade: true, FirstInboundReceived: true, TeamInvited: true,
		CompletionPercentage: 0,
	}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions, "a 100%%-onboarded account must never be flagged onboarding_stuck")
}

// Regression (US-022): an account deleted between the carteira read and
// the per-account onboarding lookup used to 500 the WHOLE feed (the old
// per-account GetByID returned nil and the error path wrapped a nil
// err). It must be skipped; the other accounts' signals survive.
func TestSuggestions_DeletedAccountIsSkippedNotFatal(t *testing.T) {
	f := newSuggestionFixture()
	planID := "plan-1"
	f.carteira.accounts = append(f.carteira.accounts, domain.SellerCarteiraAccount{
		AccountID: "acc-gone", Name: "Deletada", SubscriptionStatus: "active",
		CreatedAt: suggestionNow.AddDate(0, 0, -30), PlanID: &planID,
	})
	// No progress entry for acc-gone → GetOnboardingProgress returns nil
	// (deleted account). acc-1 still has a live signal:
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 16, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err, "a vanished account must not 500 the whole feed")
	assert.Equal(t, []string{domain.SuggestionAddKnowledge}, suggestionTypes(feed))
}

func TestSuggestions_TakeoverRateAbove30(t *testing.T) {
	f := newSuggestionFixture()
	// 16 human-touched of 40 = 40% > 30%.
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 16, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionAddKnowledge}, suggestionTypes(feed))
	s := feed.Suggestions[0]
	assert.Equal(t, domain.SuggestionSeverityWarning, s.Severity)
	assert.Equal(t, "Padaria", s.AccountName)
	assert.Equal(t, 40, s.Signal["takeover_pct"])
	assert.NotEmpty(t, s.ActionText)
}

func TestSuggestions_TakeoverRateExactly30IsNotABreach(t *testing.T) {
	f := newSuggestionFixture()
	// 12 of 40 = exactly 30% — the rule is strictly above.
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 12, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

func TestSuggestions_ChurnPaceDrop(t *testing.T) {
	f := newSuggestionFixture()
	// Pro-rated: 60 prev × 15/31 elapsed ≈ 29 expected; 5 current is
	// well under half of that.
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 5, CurrentHuman: 0, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionChurnRiskCheckin}, suggestionTypes(feed))
	s := feed.Suggestions[0]
	assert.Equal(t, domain.SuggestionSeverityCritical, s.Severity)
	assert.Equal(t, 5, s.Signal["current_month_conversations"])
	assert.Equal(t, 60, s.Signal["previous_month_conversations"])
}

func TestSuggestions_ChurnPaceOnTrackDoesNotTrigger(t *testing.T) {
	f := newSuggestionFixture()
	// 25 by day 15 of 31 vs 60 last month — on pace, no signal.
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 25, CurrentHuman: 0, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

func TestSuggestions_ChurnPaceGuards(t *testing.T) {
	f := newSuggestionFixture()
	// Day 5 of the month: too early to trust the pace even with a drop.
	f.uc.now = func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 0, CurrentHuman: 0, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)

	// New account: previous month had zero conversations — no baseline.
	f2 := newSuggestionFixture()
	f2.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 0, CurrentHuman: 0, PrevTotal: 0}}
	feed, err = f2.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

func TestSuggestions_UsageAt80PercentConversations(t *testing.T) {
	f := newSuggestionFixture()
	f.carteira.accounts[0].UsageConversations = i32p(80) // 80 of 100 = exactly 80% → triggers (>=)
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionSuggestUpgrade}, suggestionTypes(feed))
	s := feed.Suggestions[0]
	assert.Equal(t, domain.SuggestionSeverityInfo, s.Severity)
	assert.Equal(t, int64(80), s.Signal["conversations_used"])
	assert.Equal(t, int64(100), s.Signal["conversations_limit"])
}

func TestSuggestions_UsageAt80PercentTokens(t *testing.T) {
	f := newSuggestionFixture()
	f.carteira.accounts[0].UsageAITokens = i64p(85000) // 85% of 100k
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionSuggestUpgrade}, suggestionTypes(feed))
	assert.Equal(t, int64(85000), feed.Suggestions[0].Signal["tokens_used"])
}

func TestSuggestions_UsageHonorsEffectiveMaxOverride(t *testing.T) {
	f := newSuggestionFixture()
	// Plan cap is 100 but the account has a 500 override — 90 used is
	// 18%, not 90%. Raw plan caps would false-positive here (footgun).
	override := 500
	f.carteira.accounts[0].MaxConversationsPerMonthOverride = &override
	f.carteira.accounts[0].UsageConversations = i32p(90)
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

func TestSuggestions_UsageSkipsUnnegotiatedContractPlan(t *testing.T) {
	f := newSuggestionFixture()
	// Contract plan (no Stripe price) with 0 caps → teto de plataforma;
	// heavy usage must not suggest an upgrade.
	contractID := "plan-contract"
	f.carteira.accounts[0].PlanID = &contractID
	f.plans.plans = map[string]*domain.Plan{
		contractID: {ID: contractID, StripePriceID: nil},
	}
	f.carteira.accounts[0].UsageConversations = i32p(999999)
	f.carteira.accounts[0].UsageAITokens = i64p(999999999)
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

// N+1 (US-022): the plan is memoized per compute pass — 50 carteira
// accounts on the same plan must cost ONE plan lookup, not 50.
func TestSuggestions_PlanLookupMemoizedAcrossAccounts(t *testing.T) {
	f := newSuggestionFixture()
	planID := "plan-1"
	for i := 0; i < 4; i++ {
		f.carteira.accounts = append(f.carteira.accounts, domain.SellerCarteiraAccount{
			AccountID: fmt.Sprintf("acc-%d", i+2), Name: "Filial", SubscriptionStatus: "active",
			CreatedAt: suggestionNow.AddDate(0, 0, -60), PlanID: &planID,
			UsageConversations: i32p(1), UsageAITokens: i64p(1),
		})
		f.accounts.progress[fmt.Sprintf("acc-%d", i+2)] = f.accounts.progress["acc-1"]
	}
	_, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Equal(t, 1, f.plans.calls, "same plan across the carteira must be fetched once")
}

func TestSuggestions_OnboardingStuckAfter7Days(t *testing.T) {
	f := newSuggestionFixture()
	f.carteira.accounts[0].CreatedAt = suggestionNow.AddDate(0, 0, -10)
	// The milestones that count are the SALES ones: an account with an agent and
	// a channel but nothing to sell is not 40% onboarded, it is 20%.
	f.accounts.progress["acc-1"] = &domain.OnboardingProgress{
		AgentCreated: true, ChannelConnected: true,
	}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionOnboardingStuck}, suggestionTypes(feed))
	s := feed.Suggestions[0]
	assert.Equal(t, domain.SuggestionSeverityWarning, s.Severity)
	assert.Equal(t, 16, s.Signal["completion_percentage"], "percentage derived from the SALES milestones (1 of 6)")
	// What is missing is named in the same terms the percentage counts, or the
	// seller calls about the wrong thing.
	assert.Equal(t,
		[]string{"products", "customers", "proposal", "first_inbound", "team"},
		s.Signal["missing_steps"])
}

func TestSuggestions_OnboardingFreshAccountNotFlagged(t *testing.T) {
	f := newSuggestionFixture()
	// 3 days old with an incomplete checklist: still inside the grace
	// window.
	f.carteira.accounts[0].CreatedAt = suggestionNow.AddDate(0, 0, -3)
	f.accounts.progress["acc-1"] = &domain.OnboardingProgress{AgentCreated: true}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

// US-022: ban risk is per channel — one suggestion per RED number, each
// dismissible on its own.
func TestSuggestions_WhatsAppBanRiskPerChannel(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.risks = []domain.SellerChannelRisk{
		{AccountID: "acc-1", ChannelID: "ch-1", ChannelName: "Vendas", QualityRating: "RED"},
		{AccountID: "acc-1", ChannelID: "ch-2", ChannelName: "Suporte", QualityRating: "RED"},
	}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionWhatsAppBanRisk, domain.SuggestionWhatsAppBanRisk}, suggestionTypes(feed))
	require.NotNil(t, feed.Suggestions[0].ChannelID)
	require.NotNil(t, feed.Suggestions[1].ChannelID)
	assert.ElementsMatch(t,
		[]string{*feed.Suggestions[0].ChannelID, *feed.Suggestions[1].ChannelID},
		[]string{"ch-1", "ch-2"})
	assert.Equal(t, domain.SuggestionSeverityCritical, feed.Suggestions[0].Severity)
}

// Regression (US-022): dismissing ONE channel's ban risk must not hide
// a different channel of the same account — the old dismissal key was
// (account, type) so the first dismiss swallowed every later RED number
// for the rest of the cycle.
func TestSuggestions_BanRiskDismissalIsChannelScoped(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.risks = []domain.SellerChannelRisk{
		{AccountID: "acc-1", ChannelID: "ch-1", ChannelName: "Vendas", QualityRating: "RED"},
		{AccountID: "acc-1", ChannelID: "ch-2", ChannelName: "Suporte", QualityRating: "RED"},
	}
	f.repo.states = []domain.SellerSuggestionState{
		{AccountID: "acc-1", SuggestionType: domain.SuggestionWhatsAppBanRisk, ChannelID: "ch-1", Cycle: "2026-07", Status: domain.SuggestionStatusDismissed},
	}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Len(t, feed.Suggestions, 1, "only ch-1 was dismissed; ch-2's risk must survive")
	require.NotNil(t, feed.Suggestions[0].ChannelID)
	assert.Equal(t, "ch-2", *feed.Suggestions[0].ChannelID)
}

func TestSuggestions_NPSPromoterAndDetractor(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.nps = []domain.SellerNPSSignal{{AccountID: "acc-1", Score: 10, CreatedAt: suggestionNow.AddDate(0, 0, -2)}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionNPSPromoter}, suggestionTypes(feed))
	assert.Equal(t, domain.SuggestionSeverityInfo, feed.Suggestions[0].Severity)

	f2 := newSuggestionFixture()
	f2.repo.nps = []domain.SellerNPSSignal{{AccountID: "acc-1", Score: 3, CreatedAt: suggestionNow.AddDate(0, 0, -2)}}
	feed, err = f2.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Equal(t, []string{domain.SuggestionNPSDetractor}, suggestionTypes(feed))
	assert.Equal(t, domain.SuggestionSeverityCritical, feed.Suggestions[0].Severity)

	// Passive (7-8) stays silent.
	f3 := newSuggestionFixture()
	f3.repo.nps = []domain.SellerNPSSignal{{AccountID: "acc-1", Score: 7, CreatedAt: suggestionNow.AddDate(0, 0, -2)}}
	feed, err = f3.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

func TestSuggestions_DismissedThisCycleIsFiltered(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 20, PrevTotal: 60}}
	f.repo.nps = []domain.SellerNPSSignal{{AccountID: "acc-1", Score: 10, CreatedAt: suggestionNow.AddDate(0, 0, -1)}}
	f.repo.states = []domain.SellerSuggestionState{
		{AccountID: "acc-1", SuggestionType: domain.SuggestionAddKnowledge, Cycle: "2026-07", Status: domain.SuggestionStatusDismissed},
	}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	// add_knowledge dismissed → only the NPS promoter survives.
	assert.Equal(t, []string{domain.SuggestionNPSPromoter}, suggestionTypes(feed))
}

func TestSuggestions_DismissalFromPastCycleDoesNotStick(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 20, PrevTotal: 60}}
	f.repo.states = []domain.SellerSuggestionState{
		{AccountID: "acc-1", SuggestionType: domain.SuggestionAddKnowledge, Cycle: "2026-06", Status: domain.SuggestionStatusDismissed},
	}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	// June's dismissal expired — the signal resurfaces in July.
	assert.Equal(t, []string{domain.SuggestionAddKnowledge}, suggestionTypes(feed))
}

func TestSuggestions_SeverityOrdering(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 20, PrevTotal: 60}}            // warning
	f.repo.risks = []domain.SellerChannelRisk{{AccountID: "acc-1", ChannelID: "ch-1", ChannelName: "Vendas", QualityRating: "RED"}} // critical
	f.repo.nps = []domain.SellerNPSSignal{{AccountID: "acc-1", Score: 9, CreatedAt: suggestionNow.AddDate(0, 0, -1)}}               // info
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Equal(t, []string{
		domain.SuggestionWhatsAppBanRisk,
		domain.SuggestionAddKnowledge,
		domain.SuggestionNPSPromoter,
	}, suggestionTypes(feed))
}

func TestSuggestions_CacheServesAndInvalidatesOnSetState(t *testing.T) {
	f := newSuggestionFixture()
	_, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	_, err = f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Equal(t, 1, f.carteira.calls, "second call within the TTL must hit the cache")

	// SetState invalidates: the next Execute recomputes.
	require.NoError(t, f.uc.SetState(context.Background(), "seller-1", "acc-1", domain.SuggestionAddKnowledge, "", domain.SuggestionStatusDismissed))
	_, err = f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	// SetState itself lists the carteira once (ownership check) + the
	// recompute: 3 total.
	assert.Equal(t, 3, f.carteira.calls)

	// A different seller never shares the cache entry.
	f.carteira.accounts = nil
	feed, err := f.uc.Execute(context.Background(), "seller-2")
	require.NoError(t, err)
	assert.Empty(t, feed.Suggestions)
}

// Regression (US-022): a dismissal landing WHILE a compute is in flight
// must not be overwritten by that stale compute's store — the old code
// cached the pre-dismissal feed after SetState's invalidate, so the
// dismissed suggestion kept resurfacing for the whole 60s TTL.
func TestSuggestions_InFlightComputeDoesNotOverwriteConcurrentDismiss(t *testing.T) {
	f := newSuggestionFixture()
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 20, PrevTotal: 60}}

	// Mid-Execute (after the compute already read the states) a
	// concurrent request dismisses add_knowledge.
	f.repo.onListSuggestionStates = func() {
		require.NoError(t, f.uc.SetState(context.Background(), "seller-1", "acc-1", domain.SuggestionAddKnowledge, "", domain.SuggestionStatusDismissed))
	}

	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	// The in-flight compute legitimately still carries the suggestion —
	// it read pre-dismissal state. What must NOT happen is that result
	// getting cached.
	assert.Equal(t, []string{domain.SuggestionAddKnowledge}, suggestionTypes(feed))

	feed, err = f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	assert.Empty(t, suggestionTypes(feed),
		"the post-dismiss Execute must recompute (stale in-flight feed must not have been cached)")
}

func TestSetState_PersistsCurrentCycle(t *testing.T) {
	f := newSuggestionFixture()
	require.NoError(t, f.uc.SetState(context.Background(), "seller-1", "acc-1", domain.SuggestionNPSPromoter, "", domain.SuggestionStatusDone))
	require.Len(t, f.repo.upserts, 1)
	up := f.repo.upserts[0]
	assert.Equal(t, "acc-1", up.AccountID)
	assert.Equal(t, domain.SuggestionNPSPromoter, up.SuggestionType)
	assert.Equal(t, "2026-07", up.Cycle)
	assert.Equal(t, domain.SuggestionStatusDone, up.Status)
}

// US-022: ban-risk dismissals persist the channel key; account-level
// types drop a stray channel id (the key must stay canonical).
func TestSetState_ChannelScoping(t *testing.T) {
	f := newSuggestionFixture()
	require.NoError(t, f.uc.SetState(context.Background(), "seller-1", "acc-1", domain.SuggestionWhatsAppBanRisk, "ch-1", domain.SuggestionStatusDismissed))
	require.NoError(t, f.uc.SetState(context.Background(), "seller-1", "acc-1", domain.SuggestionNPSPromoter, "ch-1", domain.SuggestionStatusDone))
	require.Len(t, f.repo.upserts, 2)
	assert.Equal(t, "ch-1", f.repo.upserts[0].ChannelID)
	assert.Equal(t, "", f.repo.upserts[1].ChannelID, "channel id is meaningless on account-level types")
}

func TestSetState_RejectsUnknownTypeOrStatus(t *testing.T) {
	f := newSuggestionFixture()
	err := f.uc.SetState(context.Background(), "seller-1", "acc-1", "bogus_type", "", domain.SuggestionStatusDone)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "SELLER_INVALID_SUGGESTION", appErr.Code)

	err = f.uc.SetState(context.Background(), "seller-1", "acc-1", domain.SuggestionNPSPromoter, "", "archived")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "SELLER_INVALID_SUGGESTION", appErr.Code)
	assert.Empty(t, f.repo.upserts)
}

func TestSetState_RejectsAccountOutsideCarteira(t *testing.T) {
	f := newSuggestionFixture()
	err := f.uc.SetState(context.Background(), "seller-1", "acc-other", domain.SuggestionNPSPromoter, "", domain.SuggestionStatusDismissed)
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
	assert.Empty(t, f.repo.upserts)
}

func TestSuggestions_CarriesConvertedConversationDeepLink(t *testing.T) {
	// US-015: when the account was converted from a platform conversation
	// (US-004 stamp), every suggestion for it carries the conversation id
	// so the feed can open the chat with a pre-filled template. Absent
	// (?ref= signup) it stays nil.
	f := newSuggestionFixture()
	convID := "CONVDEEPLINK000000000000000"
	f.carteira.accounts[0].ConvertedConversationID = &convID
	f.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 16, PrevTotal: 60}}
	feed, err := f.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Len(t, feed.Suggestions, 1)
	require.NotNil(t, feed.Suggestions[0].ConversationID)
	assert.Equal(t, convID, *feed.Suggestions[0].ConversationID)

	f2 := newSuggestionFixture()
	f2.repo.convStats = []domain.SellerConvStats{{AccountID: "acc-1", CurrentTotal: 40, CurrentHuman: 16, PrevTotal: 60}}
	feed2, err := f2.uc.Execute(context.Background(), "seller-1")
	require.NoError(t, err)
	require.Len(t, feed2.Suggestions, 1)
	assert.Nil(t, feed2.Suggestions[0].ConversationID)
}
