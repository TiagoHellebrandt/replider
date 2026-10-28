package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The three defects this file pins were all found by reading a real
// production response for one account (MT, 103 products): every park card
// carried the SAME equipment list, no card said what to offer, and the page
// answered "5" for a territory holding 2794 matching establishments.

func radarParkHarness(t *testing.T, f *radarFakes, products []*domain.Product) *RadarUseCase {
	t.Helper()
	uc, accounts, prodRepo := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{UF: "MT"}}, nil)
	prodRepo.On("ListByAccountID", mock.Anything, "acc-1").Return(products, nil)
	return uc
}

func TestRadarPark_EachCardDescribesItsOwnHospital(t *testing.T) {
	f := &radarFakes{
		types: []domain.CNESEquipmentType{
			{Code: "0111", Name: "Incubadora"},
			{Code: "0564", Name: "Respirador/Ventilador"},
		},
		holders: []domain.CNESEquipmentHolder{
			{CNES: "111", Total: 40, Establishment: &domain.CNESEstablishment{CNES: "111", TradeName: "Hospital A"}},
			{CNES: "222", Total: 3, Establishment: &domain.CNESEstablishment{CNES: "222", TradeName: "Hospital B"}},
		},
		parkByCNES: map[string][]domain.CNESParkItem{
			"111": {{EquipCode: "0111", Name: "Incubadora", QtExist: 40}},
			"222": {{EquipCode: "0564", Name: "Respirador/Ventilador", QtExist: 3}},
		},
		comp:        "202607",
		countEstabs: 2794,
		countUnits:  9111,
	}
	uc := radarParkHarness(t, f, []*domain.Product{
		{ID: "p1", Name: "Incubadora neonatal", Active: true},
	})

	res, err := uc.Execute(context.Background(), "acc-1")
	require.NoError(t, err)

	var a, b RadarItem
	for _, it := range res.Items {
		switch it.CNES {
		case "111":
			a = it
		case "222":
			b = it
		}
	}
	require.NotEmpty(t, a.CNES)
	require.NotEmpty(t, b.CNES)

	assert.NotEqual(t, a.Context, b.Context,
		"two hospitals with different parks must not read identically")
	assert.Contains(t, a.Context, "40× Incubadora")
	assert.Contains(t, b.Context, "3× Respirador/Ventilador")
	assert.NotContains(t, b.Context, "Incubadora",
		"hospital B holds no incubator; naming one would be a fabricated fact")
}

func TestRadarPark_CardSaysWhatToOffer(t *testing.T) {
	f := &radarFakes{
		types:   []domain.CNESEquipmentType{{Code: "0111", Name: "Incubadora"}},
		holders: []domain.CNESEquipmentHolder{{CNES: "111", Total: 40}},
		parkByCNES: map[string][]domain.CNESParkItem{
			"111": {{EquipCode: "0111", Name: "Incubadora", QtExist: 40}},
		},
		comp: "202607",
	}
	uc := radarParkHarness(t, f, []*domain.Product{
		{ID: "p1", Name: "Incubadora neonatal Line Duo", Active: true},
		{ID: "p2", Name: "Contrato de manutenção anual", Active: true},
	})

	res, err := uc.Execute(context.Background(), "acc-1")
	require.NoError(t, err)
	require.Len(t, res.Items, 1)

	require.Len(t, res.Items[0].Matches, 1,
		"the incubator product fits; the maintenance contract fits nothing")
	assert.Equal(t, "p1", res.Items[0].Matches[0].ProductID)
	assert.Equal(t, "Incubadora", res.Items[0].Matches[0].Equipment)
	assert.Equal(t, 40, res.Items[0].Matches[0].QtExist)
}

func TestRadarPark_ReportsTheWholeUniverseNotJustThePage(t *testing.T) {
	holders := make([]domain.CNESEquipmentHolder, 0, radarParkLimit)
	park := map[string][]domain.CNESParkItem{}
	for i := 0; i < radarParkLimit; i++ {
		id := string(rune('a'+i)) + "00"
		holders = append(holders, domain.CNESEquipmentHolder{CNES: id, Total: 2})
		park[id] = []domain.CNESParkItem{{EquipCode: "0111", Name: "Incubadora", QtExist: 2}}
	}
	f := &radarFakes{
		types:       []domain.CNESEquipmentType{{Code: "0111", Name: "Incubadora"}},
		holders:     holders,
		parkByCNES:  park,
		comp:        "202607",
		countEstabs: 2794,
		countUnits:  9111,
	}
	uc := radarParkHarness(t, f, []*domain.Product{
		{ID: "p1", Name: "Incubadora neonatal", Active: true},
	})

	res, err := uc.Execute(context.Background(), "acc-1")
	require.NoError(t, err)

	require.NotNil(t, res.Park, "the page must say how big the opportunity is")
	assert.Equal(t, 2794, res.Park.Establishments)
	assert.Equal(t, 9111, res.Park.Units)
	assert.Equal(t, radarParkLimit, res.Park.Shown)
	assert.Greater(t, res.Park.Establishments, res.Park.Shown,
		"showing a page while claiming it is everything is the bug")
	assert.Equal(t, "2026-07", res.Park.Competencia)
	assert.Contains(t, res.Park.Equipment, "Incubadora")
}

func TestRadarPark_LosingTheBreakdownKeepsTheHospitals(t *testing.T) {
	// ParkForCNESIn answering nothing must degrade to the catalogue-wide
	// list, never drop the rows: a hospital with no line is still a lead.
	f := &radarFakes{
		types:   []domain.CNESEquipmentType{{Code: "0111", Name: "Incubadora"}},
		holders: []domain.CNESEquipmentHolder{{CNES: "111", Total: 40}},
		comp:    "202607",
	}
	uc := radarParkHarness(t, f, []*domain.Product{
		{ID: "p1", Name: "Incubadora neonatal", Active: true},
	})

	res, err := uc.Execute(context.Background(), "acc-1")
	require.NoError(t, err)

	require.Len(t, res.Items, 1)
	assert.Contains(t, res.Items[0].Context, "Incubadora")
	assert.Empty(t, res.Items[0].Matches,
		"with no park read there is nothing honest to pair a product with")
}

func TestDescribePark_BoundsTheLine(t *testing.T) {
	park := []domain.CNESParkItem{
		{Name: "Incubadora", QtExist: 9},
		{Name: "Ventilador", QtExist: 8},
		{Name: "Monitor", QtExist: 7},
		{Name: "Bomba", QtExist: 6},
		{Name: "Berço", QtExist: 5},
		{Name: "Ambu", QtExist: 4},
	}
	line := describePark(park, nil)
	assert.Equal(t, radarParkNamesInCard, strings.Count(line, "× "),
		"only the first few named, the rest counted")
	assert.Contains(t, line, "+2 tipos")
}
