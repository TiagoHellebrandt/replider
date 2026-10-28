package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// Command centre (I2).
//
// What these pin: a salesperson never sees work they cannot do, a broken section
// becomes a named gap rather than a false "nothing needs you", the list is
// truncated WITH its total, and there is exactly one copilot chip.

type fakeCommandCenterRepo struct {
	approvals  []domain.PendingAction
	unowned    []domain.PendingAction
	signatures []domain.PendingAction
	contracts  []domain.PendingAction
	sales      []domain.PendingAction
	reorders   []domain.PendingAction
	waiting    []domain.PendingAction

	approvalsErr error
	contractsErr error
	salesErr     error
	dismissalErr error

	dismissed map[domain.DismissalKey]struct{}

	meetings  int64
	gross     int64
	currency  string
	totalsErr error

	// gotOwnerFilter records how the approval queue was scoped — the whole
	// point of the role test.
	gotOwnerFilter string
	unownedCalled  bool
	saved          *domain.PendingActionDismissal
}

func (f *fakeCommandCenterRepo) PendingApprovals(_ context.Context, _, ownerUserID string, _ int) ([]domain.PendingAction, error) {
	f.gotOwnerFilter = ownerUserID
	return f.approvals, f.approvalsErr
}

func (f *fakeCommandCenterRepo) UnownedLeads(context.Context, string, int) ([]domain.PendingAction, error) {
	f.unownedCalled = true
	return f.unowned, nil
}

func (f *fakeCommandCenterRepo) ViewedSignatures(context.Context, string, int) ([]domain.PendingAction, error) {
	return f.signatures, nil
}

func (f *fakeCommandCenterRepo) ContractAlerts(context.Context, string, int) ([]domain.PendingAction, error) {
	return f.contracts, f.contractsErr
}

func (f *fakeCommandCenterRepo) UnconfirmedSales(context.Context, string, int) ([]domain.PendingAction, error) {
	return f.sales, f.salesErr
}

func (f *fakeCommandCenterRepo) DueReorders(context.Context, string, int) ([]domain.PendingAction, error) {
	return f.reorders, nil
}

func (f *fakeCommandCenterRepo) WaitingReplies(context.Context, string, string, int) ([]domain.PendingAction, error) {
	return f.waiting, nil
}

func (f *fakeCommandCenterRepo) PeriodTotals(context.Context, string, time.Time, time.Time) (int64, int64, string, error) {
	return f.meetings, f.gross, f.currency, f.totalsErr
}

func (f *fakeCommandCenterRepo) LiveDismissals(context.Context, string, string, time.Time) (map[domain.DismissalKey]struct{}, error) {
	return f.dismissed, f.dismissalErr
}

func (f *fakeCommandCenterRepo) Dismiss(_ context.Context, d *domain.PendingActionDismissal) error {
	f.saved = d
	return nil
}

// fakeCCLeads is the narrow slice of LeadRepository the command centre reads.
type fakeCCLeads struct {
	domain.LeadRepository
	byStatus      map[domain.LeadStatus]int64
	ownerByStatus map[domain.LeadStatus]int64
	err           error
	ownerScoped   bool
}

func (f *fakeCCLeads) CountByStatus(context.Context, string) (map[domain.LeadStatus]int64, error) {
	return f.byStatus, f.err
}

func (f *fakeCCLeads) CountByStatusForOwner(context.Context, string, string) (map[domain.LeadStatus]int64, error) {
	f.ownerScoped = true
	return f.ownerByStatus, f.err
}

func newCommandCenter(t *testing.T, repo *fakeCommandCenterRepo, leads *fakeCCLeads) *CommandCenterUseCase {
	t.Helper()
	accounts := &MockAccountRepo{}
	accounts.On("GetByID", mock.Anything, "acc").
		Return(&domain.Account{ID: "acc", Timezone: "America/Sao_Paulo"}, nil)

	uc := NewCommandCenterUseCase(repo, leads, accounts)
	uc.now = func() time.Time { return time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC) }
	return uc
}

func manager() domain.Actor {
	return domain.Actor{AccountID: "acc", UserID: "u1", Role: domain.RoleAdminRole}
}

func seller() domain.Actor {
	return domain.Actor{AccountID: "acc", UserID: "u2", Role: domain.RoleVendedor}
}

func money(cents int64) domain.PendingAction {
	return domain.PendingAction{
		Kind: domain.PendingSignatureViewed, TargetID: "sig", MoneyAtStakeCents: cents,
		CustomerName: "Clínica Bem Viver",
	}
}

func TestCommandCenter_ASalespersonSeesOnlyWorkTheyCanDo(t *testing.T) {
	repo := &fakeCommandCenterRepo{
		unowned: []domain.PendingAction{{Kind: domain.PendingUnownedLead, TargetID: "l9"}},
	}
	leads := &fakeCCLeads{ownerByStatus: map[domain.LeadStatus]int64{domain.LeadStatusNew: 3}}
	uc := newCommandCenter(t, repo, leads)

	out, err := uc.Execute(context.Background(), seller())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// A salesperson cannot assign leads, so an "unowned" row would be a nag
	// they cannot act on — the query is not even run.
	if repo.unownedCalled {
		t.Fatal("unowned leads must not be read for a salesperson")
	}
	// And their approval queue is theirs, not the floor's.
	if repo.gotOwnerFilter != "u2" {
		t.Fatalf("owner filter = %q, want the caller's id", repo.gotOwnerFilter)
	}
	if !leads.ownerScoped {
		t.Fatal("the funnel must be scoped to the salesperson's own leads")
	}
	for _, a := range out.Actions {
		if a.Kind == domain.PendingUnownedLead {
			t.Fatal("an unowned-lead row reached a salesperson")
		}
	}
}

func TestCommandCenter_AManagerSeesTheFloor(t *testing.T) {
	repo := &fakeCommandCenterRepo{
		unowned: []domain.PendingAction{{Kind: domain.PendingUnownedLead, TargetID: "l9"}},
	}
	uc := newCommandCenter(t, repo, &fakeCCLeads{
		byStatus: map[domain.LeadStatus]int64{domain.LeadStatusNew: 3},
	})

	out, err := uc.Execute(context.Background(), manager())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if repo.gotOwnerFilter != "" {
		t.Fatalf("owner filter = %q, want the whole account", repo.gotOwnerFilter)
	}
	if len(out.Actions) != 1 || out.Actions[0].Kind != domain.PendingUnownedLead {
		t.Fatalf("actions = %+v, want the unowned lead", out.Actions)
	}
}

func TestCommandCenter_AnAttendantHasNoBusinessHere(t *testing.T) {
	uc := newCommandCenter(t, &fakeCommandCenterRepo{}, &fakeCCLeads{})

	_, err := uc.Execute(context.Background(),
		domain.Actor{AccountID: "acc", UserID: "u3", Role: domain.RoleAtendenteRole})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v, want forbidden", err)
	}
}

func TestCommandCenter_ABrokenSectionIsANamedGapNotSilence(t *testing.T) {
	// Signatures/contracts were cut with their screens; the broken section
	// is now the sales sweep, with the reorders section proving the rest
	// still renders.
	repo := &fakeCommandCenterRepo{
		salesErr: errors.New("lock timeout"),
		reorders: []domain.PendingAction{money(10000000)},
	}
	uc := newCommandCenter(t, repo, &fakeCCLeads{
		byStatus: map[domain.LeadStatus]int64{domain.LeadStatusNew: 2},
	})

	out, err := uc.Execute(context.Background(), manager())
	if err != nil {
		t.Fatalf("one failing section must not fail the screen: %v", err)
	}

	// "Nothing needs you" is a CLAIM. If the contract deadlines did not load,
	// the screen has to say so rather than imply there are none.
	found := false
	for _, w := range out.Warnings {
		if w == "sales" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want the failed section named", out.Warnings)
	}
	if len(out.Actions) != 1 {
		t.Fatalf("the sections that DID load must still render: %+v", out.Actions)
	}
}

func TestCommandCenter_AFailedSnoozeReadShowsTooMuchRatherThanTooLittle(t *testing.T) {
	repo := &fakeCommandCenterRepo{
		dismissalErr: errors.New("timeout"),
		sales: []domain.PendingAction{{
			Kind: domain.PendingConfirmSale, TargetID: "alert-1",
			MoneyAtStakeCents: 4000000, CustomerName: "Hospital Santa Rita",
		}},
	}
	uc := newCommandCenter(t, repo, &fakeCCLeads{})

	out, err := uc.Execute(context.Background(), manager())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Showing a row somebody snoozed is a small annoyance. Hiding a R$40k
	// renewal because the filter query blew up is a missed deadline — so a
	// failed dismissal read must fail OPEN, and it is not a section warning
	// either (nothing is missing from the list).
	if len(out.Actions) != 1 {
		t.Fatalf("actions = %+v, want the alert shown", out.Actions)
	}
	for _, w := range out.Warnings {
		if w == "dismissals" {
			t.Fatal("a snooze-read failure hides nothing, so it must not warn")
		}
	}
}

func TestCommandCenter_SnoozedRowsAreFilteredOut(t *testing.T) {
	repo := &fakeCommandCenterRepo{
		reorders: []domain.PendingAction{money(10000000)},
		dismissed: map[domain.DismissalKey]struct{}{
			{Kind: domain.PendingSignatureViewed, TargetID: "sig"}: {},
		},
	}
	uc := newCommandCenter(t, repo, &fakeCCLeads{})

	out, _ := uc.Execute(context.Background(), manager())

	if len(out.Actions) != 0 {
		t.Fatalf("actions = %+v, want the snoozed row gone", out.Actions)
	}
	if out.TotalActions != 0 {
		t.Fatalf("total = %d, want 0 — a snoozed row is not a hidden pending one",
			out.TotalActions)
	}
}

func TestCommandCenter_TruncationReportsWhatItTruncated(t *testing.T) {
	repo := &fakeCommandCenterRepo{}
	for i := 0; i < 9; i++ {
		repo.reorders = append(repo.reorders, domain.PendingAction{
			Kind: domain.PendingSignatureViewed, TargetID: string(rune('a' + i)),
			MoneyAtStakeCents: int64(1000 * (10 - i)),
		})
	}
	uc := newCommandCenter(t, repo, &fakeCCLeads{})

	out, _ := uc.Execute(context.Background(), manager())

	if len(out.Actions) != CommandCenterVisibleActions {
		t.Fatalf("shown = %d, want %d", len(out.Actions), CommandCenterVisibleActions)
	}
	// Six rows with no total would read as "that is all of it".
	if out.TotalActions != 9 {
		t.Fatalf("total = %d, want 9", out.TotalActions)
	}
	// And the six shown are the biggest ones, not the first six read.
	if out.Actions[0].MoneyAtStakeCents != 10000 {
		t.Fatalf("first = %d, want the largest", out.Actions[0].MoneyAtStakeCents)
	}
}

func TestCommandCenter_ExactlyOneSuggestionAndItIsAboutTheTopRow(t *testing.T) {
	repo := &fakeCommandCenterRepo{reorders: []domain.PendingAction{money(10000000)}}
	uc := newCommandCenter(t, repo, &fakeCCLeads{
		byStatus: map[domain.LeadStatus]int64{domain.LeadStatusNew: 1},
	})

	out, _ := uc.Execute(context.Background(), manager())

	if out.Suggestion == nil {
		t.Fatal("want one suggestion")
	}
	// It is about the thing actually in front of the user, and it carries a
	// prompt the copilot can act on without the user retyping anything.
	if !strings.Contains(out.Suggestion.Label, "Clínica Bem Viver") {
		t.Fatalf("label = %q, want it to name the customer", out.Suggestion.Label)
	}
	if out.Suggestion.Prompt == "" {
		t.Fatal("a suggestion with no prompt is a hint the user has to retype")
	}
}

func TestCommandCenter_NothingPressingMeansNoChipAtAll(t *testing.T) {
	uc := newCommandCenter(t, &fakeCommandCenterRepo{}, &fakeCCLeads{
		// A pipeline exists, so this is not the empty-account case.
		byStatus: map[domain.LeadStatus]int64{domain.LeadStatusWon: 4},
	})

	out, _ := uc.Execute(context.Background(), manager())

	// No chip is a legitimate state. A filler suggestion trains people to
	// ignore the slot, which costs the one time it matters.
	if out.Suggestion != nil {
		t.Fatalf("suggestion = %+v, want none", out.Suggestion)
	}
	if out.Empty {
		t.Fatal("an account with won deals is not empty")
	}
}

func TestCommandCenter_AnEmptyAccountGetsActivationNotAWallOfZeros(t *testing.T) {
	uc := newCommandCenter(t, &fakeCommandCenterRepo{}, &fakeCCLeads{})

	out, _ := uc.Execute(context.Background(), manager())

	// "0 · 0 · 0 · R$ 0" teaches a new user that the product does nothing.
	if !out.Empty {
		t.Fatal("an account with no pipeline and no actions must report empty")
	}
	if out.Suggestion == nil {
		t.Fatal("the empty state is exactly when a suggestion earns its place")
	}
}

func TestCommandCenter_TilesCountTheActivePipelineNotTheWholeThing(t *testing.T) {
	uc := newCommandCenter(t, &fakeCommandCenterRepo{meetings: 3, gross: 500000, currency: "brl"},
		&fakeCCLeads{byStatus: map[domain.LeadStatus]int64{
			domain.LeadStatusNew:             10,
			domain.LeadStatusPendingApproval: 4,
			domain.LeadStatusWon:             7,
			domain.LeadStatusLost:            2,
		}})

	out, _ := uc.Execute(context.Background(), manager())

	if out.Tiles.ActiveLeads != 14 {
		t.Fatalf("active = %d, want 14 (10 + 4, excluding won/lost)", out.Tiles.ActiveLeads)
	}
	if out.Tiles.PendingApproval != 4 {
		t.Fatalf("pending = %d, want 4", out.Tiles.PendingApproval)
	}
	if out.Tiles.Meetings != 3 || out.Tiles.WonGrossCents != 500000 {
		t.Fatalf("period tiles = %+v", out.Tiles)
	}
	// The period is named: "ganhos" with no window is a number nobody can
	// check. July in São Paulo, from a clock that says 10:00 UTC on the 30th.
	if out.Tiles.PeriodLabel != "2026-07" {
		t.Fatalf("period label = %q, want 2026-07", out.Tiles.PeriodLabel)
	}
}

func TestCommandCenter_DismissIsAlwaysTheCallersOwnJudgement(t *testing.T) {
	repo := &fakeCommandCenterRepo{}
	uc := newCommandCenter(t, repo, &fakeCCLeads{})

	err := uc.Dismiss(context.Background(), seller(), &domain.PendingActionDismissal{
		Kind: domain.PendingApproveDraft, TargetID: "l1",
		// A client trying to snooze on somebody else's behalf.
		UserID: "someone-else", AccountID: "other-account",
	})
	if err != nil {
		t.Fatalf("Dismiss: %v", err)
	}

	// Hiding a deadline from the person responsible for it is exactly the
	// failure this overwrite prevents.
	if repo.saved.UserID != "u2" {
		t.Fatalf("saved user = %q, want the caller", repo.saved.UserID)
	}
	if repo.saved.AccountID != "acc" {
		t.Fatalf("saved account = %q, want the caller's", repo.saved.AccountID)
	}
	if !repo.saved.SnoozedUntil.After(uc.now()) {
		t.Fatal("a snooze must land in the future")
	}
}
