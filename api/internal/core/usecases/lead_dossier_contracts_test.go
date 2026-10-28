package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The park says HOW MANY machines a hospital declares and never whose they
// are: CNES publishes no brand. So the only machine-readable answer to "who is
// the incumbent here" is the procurement record — and a wrong answer, in a
// message to the hospital that signed with them, is not a bug but an
// accusation. These pin where the name may come from and how it is presented.

type fakeContracts struct {
	byBuyer  map[string][]domain.PublicContract
	lastScan time.Time
	saved    []domain.PublicContract
	marked   int
	listErr  error
}

func (f *fakeContracts) ListByBuyer(_ context.Context, cnpj string, _ int) ([]domain.PublicContract, error) {
	return f.byBuyer[cnpj], f.listErr
}
func (f *fakeContracts) Save(_ context.Context, items []domain.PublicContract) error {
	f.saved = append(f.saved, items...)
	return nil
}
func (f *fakeContracts) LastScan(context.Context, string) (time.Time, error) {
	return f.lastScan, nil
}
func (f *fakeContracts) MarkScanned(_ context.Context, _ string, found int) error {
	f.marked = found
	return nil
}

// The calendar's two reads. The dossier never calls them — it asks about ONE
// buyer — but the port is one port, so the fake satisfies all of it.
func (f *fakeContracts) ListExpiring(context.Context, domain.ContractCalendarFilter) ([]domain.PublicContract, error) {
	return nil, nil
}

func (f *fakeContracts) BuyersNeedingScan(context.Context, []string, []string, time.Time, int) ([]domain.ContractScanTarget, error) {
	return nil, nil
}

type fakeContractFetch struct {
	items []domain.PublicContract
	calls int
	err   error
}

func (f *fakeContractFetch) ContractsForBuyer(context.Context, string) ([]domain.PublicContract, error) {
	f.calls++
	return f.items, f.err
}

// fakeParkForContracts resolves the lead to an establishment with a CNPJ —
// which is where the buyer id comes from.
type fakeParkForContracts struct{ est *domain.CNESEstablishment }

func (f *fakeParkForContracts) GetEstablishment(string) (*domain.CNESEstablishment, error) {
	return nil, nil
}
func (f *fakeParkForContracts) FindEstablishmentByPhoneDigits(string) (*domain.CNESEstablishment, error) {
	return f.est, nil
}
func (f *fakeParkForContracts) SearchEstablishments(string, int) ([]domain.CNESEstablishment, error) {
	return nil, nil
}
func (f *fakeParkForContracts) ParkByCNES(string) ([]domain.CNESParkItem, string, error) {
	return nil, "", nil
}

func parkWithCNPJ() *fakeParkForContracts {
	cnpj := "11.111.111/1111-11"
	return &fakeParkForContracts{est: &domain.CNESEstablishment{
		CNES: "1234567", TradeName: "HOSPITAL X", CNPJ: &cnpj,
	}}
}

// leadWithPhone is what resolves to an establishment.
func leadWithPhone() (*domain.Lead, *domain.Contact) {
	phone := "+5565321216363"
	return &domain.Lead{ID: "l1", ContactID: "111"}, &domain.Contact{Phone: &phone}
}

func daysFromNow(d int) *time.Time {
	t := time.Now().AddDate(0, 0, d)
	return &t
}

func contractFixture() domain.PublicContract {
	return domain.PublicContract{
		PNCPID:       "00394460005887-2-007096/2024",
		BuyerCNPJ:    "11111111111111",
		SupplierName: "FORNECEDOR CONCORRENTE LTDA",
		Object:       "Aquisição de incubadoras neonatais",
		SignedAt:     daysFromNow(-120),
		ExpiresAt:    daysFromNow(90),
	}
}

func TestDossierContracts_NamesTheIncumbentAsCheckableFact(t *testing.T) {
	repo := &fakeContracts{byBuyer: map[string][]domain.PublicContract{
		"11111111111111": {contractFixture()},
	}, lastScan: time.Now()} // fresh: no fetch
	b := NewLeadDossierBuilder().SetCNESPark(parkWithCNPJ()).SetContracts(repo, &fakeContractFetch{})

	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)

	if len(d.Facts) != 1 {
		t.Fatalf("facts = %d", len(d.Facts))
	}
	f := d.Facts[0]
	// Registry, not research: it is public record, so the draft may state it.
	if f.Kind != domain.DossierRegistry {
		t.Fatalf("kind = %q", f.Kind)
	}
	if !strings.Contains(f.Value, "FORNECEDOR CONCORRENTE LTDA") {
		t.Fatalf("the supplier is the point: %q", f.Value)
	}
	// The expiry is the window a seller acts on.
	if !strings.Contains(f.Value, "vigência até") {
		t.Fatalf("no expiry in %q", f.Value)
	}
	// A claim about somebody's supplier is exactly the kind that gets checked.
	if !strings.Contains(f.Source, "pncp.gov.br") {
		t.Fatalf("source is not citable: %q", f.Source)
	}
}

// Nothing in the record means nothing in the dossier — and a proposal that
// argues its own case without comparing, which is the honest shape of "we do
// not know".
func TestDossierContracts_SilentWhenTheRecordHasNothing(t *testing.T) {
	repo := &fakeContracts{lastScan: time.Now()}
	b := NewLeadDossierBuilder().SetCNESPark(parkWithCNPJ()).SetContracts(repo, &fakeContractFetch{})

	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)
	if len(d.Facts) != 0 {
		t.Fatalf("invented a fact from an empty record: %+v", d.Facts)
	}
}

// A contract with no supplier name says nothing about who serves them.
func TestDossierContracts_SkipsUnnamedSuppliers(t *testing.T) {
	c := contractFixture()
	c.SupplierName = ""
	repo := &fakeContracts{byBuyer: map[string][]domain.PublicContract{
		"11111111111111": {c},
	}, lastScan: time.Now()}
	b := NewLeadDossierBuilder().SetCNESPark(parkWithCNPJ()).SetContracts(repo, &fakeContractFetch{})

	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)
	if len(d.Facts) != 0 {
		t.Fatalf("got %+v", d.Facts)
	}
}

// A contract from six years ago says little about who serves them now.
func TestDossierContracts_IgnoresStaleContracts(t *testing.T) {
	c := contractFixture()
	c.SignedAt = daysFromNow(-6 * 365)
	repo := &fakeContracts{byBuyer: map[string][]domain.PublicContract{
		"11111111111111": {c},
	}, lastScan: time.Now()}
	b := NewLeadDossierBuilder().SetCNESPark(parkWithCNPJ()).SetContracts(repo, &fakeContractFetch{})

	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)
	if len(d.Facts) != 0 {
		t.Fatalf("a six-year-old contract was presented as the incumbent: %+v", d.Facts)
	}
}

// PNCP being down costs the supplier line, never the draft.
func TestDossierContracts_FetchFailureIsSurvivable(t *testing.T) {
	repo := &fakeContracts{} // zero lastScan: cold, so it fetches
	fetch := &fakeContractFetch{err: errors.New("pncp down")}
	b := NewLeadDossierBuilder().SetCNESPark(parkWithCNPJ()).SetContracts(repo, fetch)

	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)

	if fetch.calls != 1 {
		t.Fatalf("calls = %d", fetch.calls)
	}
	if len(d.Facts) != 0 {
		t.Fatalf("got %+v", d.Facts)
	}
}

// A lead drafted twice in a week must not cost two round trips.
func TestDossierContracts_HonoursTheScanTTL(t *testing.T) {
	repo := &fakeContracts{lastScan: time.Now().Add(-1 * time.Hour)}
	fetch := &fakeContractFetch{}
	b := NewLeadDossierBuilder().SetCNESPark(parkWithCNPJ()).SetContracts(repo, fetch)

	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)
	if fetch.calls != 0 {
		t.Fatalf("re-fetched a buyer scanned an hour ago")
	}
}

// Unwired is a no-op, not a panic.
func TestDossierContracts_UnwiredIsANoOp(t *testing.T) {
	var d domain.LeadDossier
	NewLeadDossierBuilder().addContracts(context.Background(), &d, &domain.Lead{ID: "l1"}, nil)
	if len(d.Facts) != 0 {
		t.Fatal("unexpected facts")
	}
}

// Contracts wired without the park reader must be a no-op, not a panic:
// matchEstablishment reaches into it, and the first lead with a phone would
// take the process down.
func TestDossierContracts_ContractsWithoutParkIsSafe(t *testing.T) {
	b := NewLeadDossierBuilder().SetContracts(&fakeContracts{}, &fakeContractFetch{})
	lead, contact := leadWithPhone()
	var d domain.LeadDossier
	b.addContracts(context.Background(), &d, lead, contact)
	if len(d.Facts) != 0 {
		t.Fatal("unexpected facts")
	}
}
