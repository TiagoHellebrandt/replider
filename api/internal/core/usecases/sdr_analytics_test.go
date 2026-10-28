package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// SDR analytics (I1).
//
// What these tests pin is the honesty of the report, not the SQL: the period is
// bounded in the account's timezone, a broken section is a NAMED gap rather
// than an empty list that reads as "you did nothing", and the AI cost column
// says what it covers.

type fakeSDRRepo struct {
	funnel     domain.SDRFunnelCounts
	funnelErr  error
	strategies []domain.SDRStrategyRow
	sellers    []domain.SDRSellerRow
	sellersErr error
	channels   []domain.SDRChannelRow
	signals    []domain.SDRSignalRow
	costs      map[string]float64
	sales      [3]int64
	currency   string
	drill      []domain.SDRLeadRow
	// gotFrom/gotTo capture the window the repository was actually asked
	// for — the whole point of the timezone test.
	gotFrom, gotTo time.Time
	gotDrill       domain.SDRDrillFilter
}

func (f *fakeSDRRepo) Funnel(_ context.Context, _ string, from, to time.Time) (domain.SDRFunnelCounts, error) {
	f.gotFrom, f.gotTo = from, to
	return f.funnel, f.funnelErr
}

func (f *fakeSDRRepo) Strategies(context.Context, string, time.Time, time.Time) ([]domain.SDRStrategyRow, error) {
	return f.strategies, nil
}

func (f *fakeSDRRepo) Sellers(context.Context, string, time.Time, time.Time) ([]domain.SDRSellerRow, error) {
	return f.sellers, f.sellersErr
}

func (f *fakeSDRRepo) Channels(context.Context, string, time.Time, time.Time) ([]domain.SDRChannelRow, error) {
	return f.channels, nil
}

func (f *fakeSDRRepo) Signals(context.Context, string, time.Time, time.Time) ([]domain.SDRSignalRow, error) {
	return f.signals, nil
}

func (f *fakeSDRRepo) SalesTotals(context.Context, string, time.Time, time.Time) (int64, int64, int64, string, error) {
	return f.sales[0], f.sales[1], f.sales[2], f.currency, nil
}

func (f *fakeSDRRepo) AICostUSDByChannel(context.Context, string, time.Time, time.Time) (map[string]float64, error) {
	return f.costs, nil
}

func (f *fakeSDRRepo) DrillLeads(_ context.Context, filter domain.SDRDrillFilter) ([]domain.SDRLeadRow, error) {
	f.gotDrill = filter
	return f.drill, nil
}

type fakeSDREvents struct {
	metrics map[string]*domain.CaseMetrics
	err     error
}

func (f *fakeSDREvents) Record(context.Context, *domain.SalesEvent) error { return nil }

func (f *fakeSDREvents) Metrics(_ context.Context, _ string, from, _ time.Time) (*domain.CaseMetrics, error) {
	// No call counter: both windows are read concurrently, and a counter here
	// would be the fake's own race, not the code's.
	if f.err != nil {
		return nil, f.err
	}
	if m, ok := f.metrics[from.Format(time.RFC3339)]; ok {
		return m, nil
	}
	return &domain.CaseMetrics{}, nil
}

func (f *fakeSDREvents) CountSince(context.Context, string, domain.SalesEventKind, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeSDREvents) LastApproachAt(context.Context, string) (time.Time, error) {
	return time.Time{}, nil
}

func newSDRUseCase(t *testing.T, repo *fakeSDRRepo, events *fakeSDREvents, tz string) *SDRAnalyticsUseCase {
	t.Helper()
	accounts := &MockAccountRepo{}
	accounts.On("GetByID", mock.Anything, "acc").
		Return(&domain.Account{ID: "acc", Timezone: tz}, nil)

	uc := NewSDRAnalyticsUseCase(repo, events, accounts)
	// A fixed clock: 02:30 UTC on the 10th is still the 9th in São Paulo,
	// which is exactly the boundary the timezone test needs.
	uc.now = func() time.Time { return time.Date(2026, 7, 10, 2, 30, 0, 0, time.UTC) }
	return uc
}

func TestSDRAnalytics_WindowIsBoundedInTheAccountTimezone(t *testing.T) {
	repo := &fakeSDRRepo{funnel: domain.SDRFunnelCounts{domain.SDRMetricEntered: 3}}
	uc := newSDRUseCase(t, repo, &fakeSDREvents{}, "America/Sao_Paulo")

	report, err := uc.Execute(context.Background(), "acc", domain.SDRPeriodToday)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// "Hoje" for an account in São Paulo starts at ITS midnight, not the
	// platform's — otherwise every number on the screen is wrong together.
	if got := repo.gotFrom.Day(); got != 9 {
		t.Fatalf("repository asked for day %d, want 9 (local midnight)", got)
	}
	if report.Period.Timezone != "America/Sao_Paulo" {
		t.Fatalf("period timezone = %q, want the account's", report.Period.Timezone)
	}
}

func TestSDRAnalytics_ABrokenSectionIsANamedGapNotAnEmptyList(t *testing.T) {
	repo := &fakeSDRRepo{
		funnel:     domain.SDRFunnelCounts{domain.SDRMetricEntered: 10},
		sellersErr: errors.New("lock timeout"),
	}
	uc := newSDRUseCase(t, repo, &fakeSDREvents{}, "UTC")

	report, err := uc.Execute(context.Background(), "acc", domain.SDRPeriod7d)
	if err != nil {
		t.Fatalf("one failing section must not fail the whole report: %v", err)
	}

	// An empty "por vendedor" section reads as "nobody sold anything", which
	// is the worst possible lie on a screen used to pay people.
	found := false
	for _, w := range report.Warnings {
		if w == "sellers" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want the failed section named", report.Warnings)
	}
	// The rest of the report still rendered.
	if report.Totals.LeadsEntered != 10 {
		t.Fatalf("LeadsEntered = %d, want 10 — a failed section must not blank its siblings",
			report.Totals.LeadsEntered)
	}
}

func TestSDRAnalytics_AICostIsConvertedAndDisclosedAsAIOnly(t *testing.T) {
	repo := &fakeSDRRepo{
		funnel:   domain.SDRFunnelCounts{},
		channels: []domain.SDRChannelRow{{Channel: "whatsapp", Approaches: 20, Replied: 5}},
		// 6.20 USD at the default 550 cents/USD = R$34,10.
		costs: map[string]float64{"whatsapp": 6.2},
	}
	uc := newSDRUseCase(t, repo, &fakeSDREvents{}, "UTC")

	report, err := uc.Execute(context.Background(), "acc", domain.SDRPeriod30d)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(report.Channels) != 1 {
		t.Fatalf("got %d channel rows, want 1", len(report.Channels))
	}
	if got := report.Channels[0].AICostCents; got != 3410 {
		t.Fatalf("AICostCents = %d, want 3410 (6.20 USD × 550)", got)
	}
	// The caveat travels in the payload: this is OUR AI cost, not the
	// carrier's invoice, and the screen has to be able to say so.
	if !report.Cost.AIOnly {
		t.Fatal("cost disclosure must state that only AI spend is covered")
	}
	if report.Cost.RateCents != DefaultUSDBRLRateCents {
		t.Fatalf("RateCents = %d, want the applied rate exposed", report.Cost.RateCents)
	}
}

func TestSDRAnalytics_AChannelThatSpentButSentNothingStillAppears(t *testing.T) {
	repo := &fakeSDRRepo{
		funnel:   domain.SDRFunnelCounts{},
		channels: []domain.SDRChannelRow{{Channel: "whatsapp", Approaches: 4}},
		costs:    map[string]float64{"telegram": 1.0},
	}
	uc := newSDRUseCase(t, repo, &fakeSDREvents{}, "UTC")

	report, _ := uc.Execute(context.Background(), "acc", domain.SDRPeriod7d)

	// "Gastou e não abordou ninguém" is exactly the finding worth surfacing.
	var telegram *domain.SDRChannelRow
	for i := range report.Channels {
		if report.Channels[i].Channel == "telegram" {
			telegram = &report.Channels[i]
		}
	}
	if telegram == nil {
		t.Fatalf("channels = %+v, want a row for the channel that only spent", report.Channels)
	}
	if telegram.Approaches != 0 || telegram.AICostCents == 0 {
		t.Fatalf("telegram row = %+v, want zero approaches and a real cost", *telegram)
	}
}

func TestSDRAnalytics_NoPreviousPeriodMeansNoDeltasNotFakeGrowth(t *testing.T) {
	repo := &fakeSDRRepo{funnel: domain.SDRFunnelCounts{domain.SDRMetricEntered: 5}}
	// The previous window read blows up: a first-week account genuinely has
	// nothing to compare against.
	uc := newSDRUseCase(t, repo, &fakeSDREvents{err: errors.New("no data")}, "UTC")

	report, err := uc.Execute(context.Background(), "acc", domain.SDRPeriod7d)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(report.Deltas) != 0 {
		t.Fatalf("deltas = %+v, want none — a missing comparison renders '—', never +100%%",
			report.Deltas)
	}
	// The activity read failing IS a warning; the previous-period read
	// failing is not, because the comparison is a nicety.
	for _, w := range report.Warnings {
		if w == "previous" {
			t.Fatal("the previous period is not a section; its absence must not warn")
		}
	}
}

func TestSDRAnalytics_DeltaDirectionIsPerMetricNotGlobal(t *testing.T) {
	prev := 600
	repo := &fakeSDRRepo{funnel: domain.SDRFunnelCounts{domain.SDRMetricEntered: 20}}
	events := &fakeSDREvents{metrics: map[string]*domain.CaseMetrics{}}
	uc := newSDRUseCase(t, repo, events, "UTC")

	period := domain.ResolveSDRPeriod(domain.SDRPeriod7d, "UTC", uc.now())
	events.metrics[period.From.Format(time.RFC3339)] = &domain.CaseMetrics{
		ApproachesSent: 10, AvgReplySeconds: intPtr(900),
	}
	events.metrics[period.Previous().From.Format(time.RFC3339)] = &domain.CaseMetrics{
		ApproachesSent: 4, AvgReplySeconds: &prev,
	}

	report, err := uc.Execute(context.Background(), "acc", domain.SDRPeriod7d)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	byMetric := map[string]domain.SDRDelta{}
	for _, d := range report.Deltas {
		byMetric[d.Metric] = d
	}
	approaches, ok := byMetric["approaches"]
	if !ok || approaches.Better == nil || !*approaches.Better {
		t.Fatalf("approaches 4→10 should be better, got %+v", approaches)
	}
	// Reply LATENCY going up is worse. A screen that guesses "up is good"
	// puts a green arrow on a regression.
	latency, ok := byMetric["avg_reply_seconds"]
	if !ok || latency.Better == nil || *latency.Better {
		t.Fatalf("reply latency 600s→900s should be worse, got %+v", latency)
	}
}

func TestSDRAnalytics_DrillRefusesAnUnknownRung(t *testing.T) {
	repo := &fakeSDRRepo{}
	uc := newSDRUseCase(t, repo, &fakeSDREvents{}, "UTC")

	_, _, err := uc.Drill(context.Background(),
		domain.SDRDrillFilter{AccountID: "acc", Metric: "everything"}, domain.SDRPeriod7d)

	// An unknown rung must be refused, not silently matched: a drill-down
	// that opens onto an empty list reads as "there are none".
	if err == nil {
		t.Fatal("an unknown metric must be refused")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("err = %v, want VALIDATION_ERROR", err)
	}
}

func TestSDRAnalytics_DrillUsesTheSameWindowAsTheReport(t *testing.T) {
	repo := &fakeSDRRepo{drill: []domain.SDRLeadRow{{LeadID: "l1"}}}
	uc := newSDRUseCase(t, repo, &fakeSDREvents{}, "America/Sao_Paulo")

	_, period, err := uc.Drill(context.Background(), domain.SDRDrillFilter{
		AccountID: "acc", Metric: domain.SDRMetricApproached,
	}, domain.SDRPeriodToday)
	if err != nil {
		t.Fatalf("Drill: %v", err)
	}

	// The list a number opens onto has to cover exactly the window the
	// number was counted over, or an operator counting rows loses trust in
	// the whole screen.
	want := domain.ResolveSDRPeriod(domain.SDRPeriodToday, "America/Sao_Paulo", uc.now())
	if !period.From.Equal(want.From) || !period.To.Equal(want.To) {
		t.Fatalf("drill window %v–%v, want %v–%v", period.From, period.To, want.From, want.To)
	}
	if !repo.gotDrill.Period.From.Equal(want.From) {
		t.Fatal("the repository must be asked for the resolved window, not the zero value")
	}
}

func intPtr(v int) *int { return &v }
