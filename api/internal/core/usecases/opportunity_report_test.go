package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeParkReader struct {
	types   map[string][]domain.CNESEquipmentType
	counts  map[string][2]int // joined codes → {estabs, units}
	holders []domain.CNESEquipmentHolder
	comp    string
}

func (f fakeParkReader) ResolveEquipmentCodes(q string) ([]domain.CNESEquipmentType, error) {
	return f.types[q], nil
}
func (f fakeParkReader) CountEquipment(_ string, codes []string) (int, int, error) {
	key := ""
	for _, c := range codes {
		key += c + ","
	}
	v := f.counts[key]
	return v[0], v[1], nil
}
func (f fakeParkReader) EstablishmentsWithEquipment(string, []string, int, int) ([]domain.CNESEquipmentHolder, error) {
	return f.holders, nil
}
func (f fakeParkReader) NewestCompetencia(string) (string, error) { return f.comp, nil }

type fakeTenderFetcher struct{ tenders []*domain.Signal }

func (f fakeTenderFetcher) Fetch(context.Context, time.Time, []string) ([]*domain.Signal, error) {
	return f.tenders, nil
}

func reportClock() time.Time { return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC) }

func namedHolder(cnes, name string, units int) domain.CNESEquipmentHolder {
	return domain.CNESEquipmentHolder{
		CNES: cnes, Total: units,
		Establishment: &domain.CNESEstablishment{CNES: cnes, TradeName: name, Address: "Rua X"},
	}
}

func TestOpportunityReport_AssemblesTheTeaser(t *testing.T) {
	park := fakeParkReader{
		types: map[string][]domain.CNESEquipmentType{
			"ventilador": {{Code: "0564", Name: "Respirador/Ventilador"}},
		},
		counts: map[string][2]int{
			"0564,": {40, 210}, // per-category AND union (same single code)
		},
		holders: []domain.CNESEquipmentHolder{
			namedHolder("1111111", "HOSPITAL A", 30),
			{CNES: "2222222", Total: 25}, // uncached: must be skipped, not shown as a bare id
			namedHolder("3333333", "HOSPITAL B", 20),
			namedHolder("4444444", "HOSPITAL C", 15),
			namedHolder("5555555", "HOSPITAL D", 10),
		},
		comp: "202607",
	}
	tenders := fakeTenderFetcher{tenders: []*domain.Signal{
		{State: "MT", Title: "Aquisição de ventiladores pulmonares", Organization: "SES-MT", ValueCents: 100_000_00},
		{State: "SP", Title: "Ventiladores para UTI", ValueCents: 999_999_99}, // wrong UF
		{State: "MT", Title: "Compra de café", ValueCents: 50_00},             // no keyword
	}}
	uc := NewOpportunityReportUseCase(park, tenders).WithClock(reportClock)

	r, err := uc.Build(context.Background(), OpportunityReportInput{
		UF: "mt", Categories: []string{"ventilador"},
	})
	require.NoError(t, err)

	assert.Equal(t, "MT", r.UF)
	assert.Equal(t, "202607", r.Competencia)
	assert.Equal(t, 40, r.TotalEstablishments)
	assert.Equal(t, 210, r.TotalUnits)

	// Sample: default 3, named-only, uncached id skipped.
	require.Len(t, r.Sample, 3)
	assert.Equal(t, "HOSPITAL A", r.Sample[0].Name)
	assert.Equal(t, "HOSPITAL B", r.Sample[1].Name)
	assert.Equal(t, 37, r.HiddenTargets, "40 establishments minus the 3 shown — the pitch line")

	// Tenders: UF + keyword filtered, summed.
	require.Len(t, r.Tenders, 1)
	assert.Equal(t, "SES-MT", r.Tenders[0].Organization)
	assert.Equal(t, int64(100_000_00), r.TendersSumCents)
	assert.Equal(t, 1, r.TendersTotal)
}

func TestOpportunityReport_EmptyTerritoryIsHonest(t *testing.T) {
	uc := NewOpportunityReportUseCase(fakeParkReader{}, nil).WithClock(reportClock)

	r, err := uc.Build(context.Background(), OpportunityReportInput{
		UF: "AC", Categories: []string{"tomógrafo"},
	})
	require.NoError(t, err)
	assert.Zero(t, r.TotalEstablishments)
	assert.Empty(t, r.Sample)
	assert.Zero(t, r.HiddenTargets, "never negative")
}

func TestOpportunityReport_ValidatesUF(t *testing.T) {
	uc := NewOpportunityReportUseCase(fakeParkReader{}, nil)
	_, err := uc.Build(context.Background(), OpportunityReportInput{UF: "Mato Grosso"})
	require.Error(t, err)
}
