package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

type radarFakes struct {
	beds         map[string][]domain.CNESBedItem
	habs         map[string][]domain.CNESHabilitationItem
	servs        map[string][]domain.CNESServiceItem
	signals      []*domain.Signal
	signalsErr   error
	holders      []domain.CNESEquipmentHolder
	types        []domain.CNESEquipmentType
	customers    []*domain.PortfolioCustomer
	dates        []*domain.RelationshipDate
	huntedCodes  []string
	huntedCities []string
	resolvedName string
	// resolvedWords is what the ONE catalogue-wide dictionary query asked.
	resolvedWords []string
	resolveCalls  int
	// cityCodes answers CityCodesByName (nil = nothing resolves).
	cityCodes []string
	comp      string
	pairs     []domain.CoPurchasePair
	bought    map[string][]string
	// parkByCNES answers ParkForCNESIn; countEstabs/countUnits answer
	// CountEquipmentIn (the universe behind the page).
	parkByCNES  map[string][]domain.CNESParkItem
	parkCodes   []string
	countEstabs int
	countUnits  int
}

func (f *radarFakes) ParkForCNESIn(cnesIDs, equipCodes []string) (map[string][]domain.CNESParkItem, string, error) {
	f.parkCodes = equipCodes
	if f.parkByCNES == nil {
		return map[string][]domain.CNESParkItem{}, f.comp, nil
	}
	out := map[string][]domain.CNESParkItem{}
	for _, id := range cnesIDs {
		if p, ok := f.parkByCNES[id]; ok {
			out[id] = p
		}
	}
	return out, f.comp, nil
}

func (f *radarFakes) CountEquipmentIn(uf string, cityCodes, equipCodes []string) (int, int, error) {
	return f.countEstabs, f.countUnits, nil
}

func (f *radarFakes) ListForRadar(ctx context.Context, accountID string, limit, offset int) ([]*domain.Signal, error) {
	return f.signals, f.signalsErr
}
func (f *radarFakes) ResolveEquipmentCodes(name string) ([]domain.CNESEquipmentType, error) {
	f.resolvedName = name
	return f.types, nil
}
func (f *radarFakes) ResolveEquipmentCodesAny(words []string) ([]domain.CNESEquipmentType, error) {
	f.resolvedWords = words
	f.resolveCalls++
	return f.types, nil
}
func (f *radarFakes) CityCodesByName(uf string, names []string) ([]string, error) {
	return f.cityCodes, nil
}
func (f *radarFakes) ServicesForCNESIn(ids []string) (map[string][]domain.CNESServiceItem, string, error) {
	if f.servs == nil {
		return map[string][]domain.CNESServiceItem{}, "", nil
	}
	return f.servs, "202608", nil
}

func (f *radarFakes) HabilitationsForCNESIn(ids []string) (map[string][]domain.CNESHabilitationItem, string, error) {
	if f.habs == nil {
		return map[string][]domain.CNESHabilitationItem{}, "", nil
	}
	return f.habs, "202608", nil
}

func (f *radarFakes) BedsForCNESIn(ids []string) (map[string][]domain.CNESBedItem, string, error) {
	if f.beds == nil {
		return map[string][]domain.CNESBedItem{}, "", nil
	}
	return f.beds, "202608", nil
}

func (f *radarFakes) EstablishmentsWithEquipmentIn(uf string, cityCodes, codes []string, minCount, limit, offset int) ([]domain.CNESEquipmentHolder, string, error) {
	f.huntedCodes = codes
	f.huntedCities = cityCodes
	comp := f.comp
	if comp == "" {
		comp = "202607"
	}
	return f.holders, comp, nil
}

type radarPortfolioFake struct{ customers []*domain.PortfolioCustomer }

func (f *radarPortfolioFake) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.PortfolioCustomer, error) {
	return f.customers, nil
}

type radarDatesFake struct{ dates []*domain.RelationshipDate }

func (f *radarDatesFake) List(ctx context.Context, accountID string) ([]*domain.RelationshipDate, error) {
	return f.dates, nil
}

type radarCrossSellFake struct {
	pairs  []domain.CoPurchasePair
	bought map[string][]string
}

func (f *radarCrossSellFake) ItemsBoughtBy(ctx context.Context, accountID, contactID string) ([]string, error) {
	return f.bought[contactID], nil
}
func (f *radarCrossSellFake) Pairs(ctx context.Context, accountID string) ([]domain.CoPurchasePair, error) {
	return f.pairs, nil
}

func radarHarness(f *radarFakes) (*RadarUseCase, *MockAccountRepo, *MockProductRepo) {
	accounts := new(MockAccountRepo)
	products := new(MockProductRepo)
	uc := NewRadarUseCase(f, f,
		&radarPortfolioFake{customers: f.customers},
		&radarDatesFake{dates: f.dates},
		&radarCrossSellFake{pairs: f.pairs, bought: f.bought},
		products, accounts)
	uc.WithClock(func() time.Time {
		return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	})
	return uc, accounts, products
}

func TestRadar_MergesSourcesAndOrdersByDeadline(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	in5d := now.Add(5 * 24 * time.Hour)
	in2d := now.Add(2 * 24 * time.Hour)
	phone := "65 3618-8000"
	f := &radarFakes{
		signals: []*domain.Signal{{
			ID: "sig-1", Organization: "Pref. SJC", Title: "Luva meio dedo",
			ValueCents: 336000, ClosesAt: &in5d, LeadID: "lead-9",
			City: "São José dos Campos", State: "SP",
		}},
		types:   []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}},
		holders: []domain.CNESEquipmentHolder{{CNES: "123", Total: 2, Establishment: &domain.CNESEstablishment{TradeName: "Hospital São Mateus", Phone: &phone}}},
		customers: []*domain.PortfolioCustomer{
			{ContactID: "ct-1", ContactName: "Hospital Santa Rosa", NextDueAt: &in2d},
			// Beyond the 7-day window: not an opportunity yet.
			{ContactID: "ct-2", ContactName: "Longe", NextDueAt: radarPtrTime(now.Add(30 * 24 * time.Hour))},
		},
	}
	uc, accounts, products := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{UF: "mt"}}, nil)
	products.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{{ID: "p1", Name: "Tomógrafo GE"}}, nil)

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Empty(t, out.Warnings)
	if assert.Len(t, out.Items, 3) {
		// Deadline order: reorder in 2d, tender in 5d, park (no deadline) last.
		assert.Equal(t, RadarRecompra, out.Items[0].Type)
		assert.Equal(t, RadarLicitacao, out.Items[1].Type)
		assert.Equal(t, "lead-9", out.Items[1].LeadID)
		assert.Equal(t, RadarParque, out.Items[2].Type)
		assert.Equal(t, "123", out.Items[2].CNES)
		// Counts, never age — the honesty rule rides in the copy itself.
		assert.Contains(t, out.Items[2].Context, "quantidade — não idade")
		// No cities in the territory: the scope is the state.
		assert.Contains(t, out.Items[2].Context, "em MT")
		// The month the counts come from rides on the item, ISO-shaped.
		assert.Equal(t, "2026-07", out.Items[2].Competencia)
	}
	assert.Equal(t, []string{"0111"}, f.huntedCodes)
	assert.Empty(t, f.huntedCities, "no cities → whole UF")
	// Resolution is per meaningful WORD ("GE" is noise), lower-cased —
	// whole-name ILIKE would never match the dictionary's shorter entry —
	// and ONE query for the whole catalogue.
	assert.Equal(t, []string{"tomógrafo"}, f.resolvedWords)
	assert.Equal(t, 1, f.resolveCalls)
}

func TestRadar_ParkResolvesTheWholeCatalogueInOneQueryAndScopesToCities(t *testing.T) {
	f := &radarFakes{
		types:     []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}, {Code: "0564", Name: "Ventilador"}},
		holders:   []domain.CNESEquipmentHolder{{CNES: "123", Total: 3}},
		cityCodes: []string{"510340", "510840"},
		comp:      "202606",
	}
	uc, accounts, products := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{
			UF: "mt", Cities: []string{"Cuiabá", "Várzea Grande"}}}, nil)
	products.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{
			{ID: "p1", Name: "Tomógrafo computadorizado GE"},
			{ID: "p2", Name: "Ventilador pulmonar"},
			{ID: "p3", Name: "Tomógrafo Siemens"}, // duplicate word, asked once
		}, nil)

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Equal(t, 1, f.resolveCalls, "the catalogue is resolved once per request, not per product per word")
	assert.ElementsMatch(t, []string{"tomógrafo", "computadorizado", "ventilador", "pulmonar", "siemens"}, f.resolvedWords)
	assert.Equal(t, []string{"510340", "510840"}, f.huntedCities, "the territory's cities narrow the hunt")
	var park *RadarItem
	for i := range out.Items {
		if out.Items[i].Type == RadarParque {
			park = &out.Items[i]
		}
	}
	if assert.NotNil(t, park) {
		assert.Contains(t, park.Context, "no seu território (Cuiabá, Várzea Grande)")
		assert.NotContains(t, park.Context, "em MT")
		assert.Equal(t, "2026-06", park.Competencia)
		// Unnamed establishment: the honest placeholder, never a guess.
		assert.Equal(t, "Estabelecimento CNES 123", park.Title)
	}
}

func TestRadar_ParkUnresolvableCitiesFallBackToTheUF(t *testing.T) {
	f := &radarFakes{
		types:   []domain.CNESEquipmentType{{Code: "0111", Name: "Tomógrafo"}},
		holders: []domain.CNESEquipmentHolder{{CNES: "123", Total: 1}},
	}
	uc, accounts, products := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Territory: &domain.Territory{
			UF: "MT", Cities: []string{"Xyzville"}}}, nil)
	products.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{{ID: "p1", Name: "Tomógrafo"}}, nil)

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Empty(t, f.huntedCities, "a city nobody can resolve must not filter the state down to nothing")
	if assert.Len(t, out.Items, 1) {
		assert.Contains(t, out.Items[0].Context, "em MT")
	}
}

func TestRadar_FailedCollectorBecomesWarningNotEmptyFeed(t *testing.T) {
	f := &radarFakes{signalsErr: errors.New("pncp down")}
	uc, accounts, products := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil) // no territory → park honest-empty
	products.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{}, nil).Maybe()

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Equal(t, []string{"licitacao"}, out.Warnings)
	assert.Empty(t, out.Items)
}

func TestRadar_DismissedAndExpiredSignalsAreDropped(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	dismissed := now.Add(-time.Hour)
	f := &radarFakes{
		signals: []*domain.Signal{
			{ID: "dead", Organization: "X", ClosesAt: &past},
			{ID: "waved", Organization: "Y", DismissedAt: &dismissed},
		},
	}
	uc, accounts, products := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)
	products.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{}, nil).Maybe()

	out, err := uc.Execute(context.Background(), "acc-1")
	assert.NoError(t, err)
	assert.Empty(t, out.Items)
}

func radarPtrTime(t time.Time) *time.Time { return &t }

func TestRadar_CrossSellAnchorsOnWhatTheCustomerAlreadyBuys(t *testing.T) {
	f := &radarFakes{
		customers: []*domain.PortfolioCustomer{
			{ContactID: "ct-1", ContactName: "Hospital Santa Rosa"},
			// No purchases on record: nothing to anchor on, no item.
			{ContactID: "ct-2", ContactName: "Sem Compra"},
		},
		pairs: []domain.CoPurchasePair{{
			Product: "luva cirurgica", Partner: "Avental descartável",
			Customers: 4, ProductCustomers: 5,
		}},
		bought: map[string][]string{"ct-1": {"Luva cirúrgica"}},
	}
	uc, accounts, products := radarHarness(f)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)
	products.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{}, nil).Maybe()

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	var xsell []RadarItem
	for _, it := range out.Items {
		if it.Type == RadarCrossSell {
			xsell = append(xsell, it)
		}
	}
	if assert.Len(t, xsell, 1) {
		assert.Equal(t, "Hospital Santa Rosa", xsell[0].Title)
		assert.Equal(t, "ct-1", xsell[0].ContactID)
		// The reason travels with the suggestion — a recommendation with no
		// "because" is a guess the customer can hear.
		assert.Contains(t, xsell[0].Context, "Avental descartável")
	}
}
