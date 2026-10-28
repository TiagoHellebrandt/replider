package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

type leadParkReaderFake struct {
	byPhone *domain.CNESEstablishment
	byName  []domain.CNESEstablishment
	park    []domain.CNESParkItem
}

// GetEstablishment: the key-first path. Nil keeps these tests on the
// phone/name guesses they were written for.
func (f *leadParkReaderFake) GetEstablishment(string) (*domain.CNESEstablishment, error) {
	return nil, nil
}

func (f *leadParkReaderFake) FindEstablishmentByPhoneDigits(digits string) (*domain.CNESEstablishment, error) {
	return f.byPhone, nil
}
func (f *leadParkReaderFake) SearchEstablishments(query string, limit int) ([]domain.CNESEstablishment, error) {
	return f.byName, nil
}
func (f *leadParkReaderFake) ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error) {
	return f.park, "202607", nil
}

func TestLeadPark_PhoneMatchWinsAndCarriesTheMethod(t *testing.T) {
	leads := new(mockLeadRepo)
	contacts := new(MockContactRepo)
	park := &leadParkReaderFake{
		byPhone: &domain.CNESEstablishment{CNES: "2655411", TradeName: "Hospital São Mateus"},
		park:    []domain.CNESParkItem{{EquipCode: "0111", Name: "Tomógrafo", QtExist: 2}},
	}
	uc := NewLeadParkUseCase(leads, contacts, park)

	phone := "+556533881000"
	leads.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1", ContactID: "ct-1"}, nil)
	contacts.On("GetByID", mock.Anything, "ct-1", "acc-1").
		Return(&domain.Contact{ID: "ct-1", Phone: &phone}, nil)

	out, err := uc.Execute(context.Background(), "acc-1", "lead-1")

	assert.NoError(t, err)
	if assert.NotNil(t, out) {
		assert.Equal(t, "2655411", out.CNES)
		assert.Equal(t, "phone", out.MatchedBy)
		assert.Equal(t, "2026-07", out.Competencia, "the card must say which month the counts are from")
		assert.Len(t, out.Park, 1)
	}
}

func TestLeadPark_AmbiguousNameAnswersNothing(t *testing.T) {
	leads := new(mockLeadRepo)
	contacts := new(MockContactRepo)
	// TWO "Santa Casa"s: a guessed hospital's equipment on screen is an
	// invented fact, so the honest answer is no card at all.
	park := &leadParkReaderFake{byName: []domain.CNESEstablishment{
		{CNES: "1"}, {CNES: "2"},
	}}
	uc := NewLeadParkUseCase(leads, contacts, park)

	name := "Santa Casa de Misericórdia"
	leads.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1", ContactID: "ct-1"}, nil)
	contacts.On("GetByID", mock.Anything, "ct-1", "acc-1").
		Return(&domain.Contact{ID: "ct-1", Name: &name}, nil)

	out, err := uc.Execute(context.Background(), "acc-1", "lead-1")
	assert.NoError(t, err)
	assert.Nil(t, out)
}
