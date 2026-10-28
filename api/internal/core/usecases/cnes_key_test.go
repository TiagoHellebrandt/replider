package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// A lead the park hunt created carries the establishment id in its contact
// key: "cnes:5558638". Both matchers ignored it and guessed — phone digits,
// then a name search that only counts on exactly one hit — and for a hunted
// lead neither lands: the registry phone often fails E.164 normalisation
// (which is WHY the contact stayed keyed by cnes), and hospital names repeat
// across a state.
//
// Three things went quiet at once because of it: the park on the lead panel,
// the park in the DOSSIER (so the draft did not know what the hospital owns),
// and the incumbent supplier, which needs the establishment to reach its CNPJ.

func TestCNESIDFromKey(t *testing.T) {
	cases := map[string]string{
		"cnes:5558638":  "5558638",
		"CNES:5558638":  "5558638",
		" cnes:5558638": "5558638",
		// Not a key: a phone-keyed contact is the normal case and must not be
		// read as an establishment.
		"+5565321216363": "",
		"":               "",
		// Malformed: guessing from these is how a lead shows another
		// hospital's park.
		"cnes:":         "",
		"cnes:12345":    "",
		"cnes:abcdefg":  "",
		"cnes:55586380": "",
	}
	for in, want := range cases {
		if got := cnesIDFromKey(in); got != want {
			t.Errorf("cnesIDFromKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// keyPark answers ONLY GetEstablishment. The guesses must never be reached
// when the key resolves — and a fake that fails them proves it.
type keyPark struct {
	est          *domain.CNESEstablishment
	askedID      string
	guessedPhone bool
	guessedName  bool
}

func (p *keyPark) GetEstablishment(cnes string) (*domain.CNESEstablishment, error) {
	p.askedID = cnes
	return p.est, nil
}
func (p *keyPark) FindEstablishmentByPhoneDigits(string) (*domain.CNESEstablishment, error) {
	p.guessedPhone = true
	return nil, nil
}
func (p *keyPark) SearchEstablishments(string, int) ([]domain.CNESEstablishment, error) {
	p.guessedName = true
	return nil, nil
}
func (p *keyPark) ParkByCNES(string) ([]domain.CNESParkItem, string, error) {
	return []domain.CNESParkItem{{QtExist: 4}}, "202607", nil
}

func huntedLead() (*domain.Lead, *domain.Contact) {
	// Production's shape, which the first version of this fixture got wrong:
	// lead.ContactID is the contact's ROW id (a ULID), and the "cnes:<id>"
	// key lives on the contact's external_id. Getting that backwards is what
	// let a swapped-argument lookup look like it worked.
	name := "HOSPITAL CENTRAL DE ALTA COMPLEXIDADE DE MATO GROSSO"
	return &domain.Lead{ID: "l1", AccountID: "acc1", ContactID: "01CONTACTROWID"},
		&domain.Contact{ID: "01CONTACTROWID", ExternalID: "cnes:5558638", Name: &name}
}

func TestDossierMatch_UsesTheKeyBeforeGuessing(t *testing.T) {
	park := &keyPark{est: &domain.CNESEstablishment{CNES: "5558638", TradeName: "HOSPITAL CENTRAL"}}
	b := NewLeadDossierBuilder().SetCNESPark(park)

	lead, contact := huntedLead()
	got := b.matchEstablishment(lead, contact)

	if got == nil || got.CNES != "5558638" {
		t.Fatalf("the id in the key did not resolve: %+v", got)
	}
	if park.askedID != "5558638" {
		t.Fatalf("asked %q", park.askedID)
	}
	if park.guessedPhone || park.guessedName {
		t.Fatal("guessed by phone or name while the key held the answer")
	}
}

// A phone-keyed contact keeps working exactly as before.
func TestDossierMatch_StillGuessesWhenThereIsNoKey(t *testing.T) {
	park := &keyPark{}
	b := NewLeadDossierBuilder().SetCNESPark(park)

	phone := "+5565321216363"
	b.matchEstablishment(
		&domain.Lead{ID: "l1", ContactID: "5565321216363"},
		&domain.Contact{ExternalID: "5565321216363", Phone: &phone},
	)
	if !park.guessedPhone {
		t.Fatal("the phone path stopped being tried")
	}
}

type keyLeads struct {
	domain.LeadRepository
	lead *domain.Lead
}

func (r *keyLeads) GetByID(context.Context, string) (*domain.Lead, error) { return r.lead, nil }

// keyContacts answers ONLY when the arguments are in the documented order —
// GetByID(ctx, id, accountID), GetByExternalID(ctx, accountID, externalID).
//
// The previous version returned the contact whatever it was asked, and that
// is precisely how it hid a real bug: the use case passed GetByID's arguments
// swapped, so in production the contact was never loaded and this endpoint
// answered 204 for every lead. A fake that accepts anything proves nothing.
type keyContacts struct {
	domain.ContactRepository
	contact   *domain.Contact
	accountID string
}

func (r *keyContacts) GetByID(_ context.Context, id, accountID string) (*domain.Contact, error) {
	if r.contact == nil || id != r.contact.ID || accountID != r.accountID {
		return nil, nil
	}
	return r.contact, nil
}

func (r *keyContacts) GetByExternalID(_ context.Context, accountID, externalID string) (*domain.Contact, error) {
	if r.contact == nil || accountID != r.accountID || externalID != r.contact.ExternalID {
		return nil, nil
	}
	return r.contact, nil
}

// The panel's endpoint: same bug, same fix. It answered 204 for every lead the
// hunt created, so the park section never rendered.
func TestLeadPark_ResolvesAHuntedLead(t *testing.T) {
	lead, contact := huntedLead()
	park := &keyPark{est: &domain.CNESEstablishment{CNES: "5558638", TradeName: "HOSPITAL CENTRAL"}}
	uc := NewLeadParkUseCase(&keyLeads{lead: lead}, &keyContacts{contact: contact, accountID: "acc1"}, park)

	res, err := uc.Execute(context.Background(), "acc1", "l1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res == nil {
		t.Fatal("204 for a lead that carries its own CNES id")
	}
	if res.CNES != "5558638" || res.MatchedBy != "cnes" {
		t.Fatalf("got %+v", res)
	}
}

// "Onde mostra o que posso oferecer pra cada cliente?" — the answer was
// "nowhere, until you spend a draft": the product↔hospital pairing only
// happened inside the paid matcher call. But the crossing is a word match
// between the catalogue and what the registry lists, so it costs nothing.

type catalogueParkFake struct{ keyPark }

func (c *catalogueParkFake) ParkByCNES(string) ([]domain.CNESParkItem, string, error) {
	return []domain.CNESParkItem{
		{EquipCode: "0101", Name: "Incubadora", QtExist: 4},
		{EquipCode: "0102", Name: "Equipamento De Fototerapia", QtExist: 2},
		{EquipCode: "0999", Name: "Código 0999", QtExist: 9}, // unnamed by the dictionary
		{EquipCode: "0103", Name: "Tomografo Computadorizado", QtExist: 1},
		{EquipCode: "0104", Name: "Berço Aquecido", QtExist: 0}, // declared, none installed
	}, "202607", nil
}

type catalogueProducts struct {
	domain.ProductRepository
	items []*domain.Product
}

func (c *catalogueProducts) ListByAccountID(context.Context, string) ([]*domain.Product, error) {
	return c.items, nil
}

func TestLeadPark_CrossesTheCatalogueWithoutAModelCall(t *testing.T) {
	lead, contact := huntedLead()
	park := &catalogueParkFake{keyPark{
		est: &domain.CNESEstablishment{CNES: "5558638", TradeName: "HOSPITAL CENTRAL"},
	}}
	uc := NewLeadParkUseCase(&keyLeads{lead: lead}, &keyContacts{contact: contact, accountID: "acc1"}, park)
	uc.SetProducts(&catalogueProducts{items: []*domain.Product{
		{ID: "p1", Name: "Incubadora Line Duo G", Active: true},
		{ID: "p2", Name: "Fototerapia LED-PHOTO", Active: true},
		{ID: "p3", Name: "Berço Aquecido Matrix", Active: true},
		{ID: "p4", Name: "Contrato de manutenção anual", Active: true},
		{ID: "p5", Name: "Incubadora de Transporte", Active: false}, // inactive
	}})

	res, err := uc.Execute(context.Background(), "acc1", "l1")
	if err != nil || res == nil {
		t.Fatalf("Execute: %v %v", res, err)
	}

	got := map[string]int{}
	for _, m := range res.Matches {
		got[m.ProductName] = m.QtExist
	}
	if got["Incubadora Line Duo G"] != 4 {
		t.Fatalf("the incubator did not pair with the four they own: %+v", res.Matches)
	}
	if got["Fototerapia LED-PHOTO"] != 2 {
		t.Fatalf("phototherapy missed: %+v", res.Matches)
	}
	// Declared but none installed: nothing to replace, nothing to offer.
	if _, ok := got["Berço Aquecido Matrix"]; ok {
		t.Fatal("paired against an equipment they have zero of")
	}
	// A product whose name is not an equipment name pairs with nothing — a
	// list that is mostly wrong is worse than an empty one.
	if _, ok := got["Contrato de manutenção anual"]; ok {
		t.Fatal("a service contract was offered as equipment")
	}
	// An inactive product is not for sale.
	if _, ok := got["Incubadora de Transporte"]; ok {
		t.Fatal("an inactive product reached the panel")
	}
	// Biggest installed count first.
	if len(res.Matches) < 2 || res.Matches[0].QtExist < res.Matches[1].QtExist {
		t.Fatalf("not ordered by park size: %+v", res.Matches)
	}
}

// A code the dictionary never named cannot be word-matched, and pairing a
// product with "Código 0999" would be the panel inventing a fit.
func TestLeadPark_NeverPairsWithAnUnnamedCode(t *testing.T) {
	lead, contact := huntedLead()
	park := &catalogueParkFake{keyPark{
		est: &domain.CNESEstablishment{CNES: "5558638"},
	}}
	uc := NewLeadParkUseCase(&keyLeads{lead: lead}, &keyContacts{contact: contact, accountID: "acc1"}, park)
	uc.SetProducts(&catalogueProducts{items: []*domain.Product{
		{ID: "p9", Name: "Código Equipamento Genérico", Active: true},
	}})

	res, _ := uc.Execute(context.Background(), "acc1", "l1")
	for _, m := range res.Matches {
		if strings.HasPrefix(m.Equipment, "Código") {
			t.Fatalf("paired with an unnamed code: %+v", m)
		}
	}
}

// Without the catalogue wired the park still renders — the crossing is an
// addition, not a precondition.
func TestLeadPark_WithoutProductsStillShowsThePark(t *testing.T) {
	lead, contact := huntedLead()
	park := &catalogueParkFake{keyPark{est: &domain.CNESEstablishment{CNES: "5558638"}}}
	uc := NewLeadParkUseCase(&keyLeads{lead: lead}, &keyContacts{contact: contact, accountID: "acc1"}, park)

	res, err := uc.Execute(context.Background(), "acc1", "l1")
	if err != nil || res == nil || len(res.Park) == 0 {
		t.Fatalf("park lost: %v %v", res, err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("matched without a catalogue: %+v", res.Matches)
	}
}
