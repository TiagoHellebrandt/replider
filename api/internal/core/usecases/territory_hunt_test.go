package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

func TestTerritoryHunt_ResolvesAndFlagsKnownContacts(t *testing.T) {
	phone := "65 3618-8000"
	f := &radarFakes{
		types: []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}},
		holders: []domain.CNESEquipmentHolder{
			{CNES: "111", Total: 2, Establishment: &domain.CNESEstablishment{
				TradeName: "Hospital Santa Rosa", Phone: &phone}},
			{CNES: "222", Total: 1, Establishment: &domain.CNESEstablishment{
				TradeName: "Hospital São Mateus"}},
		},
	}
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{UF: "mt"}}, nil)
	contacts := new(MockContactRepo)
	// 111 was park-offered before; 222 is new ground.
	contacts.On("GetByExternalID", mock.Anything, "acc-1", "cnes:111").
		Return(&domain.Contact{ID: "ct-1"}, nil)
	contacts.On("GetByExternalID", mock.Anything, "acc-1", "cnes:222").
		Return(nil, nil)
	// The screen now asks the way the lead writer answers — the phone key
	// first, then the registry id. A door that only knew "cnes:<id>" called
	// hospitals cold that were already in the funnel.
	contacts.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).
		Return(nil, nil)

	uc := NewTerritoryHuntUseCase(f, accounts, contacts)
	out, err := uc.Execute(context.Background(), "acc-1", "tomógrafo", "")

	assert.NoError(t, err)
	assert.Equal(t, "MT", out.UF)
	assert.Equal(t, "2026-07", out.Competencia, "the month counted rides on the answer")
	assert.Empty(t, out.Cities, "no cities in the territory → whole UF")
	if assert.Len(t, out.Holders, 2) {
		assert.True(t, out.Holders[0].AlreadyKnown)
		assert.False(t, out.Holders[1].AlreadyKnown)
		assert.Equal(t, 2, out.Holders[0].Total)
		assert.Equal(t, "65 3618-8000", out.Holders[0].Phone)
	}
}

func TestTerritoryHunt_TerritoryCitiesNarrowTheHuntInsideTheirOwnUF(t *testing.T) {
	f := &radarFakes{
		types:     []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}},
		holders:   []domain.CNESEquipmentHolder{{CNES: "111", Total: 2}},
		cityCodes: []string{"510340"},
	}
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{
			UF: "MT", Cities: []string{"Cuiabá"}}}, nil)
	contacts := new(MockContactRepo)
	contacts.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil)

	uc := NewTerritoryHuntUseCase(f, accounts, contacts)

	out, err := uc.Execute(context.Background(), "acc-1", "tomógrafo", "")
	assert.NoError(t, err)
	assert.Equal(t, []string{"510340"}, f.huntedCities)
	assert.Equal(t, []string{"Cuiabá"}, out.Cities)

	// An explicit other-state UF is a whole-state ask: the home cities
	// must not leak into it.
	out, err = uc.Execute(context.Background(), "acc-1", "tomógrafo", "go")
	assert.NoError(t, err)
	assert.Equal(t, "GO", out.UF)
	assert.Empty(t, f.huntedCities)
	assert.Empty(t, out.Cities)
}

func TestTerritoryHunt_NoDictionaryMatchAnswersWithReasonNotEmptiness(t *testing.T) {
	f := &radarFakes{types: []domain.CNESEquipmentType{}}
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{UF: "MT"}}, nil)

	uc := NewTerritoryHuntUseCase(f, accounts, new(MockContactRepo))
	out, err := uc.Execute(context.Background(), "acc-1", "luva cirúrgica", "")

	assert.NoError(t, err)
	assert.Empty(t, out.Holders)
	assert.Contains(t, out.Note, "dicionário CNES")
}

func TestTerritoryHunt_NoTerritoryAndNoUFRefuses(t *testing.T) {
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)

	uc := NewTerritoryHuntUseCase(&radarFakes{}, accounts, new(MockContactRepo))
	_, err := uc.Execute(context.Background(), "acc-1", "tomógrafo", "")

	assert.Equal(t, "TERRITORY_MISSING", appErrorCode(err))
}

// A hospital the account imported from a spreadsheet is NOT cold ground. The
// flag used to key on "cnes:<id>" alone, so an establishment the base already
// had by phone was offered as a new target.
func TestTerritoryHunt_KnowsAHospitalTheBaseHasByPhone(t *testing.T) {
	ctx := context.Background()
	imported, byIdentity, unknown := "65 3618-8000", "65 3388-1000", "65 3000-0000"
	f := &radarFakes{
		types: []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}},
		holders: []domain.CNESEquipmentHolder{
			{CNES: "111", Total: 2, Establishment: &domain.CNESEstablishment{
				TradeName: "Hospital Santa Rosa", Phone: &imported}},
			{CNES: "222", Total: 1, Establishment: &domain.CNESEstablishment{
				TradeName: "Hospital São Mateus", Phone: &byIdentity}},
			{CNES: "333", Total: 1, Establishment: &domain.CNESEstablishment{
				TradeName: "Hospital Novo", Phone: &unknown}},
		},
	}
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{UF: "mt"}}, nil)

	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	// 111 came in through the CSV import: keyed by phone digits.
	_, _ = contacts.Upsert(ctx, &domain.Contact{ID: "ct-1", AccountID: "acc-1", ExternalID: "556536188000"})
	// 222 is keyed by something else entirely, but its phone is indexed.
	_, _ = contacts.Upsert(ctx, &domain.Contact{ID: "ct-2", AccountID: "acc-1", ExternalID: "compras@saomateus.br"})
	_, _ = identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: "acc-1", ContactID: "ct-2", Kind: domain.IdentityKindPhone, Value: "+556533881000"})

	uc := NewTerritoryHuntUseCase(f, accounts, contacts)
	uc.SetIdentityRepo(identities)
	out, err := uc.Execute(ctx, "acc-1", "tomógrafo", "")

	assert.NoError(t, err)
	if assert.Len(t, out.Holders, 3) {
		assert.True(t, out.Holders[0].AlreadyKnown, "imported by phone, offered as cold ground")
		assert.True(t, out.Holders[1].AlreadyKnown, "known by identity, offered as cold ground")
		assert.False(t, out.Holders[2].AlreadyKnown)
	}
}
