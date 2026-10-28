package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// ── Stubs ──────────────────────────────────────────────────────────────────

type stubChannelRepo struct {
	domain.ChannelRepository
	channel *domain.Channel
	err     error
}

func (s *stubChannelRepo) GetByID(_ context.Context, _ string) (*domain.Channel, error) {
	return s.channel, s.err
}

type stubSender struct {
	mu       sync.Mutex
	sent     []string // recipients sent in order
	failOn   map[string]error
	wamidSeq int
}

func (s *stubSender) SendTemplateAndGetID(
	_ context.Context, _ *domain.Channel, to string,
	_, _ string, _ []interface{},
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.failOn[to]; ok {
		return "", err
	}
	s.sent = append(s.sent, to)
	s.wamidSeq++
	return "wamid.test." + to, nil
}

// stubCampaignRepo is an in-memory CampaignRepository implementation tuned
// for the worker's access patterns (drain pending, mark sent/failed, update
// status). Not concurrency-safe across multiple campaigns; the worker uses
// MaxConcurrentCampaigns=1 so a single mutex is enough.
type stubCampaignRepo struct {
	mu          sync.Mutex
	campaigns   map[string]*domain.Campaign
	recipients  map[string][]*domain.CampaignRecipient // by campaign id
	statusCalls []string                               // recorded campaign status updates
	counters    map[string]map[domain.RecipientStatus]int
	sentMarks   map[string]string // recipientID → wamid
	failedMarks map[string]string
}

func newStubCampaignRepo() *stubCampaignRepo {
	return &stubCampaignRepo{
		campaigns:   map[string]*domain.Campaign{},
		recipients:  map[string][]*domain.CampaignRecipient{},
		counters:    map[string]map[domain.RecipientStatus]int{},
		sentMarks:   map[string]string{},
		failedMarks: map[string]string{},
	}
}

func (r *stubCampaignRepo) Create(_ context.Context, c *domain.Campaign) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.campaigns[c.ID] = c
	return nil
}
func (r *stubCampaignRepo) GetByID(_ context.Context, id, accountID string) (*domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.campaigns[id]
	if c == nil || c.AccountID != accountID {
		return nil, nil
	}
	return c, nil
}
func (r *stubCampaignRepo) ListByAccount(_ context.Context, _ string, _, _ int32) ([]*domain.Campaign, error) {
	return nil, nil
}
func (r *stubCampaignRepo) UpdateStatus(_ context.Context, id string, status domain.CampaignStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.campaigns[id]; ok {
		c.Status = status
	}
	r.statusCalls = append(r.statusCalls, string(status))
	return nil
}
func (r *stubCampaignRepo) IncrementCounter(_ context.Context, id string, status domain.RecipientStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.counters[id] == nil {
		r.counters[id] = map[domain.RecipientStatus]int{}
	}
	r.counters[id][status]++
	return nil
}
func (r *stubCampaignRepo) Delete(_ context.Context, _, _ string) error { return nil }
func (r *stubCampaignRepo) ListScheduledDue(_ context.Context, limit int32) ([]*domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Campaign
	for _, c := range r.campaigns {
		if c.Status == domain.CampaignScheduled {
			out = append(out, c)
			if int32(len(out)) >= limit {
				break
			}
		}
	}
	return out, nil
}
func (r *stubCampaignRepo) CreateRecipient(_ context.Context, rec *domain.CampaignRecipient) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recipients[rec.CampaignID] = append(r.recipients[rec.CampaignID], rec)
	return nil
}
func (r *stubCampaignRepo) ListRecipients(_ context.Context, campaignID string, limit, _ int32) ([]*domain.CampaignRecipient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := r.recipients[campaignID]
	if int32(len(all)) > limit {
		return all[:limit], nil
	}
	return all, nil
}
func (r *stubCampaignRepo) ListPendingRecipients(_ context.Context, campaignID string, limit int32) ([]*domain.CampaignRecipient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var pending []*domain.CampaignRecipient
	for _, rec := range r.recipients[campaignID] {
		if rec.Status == domain.RecipientPending {
			pending = append(pending, rec)
			if int32(len(pending)) >= limit {
				break
			}
		}
	}
	return pending, nil
}
func (r *stubCampaignRepo) MarkRecipientSent(_ context.Context, recipientID, providerMsgID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sentMarks[recipientID] = providerMsgID
	for _, recs := range r.recipients {
		for _, rec := range recs {
			if rec.ID == recipientID {
				rec.Status = domain.RecipientSent
				rec.ProviderMsgID = &providerMsgID
				return nil
			}
		}
	}
	return nil
}
func (r *stubCampaignRepo) MarkRecipientFailed(_ context.Context, recipientID, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failedMarks[recipientID] = errMsg
	for _, recs := range r.recipients {
		for _, rec := range recs {
			if rec.ID == recipientID {
				rec.Status = domain.RecipientFailed
				return nil
			}
		}
	}
	return nil
}
func (r *stubCampaignRepo) UpdateRecipientStatusByProviderID(_ context.Context, _ string, _ domain.RecipientStatus) error {
	return nil
}
func (r *stubCampaignRepo) GetCampaignByProviderMsgID(_ context.Context, _ string) (*domain.Campaign, error) {
	return nil, nil
}

// helpers ──────────────────────────────────────────────────────────────────

func waChannelActive() *domain.Channel {
	return &domain.Channel{
		ID: "ch-1", AccountID: "acc-1", Type: "whatsapp",
		Name: "WA", IsActive: true,
	}
}

func seedCampaign(repo *stubCampaignRepo, recipients ...string) *domain.Campaign {
	c := &domain.Campaign{
		ID: "camp-1", AccountID: "acc-1", ChannelID: "ch-1",
		TemplateName: "boas_vindas", TemplateLang: "pt_BR",
		Status:          domain.CampaignScheduled,
		TotalRecipients: len(recipients),
	}
	repo.campaigns[c.ID] = c
	for i, r := range recipients {
		repo.recipients[c.ID] = append(repo.recipients[c.ID], &domain.CampaignRecipient{
			ID: "r-" + string(rune('a'+i)), CampaignID: c.ID, Recipient: r,
			Status: domain.RecipientPending,
		})
	}
	return c
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestWorker_DrainsAllRecipients_MarksDone(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "5511111", "5511222", "5511333")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{
		PollInterval: time.Hour, // disabled — we drive ticks manually
		BatchSize:    10,
		SendDelay:    0,
	})
	w.tick(context.Background())

	assert.Equal(t, []string{"5511111", "5511222", "5511333"}, sender.sent)
	assert.Equal(t, 3, repo.counters["camp-1"][domain.RecipientSent])
	// Status went scheduled → sending → done
	assert.Equal(t, []string{"sending", "done"}, repo.statusCalls)
	assert.Equal(t, "wamid.test.5511111", repo.sentMarks["r-a"])
}

func TestWorker_PartialFailures_StillCountsDone(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "5511111", "5511222", "5511333")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{
		failOn: map[string]error{"5511222": errors.New("bad number")},
	}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.tick(context.Background())

	assert.Equal(t, 2, repo.counters["camp-1"][domain.RecipientSent])
	assert.Equal(t, 1, repo.counters["camp-1"][domain.RecipientFailed])
	assert.Contains(t, repo.failedMarks["r-b"], "bad number")
	// Final status is "done" because at least one was sent.
	assert.Equal(t, "done", repo.statusCalls[len(repo.statusCalls)-1])
}

func TestWorker_AllFail_MarksFailed(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "x", "y")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{
		failOn: map[string]error{
			"x": errors.New("boom"),
			"y": errors.New("boom"),
		},
	}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.tick(context.Background())

	assert.Equal(t, "failed", repo.statusCalls[len(repo.statusCalls)-1])
}

func TestWorker_DisabledChannel_FailsCampaign(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "x")
	ch := waChannelActive()
	ch.IsActive = false
	channelRepo := &stubChannelRepo{channel: ch}
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.tick(context.Background())

	assert.Empty(t, sender.sent)
	assert.Contains(t, repo.statusCalls, "failed")
}

func TestWorker_ChannelNotFound_FailsCampaign(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "x")
	channelRepo := &stubChannelRepo{channel: nil} // not found
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.tick(context.Background())

	assert.Contains(t, repo.statusCalls, "failed")
	assert.Empty(t, sender.sent)
}

func TestWorker_NonWhatsAppChannel_FailsCampaign(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "x")
	ch := waChannelActive()
	ch.Type = "telegram"
	channelRepo := &stubChannelRepo{channel: ch}
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.tick(context.Background())

	assert.Contains(t, repo.statusCalls, "failed")
}

func TestWorker_BatchesAcrossMultipleCalls(t *testing.T) {
	repo := newStubCampaignRepo()
	// 5 recipients, batch size 2 → must drain via 3 batches
	seedCampaign(repo, "a", "b", "c", "d", "e")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 2})
	w.tick(context.Background())

	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, sender.sent)
	assert.Equal(t, 5, repo.counters["camp-1"][domain.RecipientSent])
}

func TestWorker_RespectsContextCancellation(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "a", "b", "c")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}

	// Sender that blocks until cancelled, allowing us to verify the worker
	// stops mid-drain.
	var calls atomic.Int32
	blocking := &slowSender{
		calls: &calls,
		delay: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := NewCampaignWorker(repo, channelRepo, blocking, CampaignWorkerConfig{
		BatchSize: 10,
		SendDelay: 200 * time.Millisecond, // big delay so cancel hits during the wait
	})

	done := make(chan struct{})
	go func() {
		w.tick(ctx)
		close(done)
	}()

	// Let it process at least one before cancelling.
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not return after cancel")
	}
	// Should have stopped before processing all 3.
	assert.Less(t, int(calls.Load()), 3, "worker should have stopped early")
}

func TestWorker_SkipsAlreadySent_OnResume(t *testing.T) {
	repo := newStubCampaignRepo()
	c := seedCampaign(repo, "a", "b", "c")
	// Simulate a crash mid-flight: campaign already in 'sending', 1 recipient sent
	c.Status = domain.CampaignSending
	repo.recipients[c.ID][0].Status = domain.RecipientSent

	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.processCampaign(context.Background(), c)

	// Only 'b' and 'c' get re-sent. 'a' was already sent.
	assert.Equal(t, []string{"b", "c"}, sender.sent)
}

func TestMergeVars(t *testing.T) {
	cases := []struct {
		name     string
		camp     []string
		rec      []string
		expected []string
	}{
		{"empty rec", []string{"x", "y"}, nil, []string{"x", "y"}},
		{"rec overrides", []string{"x", "y"}, []string{"X", ""}, []string{"X", "y"}},
		{"rec longer", []string{"x"}, []string{"X", "Y"}, []string{"X", "Y"}},
		{"rec shorter falls back to camp", []string{"x", "y", "z"}, []string{"X"}, []string{"X", "y", "z"}},
		{"both empty", nil, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeVars(c.camp, c.rec)
			if c.expected == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, c.expected, got)
			}
		})
	}
}

func TestBuildBodyComponents(t *testing.T) {
	t.Run("empty vars returns nil", func(t *testing.T) {
		assert.Nil(t, buildBodyComponents(nil))
		assert.Nil(t, buildBodyComponents([]string{}))
	})
	t.Run("two vars", func(t *testing.T) {
		comps := buildBodyComponents([]string{"João", "12345"})
		assert.Len(t, comps, 1)
		body := comps[0].(map[string]interface{})
		assert.Equal(t, "body", body["type"])
		params := body["parameters"].([]map[string]string)
		assert.Equal(t, "text", params[0]["type"])
		assert.Equal(t, "João", params[0]["text"])
		assert.Equal(t, "12345", params[1]["text"])
	})
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "ab", truncate("abcd", 2))
	assert.Equal(t, "", truncate("abc", 0))
}

// ── Budget re-check at dispatch ────────────────────────────────────────────

type stubAccountRepo struct {
	domain.AccountRepository
	acc *domain.Account
}

func (s *stubAccountRepo) GetByID(_ context.Context, _ string) (*domain.Account, error) {
	return s.acc, nil
}

type stubPlanRepo struct {
	domain.PlanRepository
	plan *domain.Plan
}

func (s *stubPlanRepo) GetByID(_ context.Context, _ string) (*domain.Plan, error) {
	return s.plan, nil
}

type stubUsageRepo struct {
	domain.UsageRepository
	usage *domain.AccountUsage
}

func (s *stubUsageRepo) GetByAccountID(_ context.Context, _ string) (*domain.AccountUsage, error) {
	return s.usage, nil
}

func TestWorker_BudgetCheck_PlanDisallows_MarksFailed(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "a", "b")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	pid := "plan-1"
	// StripePriceID set → IsContractPlan=false; cap 0 stays 0 (paid plan
	// with broadcast disabled, e.g. legacy Solo). For contract plans cap 0
	// would resolve to UnlimitedLimit and this test would no longer assert
	// the rejection path.
	stripePriceID := "price_paid_test"
	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10}).
		WithBudgetCheck(
			&stubAccountRepo{acc: &domain.Account{ID: "acc-1", PlanID: &pid}},
			&stubPlanRepo{plan: &domain.Plan{ID: pid, StripePriceID: &stripePriceID, MaxBroadcastMessagesPerMonth: 0}},
			&stubUsageRepo{usage: nil},
		)

	w.tick(context.Background())

	assert.Empty(t, sender.sent, "no sends when plan disallows broadcasts")
	assert.Equal(t, "failed", repo.statusCalls[len(repo.statusCalls)-1])
}

func TestWorker_BudgetCheck_OverBudget_MarksFailed(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "a", "b", "c") // 3 pending
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	pid := "plan-1"
	stripePriceID := "price_paid_test"
	// Plan cap 5, usage shows 10 used (pre-flight reserved 3 for this campaign
	// → 10 = 7 pre-existing + 3 reserved). effectivelyUsed = 10 - 3 = 7.
	// 7 + 3 = 10 > 5 → over budget.
	used := int32(10)
	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10}).
		WithBudgetCheck(
			&stubAccountRepo{acc: &domain.Account{ID: "acc-1", PlanID: &pid}},
			&stubPlanRepo{plan: &domain.Plan{ID: pid, StripePriceID: &stripePriceID, MaxBroadcastMessagesPerMonth: 5}},
			&stubUsageRepo{usage: &domain.AccountUsage{BroadcastMessagesUsed: &used}},
		)

	w.tick(context.Background())

	assert.Empty(t, sender.sent)
	assert.Equal(t, "failed", repo.statusCalls[len(repo.statusCalls)-1])
}

func TestWorker_BudgetCheck_WithinBudget_Proceeds(t *testing.T) {
	repo := newStubCampaignRepo()
	seedCampaign(repo, "a", "b")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	pid := "plan-1"
	stripePriceID := "price_paid_test"
	// Usage 5, campaign has 2 pending → effectivelyUsed=3, 3+2=5 ≤ 10 → ok.
	used := int32(5)
	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10}).
		WithBudgetCheck(
			&stubAccountRepo{acc: &domain.Account{ID: "acc-1", PlanID: &pid}},
			&stubPlanRepo{plan: &domain.Plan{ID: pid, StripePriceID: &stripePriceID, MaxBroadcastMessagesPerMonth: 10}},
			&stubUsageRepo{usage: &domain.AccountUsage{BroadcastMessagesUsed: &used}},
		)

	w.tick(context.Background())

	assert.Equal(t, []string{"a", "b"}, sender.sent)
	assert.Equal(t, "done", repo.statusCalls[len(repo.statusCalls)-1])
}

func TestWorker_BudgetCheck_NoRepos_Skipped(t *testing.T) {
	// When budget repos are not wired, the worker must behave exactly like
	// the pre-budget-check version (backwards compat for tests / non-prod).
	repo := newStubCampaignRepo()
	seedCampaign(repo, "a")
	channelRepo := &stubChannelRepo{channel: waChannelActive()}
	sender := &stubSender{}

	w := NewCampaignWorker(repo, channelRepo, sender, CampaignWorkerConfig{BatchSize: 10})
	w.tick(context.Background())

	assert.Equal(t, []string{"a"}, sender.sent)
}

func TestStart_DoubleStartIsNoop(t *testing.T) {
	w := NewCampaignWorker(newStubCampaignRepo(), &stubChannelRepo{}, &stubSender{}, CampaignWorkerConfig{
		PollInterval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	// Second Start must return immediately without panicking.
	w.Start(ctx) // no-op (running flag)
	cancel()
}

// slowSender simulates a real adapter that takes time per request.
type slowSender struct {
	calls *atomic.Int32
	delay time.Duration
}

func (s *slowSender) SendTemplateAndGetID(ctx context.Context, _ *domain.Channel, to string, _, _ string, _ []interface{}) (string, error) {
	s.calls.Add(1)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(s.delay):
	}
	return "wamid." + to, nil
}
