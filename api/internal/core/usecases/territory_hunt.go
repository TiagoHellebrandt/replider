package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// "Abastecer o território" (Ofertas phase 2b): the interactive hunt behind
// the Radar — the seller types the equipment in their own words, the CNES
// dictionary resolves it, and the answer is who in the territory declares
// that equipment. This is the REST face of the MCP hunt tool the Copilot
// already had; the app finally gets it.
//
// Results carry an honesty flag: a hit that is already a contact
// ("cnes:<id>" external id — a previous park offer) is labeled so the
// seller does not "discover" their own pipeline.

type TerritoryHuntHolder struct {
	CNES      string `json:"cnes"`
	TradeName string `json:"trade_name,omitempty"`
	Address   string `json:"address,omitempty"`
	Phone     string `json:"phone,omitempty"`
	CityCode  string `json:"city_code,omitempty"`
	// Total is how many units of the asked equipment the establishment
	// declares to CNES. A count, never an age.
	Total int `json:"total"`
	// AlreadyKnown marks a hit that is already in this account's contacts
	// via a previous park offer.
	AlreadyKnown bool `json:"already_known"`
}

type TerritoryHuntResult struct {
	UF string `json:"uf"`
	// Cities lists the territory cities the hunt was narrowed to; empty
	// means the whole UF (no cities in the territory, or none resolved).
	Cities []string `json:"cities,omitempty"`
	// Competencia ("2026-07") is the DATASUS month the counts come from.
	Competencia string                     `json:"competencia,omitempty"`
	Matched     []domain.CNESEquipmentType `json:"matched_types"`
	Holders     []TerritoryHuntHolder      `json:"holders"`
	// Note explains an empty answer (no dictionary match, or the UF's data
	// not ingested yet) — an empty list with no reason reads as "nobody has
	// it", which is a claim the data has not earned.
	Note string `json:"note,omitempty"`
}

type TerritoryHuntUseCase struct {
	park        radarParkReader
	accountRepo domain.AccountRepository
	contacts    domain.ContactRepository
	// identities is optional (SetIdentityRepo): lets already_known see a
	// hospital the account imported by phone, not only one it park-offered.
	identities domain.ContactIdentityRepository
}

func NewTerritoryHuntUseCase(park radarParkReader, accounts domain.AccountRepository, contacts domain.ContactRepository) *TerritoryHuntUseCase {
	return &TerritoryHuntUseCase{park: park, accountRepo: accounts, contacts: contacts}
}

// SetIdentityRepo wires the contact-identity index.
func (uc *TerritoryHuntUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identities = r
}

const (
	territoryHuntLimit    = 25
	territoryHuntMaxLimit = 100
)

// TerritoryHuntOptions are the knobs the MCP tool exposes; the REST face
// uses the defaults. Zero values mean "default".
type TerritoryHuntOptions struct {
	// MinCount is the least units of the equipment a holder must declare
	// (default 1).
	MinCount int
	// Limit caps the holders returned (default 25, ceiling 100).
	Limit int
}

func (o TerritoryHuntOptions) normalized() TerritoryHuntOptions {
	if o.MinCount < 1 {
		o.MinCount = 1
	}
	if o.Limit <= 0 {
		o.Limit = territoryHuntLimit
	}
	if o.Limit > territoryHuntMaxLimit {
		o.Limit = territoryHuntMaxLimit
	}
	return o
}

func (uc *TerritoryHuntUseCase) Execute(ctx context.Context, accountID, equipment, uf string) (*TerritoryHuntResult, error) {
	return uc.ExecuteWith(ctx, accountID, equipment, uf, TerritoryHuntOptions{})
}

// ExecuteWith is Execute with the MCP tool's min-count / limit knobs.
func (uc *TerritoryHuntUseCase) ExecuteWith(ctx context.Context, accountID, equipment, uf string, opts TerritoryHuntOptions) (*TerritoryHuntResult, error) {
	opts = opts.normalized()
	equipment = strings.TrimSpace(equipment)
	if equipment == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "informe o equipamento"}
	}

	uf = strings.ToUpper(strings.TrimSpace(uf))
	var territory *domain.Territory
	if account, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && account != nil {
		territory = account.Territory
	}
	if uf == "" && territory != nil {
		uf = strings.ToUpper(strings.TrimSpace(territory.UF))
	}
	if len(uf) != 2 {
		return nil, &domain.AppError{Code: "TERRITORY_MISSING",
			Message: "a conta não tem território salvo — informe a UF"}
	}
	// The territory's cities narrow the hunt only inside the territory's
	// own UF — an explicit override to another state is a whole-state ask.
	scope := territoryScope{UF: uf}
	if territory != nil && strings.EqualFold(strings.TrimSpace(territory.UF), uf) {
		scope = resolveTerritoryScope(uc.park, territory)
	}

	res := &TerritoryHuntResult{UF: uf, Cities: scope.Cities, Matched: []domain.CNESEquipmentType{}, Holders: []TerritoryHuntHolder{}}

	types, err := uc.park.ResolveEquipmentCodes(equipment)
	if err != nil {
		return nil, err
	}
	if len(types) == 0 {
		res.Note = "nenhum tipo de equipamento do dicionário CNES casa com esse nome"
		return res, nil
	}
	res.Matched = types
	codes := make([]string, 0, len(types))
	for _, t := range types {
		codes = append(codes, t.Code)
	}

	holders, comp, err := uc.park.EstablishmentsWithEquipmentIn(uf, scope.CityCodes, codes, opts.MinCount, opts.Limit, 0)
	if err != nil {
		return nil, err
	}
	res.Competencia = domain.CompetenciaYearMonth(comp)
	if len(holders) == 0 {
		res.Note = "nada carregado ainda para essa UF/equipamento — a ingestão do CNES é mensal"
		return res, nil
	}

	resolver := newContactResolver(uc.contacts, uc.identities)
	for _, h := range holders {
		row := TerritoryHuntHolder{
			CNES:     strings.TrimSpace(h.CNES),
			CityCode: strings.TrimSpace(h.CityCode),
			Total:    h.Total,
		}
		if h.Establishment != nil {
			row.TradeName = strings.TrimSpace(h.Establishment.TradeName)
			row.Address = strings.TrimSpace(h.Establishment.Address)
			if h.Establishment.Phone != nil {
				row.Phone = strings.TrimSpace(*h.Establishment.Phone)
			}
		}
		// Best-effort: a failed lookup leaves the flag off rather than
		// killing the search. Asked through the SAME draft the writer
		// builds, so this screen, the home tile and the nightly sweep
		// cannot disagree about which hospitals are cold ground.
		draft, probe := cnesContactDraft(accountID, row.CNES, h.Establishment)
		if resolver.find(ctx, accountID,
			derefStr(draft.Phone), derefStr(draft.Email), draft.ExternalID, probe) != nil {
			row.AlreadyKnown = true
		}
		res.Holders = append(res.Holders, row)
	}
	return res, nil
}
