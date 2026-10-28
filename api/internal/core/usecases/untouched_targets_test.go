package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The four command-centre tiles all counted work already in hand, so an
// account with one lead and a whole state in front of it looked exactly like
// one that had worked the state through. A real account: ninety products,
// 5.208 matching establishments in its own territory, and a home screen
// reading "1". These pin the number that separates them — and the two things
// it must never do: invent money, or outrank a customer who is talking.

type fakePark struct {
	types        []domain.CNESEquipmentType
	holders      []domain.CNESEquipmentHolder
	establish    int
	cities       []string
	askedUF      string
	askedCity    []string
	askedOffsets []int
	resolved     []string
	err          error
}

func (f *fakePark) ResolveEquipmentCodesAny(words []string) ([]domain.CNESEquipmentType, error) {
	f.resolved = words
	return f.types, f.err
}
func (f *fakePark) CityCodesByName(_ string, _ []string) ([]string, error) {
	return f.cities, nil
}
func (f *fakePark) CountEquipmentIn(uf string, cityCodes, _ []string) (int, int, error) {
	f.askedUF, f.askedCity = uf, cityCodes
	return f.establish, 0, f.err
}
func (f *fakePark) EstablishmentsWithEquipmentIn(
	_ string, _, _ []string, _, limit, offset int,
) ([]domain.CNESEquipmentHolder, string, error) {
	f.askedOffsets = append(f.askedOffsets, offset)
	if f.err != nil {
		return nil, "", f.err
	}
	if offset >= len(f.holders) {
		return nil, "202607", nil
	}
	end := offset + limit
	if end > len(f.holders) {
		end = len(f.holders)
	}
	return f.holders[offset:end], "202607", nil
}

// fakeProducts moved here with the hunt's rewrite: this file is now the only
// one that needs it.
type fakeProducts struct {
	domain.ProductRepository
	items []*domain.Product
	err   error
}

func (f *fakeProducts) ListByAccountID(context.Context, string) ([]*domain.Product, error) {
	return f.items, f.err
}

func product(name string) *domain.Product {
	return &domain.Product{Name: name, Attributes: json.RawMessage("{}")}
}

type fakeAccounts struct {
	domain.AccountRepository
	acc *domain.Account
	err error
}

func (f *fakeAccounts) GetByID(context.Context, string) (*domain.Account, error) {
	return f.acc, f.err
}

type fakeContacts struct {
	domain.ContactRepository
	known map[string]bool
}

func (f *fakeContacts) GetByExternalID(_ context.Context, _, external string) (*domain.Contact, error) {
	if f.known[external] {
		return &domain.Contact{ExternalID: external}, nil
	}
	return nil, nil
}

func held(cnes, name string, total int) domain.CNESEquipmentHolder {
	return domain.CNESEquipmentHolder{
		CNES: cnes, Total: total,
		Establishment: &domain.CNESEstablishment{TradeName: name},
	}
}

func untouchedFixture() (*fakeAccounts, *fakeProducts, *fakePark, *fakeContacts) {
	return &fakeAccounts{acc: &domain.Account{
			ID: "acc1", Territory: &domain.Territory{UF: "MT"},
		}},
		&fakeProducts{items: []*domain.Product{product("Incubadora Line Duo G")}},
		&fakePark{
			types:     []domain.CNESEquipmentType{{Code: "0101", Name: "Incubadora"}},
			establish: 132,
			holders: []domain.CNESEquipmentHolder{
				held("111", "FEMINA HOSPITAL INFANTIL", 63),
				held("222", "SANTA CASA DE RONDONOPOLIS", 45),
			},
		},
		&fakeContacts{known: map[string]bool{}}
}

func TestUntouchedTargets_CountsTheGroundAndNamesTheHospitals(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	uc := NewUntouchedTargetsUseCase(accounts, products, park, contacts)

	got, err := uc.Execute(context.Background(), "acc1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Total != 132 {
		t.Fatalf("total = %d, want 132", got.Total)
	}
	if len(got.Actions) != 2 {
		t.Fatalf("rows = %d", len(got.Actions))
	}
	// Biggest park first — the only ordering signal CNES gives, and a real one.
	if got.Actions[0].CustomerName != "FEMINA HOSPITAL INFANTIL" {
		t.Fatalf("first row = %q", got.Actions[0].CustomerName)
	}
	// The row is a phone call, not a statistic: it names the hospital and what
	// it holds.
	if !strings.Contains(got.Actions[0].Detail, "63") {
		t.Fatalf("detail says nothing about the park: %q", got.Actions[0].Detail)
	}
	// And it never invents money — CNES counts machines, not deals.
	if got.Actions[0].MoneyAtStakeCents != 0 || got.Actions[0].Currency != "" {
		t.Fatalf("row carries a fabricated amount: %+v", got.Actions[0])
	}
	// No age: this ground has not been waiting on anybody, and a SinceAt would
	// let it climb past a customer who is.
	if got.Actions[0].SinceAt != nil {
		t.Fatal("a cold target was given an age")
	}
	if got.Actions[0].Kind != domain.PendingUntouchedTarget {
		t.Fatalf("kind = %q", got.Actions[0].Kind)
	}
}

// A hospital already in the account's contacts is not untouched — it must
// drop out of the rows AND out of the count.
func TestUntouchedTargets_ExcludesHospitalsAlreadyApproached(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	contacts.known["cnes:111"] = true
	uc := NewUntouchedTargetsUseCase(accounts, products, park, contacts)

	got, _ := uc.Execute(context.Background(), "acc1")
	if len(got.Actions) != 1 || got.Actions[0].CustomerName != "SANTA CASA DE RONDONOPOLIS" {
		t.Fatalf("rows = %+v", got.Actions)
	}
	if got.Total != 131 {
		t.Fatalf("total = %d, want 131 — the approached one is not lost ground", got.Total)
	}
}

// The sweep has to ask what the lead writer answers.
//
// It asked "is there a contact keyed cnes:<id>?", and the writer had long
// since started keying reachable establishments by phone. Production on
// 2026-09-17: 51 of 60 hunted hospitals carried a phone key, so the home
// screen offered hospitals already sitting in the funnel and the nightly
// sweep re-decided the same ones every night — `found=52 created=0`.
func TestUntouchedTargets_RecognisesAHospitalKeyedByItsPhone(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	phone := "(65) 3618-8000"
	park.holders = []domain.CNESEquipmentHolder{{
		CNES: "111", Total: 63,
		Establishment: &domain.CNESEstablishment{
			TradeName: "FEMINA HOSPITAL INFANTIL", Phone: &phone,
		},
	}}
	// The contact the radar wrote: keyed by the phone, not by "cnes:111".
	contacts.known["556536188000"] = true
	uc := NewUntouchedTargetsUseCase(accounts, products, park, contacts)

	got, _ := uc.Execute(context.Background(), "acc1")
	if len(got.Actions) != 0 {
		t.Fatalf("offered a hospital that is already in the funnel: %+v", got.Actions)
	}
}

// A registry address reaches the organization, not one hospital: thirty-three
// establishments publish saude@rondonopolis.mt.gov.br. Recognising ground by
// it would hide thirty-two buyers behind the one already contacted.
func TestUntouchedTargets_ASharedRegistryEmailIsNotProofOfContact(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	shared := "saude@rondonopolis.mt.gov.br"
	park.holders = []domain.CNESEquipmentHolder{{
		CNES: "111", Total: 63,
		Establishment: &domain.CNESEstablishment{
			TradeName: "UPA JARDIM LEBLON", Email: &shared,
		},
	}}
	contacts.known[shared] = true
	uc := NewUntouchedTargetsUseCase(accounts, products, park, contacts)

	got, _ := uc.Execute(context.Background(), "acc1")
	if len(got.Actions) != 1 {
		t.Fatalf("a shared address swallowed real ground: %+v", got.Actions)
	}
}

// No territory is a question the account has not answered, not a zero and not
// an error. Same for a catalogue whose names are not equipment names.
func TestUntouchedTargets_QuietWhenThereIsNothingToAsk(t *testing.T) {
	t.Run("no territory", func(t *testing.T) {
		accounts, products, park, contacts := untouchedFixture()
		accounts.acc.Territory = nil
		got, err := NewUntouchedTargetsUseCase(accounts, products, park, contacts).
			Execute(context.Background(), "acc1")
		if err != nil || got.Total != 0 || len(got.Actions) != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("catalogue is not equipment", func(t *testing.T) {
		accounts, products, park, contacts := untouchedFixture()
		park.types = nil
		got, err := NewUntouchedTargetsUseCase(accounts, products, park, contacts).
			Execute(context.Background(), "acc1")
		if err != nil || got.Total != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("no products", func(t *testing.T) {
		accounts, _, park, contacts := untouchedFixture()
		got, err := NewUntouchedTargetsUseCase(accounts, &fakeProducts{}, park, contacts).
			Execute(context.Background(), "acc1")
		if err != nil || got.Total != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
}

// An establishment CNES never named renders as its registry id, never as a
// guess — the same rule every other CNES surface follows.
func TestUntouchedTargets_UnnamedEstablishmentKeepsItsID(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	park.holders = []domain.CNESEquipmentHolder{{CNES: "999", Total: 4}}
	uc := NewUntouchedTargetsUseCase(accounts, products, park, contacts)

	got, _ := uc.Execute(context.Background(), "acc1")
	if len(got.Actions) != 1 || got.Actions[0].CustomerName != "Estabelecimento CNES 999" {
		t.Fatalf("rows = %+v", got.Actions)
	}
}

// The territory's cities narrow the question; the park must be asked with them.
func TestUntouchedTargets_NarrowsToTheTerritorysCities(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	accounts.acc.Territory.Cities = []string{"Cuiabá"}
	park.cities = []string{"510340"}
	uc := NewUntouchedTargetsUseCase(accounts, products, park, contacts)

	if _, err := uc.Execute(context.Background(), "acc1"); err != nil {
		t.Fatal(err)
	}
	if park.askedUF != "MT" || len(park.askedCity) != 1 || park.askedCity[0] != "510340" {
		t.Fatalf("asked uf=%q cities=%v", park.askedUF, park.askedCity)
	}
}

// A park failure is an error the command centre turns into a named warning —
// never a silent zero, which would read as "you have worked your whole state".
func TestUntouchedTargets_ParkFailureIsAnError(t *testing.T) {
	accounts, products, park, contacts := untouchedFixture()
	park.err = errors.New("cnes down")
	if _, err := NewUntouchedTargetsUseCase(accounts, products, park, contacts).
		Execute(context.Background(), "acc1"); err == nil {
		t.Fatal("a failed park read reported zero lost ground")
	}
}

// The cold row must never outrank a customer who is already talking.
func TestUntouchedTarget_RanksBelowEveryWarmAction(t *testing.T) {
	actions := []domain.PendingAction{
		{Kind: domain.PendingUntouchedTarget, TargetID: "cnes:111"},
		{Kind: domain.PendingWaitingReply, TargetID: "conv1"},
		{Kind: domain.PendingApproveDraft, TargetID: "lead1"},
	}
	domain.RankPendingActions(actions, time.Now())
	if actions[len(actions)-1].Kind != domain.PendingUntouchedTarget {
		t.Fatalf("order = %v", []domain.PendingActionKind{
			actions[0].Kind, actions[1].Kind, actions[2].Kind})
	}
}

// The bug that made the sweep report "found 0" twice against a territory with
// 5.208 matching establishments.
//
// The words came from deriveWatchKeywords, which keeps the fifteen LONGEST.
// On the real catalogue this was built for — 133 distinct words —
// "incubadora" ranks 33rd by length, "ventilador" 41st and "monitor" 86th,
// behind "antitransbordamento" and "multiparamétricos", which the CNES
// registry has never heard of. Every word that resolves was cut.
func TestUntouchedTargets_AsksAboutEveryWordNotTheLongestFew(t *testing.T) {
	// Thirty long words the registry does not know, and the one that matters
	// buried behind them — the shape of a real catalogue.
	items := []*domain.Product{}
	for _, long := range []string{
		"antitransbordamento", "multiparamétricos", "bilirrubinômetro",
		"oxigenoterapia", "autoclaváveis", "intermediária", "policarbonato",
		"reutilizáveis", "estacionária", "transcutâneo", "aquecimento",
		"reguladora", "auxiliares", "descartável", "silenciamento",
		"monitoração", "integrada", "hospitalares", "neonatal", "cuidados",
		"intensivos", "unidade", "híbrida", "portátil", "cirúrgico",
		"pediátrico", "adulto", "infantil", "digital", "eletrônico",
	} {
		items = append(items, product(long))
	}
	items = append(items, product("Incubadora Line Duo G"))

	accounts, _, park, contacts := untouchedFixture()
	uc := NewUntouchedTargetsUseCase(accounts, &fakeProducts{items: items}, park, contacts)

	if _, err := uc.ListPage(context.Background(), "acc1", 0, 10); err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	var asked bool
	for _, w := range park.resolved {
		if w == "incubadora" {
			asked = true
		}
	}
	if !asked {
		t.Fatalf("the only word the registry knows was never asked; asked %d words: %v",
			len(park.resolved), park.resolved)
	}
}
