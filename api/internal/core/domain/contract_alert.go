package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Clause alerts and the recovered-revenue ledger (§6.11, G5/G6).
//
// The red-flag list is DERIVED, not generated. Every flag below is computed from
// fields a person already confirmed — no second model call, so nothing here can
// hallucinate a penalty that is not in the contract. That is the whole reason G4
// insisted on structured, confirmed fields: this is what they are for.
//
// A flag nobody scheduled is a fact nobody acts on, so each one carries the date
// it becomes urgent and what acting on it is worth.

// ContractAlertKind is what the deadline is about.
type ContractAlertKind string

const (
	// AlertRenewalNotice is the window to say "do not renew" before it renews
	// itself. The most expensive deadline in the document.
	AlertRenewalNotice ContractAlertKind = "renewal_notice"
	// AlertAdjustment is the month the price is indexed.
	AlertAdjustment ContractAlertKind = "adjustment"
	// AlertExpiry is the term ending, renewal or not.
	AlertExpiry ContractAlertKind = "expiry"
	// AlertPenalty is a multa worth reading before somebody invokes it.
	AlertPenalty ContractAlertKind = "penalty"
	// AlertExclusivity constrains what the sales floor may promise.
	AlertExclusivity ContractAlertKind = "exclusivity"
	// AlertRegistryBalance is an ata with balance left to deliver against.
	AlertRegistryBalance ContractAlertKind = "registry_balance"
	AlertCustom          ContractAlertKind = "custom"
)

// ContractAlertStatus is the lifecycle.
type ContractAlertStatus string

const (
	AlertPending   ContractAlertStatus = "pending"
	AlertDone      ContractAlertStatus = "done"
	AlertDismissed ContractAlertStatus = "dismissed"
)

// ContractFlagSeverity orders the list. Three levels, because a list where
// everything is urgent is a list nobody reads.
type ContractFlagSeverity string

const (
	// FlagCritical is money leaving or a right about to lapse.
	FlagCritical ContractFlagSeverity = "critical"
	// FlagWarning is a deadline that is coming.
	FlagWarning ContractFlagSeverity = "warning"
	// FlagInfo is a term to know about, with no date attached.
	FlagInfo ContractFlagSeverity = "info"
)

// ContractRedFlag is one clause somebody should look at, in plain language.
type ContractRedFlag struct {
	Kind     ContractAlertKind    `json:"kind"`
	Severity ContractFlagSeverity `json:"severity"`
	// Title is the sentence a person reads: "renova sozinho se você não avisar
	// até 30/01/2027". Written here rather than in the client so the app, the
	// digest, the push and the co-pilot all say the same thing.
	Title string `json:"title"`
	// Detail is the clause behind it, when there is one worth quoting.
	Detail string `json:"detail,omitempty"`
	// DueOn is when it becomes somebody's problem. Nil for a standing term
	// (exclusivity) — and a flag with no date cannot become a scheduled alert,
	// which is honest rather than a task on an invented day.
	DueOn *time.Time `json:"due_on,omitempty"`
	// MoneyAtStakeCents is what acting on it is worth. Zero when the flag is a
	// constraint rather than an amount.
	MoneyAtStakeCents int64 `json:"money_at_stake_cents"`
	// Scheduled says an alert already exists for this flag, so the UI offers
	// "agendado" instead of a second "agendar".
	Scheduled bool `json:"scheduled"`
}

// Schedulable reports a flag that can become a dated task.
func (f ContractRedFlag) Schedulable() bool { return f.DueOn != nil }

// ContractAlert is a scheduled clause deadline.
type ContractAlert struct {
	ID         string `json:"id"`
	AccountID  string `json:"-"`
	ContractID string `json:"contract_id"`
	// ContractTitle is filled on read, so a list of alerts is readable without a
	// second fetch per row.
	ContractTitle string `json:"contract_title,omitempty"`

	Kind   ContractAlertKind `json:"kind"`
	Title  string            `json:"title"`
	Detail string            `json:"detail,omitempty"`

	DueOn             time.Time `json:"due_on"`
	MoneyAtStakeCents int64     `json:"money_at_stake_cents"`

	Status   ContractAlertStatus `json:"status"`
	DoneAt   *time.Time          `json:"done_at,omitempty"`
	DoneNote string              `json:"done_note,omitempty"`

	// LeadID is the renewal lead this alert produced, when it produced one.
	LeadID string `json:"lead_id,omitempty"`

	LastNotifiedAt  *time.Time `json:"last_notified_at,omitempty"`
	CreatedByUserID string     `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// DaysUntilDue is negative once the deadline has passed — the state that costs
// money, which must not read as zero.
func (a *ContractAlert) DaysUntilDue(now time.Time) int {
	if a == nil {
		return 0
	}
	return int(a.DueOn.Sub(startOfDay(now)).Hours() / 24)
}

// Overdue reports a deadline that went by with nobody acting.
func (a *ContractAlert) Overdue(now time.Time) bool {
	return a != nil && a.Status == AlertPending && a.DueOn.Before(startOfDay(now))
}

// ContractRecovery is contracted revenue the account actually went back and
// collected.
type ContractRecovery struct {
	ID         string `json:"id"`
	AccountID  string `json:"-"`
	ContractID string `json:"contract_id"`
	// ContractTitle is filled on read.
	ContractTitle string `json:"contract_title,omitempty"`
	AlertID       string `json:"alert_id,omitempty"`

	// Kind says which of the three leaks this plugged.
	Kind ContractAlertKind `json:"kind"`
	// AmountCents is what a HUMAN says was collected — never the computed figure.
	// The accumulated index is what was OWED; what was agreed differs every time
	// somebody negotiates, and the ledger has to match the invoice.
	AmountCents int64  `json:"amount_cents"`
	Note        string `json:"note,omitempty"`

	RecordedByUserID string    `json:"recorded_by_user_id,omitempty"`
	RecognisedOn     time.Time `json:"recognised_on"`
	CreatedAt        time.Time `json:"created_at"`
}

// ContractRecoverySummary is the headline: how much contracted revenue this
// account went back and collected in a period.
type ContractRecoverySummary struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	TotalCents int64 `json:"total_cents"`
	// ByKind splits the total, because "recuperei R$400k" lands differently when
	// it is R$300k of forgotten index and R$100k of renewals saved.
	ByKind map[string]int64 `json:"by_kind,omitempty"`
	Count  int              `json:"count"`
}

// ── The derivation ─────────────────────────────────────────────────────────

// RenewalNoticeHorizonDays is how far ahead the notice window becomes a flag.
//
// Ninety days: a renewal window of sixty days needs deciding before it opens,
// and a company that finds out on the deadline has no time to negotiate the
// alternative.
const RenewalNoticeHorizonDays = 90

// ExpiryHorizonDays is how far ahead an ending term becomes a flag. The same 90
// days the renewal lead is created on (G6), so the two never disagree.
const ExpiryHorizonDays = 90

// PenaltyFlagPercent is the rate above which a multa is worth reading. Below it
// the clause is standard and flagging every one of them trains people to ignore
// the list.
const PenaltyFlagPercent = 5.0

// DeriveRedFlags is the attention list for one contract.
//
// Deterministic, from confirmed fields only. An UNCONFIRMED contract returns
// nothing: a red flag is a claim about somebody's agreement, and making that
// claim from a reading nobody checked is how the product tells a customer their
// contract renews itself when it does not.
func DeriveRedFlags(c *Contract, adj *ContractAdjustment, now time.Time) []ContractRedFlag {
	if c == nil || !c.Confirmed() {
		return nil
	}
	today := startOfDay(now)
	var flags []ContractRedFlag

	// 1. Automatic renewal + its notice window. The pair that costs a year of a
	//    contract somebody wanted out of.
	if c.AutoRenewal {
		if deadline := c.NoticeDeadline(); deadline != nil {
			days := int(deadline.Sub(today).Hours() / 24)
			severity := FlagInfo
			switch {
			case days < 0:
				severity = FlagCritical
			case days <= RenewalNoticeHorizonDays:
				severity = FlagWarning
			}
			title := fmt.Sprintf("Renova sozinho se você não avisar até %s",
				deadline.Format("02/01/2006"))
			if days < 0 {
				// Past the window the fact changes: it is not a deadline any
				// more, it is a renewal that already happened.
				title = fmt.Sprintf("O prazo de aviso venceu em %s — o contrato já renovou",
					deadline.Format("02/01/2006"))
			}
			flags = append(flags, ContractRedFlag{
				Kind: AlertRenewalNotice, Severity: severity, Title: title,
				Detail: fmt.Sprintf("Renovação automática com aviso prévio de %d dias.", c.NoticeDays),
				DueOn:  deadline, MoneyAtStakeCents: c.ValueCents,
			})
		} else {
			// Renews itself with no stated window: the flag is the ABSENCE, which
			// is worse than a short window and easy to miss on the paper.
			flags = append(flags, ContractRedFlag{
				Kind: AlertRenewalNotice, Severity: FlagWarning,
				Title: "Renova automaticamente e o contrato não diz o prazo de aviso",
				Detail: "Confirme no documento até quando é possível avisar que não " +
					"quer renovar.",
				MoneyAtStakeCents: c.ValueCents,
			})
		}
	}

	// 2. The index. The quiet one: nobody notices a price that did not rise.
	if adj != nil {
		severity := FlagInfo
		if adj.Overdue {
			severity = FlagCritical
		} else if int(adj.DueOn.Sub(today).Hours()/24) <= RenewalNoticeHorizonDays {
			severity = FlagWarning
		}
		title := fmt.Sprintf("Reajuste previsto para %s", adj.DueOn.Format("01/2006"))
		var stake int64
		if adj.Overdue {
			title = fmt.Sprintf("Reajuste de %s não foi aplicado", adj.DueOn.Format("01/2006"))
		}
		// The percentage only enters the sentence when the series was whole. This
		// is the same gate as the suggested price, in the copy this time.
		if adj.Accumulation != nil {
			if percent := adj.Accumulation.UsablePercent(); percent != nil {
				title += fmt.Sprintf(" — %s acumulado de %s%%",
					strings.ToUpper(string(adj.Accumulation.Code)), trimPercent(*percent))
			}
		}
		if adj.GainCents != nil {
			stake = *adj.GainCents
			title += fmt.Sprintf(" (%s)", formatBRL(*adj.GainCents))
		}
		flags = append(flags, ContractRedFlag{
			Kind: AlertAdjustment, Severity: severity, Title: title,
			Detail:            adjustmentDetail(c, adj),
			DueOn:             &adj.DueOn,
			MoneyAtStakeCents: stake,
		})
	}

	// 3. The term ending. Separate from renewal: a contract that does NOT renew
	//    itself simply stops, and that is the one nobody sees coming.
	if c.EndsOn != nil {
		days := int(c.EndsOn.Sub(today).Hours() / 24)
		if days <= ExpiryHorizonDays {
			severity := FlagWarning
			title := fmt.Sprintf("Vence em %s (%d dias)", c.EndsOn.Format("02/01/2006"), days)
			if days < 0 {
				severity, title = FlagCritical,
					fmt.Sprintf("Venceu em %s e não foi renovado", c.EndsOn.Format("02/01/2006"))
			}
			flags = append(flags, ContractRedFlag{
				Kind: AlertExpiry, Severity: severity, Title: title,
				DueOn: c.EndsOn, MoneyAtStakeCents: c.ValueCents,
			})
		}
	}

	// 4. The ata's balance: pre-approved revenue that expires with the ata.
	if c.PriceRegistry && c.RegistryBalanceCents > 0 {
		title := fmt.Sprintf("Ata com saldo de %s a entregar",
			formatBRL(c.RegistryBalanceCents))
		if c.RegistryBalanceOn != nil {
			// The date the balance was known is part of the fact: an old balance
			// is a number to re-check, not a number to plan with.
			title += fmt.Sprintf(" (saldo de %s)", c.RegistryBalanceOn.Format("02/01/2006"))
		}
		due := c.EndsOn
		severity := FlagWarning
		if due != nil && due.Before(today) {
			severity = FlagCritical
		}
		flags = append(flags, ContractRedFlag{
			Kind: AlertRegistryBalance, Severity: severity, Title: title,
			Detail: "Saldo de ata é receita já aprovada: expira com a ata se ninguém " +
				"for buscar.",
			DueOn: due, MoneyAtStakeCents: c.RegistryBalanceCents,
		})
	}

	// 5. Penalties worth reading. Not every multa — a 2% late fee is standard,
	//    and flagging it teaches people to skip the list.
	for _, p := range c.Penalties {
		if p.Percent < PenaltyFlagPercent && p.AmountCents == 0 {
			continue
		}
		title := "Multa"
		if p.Percent > 0 {
			title = fmt.Sprintf("Multa de %s%%", trimPercent(p.Percent))
		} else if p.AmountCents > 0 {
			title = fmt.Sprintf("Multa de %s", formatBRL(p.AmountCents))
		}
		if p.Trigger != "" {
			title += " por " + p.Trigger
		}
		stake := p.AmountCents
		if stake == 0 && p.Percent > 0 && c.ValueCents > 0 {
			stake = percentOfCents(c.ValueCents, p.Percent)
		}
		flags = append(flags, ContractRedFlag{
			Kind: AlertPenalty, Severity: FlagInfo, Title: title,
			Detail: p.Description, MoneyAtStakeCents: stake,
		})
	}

	// 6. Exclusivity: no date, and it still belongs on the list — it is what the
	//    sales floor must not promise elsewhere.
	if c.Exclusivity {
		flags = append(flags, ContractRedFlag{
			Kind: AlertExclusivity, Severity: FlagInfo,
			Title:  "Tem cláusula de exclusividade",
			Detail: c.ExclusivityText,
		})
	}

	return flags
}

func adjustmentDetail(c *Contract, adj *ContractAdjustment) string {
	var b strings.Builder
	if c.AdjustmentIndex != "" {
		b.WriteString("Índice: " + c.AdjustmentIndex + ". ")
	}
	b.WriteString("Base: " + adj.BaseDate.Format("02/01/2006") + ".")
	if adj.Accumulation != nil && !adj.Accumulation.Complete {
		// Says WHY there is no percentage, so nobody assumes the contract simply
		// has no adjustment due.
		b.WriteString(fmt.Sprintf(" Série do índice incompleta (%d meses faltando) — "+
			"confira o acumulado antes de propor o novo valor.",
			len(adj.Accumulation.MissingMonths)))
	}
	if adj.SuggestedValueCents != nil {
		b.WriteString(fmt.Sprintf(" Valor sugerido: %s (hoje %s).",
			formatBRL(*adj.SuggestedValueCents), formatBRL(adj.CurrentValueCents)))
	}
	return b.String()
}

// PlainSummary is the contract in sentences somebody can read out loud.
//
// Composed from the confirmed fields, never asked of a model a second time: the
// summary and the fields must agree, and the cheapest way to guarantee that is
// to build one from the other.
func PlainSummary(c *Contract) []string {
	if c == nil {
		return nil
	}
	var out []string

	if len(c.Parties) > 0 {
		names := make([]string, 0, len(c.Parties))
		for _, p := range c.Parties {
			names = append(names, p.Name)
		}
		out = append(out, "Entre "+strings.Join(names, " e ")+".")
	}
	if c.Subject != "" {
		out = append(out, c.Subject)
	}
	if c.ValueCents > 0 {
		line := "Valor de " + formatBRL(c.ValueCents) + "."
		if c.PaymentTerms != "" {
			line += " Pagamento: " + c.PaymentTerms + "."
		}
		out = append(out, line)
	}
	switch {
	case c.StartsOn != nil && c.EndsOn != nil:
		out = append(out, fmt.Sprintf("Vigência de %s a %s.",
			c.StartsOn.Format("02/01/2006"), c.EndsOn.Format("02/01/2006")))
	case c.EndsOn != nil:
		out = append(out, "Vence em "+c.EndsOn.Format("02/01/2006")+".")
	}
	if c.AutoRenewal {
		if c.NoticeDays > 0 {
			out = append(out, fmt.Sprintf(
				"Renova automaticamente, salvo aviso com %d dias de antecedência.", c.NoticeDays))
		} else {
			out = append(out, "Renova automaticamente.")
		}
	}
	if c.AdjustmentIndex != "" && c.AdjustmentMonth != nil {
		out = append(out, fmt.Sprintf("Reajuste por %s, todo mês %02d.",
			c.AdjustmentIndex, *c.AdjustmentMonth))
	}
	if c.SLAText != "" {
		out = append(out, "SLA: "+c.SLAText)
	}
	if c.PriceRegistry && c.RegistryBalanceCents > 0 {
		out = append(out, "Ata com saldo de "+formatBRL(c.RegistryBalanceCents)+".")
	}
	return out
}

// ContractAlertRepository persists the scheduled deadlines.
type ContractAlertRepository interface {
	// Create writes one alert. A pending alert for the same (contract, kind,
	// date) already existing is NOT an error — the caller asked for a task on a
	// day that already has one, and the answer is that day's task.
	Create(ctx context.Context, a *ContractAlert) (*ContractAlert, error)
	Get(ctx context.Context, accountID, id string) (*ContractAlert, error)
	// ListByContract is one contract's alerts, so the detail screen can mark a
	// derived flag as already scheduled.
	ListByContract(ctx context.Context, accountID, contractID string) ([]*ContractAlert, error)
	// ListDue is the account's pending deadlines through `through`, soonest
	// first — the "precisa de você" read.
	ListDue(ctx context.Context, accountID string, through time.Time, limit int) ([]*ContractAlert, error)
	// Complete marks an alert done or dismissed.
	Complete(ctx context.Context, accountID, id string, status ContractAlertStatus, note string, at time.Time) (*ContractAlert, error)
	// AttachLead records the renewal lead an alert produced.
	AttachLead(ctx context.Context, accountID, id, leadID string) error
	// MarkNotified stamps a surfaced alert so it is not pushed hourly.
	MarkNotified(ctx context.Context, accountID string, ids []string, at time.Time) error
}

// ContractRecoveryRepository persists what was actually collected.
type ContractRecoveryRepository interface {
	Create(ctx context.Context, r *ContractRecovery) error
	// List is the ledger for a period, newest first.
	List(ctx context.Context, accountID string, from, to time.Time, limit int) ([]*ContractRecovery, error)
	// Summary is the headline number, split by kind.
	Summary(ctx context.Context, accountID string, from, to time.Time) (*ContractRecoverySummary, error)
}
