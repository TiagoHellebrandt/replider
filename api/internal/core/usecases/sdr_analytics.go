package usecases

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// SDR analytics (§6.12, I1).
//
// Six independent aggregations plus one events read, assembled into the report
// the screen renders. Every section runs on its own goroutine and a failure
// degrades to a NAMED warning rather than taking the screen down — the same
// posture as the dashboard BFF (US-197), and for the same reason: an operator
// checking their week should not lose the whole page because the signals table
// is locked.
//
// A failed section is named in Warnings rather than left as an empty list. An
// empty "por vendedor" reads as "nobody sold anything", which is a lie of the
// worst kind on a screen somebody uses to pay people.
type SDRAnalyticsUseCase struct {
	repo        domain.SDRAnalyticsRepository
	events      domain.SalesEventRepository
	accountRepo domain.AccountRepository
	// settings supplies the USD→BRL rate for the AI cost column. Optional:
	// nil falls back to the platform default rather than reporting the cost
	// as zero, because a missing rate must not make spend look free.
	settings domain.PlatformSettingRepository
	now      func() time.Time
}

func NewSDRAnalyticsUseCase(
	repo domain.SDRAnalyticsRepository,
	events domain.SalesEventRepository,
	accountRepo domain.AccountRepository,
) *SDRAnalyticsUseCase {
	return &SDRAnalyticsUseCase{
		repo:        repo,
		events:      events,
		accountRepo: accountRepo,
		now:         time.Now,
	}
}

// SetSettings wires the platform settings repository (USD→BRL rate).
func (uc *SDRAnalyticsUseCase) SetSettings(s domain.PlatformSettingRepository) {
	uc.settings = s
}

// Execute builds the report for one account over one period preset.
func (uc *SDRAnalyticsUseCase) Execute(ctx context.Context, accountID, periodKey string) (*domain.SDRAnalytics, error) {
	// The timezone comes FIRST and is not parallelised: every bound below is
	// computed from it, and "hoje" resolved in the wrong frame makes every
	// number on the screen wrong together.
	tz := domain.FallbackTimezone
	if uc.accountRepo != nil {
		account, err := uc.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if account == nil {
			return nil, domain.ErrAccountNotFound
		}
		if account.Timezone != "" {
			tz = account.Timezone
		}
	}

	period := domain.ResolveSDRPeriod(periodKey, tz, uc.now())
	report := &domain.SDRAnalytics{Period: period}

	rateCents := uc.rateCents(ctx)
	report.Cost = domain.SDRCostDisclosure{
		AIOnly:    true,
		RateCents: int(rateCents),
		Currency:  domain.CommissionLedgerCurrency,
	}

	var mu sync.Mutex
	warn := func(section string, err error) {
		slog.Warn("sdr analytics section failed", "section", section, "error", err)
		mu.Lock()
		report.Warnings = append(report.Warnings, section)
		mu.Unlock()
	}

	var costByChannel map[string]float64
	var funnelCounts domain.SDRFunnelCounts
	var prevMetrics *domain.CaseMetrics

	var wg sync.WaitGroup
	run := func(section string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				warn(section, err)
			}
		}()
	}

	run("funnel", func() error {
		counts, err := uc.repo.Funnel(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		funnelCounts = counts
		mu.Unlock()
		return nil
	})

	run("strategies", func() error {
		rows, err := uc.repo.Strategies(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		report.Strategies = rows
		mu.Unlock()
		return nil
	})

	run("sellers", func() error {
		rows, err := uc.repo.Sellers(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		report.Sellers = rows
		mu.Unlock()
		return nil
	})

	run("channels", func() error {
		rows, err := uc.repo.Channels(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		report.Channels = rows
		mu.Unlock()
		return nil
	})

	run("cost", func() error {
		costs, err := uc.repo.AICostUSDByChannel(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		costByChannel = costs
		mu.Unlock()
		return nil
	})

	run("signals", func() error {
		rows, err := uc.repo.Signals(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		report.Signals = rows
		mu.Unlock()
		return nil
	})

	run("sales", func() error {
		sales, gross, net, currency, err := uc.repo.SalesTotals(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		report.Totals.Sales = sales
		report.Totals.GrossCents = gross
		report.Totals.NetCents = net
		report.Totals.Currency = currency
		mu.Unlock()
		return nil
	})

	run("activity", func() error {
		metrics, err := uc.events.Metrics(ctx, accountID, period.From, period.To)
		if err != nil {
			return err
		}
		mu.Lock()
		report.Totals.Approaches = metrics.ApproachesSent
		report.Totals.Replies = metrics.RepliesReceived
		report.Totals.Meetings = metrics.MeetingsBooked
		report.Totals.ReplyRate = metrics.ReplyRate
		report.Totals.AvgReplySeconds = metrics.AvgReplySeconds
		mu.Unlock()
		return nil
	})

	// The previous window, for the deltas. Its failure is NOT a warning: the
	// comparison is a nicety, and an account in its first week legitimately
	// has nothing to compare against. Absent deltas render as "—".
	wg.Add(1)
	go func() {
		defer wg.Done()
		prev := period.Previous()
		metrics, err := uc.events.Metrics(ctx, accountID, prev.From, prev.To)
		if err != nil {
			slog.Debug("sdr analytics previous period unavailable", "error", err)
			return
		}
		mu.Lock()
		prevMetrics = metrics
		mu.Unlock()
	}()

	wg.Wait()

	report.Funnel = domain.BuildFunnel(funnelCounts)
	report.Totals.LeadsEntered = funnelCounts[domain.SDRMetricEntered]
	report.Totals.Researched = funnelCounts[domain.SDRMetricResearched]

	// The cost column is merged onto the channel rows AFTER both loaded, so a
	// channel that spent AI budget but sent no approach still appears — that
	// combination is exactly the finding worth seeing ("o WhatsApp gastou e
	// não abordou ninguém").
	report.Channels = mergeChannelCosts(report.Channels, costByChannel, rateCents)

	if prevMetrics != nil {
		report.Deltas = buildSDRDeltas(report.Totals, prevMetrics)
	}

	return report, nil
}

// mergeChannelCosts attaches the converted AI cost to each channel row and
// appends rows for channels that spent but produced no recorded approach.
//
// The conversion rounds ONCE per channel at the end, never per message: the
// per-message stamps are fractions of a cent, and rounding each first drifts by
// enough to make a "no markup" cost line lose an argument.
func mergeChannelCosts(rows []domain.SDRChannelRow, costUSD map[string]float64, rateCents int64) []domain.SDRChannelRow {
	seen := make(map[string]int, len(rows))
	for i, row := range rows {
		seen[row.Channel] = i
	}
	// Appended rows are collected then sorted: map iteration order is random,
	// and a table whose rows shuffle between two reloads of the same period
	// looks made up.
	var extra []domain.SDRChannelRow
	for channel, usd := range costUSD {
		cents := int64(math.Round(usd * float64(rateCents)))
		if i, ok := seen[channel]; ok {
			rows[i].AICostCents = cents
			continue
		}
		extra = append(extra, domain.SDRChannelRow{Channel: channel, AICostCents: cents})
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].Channel < extra[j].Channel })
	return append(rows, extra...)
}

// buildSDRDeltas compares the headline counters with the previous window.
//
// Direction is decided here, per metric, rather than left to the UI: reply
// LATENCY going up is worse while every other number here going up is better,
// and a screen that guesses puts a green arrow on a regression.
func buildSDRDeltas(current domain.SDRTotals, prev *domain.CaseMetrics) []domain.SDRDelta {
	deltas := []domain.SDRDelta{
		sdrDelta("leads_entered", current.LeadsEntered, prev.LeadsCreated, true),
		sdrDelta("approaches", current.Approaches, prev.ApproachesSent, true),
		sdrDelta("replies", current.Replies, prev.RepliesReceived, true),
		sdrDelta("meetings", current.Meetings, prev.MeetingsBooked, true),
	}
	if current.AvgReplySeconds != nil && prev.AvgReplySeconds != nil {
		deltas = append(deltas, sdrDelta("avg_reply_seconds",
			int64(*current.AvgReplySeconds), int64(*prev.AvgReplySeconds), false))
	}
	return deltas
}

// sdrDelta builds one comparison. upIsBetter says which direction counts as an
// improvement; a flat number leaves Better nil so the UI renders neutral rather
// than claiming a win.
func sdrDelta(metric string, current, previous int64, upIsBetter bool) domain.SDRDelta {
	d := domain.SDRDelta{Metric: metric, Current: current, Previous: previous}
	switch {
	case current > previous:
		better := upIsBetter
		d.Better = &better
	case current < previous:
		better := !upIsBetter
		d.Better = &better
	}
	return d
}

func (uc *SDRAnalyticsUseCase) rateCents(ctx context.Context) int64 {
	rate, err := getIntSetting(ctx, uc.settings, domain.SettingUSDBRLRateCents, DefaultUSDBRLRateCents)
	if err != nil || rate <= 0 {
		// A missing or corrupt rate falls back to the platform default. The
		// alternative — reporting zero — would make AI spend look free,
		// which is the one direction this number must never err in.
		return DefaultUSDBRLRateCents
	}
	return rate
}

// Drill lists the leads behind one number.
//
// The rung is validated here, before SQL: the repository selects a fixed clause
// by key, so an unknown metric must be refused rather than silently matching
// nothing (a drill-down that opens onto an empty list reads as "there are none"
// instead of "you asked wrong").
func (uc *SDRAnalyticsUseCase) Drill(ctx context.Context, f domain.SDRDrillFilter, periodKey string) ([]domain.SDRLeadRow, domain.SDRPeriod, error) {
	if !domain.ValidSDRMetric(f.Metric) {
		return nil, domain.SDRPeriod{}, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "métrica inválida",
		}
	}

	tz := domain.FallbackTimezone
	if uc.accountRepo != nil {
		account, err := uc.accountRepo.GetByID(ctx, f.AccountID)
		if err != nil {
			return nil, domain.SDRPeriod{}, err
		}
		if account == nil {
			return nil, domain.SDRPeriod{}, domain.ErrAccountNotFound
		}
		if account.Timezone != "" {
			tz = account.Timezone
		}
	}
	// The same resolver the report used, so the list a number opens onto
	// covers exactly the window the number was counted over.
	f.Period = domain.ResolveSDRPeriod(periodKey, tz, uc.now())

	rows, err := uc.repo.DrillLeads(ctx, f)
	if err != nil {
		return nil, f.Period, err
	}
	return rows, f.Period, nil
}
