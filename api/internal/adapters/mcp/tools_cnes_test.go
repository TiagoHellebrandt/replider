package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type fakeTerritoryHunter struct {
	gotAccount, gotEquipment, gotUF string
	gotOpts                         usecases.TerritoryHuntOptions
	result                          *usecases.TerritoryHuntResult
	err                             error
}

func (f *fakeTerritoryHunter) ExecuteWith(_ context.Context, accountID, equipment, uf string, opts usecases.TerritoryHuntOptions) (*usecases.TerritoryHuntResult, error) {
	f.gotAccount, f.gotEquipment, f.gotUF, f.gotOpts = accountID, equipment, uf, opts
	return f.result, f.err
}

// The tool is the Radar's hunt behind a second door: same use case, the
// tool's own output shape (phone as pointer, total_hits, already_known).
func TestFindHospitalsWithEquipment_DelegatesToTheHuntUseCase(t *testing.T) {
	hunter := &fakeTerritoryHunter{result: &usecases.TerritoryHuntResult{
		UF: "MT", Cities: []string{"Cuiabá"}, Competencia: "2026-07",
		Matched: []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}},
		Holders: []usecases.TerritoryHuntHolder{
			{CNES: "111", TradeName: "Hospital Santa Rosa", Phone: "65 3618-8000", CityCode: "510340", Total: 2, AlreadyKnown: true},
			{CNES: "222", TradeName: "Hospital São Mateus", Total: 1},
		},
	}}

	out, err := handleFindHospitalsWithEquipment(identityCtx(), Deps{TerritoryHuntUC: hunter},
		huntInput{Equipment: "tomógrafo", UF: "mt", MinCount: 2, Limit: 10})

	require.NoError(t, err)
	assert.Equal(t, "acc-1", hunter.gotAccount)
	assert.Equal(t, "tomógrafo", hunter.gotEquipment)
	assert.Equal(t, "mt", hunter.gotUF)
	assert.Equal(t, usecases.TerritoryHuntOptions{MinCount: 2, Limit: 10}, hunter.gotOpts)

	assert.Equal(t, "MT", out.UF)
	assert.Equal(t, []string{"Cuiabá"}, out.Cities)
	assert.Equal(t, "2026-07", out.Competencia)
	assert.Equal(t, 2, out.TotalHits)
	require.Len(t, out.Holders, 2)
	require.NotNil(t, out.Holders[0].Phone)
	assert.Equal(t, "65 3618-8000", *out.Holders[0].Phone)
	assert.True(t, out.Holders[0].AlreadyKnown)
	assert.Nil(t, out.Holders[1].Phone, "empty phone stays absent, not \"\"")
	assert.False(t, out.Holders[1].AlreadyKnown)
	assert.Equal(t, "510340", out.Holders[0].CityCode)
}

func TestFindHospitalsWithEquipment_EmptyAnswerKeepsTheNoteAndEmptyLists(t *testing.T) {
	hunter := &fakeTerritoryHunter{result: &usecases.TerritoryHuntResult{
		UF: "MT", Note: "nenhum tipo de equipamento do dicionário CNES casa com esse nome"}}

	out, err := handleFindHospitalsWithEquipment(identityCtx(), Deps{TerritoryHuntUC: hunter},
		huntInput{Equipment: "xyz"})

	require.NoError(t, err)
	assert.NotNil(t, out.Matched, "matched_types serialises as [] not null")
	assert.NotNil(t, out.Holders)
	assert.Empty(t, out.Holders)
	assert.Equal(t, 0, out.TotalHits)
	assert.Contains(t, out.Note, "dicionário CNES")
}

func TestFindHospitalsWithEquipment_Errors(t *testing.T) {
	t.Run("no identity", func(t *testing.T) {
		_, err := handleFindHospitalsWithEquipment(context.Background(),
			Deps{TerritoryHuntUC: &fakeTerritoryHunter{}}, huntInput{Equipment: "x"})
		assert.ErrorIs(t, err, errNoIdentity)
	})
	t.Run("empty equipment never reaches the use case", func(t *testing.T) {
		hunter := &fakeTerritoryHunter{}
		_, err := handleFindHospitalsWithEquipment(identityCtx(), Deps{TerritoryHuntUC: hunter}, huntInput{})
		require.Error(t, err)
		assert.Empty(t, hunter.gotEquipment)
	})
	t.Run("missing territory keeps the tool's wording", func(t *testing.T) {
		hunter := &fakeTerritoryHunter{err: &domain.AppError{Code: "TERRITORY_MISSING", Message: "x"}}
		_, err := handleFindHospitalsWithEquipment(identityCtx(), Deps{TerritoryHuntUC: hunter}, huntInput{Equipment: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "informe uf")
	})
	t.Run("other errors pass through toolError", func(t *testing.T) {
		hunter := &fakeTerritoryHunter{err: errors.New("db down")}
		_, err := handleFindHospitalsWithEquipment(identityCtx(), Deps{TerritoryHuntUC: hunter}, huntInput{Equipment: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "a operação falhou")
	})
}

// A deployment without the use case must not advertise the tool; with it the
// tool is listed even when the park repo (get_installed_park) is absent.
func TestFindHospitalsWithEquipment_ListedOnlyWhenWired(t *testing.T) {
	assert.False(t, listToolNamesFor(t, Deps{}, "attendant")["find_hospitals_with_equipment"])
	assert.True(t, listToolNamesFor(t, Deps{TerritoryHuntUC: &fakeTerritoryHunter{}}, "attendant")["find_hospitals_with_equipment"])
}
