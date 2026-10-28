package domain

import (
	"math"
	"strconv"
	"strings"
)

// The commission engine (§6.6, G1/G2).
//
// Pure functions over a rule and a sale. No repository, no clock, no account:
// everything it needs is an argument, so the same code answers "what will
// this pay?" in the simulator and "what did this pay?" in the statement. A
// simulator that used different arithmetic from the engine would be worse
// than no simulator, because it would be believed.

// CommissionCalc is one sale's commission, with every number that produced it.
//
// The breakdown is not decoration. A salesperson who disagrees with a payout
// asks "why?", and the answer has to be on the screen — otherwise the
// argument is settled by whoever is more senior instead of by the rule
// everybody agreed to.
type CommissionCalc struct {
	// BaseCents is what the percentage was applied to.
	BaseCents int64 `json:"base_cents"`
	// RatePercent is the rate that ended up applying, after tiers and the
	// repurchase override.
	RatePercent float64 `json:"rate_percent"`
	// AmountCents is the commission before splits.
	AmountCents int64 `json:"amount_cents"`
	// MilestoneCents is the fixed bonus, when a target was reached.
	MilestoneCents int64 `json:"milestone_cents"`
	// TotalCents is amount + milestone.
	TotalCents int64 `json:"total_cents"`

	// Shares is the split, when the rule has one. Empty means it all goes to
	// the sale's own salesperson.
	Shares []CommissionShare `json:"shares,omitempty"`

	// Explanation says in words what happened, in the customer's language.
	Explanation string `json:"explanation"`

	// Warning flags a payout somebody should look at before it goes out —
	// most importantly, commission on a sale that lost money.
	Warning string `json:"warning,omitempty"`
}

// CommissionShare is one person's cut of one sale.
type CommissionShare struct {
	Role         string  `json:"role"`
	SharePercent float64 `json:"share_percent"`
	AmountCents  int64   `json:"amount_cents"`
}

// CommissionInput is everything the engine needs beyond the rule.
type CommissionInput struct {
	Sale *Sale
	// PeriodTotalCents is what this salesperson has sold in the period,
	// INCLUDING this sale — a tier reached by the sale itself pays at the
	// tier it reached, which is how everybody explains it out loud.
	PeriodTotalCents int64
	// Repurchase says this customer had already bought before. Decided by the
	// caller (a lookup in sales), not guessed here.
	Repurchase bool
	// MilestoneAlreadyPaid stops a monthly target paying twice in one period.
	MilestoneAlreadyPaid bool
}

// CalculateCommission applies one rule to one sale.
func CalculateCommission(rule *CommissionRule, in CommissionInput) *CommissionCalc {
	if rule == nil || in.Sale == nil {
		return &CommissionCalc{Explanation: "Nenhuma regra de comissão se aplica a esta venda."}
	}

	base := CommissionBaseCents(rule, in.Sale)
	rate := rule.RateFor(in.PeriodTotalCents, in.Repurchase)

	calc := &CommissionCalc{BaseCents: base, RatePercent: rate}

	// A negative base pays nothing and is REPORTED. Clamping silently would
	// hide the sale that lost money; refusing to clamp would pay a negative
	// commission, which means docking somebody's pay for a discount their
	// manager approved.
	if base > 0 {
		calc.AmountCents = percentOfCents(base, rate)
	}
	if in.Sale.Underwater() {
		calc.Warning = "Esta venda saiu no prejuízo: o custo mais o imposto passou do valor cobrado. " +
			"Confira antes de pagar a comissão."
	}

	if !in.MilestoneAlreadyPaid {
		calc.MilestoneCents = milestoneBonus(rule, in.PeriodTotalCents)
	}
	calc.TotalCents = calc.AmountCents + calc.MilestoneCents
	calc.Shares = splitShares(rule, calc.TotalCents)
	calc.Explanation = explain(rule, in, calc)
	return calc
}

// CommissionBaseCents is what the percentage applies to.
func CommissionBaseCents(rule *CommissionRule, sale *Sale) int64 {
	if rule == nil || sale == nil {
		return 0
	}
	switch rule.Base {
	case BaseMargin:
		// Margin is gross minus what the goods cost. Tax is NOT deducted
		// here: it is a real outflow, but "margem" in a commission
		// conversation means what the deal made before the government, and
		// silently including tax would make every number smaller than the
		// one the customer expects to see.
		return sale.GrossCents - sale.CostCents
	case BaseNet:
		net := sale.GrossCents
		if rule.Deductions.Cost {
			net -= sale.CostCents
		}
		if rule.Deductions.Tax {
			net -= sale.TaxCents
		}
		return net
	default:
		return sale.GrossCents
	}
}

// milestoneBonus is the largest reached target's bonus.
//
// The largest, not the sum: hitting R$100k after R$50k is one achievement
// with two names, and paying both is how a bonus table quietly doubles.
func milestoneBonus(rule *CommissionRule, periodTotalCents int64) int64 {
	var best CommissionMilestone
	for _, m := range rule.Milestones {
		if periodTotalCents >= m.TargetCents && m.TargetCents >= best.TargetCents {
			best = m
		}
	}
	return best.BonusCents
}

// splitShares divides the commission, giving the rounding remainder to the
// FIRST share.
//
// Somebody has to get the odd cent, and picking deterministically is what
// makes the total always add up. The first share is the closer in every
// split anybody actually writes.
func splitShares(rule *CommissionRule, totalCents int64) []CommissionShare {
	if len(rule.Splits) == 0 || totalCents == 0 {
		return nil
	}
	shares := make([]CommissionShare, 0, len(rule.Splits))
	var assigned int64
	for i, s := range rule.Splits {
		amount := percentOfCents(totalCents, s.SharePercent)
		if i == len(rule.Splits)-1 {
			// The last one takes whatever is left, so the parts always sum to
			// the whole even when the percentages do not divide cleanly.
			amount = totalCents - assigned
		}
		assigned += amount
		shares = append(shares, CommissionShare{
			Role: s.Role, SharePercent: s.SharePercent, AmountCents: amount,
		})
	}
	return shares
}

// percentOfCents rounds to the nearest cent, never truncates.
//
// Truncation always favours the company, on every single sale, and a
// salesperson who checks the arithmetic finds it.
func percentOfCents(cents int64, percent float64) int64 {
	return int64(math.Round(float64(cents) * percent / 100))
}

// explain writes what happened, in Portuguese, for the person being paid.
func explain(rule *CommissionRule, in CommissionInput, calc *CommissionCalc) string {
	var b strings.Builder

	switch rule.Base {
	case BaseMargin:
		b.WriteString("Sobre a margem (valor menos custo)")
	case BaseNet:
		b.WriteString("Sobre o líquido")
		switch {
		case rule.Deductions.Cost && rule.Deductions.Tax:
			b.WriteString(" (menos custo e imposto)")
		case rule.Deductions.Cost:
			b.WriteString(" (menos custo)")
		case rule.Deductions.Tax:
			b.WriteString(" (menos imposto)")
		}
	default:
		b.WriteString("Sobre o valor cheio da venda")
	}
	b.WriteString(": ")
	b.WriteString(formatBRL(calc.BaseCents))

	if in.Repurchase && rule.RepurchaseRatePercent != nil {
		b.WriteString(", com o percentual de recompra")
	} else if len(rule.Tiers) > 0 {
		b.WriteString(", no escalão de " + formatBRL(in.PeriodTotalCents) + " no período")
	}
	b.WriteString(" → " + trimPercent(calc.RatePercent) + "% = " + formatBRL(calc.AmountCents))

	if calc.MilestoneCents > 0 {
		b.WriteString(". Mais " + formatBRL(calc.MilestoneCents) + " de bônus por meta atingida")
	}
	if calc.BaseCents <= 0 {
		b.WriteString(". A base ficou zerada ou negativa, então não há comissão a pagar")
	}
	b.WriteString(".")
	return b.String()
}

// formatBRL writes cents as R$ 1.234,56 — the way the number is read aloud in
// the country this is sold in.
func formatBRL(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	reais := cents / 100
	rest := cents % 100

	digits := []byte(strconv.FormatInt(reais, 10))
	var grouped []byte
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped = append(grouped, '.')
		}
		grouped = append(grouped, d)
	}
	return sign + "R$ " + string(grouped) + "," + pad2Cents(rest)
}

func pad2Cents(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// trimPercent drops a trailing ".0" so a flat 5% does not read as "5.0%".
func trimPercent(p float64) string {
	s := strings.TrimRight(strings.TrimRight(formatFloat(p), "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func formatFloat(p float64) string {
	return strconv.FormatFloat(p, 'f', -1, 64)
}

// SimulateCommission answers "com venda média de R$10k, o vendedor ganha
// quanto?" before a rule is saved.
//
// Runs the SAME engine on a synthetic sale, deliberately: a simulator with
// its own arithmetic is a simulator that lies, and it lies exactly when the
// rule is complicated enough for somebody to need it.
func SimulateCommission(rule *CommissionRule, grossCents int64, marginPercent float64, salesPerMonth int) *CommissionSimulation {
	if rule == nil || grossCents <= 0 {
		return nil
	}
	if salesPerMonth <= 0 {
		salesPerMonth = 1
	}
	if marginPercent <= 0 || marginPercent > 100 {
		// A distributor's typical gross margin. Used only when the caller
		// has no number of its own — a simulation has to show SOMETHING, and
		// a made-up 100% margin would flatter every rule.
		marginPercent = 30
	}

	cost := grossCents - percentOfCents(grossCents, marginPercent)
	sale := &Sale{GrossCents: grossCents, CostCents: cost, TaxCents: 0}

	// The last sale of the month, so tiers and milestones show the rule at
	// its full shape rather than at its opening rate.
	periodTotal := grossCents * int64(salesPerMonth)
	calc := CalculateCommission(rule, CommissionInput{
		Sale: sale, PeriodTotalCents: periodTotal,
	})

	// Per-sale commission excludes the milestone (paid once), so multiplying
	// it by the month's sales does not pay the bonus every time.
	monthly := calc.AmountCents*int64(salesPerMonth) + calc.MilestoneCents

	return &CommissionSimulation{
		GrossCents: grossCents, SalesPerMonth: salesPerMonth,
		PerSaleCents: calc.TotalCents, MonthlyCents: monthly,
		EffectiveRatePercent: effectiveRate(monthly, periodTotal),
		Calc:                 calc,
	}
}

// CommissionSimulation is what the builder shows before saving.
type CommissionSimulation struct {
	GrossCents    int64 `json:"gross_cents"`
	SalesPerMonth int   `json:"sales_per_month"`
	PerSaleCents  int64 `json:"per_sale_cents"`
	MonthlyCents  int64 `json:"monthly_cents"`
	// EffectiveRatePercent is the commission over the whole month's GROSS —
	// the number an owner actually needs, because it is what the rule costs
	// the company regardless of which base it was written against.
	EffectiveRatePercent float64         `json:"effective_rate_percent"`
	Calc                 *CommissionCalc `json:"calc"`
}

func effectiveRate(commissionCents, grossCents int64) float64 {
	if grossCents == 0 {
		return 0
	}
	return math.Round(float64(commissionCents)/float64(grossCents)*10000) / 100
}
