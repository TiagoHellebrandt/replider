package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ContractAlertWorker turns a contract's dates into work (§6.11, G6).
//
// This is the encaixe nobody in the market makes: contract management elsewhere
// is legal-passive — a repository that answers questions when asked. This one is
// commercial-active. Ninety days before a term ends, a renewal lead appears in
// the funnel with the value, the accumulated index and the history already in it.
// The month the price is indexed, a task appears saying how much it may rise.
//
// It creates TASKS and LEADS, never messages. The approach still goes through the
// approval queue like everything else, because a machine that decides on its own
// to open a renegotiation with a hospital is one wrong reading away from a
// customer who trusts nothing the system says.
type ContractAlertWorker struct {
	contracts domain.ContractRepository
	alerts    domain.ContractAlertRepository
	// adjustments prices the index. Optional: without it the dates still fire and
	// carry no percentage, which is the honest half.
	adjustments ContractIndexPricer
	// leads opens the renewal lead. Optional for the same reason — a deployment
	// with no funnel still wants the deadline.
	leads RenewalLeadOpener

	interval time.Duration
	horizon  time.Duration
	batch    int
	now      func() time.Time
	// runLog stamps completed sweeps so a boot decides honestly whether the
	// 24h tick was missed (daily deploys). Optional — see WithRunLog.
	runLog domain.WorkerRunRepository
}

// contractAlertWorkerName keys the worker_runs stamp.
const contractAlertWorkerName = "contract_alerts"

// WithRunLog wires the worker_runs stamp (boot sweep when the last completed
// sweep is older than the interval).
func (w *ContractAlertWorker) WithRunLog(r domain.WorkerRunRepository) *ContractAlertWorker {
	w.runLog = r
	return w
}

// ContractIndexPricer is the adjustment calculation.
// *usecases.ContractAdjustmentUseCase satisfies it.
type ContractIndexPricer interface {
	For(ctx context.Context, contract *domain.Contract) *domain.ContractAdjustment
}

// RenewalLeadOpener puts a renewal into the pipeline.
//
// A narrow port: the worker knows nothing about funnels, contacts or phone
// numbers, and returning the lead id is what lets the alert point at it.
type RenewalLeadOpener interface {
	OpenRenewalLead(ctx context.Context, in RenewalLeadRequest) (string, error)
}

// RenewalLeadRequest is the context a salesperson needs to open the call with.
type RenewalLeadRequest struct {
	AccountID  string
	ContactID  string
	ContractID string
	Title      string
	ValueCents int64
	EndsOn     *time.Time
	// Context is the briefing: current value, applicable adjustment, what the
	// deadline is. Written by the worker so the lead is actionable on sight.
	Context string
}

func NewContractAlertWorker(
	contracts domain.ContractRepository, alerts domain.ContractAlertRepository,
) *ContractAlertWorker {
	return &ContractAlertWorker{
		contracts: contracts, alerts: alerts,
		// Daily: these are calendar deadlines measured in months, and a sweep
		// that ran hourly would produce the same rows 24 times.
		interval: 24 * time.Hour,
		horizon:  domain.ExpiryHorizonDays * 24 * time.Hour,
		batch:    200,
		now:      time.Now,
	}
}

func (w *ContractAlertWorker) WithPricer(p ContractIndexPricer) *ContractAlertWorker {
	w.adjustments = p
	return w
}

func (w *ContractAlertWorker) WithLeads(l RenewalLeadOpener) *ContractAlertWorker {
	w.leads = l
	return w
}

func (w *ContractAlertWorker) WithInterval(d time.Duration) *ContractAlertWorker {
	w.interval = d
	return w
}

func (w *ContractAlertWorker) WithClock(now func() time.Time) *ContractAlertWorker {
	w.now = now
	return w
}

// Start runs until the context is cancelled.
func (w *ContractAlertWorker) Start(ctx context.Context) {
	if w.contracts == nil || w.alerts == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// No unconditional immediate pass: a deploy loop that restarts three
	// times in a minute must not become three sweeps. But a missed tick
	// costs a day on a ninety-day window and a 24h ticker under daily
	// deploys never fires, so the boot sweep runs when the stamped last
	// sweep is older than the interval (or there is none).
	if bootPassDue(ctx, w.runLog, contractAlertWorkerName, w.interval, w.now()) {
		w.RunOnce(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce does one sweep.
//
// Idempotent by construction: every alert it writes is uniquely keyed on
// (contract, kind, date) while pending, so running twice today produces the same
// rows. The renewal LEAD is guarded separately — see below.
func (w *ContractAlertWorker) RunOnce(ctx context.Context) {
	if w.contracts == nil || w.alerts == nil {
		return
	}
	now := w.now()
	contracts, err := w.contracts.DueForAttention(ctx, now.Add(w.horizon), w.batch)
	if err != nil {
		slog.ErrorContext(ctx, "contract alerts: could not read the contracts", "error", err)
		return
	}

	var created, leads int
	for _, contract := range contracts {
		var adj *domain.ContractAdjustment
		if w.adjustments != nil {
			adj = w.adjustments.For(ctx, contract)
		}

		existing, err := w.alerts.ListByContract(ctx, contract.AccountID, contract.ID)
		if err != nil {
			slog.WarnContext(ctx, "contract alerts: could not read existing alerts",
				"contract_id", contract.ID, "error", err)
			continue
		}
		pending := map[string]*domain.ContractAlert{}
		for _, a := range existing {
			if a.Status == domain.AlertPending {
				pending[string(a.Kind)+"|"+a.DueOn.Format("2006-01-02")] = a
			}
		}
		// Whether this contract's renewal has ALREADY been put in the pipeline,
		// in any state. Checked across every alert rather than the pending ones:
		// a closed renewal alert that produced a lead must not produce a second
		// one next sweep.
		leadOpened := false
		for _, a := range existing {
			if a.LeadID != "" {
				leadOpened = true
				break
			}
		}

		for _, flag := range domain.DeriveRedFlags(contract, adj, now) {
			if !flag.Schedulable() {
				// Exclusivity and penalties have no date. They belong on the
				// screen, not on a schedule — a task with no deadline is a task
				// nobody closes.
				continue
			}
			key := string(flag.Kind) + "|" + flag.DueOn.Format("2006-01-02")
			alert, ok := pending[key]
			if !ok {
				alert, err = w.alerts.Create(ctx, &domain.ContractAlert{
					ID: ulid.Make().String(), AccountID: contract.AccountID,
					ContractID: contract.ID, Kind: flag.Kind,
					Title: flag.Title, Detail: flag.Detail,
					MoneyAtStakeCents: flag.MoneyAtStakeCents,
					DueOn:             *flag.DueOn,
				})
				if err != nil {
					slog.WarnContext(ctx, "contract alerts: could not create an alert",
						"contract_id", contract.ID, "kind", flag.Kind, "error", err)
					continue
				}
				created++
			}

			// The renewal lead: only for the deadline that IS a renewal, only
			// once per contract, and only when there is somebody to call.
			if leadOpened || w.leads == nil || alert == nil {
				continue
			}
			if flag.Kind != domain.AlertExpiry && flag.Kind != domain.AlertRenewalNotice {
				continue
			}
			if strings.TrimSpace(contract.ContactID) == "" {
				// No contact on the contract: the alert stands, the lead cannot.
				// Better a deadline with no lead than a lead with no counterpart.
				continue
			}
			leadID, err := w.leads.OpenRenewalLead(ctx, RenewalLeadRequest{
				AccountID: contract.AccountID, ContactID: contract.ContactID,
				ContractID: contract.ID, Title: contract.Title,
				ValueCents: contract.ValueCents, EndsOn: contract.EndsOn,
				Context: RenewalBriefing(contract, adj, now),
			})
			if err != nil {
				slog.WarnContext(ctx, "contract alerts: could not open the renewal lead",
					"contract_id", contract.ID, "error", err)
				continue
			}
			if err := w.alerts.AttachLead(ctx, contract.AccountID, alert.ID, leadID); err != nil {
				slog.WarnContext(ctx, "contract alerts: lead opened but not linked — "+
					"the next sweep may open a second one",
					"contract_id", contract.ID, "lead_id", leadID, "error", err)
				continue
			}
			leadOpened = true
			leads++
		}
	}

	if created > 0 || leads > 0 {
		slog.InfoContext(ctx, "contract alerts: swept",
			"alerts_created", created, "renewal_leads", leads)
	}
	stampRun(ctx, w.runLog, contractAlertWorkerName, w.now())
}

// RenewalBriefing is what the salesperson reads before the call.
//
// Deliberately not generated by a model: it is the same four facts every time —
// what the contract is worth, when it ends, what the index allows, and what the
// deadline to act is. A model would spend a call to rephrase them into something
// less specific, and the person opening the lead needs the numbers, not prose.
func RenewalBriefing(c *domain.Contract, adj *domain.ContractAdjustment, now time.Time) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Renovação de contrato: " + c.Title + ".")

	if c.ValueCents > 0 {
		b.WriteString(" Valor atual " + brl(c.ValueCents) + ".")
	}
	if c.EndsOn != nil {
		days := int(c.EndsOn.Sub(startOfDayUTC(now)).Hours() / 24)
		if days >= 0 {
			b.WriteString(fmt.Sprintf(" Vence em %s (%d dias).",
				c.EndsOn.Format("02/01/2006"), days))
		} else {
			b.WriteString(fmt.Sprintf(" Venceu em %s.", c.EndsOn.Format("02/01/2006")))
		}
	}
	if c.AutoRenewal {
		if deadline := c.NoticeDeadline(); deadline != nil {
			b.WriteString(" Renova automaticamente se ninguém avisar até " +
				deadline.Format("02/01/2006") + ".")
		} else {
			b.WriteString(" Renova automaticamente.")
		}
	}

	// The index line only claims a number when the series was whole — the same
	// gate as everywhere else, in the briefing this time, because this text is
	// what somebody reads out loud on the call.
	if adj != nil && adj.Accumulation != nil {
		if percent := adj.Accumulation.UsablePercent(); percent != nil {
			b.WriteString(fmt.Sprintf(" %s acumulado desde %s: %s%%.",
				strings.ToUpper(string(adj.Accumulation.Code)),
				adj.BaseDate.Format("02/2006"), trimPercentValue(*percent)))
			if adj.SuggestedValueCents != nil {
				b.WriteString(" Valor sugerido " + brl(*adj.SuggestedValueCents) + ".")
			}
		} else {
			b.WriteString(" Reajuste previsto, mas a série do índice está incompleta — " +
				"confira o acumulado antes de propor valor.")
		}
	}
	if c.PriceRegistry && c.RegistryBalanceCents > 0 {
		b.WriteString(" Ata com saldo de " + brl(c.RegistryBalanceCents) + " a entregar.")
	}
	return b.String()
}

func startOfDayUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// brl writes cents the way the number is read aloud in the country this is sold
// in. Local to this package: the domain has its own, unexported.
func brl(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	units := fmt.Sprintf("%d", cents/100)
	var grouped strings.Builder
	for i, d := range units {
		if i > 0 && (len(units)-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(d)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, grouped.String(), cents%100)
}

func trimPercentValue(p float64) string {
	s := fmt.Sprintf("%.2f", p)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" {
		return "0"
	}
	return s
}
