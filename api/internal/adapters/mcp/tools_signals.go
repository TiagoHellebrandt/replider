package mcp

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Buying signals (§2.1).
//
// The reason this tool matters more than most: a tender is worth acting on
// today and worthless next week, and a salesperson reads their co-pilot
// before they open a dashboard.

func registerSignalTools(s *mcpsdk.Server, deps Deps) {
	if deps.SignalRepo == nil {
		return
	}

	type signalOut struct {
		ID           string `json:"id"`
		Source       string `json:"source"`
		Organization string `json:"organization"`
		CNPJ         string `json:"cnpj,omitempty"`
		Title        string `json:"title"`
		Description  string `json:"description,omitempty"`
		URL          string `json:"url,omitempty"`
		// ValueCents is zero when the notice did not disclose a value, which
		// is common in public tenders and is NOT the same as zero reais.
		ValueCents int64  `json:"value_cents,omitempty"`
		City       string `json:"city,omitempty"`
		State      string `json:"state,omitempty"`
		// DaysLeft is nil when there is no published deadline. Three days is
		// a reason to write today; thirty is a reason to prepare properly.
		DaysLeft *int `json:"days_left,omitempty"`
	}
	type listSignalsOutput struct {
		Signals []signalOut `json:"signals"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_signals",
		Description: "Lista sinais públicos de intenção de compra (licitações abertas), prazo mais " +
			"próximo primeiro. Já vêm filtrados pelos interesses da conta e sem os que " +
			"venceram — abordar alguém sobre um pregão encerrado prova que ninguém leu.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		Limit int `json:"limit,omitempty" jsonschema:"máximo de sinais (padrão 20, máx 50)"`
	}) (*mcpsdk.CallToolResult, *listSignalsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > maxConversationPageSize {
			limit = maxConversationPageSize
		}

		list, err := deps.SignalRepo.List(ctx, id.AccountID, limit, 0)
		if err != nil {
			return nil, nil, toolError(err)
		}
		now := time.Now()
		out := &listSignalsOutput{Signals: make([]signalOut, 0, len(list))}
		for _, sig := range list {
			out.Signals = append(out.Signals, signalOut{
				ID: sig.ID, Source: string(sig.Source),
				Organization: sig.Organization, CNPJ: sig.CNPJ,
				Title: sig.Title, Description: sig.Description, URL: sig.URL,
				ValueCents: sig.ValueCents, City: sig.City, State: sig.State,
				DaysLeft: sig.DaysLeft(now),
			})
		}
		return nil, out, nil
	})

	type dismissInput struct {
		SignalID string `json:"signal_id"`
	}
	type okOutput struct {
		OK bool `json:"ok"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "dismiss_signal",
		Description: "Descarta um sinal (\"não é para nós\"). Fica registrado, não apagado — o próximo " +
			"ciclo de coleta não traz o mesmo edital de volta.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in dismissInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if err := deps.SignalRepo.Dismiss(ctx, in.SignalID, id.AccountID, time.Now()); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true}, nil
	})

	type watchInput struct {
		Source string `json:"source,omitempty" jsonschema:"pncp (padrão), new_company ou job_post"`
		// Keywords match the tender's object and the buyer's name. ANY match
		// counts — a supplier of monitors sells to "monitor multiparâmetro"
		// and to "monitorização".
		Keywords      []string `json:"keywords"`
		States        []string `json:"states" jsonschema:"OBRIGATÓRIO. UFs onde a empresa entrega, ex: MT, SP. Sem UF o monitoramento varre o país inteiro e é recusado."`
		MinValueCents int64    `json:"min_value_cents,omitempty"`
	}
	type watchOutput struct {
		ID string `json:"id"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_signal_watch",
		Description: "Cria um monitoramento de licitações por palavra-chave, UF e valor mínimo. " +
			"É o que faz os sinais aparecerem — sem nenhum monitoramento, nada é coletado.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in watchInput) (*mcpsdk.CallToolResult, *watchOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		source := domain.SignalSource(in.Source)
		if source == "" {
			source = domain.SignalPNCP
		}
		switch source {
		case domain.SignalPNCP, domain.SignalNewCompany, domain.SignalJobPost:
		default:
			return nil, nil, toolError(&domain.AppError{
				Code: "VALIDATION_ERROR", Message: "fonte de sinal desconhecida"})
		}

		watch := &domain.SignalWatch{
			ID: ulid.Make().String(), AccountID: id.AccountID, Source: source,
			Keywords: in.Keywords, States: in.States,
			MinValueCents: in.MinValueCents, IsActive: true,
		}
		// Same rule as the HTTP route: no keyword matches nothing, and no UF
		// is not a territory (domain.SignalWatch.NormalizeAndValidate).
		if err := watch.NormalizeAndValidate(); err != nil {
			return nil, nil, toolError(err)
		}
		if err := deps.SignalRepo.UpsertWatch(ctx, watch); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &watchOutput{ID: watch.ID}, nil
	})
}

// Target audiences (§2, D3).
//
// The tool exists for the same reason the screen does: the value is the
// SECOND and third audience, and a salesperson is far more likely to ask
// "para quem mais eu poderia vender?" out loud than to open a settings page.
func registerAudienceTools(s *mcpsdk.Server, deps Deps) {
	if deps.AudienceUC == nil {
		return
	}

	type audienceOut struct {
		ID            string   `json:"id,omitempty"`
		Name          string   `json:"name"`
		Segment       string   `json:"segment"`
		Size          string   `json:"size,omitempty"`
		DecisionMaker string   `json:"decision_maker,omitempty"`
		Regions       []string `json:"regions,omitempty"`
		Keywords      []string `json:"keywords,omitempty"`
		// Unexplored marks the one deliberately adjacent suggestion — the
		// segment that buys the same solution for a different reason and
		// that this company probably has not considered.
		Unexplored bool `json:"unexplored,omitempty"`
	}
	type audienceListOut struct {
		Audiences []audienceOut `json:"audiences"`
	}

	project := func(list []*domain.Audience) *audienceListOut {
		out := &audienceListOut{Audiences: make([]audienceOut, 0, len(list))}
		for _, a := range list {
			out.Audiences = append(out.Audiences, audienceOut{
				ID: a.ID, Name: a.Name, Segment: a.Segment, Size: a.Size,
				DecisionMaker: a.DecisionMaker, Regions: a.Regions,
				Keywords: a.Keywords, Unexplored: a.Unexplored,
			})
		}
		return out
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_audiences",
		Description: "Lista os públicos-alvo salvos da conta.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *audienceListOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		list, err := deps.AudienceUC.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, project(list), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "suggest_audiences",
		Description: "Propõe 3 públicos-alvo a partir do que a empresa faz e já vende. Dois óbvios e " +
			"UM adjacente — o segmento que compra a mesma solução por outro motivo e que " +
			"provavelmente ninguém considerou. NÃO salva nada: mostre ao usuário e pergunte.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *audienceListOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		list, err := deps.AudienceUC.Suggest(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, project(list), nil
	})

	type saveAudienceInput struct {
		Name          string   `json:"name,omitempty"`
		Segment       string   `json:"segment"`
		Size          string   `json:"size,omitempty"`
		DecisionMaker string   `json:"decision_maker,omitempty" jsonschema:"o CARGO a alcançar — decide como a abordagem é escrita"`
		Regions       []string `json:"regions,omitempty"`
		Keywords      []string `json:"keywords,omitempty"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "save_audience",
		Description: "Salva um público-alvo. Use depois que o usuário escolher entre as sugestões.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in saveAudienceInput) (*mcpsdk.CallToolResult, *audienceOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		saved, err := deps.AudienceUC.Save(ctx, id.AccountID, &domain.Audience{
			Name: in.Name, Segment: in.Segment, Size: in.Size,
			DecisionMaker: in.DecisionMaker, Regions: in.Regions,
			Keywords: in.Keywords,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := project([]*domain.Audience{saved}).Audiences[0]
		return nil, &out, nil
	})
}
