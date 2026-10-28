package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

type fakeBuyerContacts struct {
	byCNPJ map[string]*domain.Contact
	err    error
	calls  int
}

func (f *fakeBuyerContacts) GetByExternalID(_ context.Context, _, externalID string) (*domain.Contact, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byCNPJ[externalID], nil
}

type fakeBuyerLeads struct {
	status  map[string]domain.LeadStatus
	drafted map[string]bool
	leadID  map[string]string
	stage   map[string]string
	err     error
	calls   int
}

func (f *fakeBuyerLeads) LeadStateForContact(_ context.Context, _, contactID string) (domain.RadarLeadState, error) {
	f.calls++
	if f.err != nil {
		return domain.RadarLeadState{}, f.err
	}
	st := f.status[contactID]
	if st == "" {
		return domain.RadarLeadState{}, nil
	}
	return domain.RadarLeadState{
		LeadID:  f.leadID[contactID],
		Status:  st,
		Drafted: f.drafted[contactID],
		Stage:   f.stage[contactID],
	}, nil
}

type fakeBuyerSales struct {
	sold  map[string]bool
	err   error
	calls int
}

func (f *fakeBuyerSales) HasSale(_ context.Context, _, contactID string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.sold[contactID], nil
}

type fakeBuyerHistory struct {
	byCNPJ map[string][]domain.PublicContract
	err    error
	calls  int
}

func (f *fakeBuyerHistory) ListByBuyer(_ context.Context, cnpj string, _ int) ([]domain.PublicContract, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byCNPJ[cnpj], nil
}

func buyerName(s string) *string { return &s }

// The two facts that change the first line of a message, neither of which the
// panel could say before.
func TestAnnotateBuyers_SaysWhoTheyAreAndWhoSellsToThem(t *testing.T) {
	signed := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"03370251000156": {ID: "c-1", Name: buyerName("Prefeitura de Itiquira")},
	}}
	leads := &fakeBuyerLeads{}
	sales := &fakeBuyerSales{sold: map[string]bool{"c-1": true}}
	history := &fakeBuyerHistory{byCNPJ: map[string][]domain.PublicContract{
		"03370251000156": {
			{SupplierName: "Medicalpar", Object: "aquisição de ventiladores pulmonares", SignedAt: &signed},
			{SupplierName: "Outra", Object: "aquisição de autoclave hospitalar"},
		},
	}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, sales, history)

	items := []RadarItem{{ID: "a", CNPJ: "03370251000156"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	b := items[0].Buyer
	assert.Equal(t, BuyerSold, b.Relationship)
	assert.Equal(t, "c-1", b.ContactID)
	assert.Equal(t, "Prefeitura de Itiquira", b.Name)
	assert.Equal(t, 2, b.Contracts)
	// Newest first: the supplier worth naming is the one holding it now.
	assert.Equal(t, "Medicalpar", b.LastSupplier)
	assert.Equal(t, &signed, b.LastContractAt)
}

// BOTH BUGS THIS SHIPPED WITH, in production, one after the other.
//
// v1 read this exact contact row as a SALE ("você já vendeu"). v2 read it as
// an APPROACH ("você já abordou"). Measured on the account on 16/09/2026:
// zero channels connected, zero outbound messages ever sent, no e-mail stored
// for the buyer and no draft written. Instituto Patris had been noticed by the
// radar and nothing else.
//
// A row this product creates by itself is never evidence that something was
// done to somebody.
func TestAnnotateBuyers_NoticingABuyerIsNotContactingThem(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"37678845000140": {ID: "c-patris", Name: buyerName("INSTITUTO PATRIS")},
	}}
	// The real shape: a lead exists (the radar made it), status `prospected`,
	// nothing written.
	leads := &fakeBuyerLeads{status: map[string]domain.LeadStatus{"c-patris": domain.LeadStatusProspected}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

	items := []RadarItem{{ID: "a", CNPJ: "37678845000140"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerListed, items[0].Buyer.Relationship,
		"a card in the funnel is not a conversation")
	assert.Equal(t, "c-patris", items[0].Buyer.ContactID)
}

// A written offer still waiting on a human has NOT been sent. On this account
// fifty of them sat in that queue and not one had left.
func TestAnnotateBuyers_ADraftInTheQueueIsNotASend(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"111": {ID: "c-1"},
	}}
	leads := &fakeBuyerLeads{
		status:  map[string]domain.LeadStatus{"c-1": domain.LeadStatusPendingApproval},
		drafted: map[string]bool{"c-1": true},
	}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

	items := []RadarItem{{ID: "a", CNPJ: "111"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerQueued, items[0].Buyer.Relationship)
}

// The one status ladder that DOES prove an approach left the building.
func TestAnnotateBuyers_ADeliveredApproachIsTheOnlyProofOfContact(t *testing.T) {
	for _, st := range []domain.LeadStatus{
		domain.LeadStatusContacted, domain.LeadStatusResponded,
		domain.LeadStatusQualified, domain.LeadStatusWon,
	} {
		contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{"111": {ID: "c-1"}}}
		leads := &fakeBuyerLeads{status: map[string]domain.LeadStatus{"c-1": st}}
		uc := &RadarUseCase{}
		uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

		items := []RadarItem{{ID: "a", CNPJ: "111"}}
		uc.annotateBuyers(context.Background(), "acc1", items)

		require.NotNil(t, items[0].Buyer)
		assert.Equal(t, BuyerContacted, items[0].Buyer.Relationship, "status %s", st)
	}
}

// An opt-out marks a lead lost without anything having been sent, so `lost`
// must never be read as proof of an approach.
func TestAnnotateBuyers_ALostLeadIsNotProofOfAnApproach(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{"111": {ID: "c-1"}}}
	leads := &fakeBuyerLeads{status: map[string]domain.LeadStatus{"c-1": domain.LeadStatusLost}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

	items := []RadarItem{{ID: "a", CNPJ: "111"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerListed, items[0].Buyer.Relationship)
}

// A failed lead read must not promote anybody. Silence is the safe answer;
// every wrong grade here is a sentence the seller says out loud.
func TestAnnotateBuyers_AFailedLeadReadNeverPromotes(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{"111": {ID: "c-1"}}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, &fakeBuyerLeads{err: errors.New("db down")},
		&fakeBuyerSales{}, &fakeBuyerHistory{})

	items := []RadarItem{{ID: "a", CNPJ: "111"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerCold, items[0].Buyer.Relationship)
}

// THE SECOND BUG, same day, same screen: the incumbent was `rows[0]`.
//
// Itiquira's two most recent published contracts are marquee rentals, so the
// panel named a funfair company as the competitor to beat. The supplier that
// matters is further down the list, and the only thing separating them is the
// trade gate the feed already uses.
func TestAnnotateBuyers_NamesTheIncumbentInOurTradeNotTheMostRecent(t *testing.T) {
	old := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	history := &fakeBuyerHistory{byCNPJ: map[string][]domain.PublicContract{
		"03370251000156": {
			{
				SupplierName: "PARQUE DE DIVERSOES XAVIER LTDA",
				BuyerName:    "MUNICIPIO DE ITIQUIRA",
				Object:       "CONTRATAÇÃO DE EMPRESA ESPECIALIZADA PARA A LOCAÇÃO DE TENDAS",
			},
			{
				SupplierName: "M.S DIAGNOSTICA LTDA",
				BuyerName:    "MUNICIPIO DE ITIQUIRA",
				Object:       "AQUISIÇÃO DE REAGENTES E INSUMOS PARA O LABORATÓRIO MUNICIPAL",
				SignedAt:     &old,
			},
		},
	}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	items := []RadarItem{{ID: "a", CNPJ: "03370251000156"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	b := items[0].Buyer
	assert.Equal(t, "M.S DIAGNOSTICA LTDA", b.LastSupplier)
	// And the count is the count of what competes with us, not of everything
	// this body has ever signed.
	assert.Equal(t, 1, b.Contracts)
}

// A buyer with contracts, none of them ours, must not be reported as having
// none scanned — the panel says "nothing in your trade on record", which is a
// different sentence from silence.
func TestAnnotateBuyers_NoInTradeContractStillReportsThatWeLooked(t *testing.T) {
	history := &fakeBuyerHistory{byCNPJ: map[string][]domain.PublicContract{
		"111": {{SupplierName: "Tendas", Object: "locação de tendas"}},
	}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	items := []RadarItem{{ID: "a", CNPJ: "111"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.True(t, items[0].Buyer.Scanned)
	assert.Zero(t, items[0].Buyer.Contracts)
	assert.Empty(t, items[0].Buyer.LastSupplier)
}

// A stranger is still worth a block — "they have never heard of you" is a
// fact the seller acts on.
func TestAnnotateBuyers_AColdBuyerWithHistoryStillGetsABlock(t *testing.T) {
	history := &fakeBuyerHistory{byCNPJ: map[string][]domain.PublicContract{
		"111": {{SupplierName: "Concorrente", BuyerName: "HOSPITAL X", Object: "aquisição de monitores multiparamétricos"}},
	}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	items := []RadarItem{{ID: "a", CNPJ: "111"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerCold, items[0].Buyer.Relationship)
	assert.Equal(t, "Concorrente", items[0].Buyer.LastSupplier)
}

// Nothing learned is not a block. An empty section reads as a failure; an
// absent one reads as nothing to report.
func TestAnnotateBuyers_NothingKnownSendsNothing(t *testing.T) {
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, nil)

	items := []RadarItem{{ID: "a", CNPJ: "999"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	assert.Nil(t, items[0].Buyer)
}

// A park row names no CNPJ, and a feed is mostly park rows. Looking anyway
// would be N pointless queries per load.
func TestAnnotateBuyers_RowsWithoutACNPJAreNotLookedUp(t *testing.T) {
	contacts := &fakeBuyerContacts{}
	history := &fakeBuyerHistory{}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	items := []RadarItem{{ID: "cnes-1"}, {ID: "cnes-2"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	assert.Zero(t, contacts.calls)
	assert.Zero(t, history.calls)
}

// The same buyer on three rows is one lookup, not three.
func TestAnnotateBuyers_LooksEachBuyerUpOnce(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"111": {ID: "c-1"},
	}}
	leads := &fakeBuyerLeads{}
	sales := &fakeBuyerSales{}
	history := &fakeBuyerHistory{}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, sales, history)

	items := []RadarItem{
		{ID: "a", CNPJ: "111"}, {ID: "b", CNPJ: "111"}, {ID: "c", CNPJ: "111"},
	}
	uc.annotateBuyers(context.Background(), "acc1", items)

	assert.Equal(t, 1, contacts.calls)
	assert.Equal(t, 1, sales.calls)
	for i := range items {
		require.NotNil(t, items[i].Buyer, "row %d lost its block", i)
	}
}

// A failed read costs a section, never the feed.
func TestAnnotateBuyers_AFailedReadIsNotAFailedLoad(t *testing.T) {
	uc := &RadarUseCase{}
	uc.SetBuyerContext(
		&fakeBuyerContacts{err: errors.New("db down")},
		&fakeBuyerLeads{},
		&fakeBuyerSales{},
		&fakeBuyerHistory{err: errors.New("db down")},
	)

	items := []RadarItem{{ID: "a", CNPJ: "111"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	assert.Nil(t, items[0].Buyer)
}

// A park row is about a HOSPITAL, and the hospital may already be a card.
//
// The block was keyed on the CNPJ, which only a tender publishes — so every
// park row arrived as a stranger. On the screenshot from 17/09/2026 the panel
// for HOSPITAL MUNICIPAL DE CUIABÁ listed 872 devices and said nothing about
// the fact that this account already holds its card, which is the one thing
// that decides how the seller opens.
func TestAnnotateBuyers_AParkRowFindsTheCardTheAccountAlreadyHolds(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"cnes:9209352": {ID: "c-hmc", Name: buyerName("Hospital Municipal de Cuiabá")},
	}}
	leads := &fakeBuyerLeads{
		status: map[string]domain.LeadStatus{"c-hmc": domain.LeadStatusProspected},
		leadID: map[string]string{"c-hmc": "lead-hmc"},
		stage:  map[string]string{"c-hmc": "Clientes"},
	}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

	// No CNPJ — exactly what a park row looked like.
	items := []RadarItem{{ID: "cnes-9209352", contactKeys: []string{"cnes:9209352"}}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerListed, items[0].Buyer.Relationship)
	assert.Equal(t, "c-hmc", items[0].Buyer.ContactID)
}

// "Já está no seu quadro" without a way to open it leaves the seller hunting
// a name across seventy-three cards, and "in the funnel" is not a position:
// a card in Ganhos and one nobody touched are both in the funnel.
func TestAnnotateBuyers_CarriesTheCardAndTheColumnItSitsIn(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"37678845000140": {ID: "c-1", Name: buyerName("INSTITUTO PATRIS")},
	}}
	leads := &fakeBuyerLeads{
		status:  map[string]domain.LeadStatus{"c-1": domain.LeadStatusPendingApproval},
		drafted: map[string]bool{"c-1": true},
		leadID:  map[string]string{"c-1": "lead-9"},
		stage:   map[string]string{"c-1": "Aguardando aprovação"},
	}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

	items := []RadarItem{{ID: "a", CNPJ: "37678845000140"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerQueued, items[0].Buyer.Relationship)
	assert.Equal(t, "lead-9", items[0].Buyer.LeadID)
	assert.Equal(t, "Aguardando aprovação", items[0].Buyer.Stage)
}

// A sale does not take the card off the board — the seller still wants to open
// it. Grading by sale used to return before the lead was ever read.
func TestAnnotateBuyers_ASoldBuyerStillCarriesItsCard(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"03370251000156": {ID: "c-1"},
	}}
	leads := &fakeBuyerLeads{
		status: map[string]domain.LeadStatus{"c-1": domain.LeadStatusWon},
		leadID: map[string]string{"c-1": "lead-won"},
		stage:  map[string]string{"c-1": "Ganhos"},
	}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads,
		&fakeBuyerSales{sold: map[string]bool{"c-1": true}}, &fakeBuyerHistory{})

	items := []RadarItem{{ID: "a", CNPJ: "03370251000156"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, BuyerSold, items[0].Buyer.Relationship)
	assert.Equal(t, "lead-won", items[0].Buyer.LeadID)
	assert.Equal(t, "Ganhos", items[0].Buyer.Stage)
}

// A row with no block reads as "not on your board", which is a claim. The two
// halves of this annotation cost very different things, and sharing one budget
// made the cheap half as scarce as the expensive one: a park page of 25
// hospitals under a cap of 12 told the seller thirteen lies by omission.
func TestAnnotateBuyers_EveryRowOnThePageGetsAnAnswer(t *testing.T) {
	byKey := map[string]*domain.Contact{}
	status := map[string]domain.LeadStatus{}
	items := make([]RadarItem, 0, radarParkLimit)
	for i := 0; i < radarParkLimit; i++ {
		key := "cnes:" + string(rune('a'+i))
		byKey[key] = &domain.Contact{ID: "c-" + key}
		status["c-"+key] = domain.LeadStatusProspected
		items = append(items, RadarItem{ID: key, contactKeys: []string{key}})
	}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{byCNPJ: byKey},
		&fakeBuyerLeads{status: status}, &fakeBuyerSales{}, &fakeBuyerHistory{})

	uc.annotateBuyers(context.Background(), "acc1", items)

	for i := range items {
		require.NotNil(t, items[i].Buyer, "row %d left without an answer", i)
		assert.Equal(t, BuyerListed, items[i].Buyer.Relationship)
	}
}

// The expensive half keeps its own, tighter budget — and a buyer past it is
// marked unscanned rather than reported as having no contracts in our trade.
func TestAnnotateBuyers_HistoryPastTheBudgetIsUnscannedNotEmpty(t *testing.T) {
	byCNPJ := map[string][]domain.PublicContract{}
	items := make([]RadarItem, 0, radarBuyerLookups+3)
	for i := 0; i < radarBuyerLookups+3; i++ {
		cnpj := "cnpj-" + string(rune('a'+i))
		byCNPJ[cnpj] = []domain.PublicContract{
			{SupplierName: "Alguém", Object: "aquisição de ventiladores pulmonares",
				BuyerName: "HOSPITAL MUNICIPAL"},
		}
		items = append(items, RadarItem{ID: cnpj, CNPJ: cnpj})
	}
	history := &fakeBuyerHistory{byCNPJ: byCNPJ}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	uc.annotateBuyers(context.Background(), "acc1", items)

	assert.Equal(t, radarBuyerLookups, history.calls,
		"the contract scan must stay inside its own budget")
	// Past the budget there is nothing to report and nothing is claimed: the
	// block is absent rather than an empty history the seller would read as
	// "nobody sells to them".
	assert.Nil(t, items[len(items)-1].Buyer)
	require.NotNil(t, items[0].Buyer)
	assert.True(t, items[0].Buyer.Scanned)
}

// A public hospital rarely signs anything — its secretaria does.
//
// Measured over the 20 biggest parks in Mato Grosso on 2026-09-18: 10
// establishments publish their own CNPJ, 11 publish only the maintaining
// entity's, and none publish neither. The entity's number was arriving in
// the DATASUS payload the whole time and being dropped, so for more than half
// the page the radar had no buyer key and the history section was blank.
func TestAnnotateBuyers_ReadsTheRecordOfWhoBuysForTheHospital(t *testing.T) {
	history := &fakeBuyerHistory{byCNPJ: map[string][]domain.PublicContract{
		"57252971000146": {{
			SupplierName: "Medicalpar", Object: "aquisição de ventiladores pulmonares",
			BuyerName: "SECRETARIA ESTADUAL DE SAUDE DE MATO GROSSO",
		}},
	}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	items := []RadarItem{{
		ID: "cnes-1", contactKeys: []string{"cnes:5558638"},
		BuyerEntityCNPJ: "57252971000146",
		BuyerEntityName: "SECRETARIA ESTADUAL DE SAUDE DE MATO GROSSO",
	}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, 1, items[0].Buyer.Contracts)
	assert.Equal(t, "Medicalpar", items[0].Buyer.LastSupplier)
	// And it says WHOSE record that was. "Este comprador já contratou" would
	// be false about the hospital, and would read identically on every unit
	// the secretaria maintains.
	assert.Equal(t, "SECRETARIA ESTADUAL DE SAUDE DE MATO GROSSO",
		items[0].Buyer.HistoryVia)
}

// The entity is the PURCHASER, never the hospital's identity.
//
// Seven of those twenty parks share 57252971000146. Keying rows on it would
// hand all seven the first one's card, its stage and its relationship — one
// buyer where there are seven.
func TestAnnotateBuyers_HospitalsSharingASecretariaStayApart(t *testing.T) {
	contacts := &fakeBuyerContacts{byCNPJ: map[string]*domain.Contact{
		"cnes:111": {ID: "c-um"},
		"cnes:222": {ID: "c-dois"},
	}}
	leads := &fakeBuyerLeads{
		status: map[string]domain.LeadStatus{
			"c-um": domain.LeadStatusProspected, "c-dois": domain.LeadStatusPendingApproval,
		},
		drafted: map[string]bool{"c-dois": true},
		leadID:  map[string]string{"c-um": "lead-um", "c-dois": "lead-dois"},
	}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(contacts, leads, &fakeBuyerSales{}, &fakeBuyerHistory{})

	items := []RadarItem{
		{ID: "a", contactKeys: []string{"cnes:111"}, BuyerEntityCNPJ: "57252971000146"},
		{ID: "b", contactKeys: []string{"cnes:222"}, BuyerEntityCNPJ: "57252971000146"},
	}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	require.NotNil(t, items[1].Buyer)
	assert.Equal(t, "lead-um", items[0].Buyer.LeadID)
	assert.Equal(t, "lead-dois", items[1].Buyer.LeadID,
		"two hospitals of one secretaria must not share a card")
	assert.Equal(t, BuyerQueued, items[1].Buyer.Relationship)
}

// A hospital that answers for itself says so plainly: no "via" caveat to
// explain away a record that IS its own.
func TestAnnotateBuyers_ItsOwnRecordCarriesNoVia(t *testing.T) {
	history := &fakeBuyerHistory{byCNPJ: map[string][]domain.PublicContract{
		"03099157000104": {{
			SupplierName: "Fornecedor", Object: "aquisição de monitores multiparamétricos",
			BuyerName: "SANTA CASA DE RONDONOPOLIS",
		}},
	}}
	uc := &RadarUseCase{}
	uc.SetBuyerContext(&fakeBuyerContacts{}, &fakeBuyerLeads{}, &fakeBuyerSales{}, history)

	items := []RadarItem{{ID: "a", CNPJ: "03099157000104"}}
	uc.annotateBuyers(context.Background(), "acc1", items)

	require.NotNil(t, items[0].Buyer)
	assert.Equal(t, 1, items[0].Buyer.Contracts)
	assert.Empty(t, items[0].Buyer.HistoryVia)
}
