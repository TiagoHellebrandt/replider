package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Command centre (§6.13, I2).
//
// One round trip for the whole home screen: four counted tiles, a mini-funnel,
// the ranked "needs you" list and at most one copilot suggestion.
//
// The section-per-goroutine-with-named-warnings posture is the same as the
// dashboard BFF and the SDR report — for the same reason. This is the first
// screen someone opens; losing all of it because the reorder table is locked
// would be worse than losing one row of it, and rendering an empty "needs you"
// would be worse still, because "nothing needs you" is a claim.
type CommandCenterUseCase struct {
	repo     domain.CommandCenterRepository
	leadRepo domain.LeadRepository
	accounts domain.AccountRepository
	// overnight surfaces the morning briefing (K2). Optional.
	overnight domain.OvernightDraftRepository
	// untouched is the territory nobody has approached (the opportunity being
	// LOST). Optional: an environment with no CNES park simply has no tile.
	untouched *UntouchedTargetsUseCase
	now       func() time.Time
}

// SetUntouchedTargets wires the "never approached" source. Optional — without
// it the tile reads zero, which is why it is wired in main.go rather than left
// as a nice-to-have: a zero here is indistinguishable from a worked-through
// territory, and that is the exact confusion this number exists to end.
func (uc *CommandCenterUseCase) SetUntouchedTargets(u *UntouchedTargetsUseCase) {
	uc.untouched = u
}

// SetOvernightRepo wires the morning briefing (K2).
func (uc *CommandCenterUseCase) SetOvernightRepo(r domain.OvernightDraftRepository) {
	uc.overnight = r
}

func NewCommandCenterUseCase(
	repo domain.CommandCenterRepository,
	leadRepo domain.LeadRepository,
	accounts domain.AccountRepository,
) *CommandCenterUseCase {
	return &CommandCenterUseCase{
		repo:     repo,
		leadRepo: leadRepo,
		accounts: accounts,
		now:      time.Now,
	}
}

// commandCenterActionsPerKind bounds each source read.
//
// Twelve, not "all": the ranking only ever shows the top handful, and an account
// with four thousand stale drafts must not turn the home screen into a bulk
// download. Reading twelve per kind still guarantees the biggest item in every
// category reaches the ranking, because each query is ordered by its own
// strongest signal (amount, then age) before the limit applies.
const commandCenterActionsPerKind = 12

// CommandCenterVisibleActions is how many rows the screen shows.
//
// Six. Past that the list stops being "what needs me" and becomes a backlog, and
// a backlog is a thing people scroll instead of doing. The count that was
// truncated is NOT hidden — the payload's TotalActions says how many there are,
// so nobody reads six rows and concludes that is all of it.
const CommandCenterVisibleActions = 6

// CommandCenterResponse wraps the domain payload with the truncation counter.
type CommandCenterResponse struct {
	domain.CommandCenter
	// TotalActions is how many actions exist after snoozes, before the
	// visible-rows cut. Exposed so the screen can say "+7" instead of
	// implying the six shown are everything.
	TotalActions int `json:"total_actions"`
}

// Execute builds the home payload for one user.
func (uc *CommandCenterUseCase) Execute(ctx context.Context, actor domain.Actor) (*CommandCenterResponse, error) {
	// A manager sees the floor; a salesperson sees their own work. Anyone
	// else has no business on a sales command centre.
	isManager := domain.RoleSatisfies(actor.Role, domain.RoleGestor)
	if !isManager && actor.Role != domain.RoleVendedor {
		return nil, domain.ErrForbidden
	}

	tz := domain.FallbackTimezone
	if uc.accounts != nil {
		account, err := uc.accounts.GetByID(ctx, actor.AccountID)
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

	now := uc.now()
	// The tiles cover the current month in the ACCOUNT's timezone: "ganhos"
	// on the home screen has to mean the month the salesperson thinks it is.
	// MTD rather than a rolling 30 days because this number gets compared
	// against a commission statement, which is monthly.
	period := domain.ResolveSDRPeriod(domain.SDRPeriodMTD, tz, now)

	out := &CommandCenterResponse{}
	out.Tiles.PeriodLabel = period.From.Format("2006-01")

	var mu sync.Mutex
	warn := func(section string, err error) {
		slog.Warn("command centre section failed", "section", section, "error", err)
		mu.Lock()
		out.Warnings = append(out.Warnings, section)
		mu.Unlock()
	}

	var actions []domain.PendingAction
	collect := func(section string, fn func() ([]domain.PendingAction, error)) {
		rows, err := fn()
		if err != nil {
			warn(section, err)
			return
		}
		mu.Lock()
		actions = append(actions, rows...)
		mu.Unlock()
	}

	var briefing *domain.OvernightDraftRun
	var dismissed map[domain.DismissalKey]struct{}
	var byStatus map[domain.LeadStatus]int64

	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	// A salesperson's approval queue is their own; a manager's is the floor's.
	ownerFilter := ""
	if !isManager {
		ownerFilter = actor.UserID
	}

	// The ground nobody has walked. Its own section so a slow park query
	// degrades to a named warning instead of holding up the screen — and it
	// carries a TILE as well as rows, which no other source does.
	run(func() {
		if uc.untouched == nil {
			return
		}
		res, err := uc.untouched.Execute(ctx, actor.AccountID)
		if err != nil {
			warn("untouched", err)
			return
		}
		mu.Lock()
		out.Tiles.UntouchedTargets = int64(res.Total)
		actions = append(actions, res.Actions...)
		mu.Unlock()
	})

	run(func() {
		collect("approvals", func() ([]domain.PendingAction, error) {
			return uc.repo.PendingApprovals(ctx, actor.AccountID, ownerFilter, commandCenterActionsPerKind)
		})
	})
	run(func() {
		// Unowned leads are a manager's problem: a salesperson cannot assign
		// them, so the row would be a nag they cannot act on.
		if !isManager {
			return
		}
		collect("unowned", func() ([]domain.PendingAction, error) {
			return uc.repo.UnownedLeads(ctx, actor.AccountID, commandCenterActionsPerKind)
		})
	})
	// Signatures and contract alerts are DELIBERATELY not collected: their
	// screens were cut on 2026-08-18, so their deep links
	// (/settings/signatures, /settings/contracts/alerts) 404 — an action
	// the person cannot act on is a nag, and one that lands on NotFound is
	// worse. Re-enable together with the screens, not before.
	run(func() {
		collect("sales", func() ([]domain.PendingAction, error) {
			return uc.repo.UnconfirmedSales(ctx, actor.AccountID, commandCenterActionsPerKind)
		})
	})
	run(func() {
		collect("reorders", func() ([]domain.PendingAction, error) {
			return uc.repo.DueReorders(ctx, actor.AccountID, commandCenterActionsPerKind)
		})
	})
	run(func() {
		collect("waiting", func() ([]domain.PendingAction, error) {
			return uc.repo.WaitingReplies(ctx, actor.AccountID, ownerFilter, commandCenterActionsPerKind)
		})
	})

	run(func() {
		meetings, gross, currency, err := uc.repo.PeriodTotals(ctx, actor.AccountID, period.From, period.To)
		if err != nil {
			warn("period", err)
			return
		}
		mu.Lock()
		out.Tiles.Meetings = meetings
		out.Tiles.WonGrossCents = gross
		out.Tiles.Currency = currency
		mu.Unlock()
	})

	run(func() {
		var counts map[domain.LeadStatus]int64
		var err error
		if isManager {
			counts, err = uc.leadRepo.CountByStatus(ctx, actor.AccountID)
		} else {
			counts, err = uc.leadRepo.CountByStatusForOwner(ctx, actor.AccountID, actor.UserID)
		}
		if err != nil {
			warn("funnel", err)
			return
		}
		mu.Lock()
		byStatus = counts
		mu.Unlock()
	})

	run(func() {
		// The morning briefing (K2). Best-effort and NOT a warning when it
		// fails: it is the good news, and a screen that says "could not load
		// the good news" is worse than one that simply does not mention it.
		if uc.overnight == nil {
			return
		}
		latest, err := uc.overnight.LatestUnseen(ctx, actor.AccountID)
		if err != nil {
			slog.WarnContext(ctx, "command centre: overnight briefing unavailable", "error", err)
			return
		}
		mu.Lock()
		briefing = latest
		mu.Unlock()
	})

	run(func() {
		live, err := uc.repo.LiveDismissals(ctx, actor.AccountID, actor.UserID, now)
		if err != nil {
			// A failed snooze read must NOT hide the list. Showing a row
			// somebody snoozed is a small annoyance; hiding a renewal
			// deadline because a filter query blew up is a missed deadline.
			slog.Warn("command centre dismissals unavailable", "error", err)
			return
		}
		mu.Lock()
		dismissed = live
		mu.Unlock()
	})

	wg.Wait()

	out.Funnel = domain.BuildMiniFunnel(byStatus)
	for status, n := range byStatus {
		if status != domain.LeadStatusWon && status != domain.LeadStatusLost {
			out.Tiles.ActiveLeads += n
		}
	}
	out.Tiles.PendingApproval = byStatus[domain.LeadStatusPendingApproval]

	actions = domain.FilterDismissed(actions, dismissed)
	domain.RankPendingActions(actions, now)
	out.TotalActions = len(actions)
	if len(actions) > CommandCenterVisibleActions {
		actions = actions[:CommandCenterVisibleActions]
	}
	out.Actions = actions

	// Empty means "no pipeline at all", not "nothing pending": a brand-new
	// account reading "0 · 0 · 0 · R$ 0" learns that the product does nothing,
	// so the client shows the activation checklist instead.
	out.Empty = len(byStatus) == 0 && out.TotalActions == 0

	if briefing.Worth() {
		out.Briefing = briefing
	}

	out.Suggestion = suggestForCommandCenter(out.Actions, out.Empty)
	return out, nil
}

// suggestForCommandCenter picks the ONE copilot chip.
//
// One, never a list: a screen with three suggestions is a screen where all three
// are ignored. It is derived from the top-ranked action rather than from a
// generic tips table, so the chip is about the thing actually in front of the
// user — and it carries a prompt the copilot can act on immediately, because a
// suggestion the user has to retype is a suggestion nobody taps.
//
// Returns nil when there is nothing pressing. No chip is a legitimate state; a
// filler suggestion trains people to ignore the slot.
func suggestForCommandCenter(actions []domain.PendingAction, empty bool) *domain.CommandCenterSuggestion {
	if empty {
		return &domain.CommandCenterSuggestion{
			Label:  "Encontrar meus primeiros clientes",
			Prompt: "Me ajude a encontrar os primeiros clientes para prospectar: pergunte o que eu vendo e para quem, e monte a lista.",
		}
	}
	if len(actions) == 0 {
		return nil
	}

	top := actions[0]
	who := top.CustomerName
	if who == "" {
		who = "esse cliente"
	}

	switch top.Kind {
	case domain.PendingSignatureViewed:
		return &domain.CommandCenterSuggestion{
			Label: fmt.Sprintf("Retomar %s", who),
			Prompt: fmt.Sprintf(
				"%s abriu o contrato e não assinou. Me mostre o histórico da conversa e sugira uma mensagem curta de retomada — não envie nada sem eu aprovar.", who),
		}
	case domain.PendingContractAlert:
		return &domain.CommandCenterSuggestion{
			Label: fmt.Sprintf("Preparar renovação de %s", who),
			Prompt: fmt.Sprintf(
				"Me prepare para a renovação de %s: o que o contrato diz sobre prazo, reajuste e multa, e o que eu deveria propor.", who),
		}
	case domain.PendingConfirmSale:
		return &domain.CommandCenterSuggestion{
			Label:  "Conferir vendas pendentes",
			Prompt: "Liste as vendas que a IA extraiu e ainda não foram confirmadas, com o valor de cada uma, para eu revisar.",
		}
	case domain.PendingApproveDraft:
		return &domain.CommandCenterSuggestion{
			Label:  "Revisar a fila de aprovação",
			Prompt: "Me mostre as abordagens esperando aprovação, começando pelas mais antigas, com o texto de cada uma.",
		}
	case domain.PendingReorderDue:
		return &domain.CommandCenterSuggestion{
			Label: fmt.Sprintf("Falar com %s", who),
			Prompt: fmt.Sprintf(
				"%s já deveria ter recomprado. Me mostre o que comprou da última vez e sugira uma mensagem — não envie sem eu aprovar.", who),
		}
	case domain.PendingUnownedLead:
		return &domain.CommandCenterSuggestion{
			Label:  "Distribuir os leads sem dono",
			Prompt: "Liste os leads sem responsável e sugira para quem distribuir cada um, considerando quem está com menos carga.",
		}
	}
	return nil
}

// SnoozeFrom turns an hour count into a deadline on the use case's own clock, so
// a caller that only knows "72 hours" does not have to reach for time.Now and
// drift from the clock the validation uses.
func (uc *CommandCenterUseCase) SnoozeFrom(hours int) time.Time {
	return uc.now().Add(time.Duration(hours) * time.Hour)
}

// Dismiss snoozes one action for this user.
func (uc *CommandCenterUseCase) Dismiss(ctx context.Context, actor domain.Actor, d *domain.PendingActionDismissal) error {
	d.AccountID = actor.AccountID
	// The snooze is the CALLER's judgement, never a target the request can
	// name: letting a client dismiss on somebody else's behalf would hide a
	// deadline from the person responsible for it.
	d.UserID = actor.UserID
	if err := d.Validate(uc.now()); err != nil {
		return err
	}
	return uc.repo.Dismiss(ctx, d)
}
