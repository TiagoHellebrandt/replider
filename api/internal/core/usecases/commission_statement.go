package usecases

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// The commission engine and the monthly close (§6.6, G2).
//
// Every sale of a month goes through the rule that governs it and produces
// lines somebody can point at. The recomputation is idempotent and touches
// PENDING lines only: money that has left the company is history, and a rule
// edited in March must not silently rewrite what was transferred in February.
// A change that lands after payment becomes a visible adjustment instead.

// CommissionSaleReader is the narrow view of sales the statement needs.
//
// Declared here rather than added to domain.SaleRepository: that interface has
// a dozen mocks, and this needs two queries neither of which any of them wants
// to grow.
type CommissionSaleReader interface {
	// ListByPeriod is every sale closed in [from, to), oldest first — the
	// order matters, because a tier reached mid-month must pay the tier the
	// sale reached, not the one the month ended on.
	ListByPeriod(ctx context.Context, accountID string, from, to time.Time) ([]*domain.Sale, error)
	// CountByContactBefore is how many times this customer bought BEFORE a
	// moment. Zero means the sale being calculated is their first, which is
	// what "recompra" turns on.
	CountByContactBefore(ctx context.Context, accountID, contactID string, before time.Time) (int, error)
}

// CommissionTeamReader resolves a seller's team, for team-scoped rules, and
// their display name for the statement.
//
// Optional: an account with no team concept still pays commissions, and a
// missing resolver means team-scoped rules simply never match rather than the
// whole statement failing.
type CommissionTeamReader interface {
	TeamOf(ctx context.Context, accountID, userID string) (string, error)
}

type CommissionStatementUseCase struct {
	// lifecycle emits commission.closed (J2). Optional.
	lifecycle LifecycleEmitter
	entries   domain.CommissionLineRepository
	rules     domain.CommissionRuleRepository
	sales     CommissionSaleReader
	accounts  domain.AccountRepository

	// teams and names are optional decorations.
	teams CommissionTeamReader
	users domain.UserRepository

	// now is injectable so the cycle-open guard is testable without waiting
	// for a month to end.
	now func() time.Time
}

func NewCommissionStatementUseCase(
	entries domain.CommissionLineRepository,
	rules domain.CommissionRuleRepository,
	sales CommissionSaleReader,
	accounts domain.AccountRepository,
) *CommissionStatementUseCase {
	return &CommissionStatementUseCase{
		entries: entries, rules: rules, sales: sales, accounts: accounts,
		now: time.Now,
	}
}

func (uc *CommissionStatementUseCase) SetTeams(r CommissionTeamReader)  { uc.teams = r }
func (uc *CommissionStatementUseCase) SetUsers(r domain.UserRepository) { uc.users = r }
func (uc *CommissionStatementUseCase) SetClock(now func() time.Time)    { uc.now = now }

// timezoneOf is the account's own clock. A commission month that ends at a
// foreign midnight moves a deal into the wrong month for the person who closed
// it, which is the single most upsetting kind of wrong on a payslip.
func (uc *CommissionStatementUseCase) timezoneOf(ctx context.Context, accountID string) string {
	if uc.accounts == nil {
		return domain.FallbackTimezone
	}
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return domain.FallbackTimezone
	}
	return account.Timezone
}

// CurrentCycle is the month an account is living in right now.
func (uc *CommissionStatementUseCase) CurrentCycle(ctx context.Context, accountID string) string {
	return domain.CycleOf(uc.now(), uc.timezoneOf(ctx, accountID))
}

// Recompute applies the rules to every sale of a cycle and rewrites the
// PENDING lines.
//
// Returns the statements it produced, one per salesperson, so the caller can
// show the result of the close without a second read.
func (uc *CommissionStatementUseCase) Recompute(
	ctx context.Context, accountID, cycle string,
) ([]*domain.CommissionStatement, error) {
	tz := uc.timezoneOf(ctx, accountID)
	from, to, err := domain.CycleRange(cycle, tz)
	if err != nil {
		return nil, err
	}

	rules, err := uc.rules.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	sales, err := uc.sales.ListByPeriod(ctx, accountID, from, to)
	if err != nil {
		return nil, err
	}
	paid, err := uc.entries.SumPaidByCycle(ctx, accountID, cycle)
	if err != nil {
		return nil, err
	}

	// Period totals accumulate as the month is walked, in sale order, so a
	// tier reached by a sale pays at the tier it reached.
	periodTotal := map[string]int64{}
	milestonePaidFor := map[string]bool{}
	computed := map[string][]*domain.CommissionLine{}
	// What the recomputation now believes each seller earned, so an already
	// PAID month can be compared against it.
	believed := map[string]int64{}

	for _, sale := range sales {
		if sale == nil || strings.TrimSpace(sale.SellerUserID) == "" {
			// A house-account sale pays nobody. Inventing a salesperson would
			// pay somebody for work they did not do.
			continue
		}
		seller := sale.SellerUserID

		teamID := uc.teamOf(ctx, accountID, seller)
		rule := domain.PickCommissionRule(rules, seller, teamID, productIDsOf(sale))
		if rule == nil {
			continue
		}

		periodTotal[seller] += sale.GrossCents
		repurchase := uc.isRepurchase(ctx, accountID, sale)

		calc := domain.CalculateCommission(rule, domain.CommissionInput{
			Sale:                 sale,
			PeriodTotalCents:     periodTotal[seller],
			Repurchase:           repurchase,
			MilestoneAlreadyPaid: milestonePaidFor[seller],
		})
		if calc.MilestoneCents > 0 {
			milestonePaidFor[seller] = true
		}

		for _, entry := range uc.entriesFor(accountID, cycle, seller, sale, rule, calc) {
			computed[entry.SellerUserID] = append(computed[entry.SellerUserID], entry)
			believed[entry.SellerUserID] += entry.AmountCents
		}
	}

	// A month that was already paid is not rewritten. The difference between
	// what the rules now say and what was transferred becomes ONE adjustment
	// line, visible, on the same cycle — so the statement still reconciles
	// with the bank while the history stays intact.
	var flat []*domain.CommissionLine
	for seller, list := range computed {
		if settled := paid[seller]; settled != 0 {
			delta := believed[seller] - settled
			if delta != 0 {
				flat = append(flat, uc.adjustment(accountID, cycle, seller, delta,
					"Diferença apurada depois do pagamento deste ciclo."))
			}
			continue
		}
		flat = append(flat, list...)
	}
	// Sellers who were paid but whose sales all vanished (a deleted sale, a
	// rule turned off) still need their delta recorded.
	for seller, settled := range paid {
		if _, recomputed := computed[seller]; recomputed || settled == 0 {
			continue
		}
		flat = append(flat, uc.adjustment(accountID, cycle, seller, -settled,
			"As vendas deste ciclo deixaram de gerar comissão."))
	}

	if err := uc.entries.ReplacePendingForCycle(ctx, accountID, cycle, flat); err != nil {
		return nil, err
	}
	return uc.List(ctx, accountID, cycle, "")
}

// entriesFor turns one calculation into the lines it pays.
//
// A split fans the same commission out to several people; without one, the
// whole amount goes to the sale's own salesperson.
func (uc *CommissionStatementUseCase) entriesFor(
	accountID, cycle, seller string, sale *domain.Sale,
	rule *domain.CommissionRule, calc *domain.CommissionCalc,
) []*domain.CommissionLine {
	var out []*domain.CommissionLine

	if len(calc.Shares) > 0 {
		for _, share := range calc.Shares {
			target := seller
			role := share.Role
			// A split names ROLES, not people: only the closer's identity is
			// known from the sale. The rest are recorded against the seller
			// with their role visible, so the admin can reassign the line
			// before paying rather than the system guessing who "quem
			// indicou" was.
			out = append(out, &domain.CommissionLine{
				ID: ulid.Make().String(), AccountID: accountID, Cycle: cycle,
				SellerUserID: target, Kind: domain.LineCommission,
				SaleID: sale.ID, RuleID: rule.ID, Role: role,
				BaseCents: calc.BaseCents, RatePercent: calc.RatePercent,
				AmountCents: share.AmountCents,
				Explanation: calc.Explanation, Status: domain.CommissionPending,
			})
		}
	} else if calc.AmountCents != 0 {
		out = append(out, &domain.CommissionLine{
			ID: ulid.Make().String(), AccountID: accountID, Cycle: cycle,
			SellerUserID: seller, Kind: domain.LineCommission,
			SaleID: sale.ID, RuleID: rule.ID,
			BaseCents: calc.BaseCents, RatePercent: calc.RatePercent,
			AmountCents: calc.AmountCents,
			Explanation: calc.Explanation, Status: domain.CommissionPending,
		})
	}

	// The milestone is the period's, not the sale's — it carries no sale id so
	// deleting that sale cannot delete the month's bonus.
	if calc.MilestoneCents > 0 {
		out = append(out, &domain.CommissionLine{
			ID: ulid.Make().String(), AccountID: accountID, Cycle: cycle,
			SellerUserID: seller, Kind: domain.LineMilestone,
			RuleID: rule.ID, AmountCents: calc.MilestoneCents,
			Explanation: "Bônus por meta atingida no período.",
			Status:      domain.CommissionPending,
		})
	}
	return out
}

func (uc *CommissionStatementUseCase) adjustment(
	accountID, cycle, seller string, cents int64, why string,
) *domain.CommissionLine {
	return &domain.CommissionLine{
		ID: ulid.Make().String(), AccountID: accountID, Cycle: cycle,
		SellerUserID: seller, Kind: domain.LineAdjustment,
		AmountCents: cents, Explanation: why,
		Status: domain.CommissionPending,
	}
}

func (uc *CommissionStatementUseCase) teamOf(ctx context.Context, accountID, userID string) string {
	if uc.teams == nil {
		return ""
	}
	team, err := uc.teams.TeamOf(ctx, accountID, userID)
	if err != nil {
		return ""
	}
	return team
}

// isRepurchase asks whether this customer had bought before this sale.
//
// A lookup, not a guess: paying the repurchase rate on somebody's first order
// (or the new-customer rate on their tenth) is the kind of error that only
// shows up in an argument.
func (uc *CommissionStatementUseCase) isRepurchase(ctx context.Context, accountID string, sale *domain.Sale) bool {
	if uc.sales == nil || strings.TrimSpace(sale.ContactID) == "" {
		return false
	}
	count, err := uc.sales.CountByContactBefore(ctx, accountID, sale.ContactID, sale.SoldAt)
	if err != nil {
		return false
	}
	return count > 0
}

func productIDsOf(sale *domain.Sale) []string {
	var ids []string
	for _, item := range sale.Items {
		if strings.TrimSpace(item.ProductID) != "" {
			ids = append(ids, item.ProductID)
		}
	}
	return ids
}

// List builds the statements of a cycle. An empty sellerUserID returns every
// salesperson, sorted by what they are owed — the admin's review order.
func (uc *CommissionStatementUseCase) List(
	ctx context.Context, accountID, cycle, sellerUserID string,
) ([]*domain.CommissionStatement, error) {
	if _, _, err := domain.CycleRange(cycle, uc.timezoneOf(ctx, accountID)); err != nil {
		return nil, err
	}
	entries, err := uc.entries.ListByCycle(ctx, accountID, cycle, sellerUserID)
	if err != nil {
		return nil, err
	}

	bySeller := map[string][]*domain.CommissionLine{}
	order := []string{}
	for _, e := range entries {
		if _, seen := bySeller[e.SellerUserID]; !seen {
			order = append(order, e.SellerUserID)
		}
		bySeller[e.SellerUserID] = append(bySeller[e.SellerUserID], e)
	}

	out := make([]*domain.CommissionStatement, 0, len(order))
	for _, seller := range order {
		st := domain.BuildStatement(cycle, seller, bySeller[seller])
		uc.decorate(ctx, accountID, st)
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PendingCents > out[j].PendingCents
	})
	return out, nil
}

// decorate adds the sales the commission was earned on and the warnings a
// human should read before paying.
func (uc *CommissionStatementUseCase) decorate(
	ctx context.Context, accountID string, st *domain.CommissionStatement,
) {
	seen := map[string]struct{}{}
	for _, e := range st.Entries {
		if e.Kind != domain.LineCommission || e.SaleID == "" {
			continue
		}
		if _, dup := seen[e.SaleID]; dup {
			continue
		}
		seen[e.SaleID] = struct{}{}
		st.SalesCount++
	}
	if st.SellerName == "" && uc.users != nil {
		if u, err := uc.users.GetByID(ctx, st.SellerUserID); err == nil && u != nil {
			st.SellerName = u.Name
		}
	}
	if st.TotalCents < 0 {
		st.Warnings = append(st.Warnings,
			"O total deste vendedor ficou negativo — confira os ajustes antes de fechar.")
	}
	for _, e := range st.Entries {
		if e.Kind == domain.LineCommission && e.BaseCents <= 0 {
			st.Warnings = append(st.Warnings,
				"Há venda com base zerada ou negativa neste extrato.")
			break
		}
	}
}

// Adjust records a human correction.
//
// Corrections are ADDED, never edited into an existing line: "o cálculo deu
// R$800 e combinamos R$900" is two facts, and collapsing them into one number
// destroys the only evidence of the second.
func (uc *CommissionStatementUseCase) Adjust(
	ctx context.Context, accountID, cycle, sellerUserID string, cents int64, note string,
) (*domain.CommissionLine, error) {
	if _, _, err := domain.CycleRange(cycle, uc.timezoneOf(ctx, accountID)); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sellerUserID) == "" {
		return nil, &domain.AppError{
			Code: "COMMISSION_ADJUST_INVALID", Message: "escolha o vendedor"}
	}
	if cents == 0 {
		return nil, &domain.AppError{
			Code: "COMMISSION_ADJUST_INVALID", Message: "informe um valor diferente de zero"}
	}
	// An adjustment without a reason is a number nobody can defend next month.
	if strings.TrimSpace(note) == "" {
		return nil, &domain.AppError{
			Code: "COMMISSION_ADJUST_NEEDS_NOTE", Message: "explique o motivo do ajuste"}
	}

	entry := &domain.CommissionLine{
		ID: ulid.Make().String(), AccountID: accountID, Cycle: cycle,
		SellerUserID: sellerUserID, Kind: domain.LineAdjustment,
		AmountCents: cents, Note: strings.TrimSpace(note),
		Status: domain.CommissionPending,
	}
	if err := uc.entries.Insert(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// MarkPaid settles a seller's month.
//
// The transfer itself happens outside the system (PIX/TED) — this records that
// it happened, which is what makes the next recomputation refuse to touch it.
// Paying a cycle that has not ended is allowed but must be explicit:
// sales landing after the transfer would otherwise turn up as lines nobody
// budgeted for.
func (uc *CommissionStatementUseCase) MarkPaid(
	ctx context.Context, accountID, cycle, sellerUserID, note string, allowOpenCycle bool,
) (*domain.CommissionStatement, error) {
	tz := uc.timezoneOf(ctx, accountID)
	if _, _, err := domain.CycleRange(cycle, tz); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sellerUserID) == "" {
		return nil, &domain.AppError{
			Code: "COMMISSION_PAY_INVALID", Message: "escolha o vendedor"}
	}
	now := uc.now()
	if !allowOpenCycle && domain.IsCycleOpen(cycle, tz, now) {
		return nil, &domain.AppError{
			Code: "COMMISSION_CYCLE_STILL_OPEN",
			Message: "este mês ainda não terminou — confirme se quer adiantar a comissão, " +
				"porque vendas depois do pagamento vão gerar novos lançamentos",
		}
	}

	settled, total, err := uc.entries.MarkPaid(ctx, accountID, cycle, sellerUserID, strings.TrimSpace(note), now)
	if err != nil {
		return nil, err
	}
	if len(settled) == 0 {
		return nil, &domain.AppError{
			Code: "COMMISSION_NOTHING_PENDING", Message: "não há comissão pendente neste ciclo"}
	}
	_ = total

	// J2: the cycle is settled — real money left the company, which is exactly
	// the moment an accounting integration wants to hear about.
	var paidCents int64
	for _, line := range settled {
		paidCents += line.AmountCents
	}
	emitLifecycle(uc.lifecycle, accountID, domain.EventCommissionClosed,
		domain.LifecyclePayload{
			ID:          cycle,
			AmountCents: paidCents,
			Currency:    domain.CommissionLedgerCurrency,
			Confirmed:   true,
		})

	list, err := uc.List(ctx, accountID, cycle, sellerUserID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return domain.BuildStatement(cycle, sellerUserID, settled), nil
	}
	return list[0], nil
}

// SetLifecycleEmitter wires the commission.closed webhook (J2). Optional.
func (uc *CommissionStatementUseCase) SetLifecycleEmitter(e LifecycleEmitter) {
	uc.lifecycle = e
}

// EstimateForSale answers "se fechar, você ganha quanto?" for the caller (G3).
//
// The rule that would govern THEIR sale, not the account's average: a
// salesperson on an exception rate must not be shown the house number, because
// they will remember the number they were shown.
//
// Returns nil when no rule applies — a zero would read as "you earn nothing on
// this", which is a different and much worse claim than "no rule covers it".
func (uc *CommissionStatementUseCase) EstimateForSale(
	ctx context.Context, accountID, sellerUserID string,
	grossCents int64, marginPercent float64,
) (*domain.CommissionSimulation, error) {
	if grossCents <= 0 {
		return nil, &domain.AppError{
			Code: "COMMISSION_SIMULATION_INVALID", Message: "informe um valor de venda"}
	}
	rules, err := uc.rules.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	rule := domain.PickCommissionRule(rules, sellerUserID,
		uc.teamOf(ctx, accountID, sellerUserID), nil)
	if rule == nil {
		return nil, nil
	}
	// One sale, so tiers show the rate this deal would actually pay rather than
	// the rate a full month of them would reach.
	return domain.SimulateCommission(rule, grossCents, marginPercent, 1), nil
}

// PendingForSeller is the salesperson's own running total for the month (G3).
func (uc *CommissionStatementUseCase) PendingForSeller(
	ctx context.Context, accountID, sellerUserID string,
) (int64, string, error) {
	cycle := uc.CurrentCycle(ctx, accountID)
	total, err := uc.entries.PendingTotalForSeller(ctx, accountID, cycle, sellerUserID)
	if err != nil {
		return 0, cycle, err
	}
	return total, cycle, nil
}
