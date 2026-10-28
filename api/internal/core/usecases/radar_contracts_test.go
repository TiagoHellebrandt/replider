package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

type radarCalendarFake struct {
	cal        *ContractCalendar
	err        error
	windowDays int
}

func (f *radarCalendarFake) Execute(ctx context.Context, accountID string, windowDays int) (*ContractCalendar, error) {
	f.windowDays = windowDays
	return f.cal, f.err
}

type radarQueueFake struct {
	leads []*domain.Lead
	err   error
	// seen is the filter the radar actually asked with. The whole point of
	// the scoping tests is the filter, not the answer.
	seen  domain.ApprovalQueueFilter
	calls int
}

func (f *radarQueueFake) ListForApprovalQueue(ctx context.Context, filter domain.ApprovalQueueFilter, limit, offset int32) ([]*domain.Lead, error) {
	f.seen = filter
	f.calls++
	return f.leads, f.err
}

// contractHarness is radarHarness plus the two optional collaborators, with an
// account that has no territory so the park collector stops early and leaves
// the assertions to the contract rows.
func contractHarness(cal *radarCalendarFake, queue *radarQueueFake) *RadarUseCase {
	uc, accounts, _ := radarHarness(&radarFakes{})
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)
	if cal != nil {
		uc.SetContractCalendar(cal)
	}
	if queue != nil {
		uc.SetLeads(queue)
	}
	return uc
}

func radarCalendarWith(entries ...ContractCalendarEntry) *radarCalendarFake {
	return &radarCalendarFake{cal: &ContractCalendar{Entries: entries}}
}

// radarNow is the clock radarHarness pins. Reorder and date rows are filtered
// against it, so a test that reaches for time.Now() builds rows the use case
// then drops — silently, since an empty feed is a legitimate answer.
var radarNow = time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

func radarIn(d time.Duration) *time.Time {
	t := radarNow.Add(d)
	return &t
}

func TestRadarContracts_AnAtaAndAContractAreDifferentClaims(t *testing.T) {
	expiry := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	cal := radarCalendarWith(
		ContractCalendarEntry{
			PNCPID: "pncp-1", BuyerName: "Santa Casa de Cuiabá",
			Object: "ventiladores pulmonares", SupplierName: "Medicalpar",
			ValueCents: 34000000, City: "Cuiabá", UF: "MT", ExpiresAt: &expiry,
		},
		ContractCalendarEntry{
			PNCPID: "pncp-2", BuyerName: "Consórcio do Vale do Arinos",
			Object: "incubadora neonatal", City: "Juara", UF: "MT",
			ExpiresAt: &expiry, PriceRegistry: true,
		},
	)

	out, err := contractHarness(cal, nil).Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Empty(t, out.Warnings)
	byID := map[string]RadarItem{}
	for _, it := range out.Items {
		byID[it.ID] = it
	}
	contract, ok := byID["contract-pncp-1"]
	if assert.True(t, ok, "the expiring contract must be a row") {
		assert.Equal(t, RadarContrato, contract.Type)
		// The whole pitch of this row, and it is only true for a contract:
		// an ata needs no edital at all, so the badge must not be shared.
		assert.Equal(t, RadarEdgeBeforeTender, contract.Edge)
		assert.Equal(t, "Medicalpar", contract.Incumbent)
		assert.Equal(t, "Cuiabá", contract.City)
		assert.Equal(t, "MT", contract.UF)
		assert.Equal(t, int64(34000000), contract.ValueCents)
		assert.Contains(t, contract.Context, "fornecedor atual: Medicalpar")
	}
	ata, ok := byID["contract-pncp-2"]
	if assert.True(t, ok, "the price registry must be a row") {
		assert.Equal(t, RadarAta, ata.Type)
		assert.Equal(t, RadarEdgeNoTender, ata.Edge)
	}
	assert.Equal(t, CalendarWindowDays, cal.windowDays)
}

// A buyer who rides other bodies' registries can buy today, with no process of
// their own — so the row makes the ata's claim even though the record is a
// plain contract.
func TestRadarContracts_ABuyerWhoRidesOthersNeedsNoTender(t *testing.T) {
	expiry := *radarIn(40 * 24 * time.Hour)
	cal := radarCalendarWith(ContractCalendarEntry{
		PNCPID: "pncp-3", BuyerName: "Hospital de Várzea Grande",
		ExpiresAt: &expiry, RidesOthers: true,
	})

	out, err := contractHarness(cal, nil).Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	if assert.Len(t, out.Items, 1) {
		assert.Equal(t, RadarAta, out.Items[0].Type)
		assert.Equal(t, RadarEdgeNoTender, out.Items[0].Edge)
		assert.Contains(t, out.Items[0].Context, "compra por adesão")
	}
}

// An unnamed supplier is stated, never skipped: a row that simply omits it
// reads as "nobody supplies them today", which is a far more encouraging
// claim than the published record actually makes.
func TestRadarContracts_AnUnnamedSupplierIsSaidOutLoud(t *testing.T) {
	expiry := *radarIn(30 * 24 * time.Hour)
	cal := radarCalendarWith(ContractCalendarEntry{
		PNCPID: "pncp-4", BuyerName: "Secretaria de Saúde", ExpiresAt: &expiry,
	})

	out, err := contractHarness(cal, nil).Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	if assert.Len(t, out.Items, 1) {
		assert.Empty(t, out.Items[0].Incumbent)
		assert.Contains(t, out.Items[0].Context, "não nomeia fornecedor")
	}
}

func TestRadarContracts_AFailedCalendarIsANamedWarningNotSilence(t *testing.T) {
	cal := &radarCalendarFake{err: errors.New("pncp down")}

	out, err := contractHarness(cal, nil).Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Contains(t, out.Warnings, "contrato")
	assert.Empty(t, out.Items)
}

// An unwired calendar keeps the five original sources instead of failing —
// and must not invent a warning about a collector nobody asked for.
func TestRadarContracts_AnUnwiredCalendarIsNotAFailure(t *testing.T) {
	out, err := contractHarness(nil, nil).Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Empty(t, out.Warnings)
}

func TestRadarReady_AVendedorSeesOnlyTheirOwnDrafts(t *testing.T) {
	expiry := *radarIn(20 * 24 * time.Hour)
	cal := radarCalendarWith(ContractCalendarEntry{
		PNCPID: "pncp-5", BuyerName: "Hospital Regional", ExpiresAt: &expiry,
	})
	queue := &radarQueueFake{}
	uc := contractHarness(cal, queue)

	_, err := uc.ExecuteFor(context.Background(), domain.Actor{
		AccountID: "acc-1", UserID: "user-7", Role: domain.RoleVendedor,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, queue.calls)
	// The mark is a claim about MY work: pointing a vendedor at a colleague's
	// draft opens a queue that does not contain it.
	assert.Equal(t, "user-7", queue.seen.OwnerUserID)
	assert.Equal(t, domain.LeadStatusPendingApproval, queue.seen.Status)
	assert.Equal(t, "acc-1", queue.seen.AccountID)
}

func TestRadarReady_AGestorSeesTheWholeFloor(t *testing.T) {
	queue := &radarQueueFake{}
	// One row, because markReady deliberately does not read the queue for an
	// empty feed — there would be nothing to mark.
	uc := contractHarness(radarCalendarWith(ContractCalendarEntry{
		PNCPID: "pncp-6", BuyerName: "Hospital", ExpiresAt: radarIn(30 * 24 * time.Hour),
	}), queue)

	_, err := uc.ExecuteFor(context.Background(), domain.Actor{
		AccountID: "acc-1", UserID: "user-1", Role: domain.RoleGestor,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, queue.calls)
	assert.Empty(t, queue.seen.OwnerUserID, "a gestor's queue is the account's")
}

// The approval queue refuses an attendant outright. The radar must not leak
// the shape of a pipeline they cannot open.
func TestRadarReady_AnAttendantIsNeverToldWhatIsDrafted(t *testing.T) {
	queue := &radarQueueFake{}
	// With a row present, so the zero reads below are the ROLE's doing and
	// not the empty-feed short-circuit.
	uc := contractHarness(radarCalendarWith(ContractCalendarEntry{
		PNCPID: "pncp-7", BuyerName: "Hospital", ExpiresAt: radarIn(30 * 24 * time.Hour),
	}), queue)

	_, err := uc.ExecuteFor(context.Background(), domain.Actor{
		AccountID: "acc-1", UserID: "user-2", Role: domain.RoleAtendenteRole,
	})

	assert.NoError(t, err)
	assert.Zero(t, queue.calls, "no read at all, not a filtered one")
}

func TestRadarReady_MarksTheRowsThatCarryEitherHandle(t *testing.T) {
	in5d := radarIn(5 * 24 * time.Hour)
	f := &radarFakes{
		signals: []*domain.Signal{{
			ID: "sig-1", Organization: "Pref. Cuiabá", Title: "Ventiladores",
			ClosesAt: in5d, LeadID: "lead-9",
		}},
		customers: []*domain.PortfolioCustomer{
			{ContactID: "ct-1", ContactName: "Clínica Santa Rita", NextDueAt: in5d},
			{ContactID: "ct-2", ContactName: "Sem rascunho", NextDueAt: in5d},
		},
	}
	uc, accounts, _ := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)
	uc.SetLeads(&radarQueueFake{leads: []*domain.Lead{
		// A tender the overnight run already drafted: known by LEAD id.
		{ID: "lead-9", ContactID: "ct-9"},
		// A customer's reorder draft: known only by CONTACT id.
		{ID: "lead-3", ContactID: "ct-1"},
	}})

	out, err := uc.ExecuteFor(context.Background(), domain.Actor{
		AccountID: "acc-1", UserID: "u", Role: domain.RoleGestor,
	})

	assert.NoError(t, err)
	ready := map[string]bool{}
	for _, it := range out.Items {
		ready[it.ID] = it.Ready
	}
	assert.True(t, ready["sig-1"], "matched by lead id")
	assert.True(t, ready["reorder-ct-1"], "matched by contact id")
	assert.False(t, ready["reorder-ct-2"], "nothing drafted for this one")
}

// A failed queue read leaves every row unmarked. A row that wrongly says
// "prepare" costs one click; a row that wrongly says "ready to send" opens
// onto nothing and reads as broken.
func TestRadarReady_AFailedQueueReadMarksNothing(t *testing.T) {
	f := &radarFakes{customers: []*domain.PortfolioCustomer{
		{ContactID: "ct-1", ContactName: "Clínica", NextDueAt: radarIn(5 * 24 * time.Hour)},
	}}
	uc, accounts, _ := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)
	uc.SetLeads(&radarQueueFake{err: errors.New("db down")})

	out, err := uc.ExecuteFor(context.Background(), domain.Actor{
		AccountID: "acc-1", UserID: "u", Role: domain.RoleGestor,
	})

	assert.NoError(t, err)
	if assert.Len(t, out.Items, 1) {
		assert.False(t, out.Items[0].Ready)
	}
}

// Execute (account id only) is still a legitimate entry point — it just has
// nobody to answer "is this MY draft?" for, so it marks nothing.
func TestRadarReady_WithoutAnActorNothingIsMarked(t *testing.T) {
	queue := &radarQueueFake{}
	uc := contractHarness(radarCalendarWith(ContractCalendarEntry{
		PNCPID: "pncp-8", BuyerName: "Hospital", ExpiresAt: radarIn(30 * 24 * time.Hour),
	}), queue)

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Len(t, out.Items, 1, "the row is there — only the mark is missing")
	assert.Zero(t, queue.calls)
}
