package mcp

import (
	"context"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Cadences and strategies (§1.5 E1, §4 E2/E3).
//
// Parity with the screens. The one that earns it most is pause_sequence: a
// salesperson who realises mid-call that a prospect is annoyed needs to stop
// the follow-up right then, from a phone, not after finding a laptop.

func registerSequenceTools(s *mcpsdk.Server, deps Deps) {
	if deps.SequenceUC == nil {
		return
	}

	type stepOut struct {
		Order      int    `json:"order"`
		Kind       string `json:"kind"`
		DelayHours int    `json:"delay_hours,omitempty"`
		TaskTitle  string `json:"task_title,omitempty"`
	}
	type sequenceOut struct {
		ID       string    `json:"id"`
		Name     string    `json:"name"`
		Steps    []stepOut `json:"steps"`
		IsActive bool      `json:"is_active"`
		// StopOnReply is always true and is reported anyway: it is the rule
		// the whole feature rests on, and a co-pilot explaining a cadence
		// should be able to say it.
		StopOnReply bool `json:"stop_on_reply"`
	}
	type listOut struct {
		Sequences []sequenceOut `json:"sequences"`
	}

	project := func(list []*domain.Sequence) *listOut {
		out := &listOut{Sequences: make([]sequenceOut, 0, len(list))}
		for _, seq := range list {
			item := sequenceOut{
				ID: seq.ID, Name: seq.Name, IsActive: seq.IsActive,
				StopOnReply: seq.StopOnReply,
				Steps:       make([]stepOut, 0, len(seq.Steps)),
			}
			for _, step := range seq.Steps {
				item.Steps = append(item.Steps, stepOut{
					Order: step.Order, Kind: string(step.Kind),
					DelayHours: step.DelayHours, TaskTitle: step.TaskTitle,
				})
			}
			out.Sequences = append(out.Sequences, item)
		}
		return out
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_sequences",
		Description: "Lista as cadências de follow-up da conta e seus passos. Toda cadência para " +
			"sozinha quando o lead responde — isso não é configurável.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *listOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		list, err := deps.SequenceUC.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, project(list), nil
	})

	type enrollInput struct {
		SequenceID string `json:"sequence_id"`
		LeadID     string `json:"lead_id"`
	}
	type okOut struct {
		OK bool `json:"ok"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "add_lead_to_sequence",
		Description: "Coloca um lead numa cadência. Um lead fica em UMA cadência por vez — duas " +
			"significam dois follow-ups por semana da mesma empresa.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in enrollInput) (*mcpsdk.CallToolResult, *okOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if _, err := deps.SequenceUC.Enroll(ctx, id.AccountID, in.SequenceID, in.LeadID); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOut{OK: true}, nil
	})

	type pauseInput struct {
		LeadID string `json:"lead_id"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "pause_sequence",
		Description: "Para a cadência de um lead agora. Use quando o usuário disser que o contato " +
			"pediu para parar, ficou incomodado, ou que a conversa saiu do automático.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in pauseInput) (*mcpsdk.CallToolResult, *okOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if err := deps.SequenceUC.Pause(ctx, id.AccountID, in.LeadID); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOut{OK: true}, nil
	})
}

func registerStrategyTools(s *mcpsdk.Server, deps Deps) {
	if deps.StrategyRepo == nil {
		return
	}

	type strategyOut struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Version    int      `json:"version"`
		Status     string   `json:"status"`
		ICP        string   `json:"icp,omitempty"`
		Tone       string   `json:"tone,omitempty"`
		Arguments  []string `json:"arguments,omitempty"`
		AuthorName string   `json:"author_name,omitempty"`
		// ReplyRate is nil when nothing was sent under this version: zero out
		// of zero is unknown, not zero.
		ReplyRate  *float64 `json:"reply_rate,omitempty"`
		Approaches int      `json:"approaches,omitempty"`
	}
	type listOut struct {
		Strategies []strategyOut `json:"strategies"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_strategies",
		Description: "Lista as estratégias de venda da conta com os números de cada VERSÃO — porque " +
			"'converte 14%' não significa nada se os argumentos mudaram desde então.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *listOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		list, err := deps.StrategyRepo.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		// One stats query for the whole list, not one per strategy.
		current := map[string]domain.StrategyStat{}
		if stats, err := deps.StrategyRepo.Stats(ctx, id.AccountID); err == nil {
			for _, stat := range stats {
				// Keeps the highest version seen, which is the one the
				// strategy is on now.
				if prev, ok := current[stat.StrategyID]; !ok || stat.Version > prev.Version {
					current[stat.StrategyID] = stat
				}
			}
		}

		out := &listOut{Strategies: make([]strategyOut, 0, len(list))}
		for _, s := range list {
			item := strategyOut{
				ID: s.ID, Name: s.Name, Version: s.Version,
				Status: string(s.Status), ICP: s.ICP, Tone: s.Tone,
				Arguments: s.Arguments, AuthorName: s.AuthorName,
			}
			if stat, ok := current[s.ID]; ok && stat.Version == s.Version {
				item.ReplyRate = stat.ReplyRate()
				item.Approaches = stat.Approaches
			}
			out.Strategies = append(out.Strategies, item)
		}
		return nil, out, nil
	})
}

// Sales (§5, F1).
//
// register_sale earns its place: closing happens on a call, and the moment a
// salesperson will actually record it is right then, out loud, to a co-pilot
// — not later, in a form, if they remember.
func registerSaleTools(s *mcpsdk.Server, deps Deps) {
	if deps.SaleUC == nil {
		return
	}

	type saleItemIn struct {
		Description string `json:"description"`
		Quantity    int    `json:"quantity,omitempty"`
		// ProductID keys the reorder engine and the cross-sell on the
		// catalogue product rather than on how the item was typed.
		ProductID string `json:"product_id,omitempty" jsonschema:"id do produto do catálogo, quando for um"`
		UnitCents int64  `json:"unit_cents,omitempty" jsonschema:"preço unitário em CENTAVOS"`
	}
	type registerInput struct {
		// ContactID or LeadID: one of the two. From a lead the customer is
		// resolved server-side and the lead is marked won.
		ContactID string `json:"contact_id,omitempty" jsonschema:"id ou telefone/e-mail do cliente"`
		LeadID    string `json:"lead_id,omitempty" jsonschema:"id do lead que fechou; dispensa contact_id e marca o lead como ganho"`
		// GrossCents in CENTS. Named so the model cannot pass reais and be
		// off by a hundred — which on money is the whole point.
		GrossCents int64        `json:"gross_cents" jsonschema:"valor bruto em CENTAVOS (R$ 1.500,00 = 150000)"`
		CostCents  int64        `json:"cost_cents,omitempty" jsonschema:"custo da mercadoria em CENTAVOS"`
		TaxCents   int64        `json:"tax_cents,omitempty" jsonschema:"imposto em CENTAVOS"`
		Currency   string       `json:"currency,omitempty" jsonschema:"brl ou usd; padrão é a moeda da conta"`
		SoldAt     string       `json:"sold_at,omitempty" jsonschema:"data da venda (dd/mm/aaaa ou aaaa-mm-dd); padrão é hoje"`
		CycleDays  int          `json:"cycle_days,omitempty" jsonschema:"de quantos em quantos dias o cliente costuma recomprar"`
		Items      []saleItemIn `json:"items,omitempty"`
		Notes      string       `json:"notes,omitempty"`
	}
	type registerOutput struct {
		ID       string `json:"id"`
		NetCents int64  `json:"net_cents"`
		// Underwater is reported so the co-pilot can say it out loud: a sale
		// that cost more than it brought in is not refused, but nobody
		// should find out about it in a commission statement.
		Underwater bool `json:"underwater"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "register_sale",
		Description: "Registra uma venda fechada. Peça o custo e o imposto: comissão sobre o BRUTO é " +
			"comissão sobre dinheiro que a empresa não teve. Confirme os valores com o " +
			"usuário antes de chamar — isso entra no cálculo de comissão de alguém.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in registerInput) (*mcpsdk.CallToolResult, *registerOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		sale := &domain.Sale{
			ContactID: in.ContactID, LeadID: in.LeadID, GrossCents: in.GrossCents,
			CostCents: in.CostCents, TaxCents: in.TaxCents,
			Currency:  strings.ToLower(strings.TrimSpace(in.Currency)),
			CycleDays: in.CycleDays, Notes: in.Notes,
			// The person told the co-pilot; that is them recording it, not a
			// model reading a conversation on its own.
			Source:       domain.SaleManual,
			SellerUserID: id.UserID,
		}
		if in.SoldAt != "" {
			parsed, ok := usecases.ParseSaleDate(in.SoldAt)
			if !ok {
				return nil, nil, toolError(&domain.AppError{
					Code: "VALIDATION_ERROR", Message: "data da venda não reconhecida (use dd/mm/aaaa)"})
			}
			sale.SoldAt = parsed
		}
		for _, item := range in.Items {
			sale.Items = append(sale.Items, domain.SaleItem{
				Description: item.Description, Quantity: item.Quantity,
				ProductID: item.ProductID, UnitCents: item.UnitCents,
			})
		}

		saved, err := deps.SaleUC.Register(ctx, id.AccountID, sale)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &registerOutput{
			ID: saved.ID, NetCents: saved.NetCents(), Underwater: saved.Underwater(),
		}, nil
	})
}

// Portfolio and repurchase (§6.5, F2/F3).
//
// upcoming_reorders is the tool a salesperson wants at 8am: who is about to
// run out, in order. It is read-only — the worker is what puts suggestions in
// the approval queue, and a tool that could send would be a second, weaker
// path to the same decision.
func registerPortfolioTools(s *mcpsdk.Server, deps Deps) {
	if deps.PortfolioRepo == nil {
		return
	}

	type customerOut struct {
		ContactID   string `json:"contact_id"`
		ContactName string `json:"contact_name"`
		Purchases   int    `json:"purchases"`
		// NetCents leads over gross: what a customer is worth is what the
		// company kept.
		NetCents int64 `json:"net_cents"`
		// DaysUntilDue is NEGATIVE when they have already run out, which is
		// the most expensive state in the model and must not read as zero.
		DaysUntilDue     *int `json:"days_until_due,omitempty"`
		DaysSinceContact *int `json:"days_since_contact,omitempty"`
	}
	type listOut struct {
		Customers []customerOut `json:"customers"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_portfolio",
		Description: "A carteira: os clientes da conta — importados ou conquistados — mais urgente " +
			"primeiro: quem está prestes a ficar sem, quem já ficou, e há quantos dias ninguém fala " +
			"com eles. ATENÇÃO: `purchases: 0` significa cliente importado que ainda não comprou " +
			"aqui, não cliente sem valor. Esta descrição dizia \"clientes que já compraram\" e a " +
			"lista devolvia 82 contatos com zero compras — quem lesse concluía que a carteira estava " +
			"quebrada.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		Limit int `json:"limit,omitempty" jsonschema:"máximo de clientes (padrão 20, máx 50)"`
	}) (*mcpsdk.CallToolResult, *listOut, error) {
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

		list, err := deps.PortfolioRepo.List(ctx, id.AccountID, limit, 0)
		if err != nil {
			return nil, nil, toolError(err)
		}
		now := time.Now()
		out := &listOut{Customers: make([]customerOut, 0, len(list))}
		for _, c := range list {
			out.Customers = append(out.Customers, customerOut{
				ContactID: c.ContactID, ContactName: c.ContactName,
				Purchases: c.Purchases, NetCents: c.NetCents,
				DaysUntilDue:     c.DaysUntilDue(now),
				DaysSinceContact: c.DaysSinceContact(now),
			})
		}
		return nil, out, nil
	})

	type reorderOut struct {
		ContactID   string `json:"contact_id"`
		ContactName string `json:"contact_name"`
		DueAt       string `json:"due_at"`
		LastGross   int64  `json:"last_gross_cents"`
		Items       string `json:"items,omitempty"`
	}
	type reordersOut struct {
		Reorders []reorderOut `json:"reorders"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upcoming_reorders",
		Description: "Quem está prestes a ficar sem o que comprou, com a data prevista. As sugestões " +
			"de abordagem já vão sozinhas para a fila de aprovação — isto aqui é para " +
			"o vendedor decidir quem ligar antes.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *reordersOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		scoped, ok := deps.PortfolioRepo.(interface {
			DueForReorderByAccount(ctx context.Context, accountID string, within time.Time, limit int) ([]*domain.ReorderCandidate, error)
		})
		if !ok {
			return nil, nil, toolError(&domain.AppError{
				Code: "PORTFOLIO_UNAVAILABLE", Message: "previsões de recompra indisponíveis"})
		}

		horizon := time.Now().Add(domain.ReorderLeadTimeDays * 24 * time.Hour)
		list, err := scoped.DueForReorderByAccount(ctx, id.AccountID, horizon, maxConversationPageSize)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &reordersOut{Reorders: make([]reorderOut, 0, len(list))}
		for _, c := range list {
			item := reorderOut{
				ContactID: c.ContactID, ContactName: c.ContactName,
				DueAt: c.DueAt.Format("2006-01-02"), LastGross: c.GrossCents,
			}
			if len(c.Items) > 0 {
				item.Items = c.Items[0].Description
			}
			out.Reorders = append(out.Reorders, item)
		}
		return nil, out, nil
	})

	if deps.ProductCycles == nil {
		return
	}

	type setCycleInput struct {
		ProductID string `json:"product_id"`
		// ConsumptionCycleDays is what makes a product appear in the reorder
		// engine at all. Zero removes it.
		ConsumptionCycleDays int `json:"consumption_cycle_days" jsonschema:"de quantos em quantos dias um cliente costuma consumir um pedido deste produto"`
		ShelfLifeDays        int `json:"shelf_life_days,omitempty" jsonschema:"validade — número DIFERENTE do consumo: gaze vence mesmo sem ninguém usar"`
	}
	type okOut struct {
		OK bool `json:"ok"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "set_product_cycle",
		Description: "Define de quanto em quanto tempo um produto é reposto. É o que faz o motor de " +
			"recompra enxergar esse produto — sem ciclo, nenhum cliente é lembrado dele.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in setCycleInput) (*mcpsdk.CallToolResult, *okOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if in.ConsumptionCycleDays < 0 || in.ConsumptionCycleDays > 3650 ||
			in.ShelfLifeDays < 0 || in.ShelfLifeDays > 3650 {
			return nil, nil, toolError(&domain.AppError{
				Code: "VALIDATION_ERROR", Message: "ciclo inválido"})
		}
		err := deps.ProductCycles.SetCycles(ctx, in.ProductID, id.AccountID,
			in.ConsumptionCycleDays, in.ShelfLifeDays)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOut{OK: true}, nil
	})
}
