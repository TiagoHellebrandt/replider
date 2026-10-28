package mcp

import (
	"context"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Relationship routines and cross-sell (§6.5, F4/F5).
//
// The tools that earn their place on a phone. "É aniversário de quem essa
// semana?" and "o que mais eu ofereço pra esse cliente?" are both questions a
// salesperson has in the car on the way to a visit, and both have an answer
// sitting in data nobody opens a laptop to read.

func registerRelationshipTools(s *mcpsdk.Server, deps Deps) {
	if deps.RelationshipUC != nil {
		registerRelationshipDateTools(s, deps)
	}
	if deps.CrossSellUC != nil {
		registerCrossSellTool(s, deps)
	}
}

func registerRelationshipDateTools(s *mcpsdk.Server, deps Deps) {
	type dateOut struct {
		ID        string `json:"id"`
		Kind      string `json:"kind"`
		ContactID string `json:"contact_id,omitempty"`
		Month     int    `json:"month,omitempty"`
		Day       int    `json:"day,omitempty"`
		DueAt     string `json:"due_at,omitempty"`
		Label     string `json:"label,omitempty"`
		Notes     string `json:"notes,omitempty"`
	}
	type listOut struct {
		Dates []dateOut `json:"dates"`
	}

	project := func(list []*domain.RelationshipDate) *listOut {
		out := &listOut{Dates: make([]dateOut, 0, len(list))}
		for _, d := range list {
			item := dateOut{
				ID: d.ID, Kind: string(d.Kind), ContactID: d.ContactID,
				Month: d.Month, Day: d.Day, Label: d.Label, Notes: d.Notes,
			}
			if d.DueAt != nil {
				item.DueAt = d.DueAt.Format(time.RFC3339)
			}
			out.Dates = append(out.Dates, item)
		}
		return out
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_relationship_dates",
		Description: "Lista as datas de relacionamento da conta: aniversários, aberturas de " +
			"empresa, datas do setor e manutenções preventivas agendadas.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *listOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		list, err := deps.RelationshipUC.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, project(list), nil
	})

	type greetingInput struct {
		// Kind is birthday | founded | sector | maintenance | custom.
		Kind string `json:"kind"`
		// ContactID is required for anything that greets a PERSON. A date
		// with no contact is account-wide and is never auto-sent: firing it
		// would message the whole portfolio, which is a campaign.
		ContactID string `json:"contact_id,omitempty"`
		Month     int    `json:"month,omitempty"`
		Day       int    `json:"day,omitempty"`
		// DueAt is an RFC3339 instant, for a one-off (maintenance).
		DueAt string `json:"due_at,omitempty"`
		// SectorKey fills the date from the shipped Brazilian health
		// calendar: enfermagem, medico, dentista, farmaceutico…
		SectorKey string `json:"sector_key,omitempty"`
		Label     string `json:"label,omitempty"`
		Notes     string `json:"notes,omitempty"`
	}
	type savedOut struct {
		ID string `json:"id"`
		// Queued says plainly what happens next, because the answer is not
		// "it was sent". The worker drafts on the day and a human approves.
		Queued string `json:"what_happens_next"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "schedule_greeting",
		Description: "Agenda uma data de relacionamento (aniversário, aniversário da empresa, " +
			"data do setor, manutenção preventiva). No dia, o sistema escreve o rascunho e " +
			"coloca na fila de aprovação — nada é enviado sem um humano aprovar. " +
			"Use sector_key para as datas profissionais " +
			"brasileiras (enfermagem 12/05, medico 18/10, dentista 25/10 e outras).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in greetingInput) (*mcpsdk.CallToolResult, *savedOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		input := usecases.RelationshipDateInput{
			Kind: in.Kind, ContactID: in.ContactID,
			Month: in.Month, Day: in.Day,
			SectorKey: in.SectorKey, Label: in.Label, Notes: in.Notes,
		}
		if in.DueAt != "" {
			at, err := time.Parse(time.RFC3339, in.DueAt)
			if err != nil {
				return nil, nil, toolError(&domain.AppError{
					Code: "VALIDATION_ERROR", Message: "due_at deve estar em RFC3339"})
			}
			input.DueAt = &at
		}
		date, err := deps.RelationshipUC.Save(ctx, id.AccountID, input)
		if err != nil {
			return nil, nil, toolError(err)
		}

		next := "No dia, o rascunho vai para a fila de aprovação."
		if date.ContactID == "" {
			next = "Data da conta inteira: aparece como próxima data na tela, " +
				"mas não vira mensagem sozinha — para falar com todo mundo, use uma campanha."
		}
		return nil, &savedOut{ID: date.ID, Queued: next}, nil
	})
}

func registerCrossSellTool(s *mcpsdk.Server, deps Deps) {
	type suggestionOut struct {
		Suggest string `json:"suggest"`
		// Because is the evidence. A suggestion without a reason is a guess
		// the customer can hear.
		Because    string  `json:"because"`
		Confidence float64 `json:"confidence"`
		Customers  int     `json:"customers"`
	}
	type crossSellOut struct {
		Suggestions []suggestionOut `json:"suggestions"`
		Bought      []string        `json:"already_buys"`
		// Note explains an empty answer, which is the common one for a young
		// account — otherwise a co-pilot reports "nenhuma sugestão" as if the
		// customer were a dead end.
		Note string `json:"note,omitempty"`
	}
	type input struct {
		ContactID string `json:"contact_id"`
		Limit     int    `json:"limit,omitempty"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "suggest_cross_sell",
		Description: "Sugere o que mais oferecer a um cliente, a partir dos padrões de co-compra " +
			"das vendas DESTA conta (quem compra luva costuma comprar máscara). Nunca sugere " +
			"algo que o cliente já compra, e sempre diz em que evidência se baseia.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in input) (*mcpsdk.CallToolResult, *crossSellOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		result, err := deps.CrossSellUC.Suggest(ctx, id.AccountID, in.ContactID, in.Limit)
		if err != nil {
			return nil, nil, toolError(err)
		}

		out := &crossSellOut{
			Suggestions: make([]suggestionOut, 0, len(result.Suggestions)),
			Bought:      result.Bought,
		}
		for _, s := range result.Suggestions {
			out.Suggestions = append(out.Suggestions, suggestionOut{
				Suggest: s.Suggest, Because: s.Because,
				Confidence: s.Confidence, Customers: s.Customers,
			})
		}
		switch {
		case len(result.Bought) == 0:
			out.Note = "Este contato não tem venda registrada, então não há em que ancorar uma sugestão."
		case len(out.Suggestions) == 0:
			out.Note = "Ainda não há padrão de co-compra suficiente nas vendas desta conta " +
				"para sugerir algo com honestidade."
		}
		return nil, out, nil
	})
}
