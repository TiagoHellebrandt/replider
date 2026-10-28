package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

type parkReaderFake struct {
	est  *domain.CNESEstablishment
	park []domain.CNESParkItem
}

func (f *parkReaderFake) GetEstablishment(cnes string) (*domain.CNESEstablishment, error) {
	return f.est, nil
}
func (f *parkReaderFake) ResolveEquipmentCodes(nameQuery string) ([]domain.CNESEquipmentType, error) {
	return nil, nil
}
func (f *parkReaderFake) ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error) {
	return f.park, "202607", nil
}

type drafterFake struct {
	drafted string
	err     error
}

func (f *drafterFake) Draft(ctx context.Context, accountID, leadID string) (*domain.Lead, error) {
	f.drafted = leadID
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Lead{ID: leadID, Status: domain.LeadStatusPendingApproval}, nil
}

func TestRadarParkOffer_CreatesLeadWithHonestParkFacts(t *testing.T) {
	phone := "65 3388-1000"
	park := &parkReaderFake{
		est: &domain.CNESEstablishment{TradeName: "Hospital São Mateus",
			Address: "Av. X, Cuiabá", Phone: &phone},
		park: []domain.CNESParkItem{{EquipCode: "0111", Name: "Tomógrafo", QtExist: 2}},
	}
	contacts := new(MockContactRepo)
	leads := new(mockLeadRepo)
	funnels := new(MockSalesFunnelRepo)
	drafter := &drafterFake{}
	uc := NewRadarParkOfferUseCase(park, contacts, leads, funnels, drafter)

	funnels.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.SalesFunnel{{ID: "fn-1", AccountID: "acc-1", IsDefault: true}}, nil)
	// Nobody in the base under any spelling of the number, nor under the
	// old "cnes:" key.
	contacts.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil).Maybe()
	contacts.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		// A reachable establishment is keyed like every door keys a phone —
		// so the hospital somebody imported from a spreadsheet is this row.
		return c.ExternalID == "556533881000" && c.Name != nil && *c.Name == "Hospital São Mateus" &&
			c.CompanyName != nil && *c.CompanyName == "Hospital São Mateus" &&
			// The registry phone was normalized to E.164, Brazil-prefixed.
			c.Phone != nil && *c.Phone == "+556533881000"
	})).Return(&domain.Contact{ID: "ct-1", AccountID: "acc-1", ExternalID: "556533881000"}, nil)
	leads.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").Return(nil, nil)
	leads.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		return l.Source == "cnes_park" && l.FunnelID == "fn-1" && l.ContactID == "ct-1" &&
			// The public facts ride the enrichment: counts, never age.
			string(l.Enrichment) != "" &&
			assert.ObjectsAreEqual(true,
				containsAll(string(l.Enrichment), "cnes_park", "2x Tomógrafo"))
	})).Return(nil)

	lead, err := uc.Offer(context.Background(), "acc-1", "1234567")

	assert.NoError(t, err)
	assert.NotNil(t, lead)
	assert.NotEmpty(t, drafter.drafted)
}

func TestRadarParkOffer_UnknownCNESRefusesBeforeAnyWrite(t *testing.T) {
	uc := NewRadarParkOfferUseCase(&parkReaderFake{est: nil},
		new(MockContactRepo), new(mockLeadRepo), new(MockSalesFunnelRepo), &drafterFake{})

	_, err := uc.Offer(context.Background(), "acc-1", "0000000")

	if assert.Error(t, err) {
		assert.Equal(t, "CNES_NOT_FOUND", appErrorCode(err))
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Production, 2026-09-09: "Hospital Central de Alta Complexidade de Mato
// Grosso" sat on the home screen twice — once from Places, once from CNES,
// neither carrying a phone, so there was nothing but the name to key on. Two
// leads for one hospital means approaching it twice with two texts.
func TestRadarParkOffer_ReusesTheHospitalPlacesAlreadyFound(t *testing.T) {
	park := &parkReaderFake{
		// No phone: the case where the name is all there is.
		est: &domain.CNESEstablishment{
			TradeName: "HOSPITAL CENTRAL DE ALTA COMPLEXIDADE DE MATO GROSSO",
			Address:   "Cuiabá",
		},
	}
	contacts := new(MockContactRepo)
	leads := new(mockLeadRepo)
	funnels := new(MockSalesFunnelRepo)
	uc := NewRadarParkOfferUseCase(park, contacts, leads, funnels, &drafterFake{})

	funnels.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.SalesFunnel{{ID: "fn-1", AccountID: "acc-1", IsDefault: true}}, nil)

	// The Places hunt already filed this hospital, in title case.
	placesName := "Hospital Central de Alta Complexidade de Mato Grosso"
	contacts.On("ListByAccountID", mock.Anything, "acc-1", mock.Anything, mock.Anything, mock.Anything).
		Return([]*domain.Contact{{
			ID: "ct-places", AccountID: "acc-1", ExternalID: "ChIJP1TtYACxnZM", Name: &placesName,
		}}, nil)

	leads.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-places").Return(nil, nil)
	leads.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		return l.ContactID == "ct-places"
	})).Return(nil)
	// The twin is completed from the registry, not left half-blank: it is the
	// same hospital, and what the registry publishes is what the card lacks.
	contacts.On("Update", mock.Anything, mock.Anything).Return(nil)

	_, reused, err := uc.EnsureLead(context.Background(), "acc-1", "5558638")

	assert.NoError(t, err)
	assert.False(t, reused, "a first park lead on an existing contact is still new")
	contacts.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// The address the approval queue was waiting for.
//
// The registry publishes an e-mail for 3.208 of Mato Grosso's 5.266
// establishments and this door copied the name and the phone and left it
// behind. Production on 2026-09-17: 48 of the account's 60 hunted hospitals
// had a published address, not one contact carried one, and 50 drafts sat
// approved-ready with nowhere to send them.
func TestRadarParkOffer_CarriesTheRegistrysEmailOntoTheContact(t *testing.T) {
	email := "UHE@ses.mt.gov.br"
	phone := "(65)3618-8800"
	park := &parkReaderFake{est: &domain.CNESEstablishment{
		CNES: "9841903", TradeName: "HOSPITAL ESTADUAL SANTA CASA",
		Phone: &phone, Email: &email,
	}}
	contacts := new(MockContactRepo)
	leads := new(mockLeadRepo)
	funnels := new(MockSalesFunnelRepo)
	uc := NewRadarParkOfferUseCase(park, contacts, leads, funnels, &drafterFake{})

	funnels.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.SalesFunnel{{ID: "fn-1", AccountID: "acc-1", IsDefault: true}}, nil)
	contacts.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil)
	var saved *domain.Contact
	contacts.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		saved = c
		return true
	})).Return(&domain.Contact{ID: "ct-1", AccountID: "acc-1"}, nil)
	leads.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").Return(nil, nil)
	leads.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	_, _, err := uc.EnsureLead(context.Background(), "acc-1", "9841903")

	assert.NoError(t, err)
	assert.NotNil(t, saved.Email)
	assert.Equal(t, "uhe@ses.mt.gov.br", *saved.Email)
	// And it is NOT the key: thirty-three establishments publish
	// saude@rondonopolis.mt.gov.br, and keying on that would hand the seller
	// one card where there are thirty-three buyers.
	assert.NotEqual(t, "uhe@ses.mt.gov.br", saved.ExternalID)
}

// A hospital the account already met through a published tender is filed
// under the buyer's CNPJ. The registry knows the same organization by another
// name, and must not mint a second card for it.
func TestRadarParkOffer_ReusesTheBuyerATenderAlreadyFiled(t *testing.T) {
	cnpj := "37.678.845/0001-40"
	park := &parkReaderFake{est: &domain.CNESEstablishment{
		CNES: "5558638", TradeName: "INSTITUTO PATRIS", CNPJ: &cnpj,
	}}
	contacts := new(MockContactRepo)
	leads := new(mockLeadRepo)
	funnels := new(MockSalesFunnelRepo)
	uc := NewRadarParkOfferUseCase(park, contacts, leads, funnels, &drafterFake{})

	funnels.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.SalesFunnel{{ID: "fn-1", AccountID: "acc-1", IsDefault: true}}, nil)
	// No name twin, so the CNPJ is the only bridge left.
	contacts.On("ListByAccountID", mock.Anything, "acc-1", mock.Anything, mock.Anything, mock.Anything).
		Return([]*domain.Contact{}, nil)
	contacts.On("GetByExternalID", mock.Anything, "acc-1", "37678845000140").
		Return(&domain.Contact{ID: "ct-pncp", AccountID: "acc-1", ExternalID: "37678845000140"}, nil)
	contacts.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil)
	contacts.On("Update", mock.Anything, mock.Anything).Return(nil)
	leads.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-pncp").Return(nil, nil)
	leads.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	_, _, err := uc.EnsureLead(context.Background(), "acc-1", "5558638")

	assert.NoError(t, err)
	contacts.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// "Santa Casa" names half the hospitals in Brazil. Two matches is not a
// match: showing one hospital twice is cheap, merging two is not.
func TestRadarParkOffer_AnAmbiguousNameMintsItsOwnContact(t *testing.T) {
	park := &parkReaderFake{
		est: &domain.CNESEstablishment{TradeName: "SANTA CASA", Address: "Cuiabá"},
	}
	contacts := new(MockContactRepo)
	leads := new(mockLeadRepo)
	funnels := new(MockSalesFunnelRepo)
	uc := NewRadarParkOfferUseCase(park, contacts, leads, funnels, &drafterFake{})

	funnels.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.SalesFunnel{{ID: "fn-1", AccountID: "acc-1", IsDefault: true}}, nil)

	one, two := "Santa Casa", "SANTA CASA"
	contacts.On("ListByAccountID", mock.Anything, "acc-1", mock.Anything, mock.Anything, mock.Anything).
		Return([]*domain.Contact{
			{ID: "ct-a", AccountID: "acc-1", Name: &one},
			{ID: "ct-b", AccountID: "acc-1", Name: &two},
		}, nil)
	contacts.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil).Maybe()
	contacts.On("Upsert", mock.Anything, mock.Anything).
		Return(&domain.Contact{ID: "ct-new", AccountID: "acc-1", ExternalID: "cnes:9"}, nil)
	leads.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-new").Return(nil, nil)
	leads.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	_, _, err := uc.EnsureLead(context.Background(), "acc-1", "9")

	assert.NoError(t, err)
	contacts.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
}
