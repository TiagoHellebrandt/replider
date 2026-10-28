package usecases

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

type calendarRepo struct {
	rows    []domain.PublicContract
	byBuyer []domain.PublicContract
	got     domain.ContractCalendarFilter
}

func (r *calendarRepo) ListByBuyer(context.Context, string, int) ([]domain.PublicContract, error) {
	return r.byBuyer, nil
}
func (r *calendarRepo) Save(context.Context, []domain.PublicContract) error { return nil }
func (r *calendarRepo) LastScan(context.Context, string) (time.Time, error) {
	return time.Time{}, nil
}
func (r *calendarRepo) MarkScanned(context.Context, string, int) error { return nil }
func (r *calendarRepo) ListExpiring(_ context.Context, f domain.ContractCalendarFilter) ([]domain.PublicContract, error) {
	r.got = f
	return r.rows, nil
}
func (r *calendarRepo) BuyersNeedingScan(context.Context, []string, []string, time.Time, int) ([]domain.ContractScanTarget, error) {
	return nil, nil
}

type calendarWatches struct{ list []*domain.SignalWatch }

func (w calendarWatches) ListWatches(context.Context, string) ([]*domain.SignalWatch, error) {
	return w.list, nil
}

func calendarUC(t *testing.T, repo *calendarRepo, account *domain.Account, watches calendarWatches) *ContractCalendarUseCase {
	t.Helper()
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc1").Return(account, nil)
	uc := NewContractCalendarUseCase(repo, accounts, watches)
	uc.now = func() time.Time { return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC) }
	return uc
}

func calendarDaysFromNow(n int) *time.Time {
	d := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n)
	return &d
}

// The calendar is the only forward-looking read in the product: a contract
// with a published end date is a purchase that MUST happen again, months
// before any tender exists.
func TestContractCalendar_CountsTheDaysAndNamesTheDoor(t *testing.T) {
	repo := &calendarRepo{rows: []domain.PublicContract{
		{
			PNCPID: "c1", BuyerName: "Hospital A", Object: "monitor multiparametrico",
			ExpiresAt: calendarDaysFromNow(45), AtaPNCPID: "ata-1", FromAdhesion: true,
		},
		{PNCPID: "c2", BuyerName: "Hospital B", Object: "ventilador", ExpiresAt: calendarDaysFromNow(-5)},
	}}
	uc := calendarUC(t, repo,
		&domain.Account{ID: "acc1", Territory: &domain.Territory{UF: "MT"}},
		calendarWatches{list: []*domain.SignalWatch{
			{Keywords: []string{"monitor", "ventilador"}, IsActive: true},
		}})

	out, err := uc.Execute(context.Background(), "acc1", 0)
	require.NoError(t, err)
	require.Len(t, out.Entries, 2)

	assert.Equal(t, 45, *out.Entries[0].DaysLeft)
	assert.True(t, out.Entries[0].PriceRegistry, "an ata is a year of selling, not one closed deal")
	assert.True(t, out.Entries[0].RidesOthers, "this buyer can buy without running a tender")
	assert.Contains(t, out.Entries[0].URL, "c1", "every claim must be checkable")

	// Already closed is a negative count, not a hidden row: the buyer is very
	// probably mid-process right now.
	assert.Equal(t, -5, *out.Entries[1].DaysLeft)
	assert.False(t, out.Entries[1].PriceRegistry)
}

// The window opens BEHIND today and closes six months out — the head start the
// instrument actually gives.
func TestContractCalendar_LooksBackAMonthAndForwardSixMonths(t *testing.T) {
	repo := &calendarRepo{}
	uc := calendarUC(t, repo,
		&domain.Account{ID: "acc1", Territory: &domain.Territory{UF: "MT"}},
		calendarWatches{list: []*domain.SignalWatch{{Keywords: []string{"monitor"}, IsActive: true}}})

	_, err := uc.Execute(context.Background(), "acc1", 0)
	require.NoError(t, err)

	assert.Equal(t, []string{"MT"}, repo.got.States)
	assert.True(t, repo.got.From.Before(uc.now()), "recently expired stays visible")
	assert.Equal(t, 180, int(repo.got.To.Sub(uc.now()).Hours()/24))
}

// Three different facts render as zero rows, and the screen has to tell them
// apart or a working feature gets reported as broken.
func TestContractCalendar_EmptyAnswersSayWhichKindOfEmpty(t *testing.T) {
	words := calendarWatches{list: []*domain.SignalWatch{{Keywords: []string{"monitor"}, IsActive: true}}}

	noTerritory := calendarUC(t, &calendarRepo{}, &domain.Account{ID: "acc1"}, words)
	out, err := noTerritory.Execute(context.Background(), "acc1", 0)
	require.NoError(t, err)
	assert.Empty(t, out.States, "no territory: the screen says where to set it")

	noWords := calendarUC(t, &calendarRepo{},
		&domain.Account{ID: "acc1", Territory: &domain.Territory{UF: "MT"}},
		calendarWatches{})
	out, err = noWords.Execute(context.Background(), "acc1", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"MT"}, out.States)
	assert.Empty(t, out.Keywords, "no words: the screen says to add products")

	stillReading := calendarUC(t, &calendarRepo{},
		&domain.Account{ID: "acc1", Territory: &domain.Territory{UF: "MT"}}, words)
	out, err = stillReading.Execute(context.Background(), "acc1", 0)
	require.NoError(t, err)
	assert.True(t, out.Scanning, "day-one empty means still reading, not nothing found")
}

// The calendar must match what the radar matches. A second keyword list to
// configure is a second list to drift, and the day they disagree neither can
// be trusted.
func TestContractCalendar_ReusesTheRadarsWords(t *testing.T) {
	repo := &calendarRepo{}
	uc := calendarUC(t, repo,
		&domain.Account{ID: "acc1", Territory: &domain.Territory{UF: "MT"}},
		calendarWatches{list: []*domain.SignalWatch{
			{Keywords: []string{"monitor", "Monitor", "ventilador"}, IsActive: true},
			{Keywords: []string{"incubadora"}, IsActive: false}, // switched off
		}})

	out, err := uc.Execute(context.Background(), "acc1", 0)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"monitor", "ventilador"}, out.Keywords,
		"deduped, and a watch somebody turned off stays off")
	assert.NotContains(t, out.Keywords, "incubadora")
}

// A calendar you can only read is a calendar read once.
//
// Every other surface in this product ends in something you can do. This one
// ended in "open the public record", which is a link to a government website —
// and the whole point of knowing a contract ends in January is to be working
// that buyer in October.
func TestWorkContractWindow_PutsTheBuyerInTheFunnel(t *testing.T) {
	expires := time.Date(2027, 1, 14, 0, 0, 0, 0, time.UTC)
	contract := domain.PublicContract{
		PNCPID: "03507415002864-2-000266/2026", BuyerCNPJ: "03507415002864",
		BuyerName: "ESTADO DE MATO GROSSO", SupplierName: "Concorrente Ltda",
		Object: "monitores multiparametricos", ValueCents: 42000000,
		ExpiresAt: &expires, UF: "MT", City: "Cuiabá",
		AtaPNCPID: "ata-1", FromAdhesion: true,
	}
	uc, leads, contacts := workWindowUC(t, contract)

	lead, err := uc.Execute(context.Background(), "acc1", contract.PNCPID)
	require.NoError(t, err)
	require.NotNil(t, lead)

	assert.Equal(t, ContractLeadSource, lead.Source)
	assert.Equal(t, domain.LeadStatusProspected, lead.Status,
		"nothing is drafted and nothing is sent")
	require.Len(t, leads.saved, 1)

	// The buyer is keyed on the CNPJ — the SAME key the tender flow uses, so a
	// hospital arriving twice is one customer with two reasons to be called.
	require.Len(t, contacts.resolved, 1)
	assert.Equal(t, "03507415002864", contacts.resolved[0].ExternalID)

	var e map[string]any
	require.NoError(t, json.Unmarshal(lead.Enrichment, &e))
	assert.Equal(t, "2027-01-14", e["contrato_vence_em"],
		"the date is the entire reason this card exists")
	assert.Equal(t, "Concorrente Ltda", e["fornecedor_atual"])
	assert.Equal(t, "420.000,00 BRL", e["contrato_valor"])
	assert.Equal(t, "sim", e["compra_por_adesao"],
		"this body can buy without running a tender at all")
	assert.Contains(t, e["contrato_url"], contract.PNCPID)
}

// Clicking the same row twice is navigation, not creation. A duplicate lead is
// a duplicate conversation with the same hospital.
func TestWorkContractWindow_IsIdempotent(t *testing.T) {
	contract := domain.PublicContract{
		PNCPID: "03507415002864-2-000266/2026", BuyerCNPJ: "03507415002864",
		BuyerName: "ESTADO DE MATO GROSSO", Object: "monitores",
	}
	uc, leads, _ := workWindowUC(t, contract)
	leads.existing = &domain.Lead{ID: "already-here", AccountID: "acc1"}

	lead, err := uc.Execute(context.Background(), "acc1", contract.PNCPID)
	require.NoError(t, err)
	assert.Equal(t, "already-here", lead.ID)
	assert.Empty(t, leads.saved, "no second card for the same buyer")
}

// The control number carries the buyer's CNPJ as its first segment, and that
// is how the contract is found. Garbage must read as "not in your calendar",
// never as an unfiltered query over a shared public table.
func TestWorkContractWindow_AMalformedIDIsNotFound(t *testing.T) {
	uc, _, _ := workWindowUC(t, domain.PublicContract{
		PNCPID: "03507415002864-2-000266/2026", BuyerCNPJ: "03507415002864",
	})

	for _, bad := range []string{"", "   ", "not-a-control-number", "123-1-1/2026"} {
		_, err := uc.Execute(context.Background(), "acc1", bad)
		assert.ErrorIs(t, err, domain.ErrContractWindowNotFound, "id %q", bad)
	}
}

// --- harness -----------------------------------------------------------

type workWindowLeads struct {
	saved    []*domain.Lead
	existing *domain.Lead
	domain.LeadRepository
}

func (r *workWindowLeads) GetByFunnelAndContact(context.Context, string, string) (*domain.Lead, error) {
	if r.existing == nil {
		return nil, nil
	}
	return r.existing, nil
}

func (r *workWindowLeads) Upsert(_ context.Context, l *domain.Lead) error {
	r.saved = append(r.saved, l)
	return nil
}

type workWindowContacts struct {
	resolved []*domain.Contact
	domain.ContactRepository
}

func (r *workWindowContacts) GetByExternalID(_ context.Context, _, externalID string) (*domain.Contact, error) {
	for _, c := range r.resolved {
		if c.ExternalID == externalID {
			return c, nil
		}
	}
	return nil, nil
}

// Upsert, not Create: the shared contact resolver writes through Upsert, which
// is what makes "the same buyer arriving twice is one customer" true.
func (r *workWindowContacts) Upsert(_ context.Context, c *domain.Contact) (*domain.Contact, error) {
	r.resolved = append(r.resolved, c)
	return c, nil
}

type workWindowFunnels struct{ domain.SalesFunnelRepository }

func (workWindowFunnels) ListByAccountID(context.Context, string) ([]*domain.SalesFunnel, error) {
	return []*domain.SalesFunnel{{ID: "fn-default", AccountID: "acc1", IsDefault: true}}, nil
}

func workWindowUC(t *testing.T, c domain.PublicContract) (
	*WorkContractWindowUseCase, *workWindowLeads, *workWindowContacts,
) {
	t.Helper()
	leads := &workWindowLeads{}
	contacts := &workWindowContacts{}
	repo := &calendarRepo{byBuyer: []domain.PublicContract{c}}
	return NewWorkContractWindowUseCase(repo, contacts, workWindowFunnels{}, leads),
		leads, contacts
}
