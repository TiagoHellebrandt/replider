package mcp

import (
	"context"
	"errors"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Installed-park tool (tese-agente-autonomo): what equipment a hospital
// declares to CNES/DATASUS. Public data, refreshed monthly by the
// CNESParkWorker — the copilot's answer to "o que esse hospital tem?".
func registerCNESTools(s *mcpsdk.Server, deps Deps) {
	if deps.CNESParkRepo != nil {
		registerInstalledParkTool(s, deps)
	}
	if deps.TerritoryHuntUC != nil {
		registerTerritoryHuntTool(s, deps)
	}
}

// TerritoryHunter is the one method find_hospitals_with_equipment needs from
// usecases.TerritoryHuntUseCase (an interface so the tool is testable
// against a fake).
type TerritoryHunter interface {
	ExecuteWith(ctx context.Context, accountID, equipment, uf string, opts usecases.TerritoryHuntOptions) (*usecases.TerritoryHuntResult, error)
}

func registerInstalledParkTool(s *mcpsdk.Server, deps Deps) {
	type parkInput struct {
		CNES string `json:"cnes,omitempty" jsonschema:"código CNES do estabelecimento (7 dígitos), se souber"`
		Name string `json:"name,omitempty" jsonschema:"parte do nome do hospital/clínica para buscar, quando não souber o CNES"`
	}
	type parkEstablishment struct {
		CNES      string  `json:"cnes"`
		TradeName string  `json:"trade_name"`
		LegalName string  `json:"legal_name,omitempty"`
		Address   string  `json:"address,omitempty"`
		District  string  `json:"district,omitempty"`
		Phone     *string `json:"phone,omitempty"`
	}
	type parkOutput struct {
		Establishment *parkEstablishment    `json:"establishment,omitempty"`
		Competencia   string                `json:"competencia,omitempty"`
		Park          []domain.CNESParkItem `json:"park"`
		// Matches lists the candidates when a name search found more than
		// one — the model asks the user which, it does not guess.
		Matches []parkEstablishment `json:"matches,omitempty"`
		Note    string              `json:"note,omitempty"`
	}

	toEst := func(e *domain.CNESEstablishment) *parkEstablishment {
		if e == nil {
			return nil
		}
		return &parkEstablishment{
			CNES: strings.TrimSpace(e.CNES), TradeName: e.TradeName,
			LegalName: e.LegalName, Address: e.Address,
			District: e.District, Phone: e.Phone,
		}
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_installed_park",
		Description: "O parque de equipamentos que um hospital/clínica declara ao CNES (dado público " +
			"DATASUS, atualizado mensalmente com ~45 dias de defasagem): tomógrafo, ressonância, " +
			"raio-X, ventilador, ultrassom etc., com quantidade existente e em uso. " +
			"Passe o código CNES OU um trecho do nome. Se vierem vários `matches`, pergunte ao " +
			"usuário qual é — nunca escolha sozinho. Use para preparar uma oferta: quem tem " +
			"ventilador precisa de manutenção; quem tem tomógrafo consome insumo; parque antigo é " +
			"conversa de upgrade.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in parkInput) (*mcpsdk.CallToolResult, any, error) {
		if _, ok := IdentityFrom(ctx); !ok {
			return nil, nil, errNoIdentity
		}
		cnesID := strings.TrimSpace(in.CNES)
		if cnesID == "" {
			name := strings.TrimSpace(in.Name)
			if name == "" {
				return nil, nil, errors.New("informe cnes ou name")
			}
			found, err := deps.CNESParkRepo.SearchEstablishments(name, 6)
			if err != nil {
				return nil, nil, toolError(err)
			}
			if len(found) == 0 {
				return nil, parkOutput{Park: []domain.CNESParkItem{},
					Note: "nenhum estabelecimento em cache com esse nome; o cache cobre o território das contas e cresce diariamente"}, nil
			}
			if len(found) > 1 {
				out := parkOutput{Park: []domain.CNESParkItem{}}
				for i := range found {
					out.Matches = append(out.Matches, *toEst(&found[i]))
				}
				return nil, out, nil
			}
			cnesID = strings.TrimSpace(found[0].CNES)
		}

		park, comp, err := deps.CNESParkRepo.ParkByCNES(cnesID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		est, err := deps.CNESParkRepo.GetEstablishment(cnesID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := parkOutput{Establishment: toEst(est), Competencia: domain.CompetenciaYearMonth(comp), Park: park}
		if len(park) == 0 {
			out.Note = "sem parque carregado para esse CNES (fora do território ingerido, ou sem equipamento declarado)"
		} else {
			out.Note = "quantidades declaradas na competência " + out.Competencia + " — o CNES nunca informa idade; " +
				"um item 'código CNES NNNN' é um código sem nome no dicionário oficial, não invente o nome"
		}
		return nil, out, nil
	})
}

type huntInput struct {
	Equipment string `json:"equipment" jsonschema:"nome (ou parte) do equipamento: ventilador, tomógrafo, ultrassom…"`
	UF        string `json:"uf,omitempty" jsonschema:"UF de duas letras; vazio usa o território da conta"`
	MinCount  int    `json:"min_count,omitempty" jsonschema:"mínimo de unidades para entrar na lista (padrão 1)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"máximo de resultados (padrão 25, teto 100)"`
}

type huntHolder struct {
	CNES      string  `json:"cnes"`
	Total     int     `json:"total"`
	TradeName string  `json:"trade_name,omitempty"`
	Address   string  `json:"address,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	CityCode  string  `json:"city_code"`
	// AlreadyKnown marks a hit that is already in this account's contacts
	// (a previous park offer, or a spreadsheet import by phone) — the
	// model must not "discover" the user's own pipeline.
	AlreadyKnown bool `json:"already_known"`
}

type huntOutput struct {
	UF string `json:"uf"`
	// Cities lists the territory cities the hunt was narrowed to; empty
	// means the whole UF.
	Cities []string `json:"cities,omitempty"`
	// Competencia ("2026-07") is the DATASUS month the counts come from.
	Competencia string                     `json:"competencia,omitempty"`
	Matched     []domain.CNESEquipmentType `json:"matched_types"`
	Holders     []huntHolder               `json:"holders"`
	Note        string                     `json:"note,omitempty"`
	TotalHits   int                        `json:"total_hits"`
}

// handleFindHospitalsWithEquipment is the tool body: the Radar's hunt use
// case, reshaped to the tool's output. One implementation for both doors.
func handleFindHospitalsWithEquipment(ctx context.Context, deps Deps, in huntInput) (*huntOutput, error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}
	if strings.TrimSpace(in.Equipment) == "" {
		return nil, errors.New("informe equipment")
	}
	res, err := deps.TerritoryHuntUC.ExecuteWith(ctx, id.AccountID, in.Equipment, in.UF,
		usecases.TerritoryHuntOptions{MinCount: in.MinCount, Limit: in.Limit})
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) && appErr.Code == "TERRITORY_MISSING" {
			return nil, errors.New("informe uf (a conta não tem território salvo)")
		}
		return nil, toolError(err)
	}
	out := &huntOutput{UF: res.UF, Cities: res.Cities, Competencia: res.Competencia,
		Matched: res.Matched, Holders: make([]huntHolder, 0, len(res.Holders)),
		Note: res.Note, TotalHits: len(res.Holders)}
	if out.Matched == nil {
		out.Matched = []domain.CNESEquipmentType{}
	}
	for _, h := range res.Holders {
		row := huntHolder{CNES: h.CNES, Total: h.Total, TradeName: h.TradeName,
			Address: h.Address, CityCode: h.CityCode, AlreadyKnown: h.AlreadyKnown}
		if h.Phone != "" {
			phone := h.Phone
			row.Phone = &phone
		}
		out.Holders = append(out.Holders, row)
	}
	return out, nil
}

func registerTerritoryHuntTool(s *mcpsdk.Server, deps Deps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "find_hospitals_with_equipment",
		Description: "Caça no território: estabelecimentos de saúde que declaram ao CNES ter determinado " +
			"equipamento (ex.: quem tem ≥5 ventiladores em MT), maior parque primeiro, com nome, " +
			"endereço e telefone quando o cadastro já está em cache. Dado público DATASUS. " +
			"Use para achar alvos de oferta: quem tem o equipamento compra insumo, manutenção e upgrade. " +
			"`already_known` marca quem já é contato da conta — não trate como descoberta. " +
			"Cada resultado pode virar lead com create_lead — mas pergunte antes de criar em massa.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in huntInput) (*mcpsdk.CallToolResult, any, error) {
		out, err := handleFindHospitalsWithEquipment(ctx, deps, in)
		if err != nil {
			return nil, nil, err
		}
		return nil, *out, nil
	})
}
