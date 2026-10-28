package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Client-facing commission rules (§6.6, G1).
//
// Admin-only, like the HTTP routes: a commission rule is a pay policy, and the
// co-pilot reading one out loud to the wrong person is reading a salary.
//
// simulate_commission exists as its own tool rather than as a flag on the
// upsert because the answer to "e se eu pagar 3% sobre a margem?" must be
// obtainable WITHOUT writing anything — a co-pilot that has to save a rule to
// price it would leave half-considered policies in the account.
func registerCommissionTools(s *mcpsdk.Server, deps Deps) {
	if deps.CommissionRuleUC == nil {
		return
	}

	// tierIn/milestoneIn/splitIn mirror the domain rather than accepting a
	// formula string. The model cannot invent an operator that the engine
	// does not implement, which is the same reason the UI is a builder.
	type tierIn struct {
		FromCents   int64   `json:"from_cents" jsonschema:"total vendido no mês a partir do qual este escalão vale, em CENTAVOS"`
		RatePercent float64 `json:"rate_percent" jsonschema:"percentual deste escalão (2.5 = 2,5%)"`
	}
	type milestoneIn struct {
		TargetCents int64  `json:"target_cents" jsonschema:"meta do período em CENTAVOS"`
		BonusCents  int64  `json:"bonus_cents" jsonschema:"bônus fixo em CENTAVOS ao bater a meta"`
		Label       string `json:"label,omitempty"`
	}
	type splitIn struct {
		Role         string  `json:"role" jsonschema:"papel como a empresa chama (closer, quem indicou, gerente)"`
		SharePercent float64 `json:"share_percent" jsonschema:"fatia DA COMISSÃO, não da venda; o conjunto precisa somar 100"`
	}
	type ruleIn struct {
		ID   string `json:"id,omitempty" jsonschema:"id da regra para editar; vazio cria uma nova"`
		Name string `json:"name" jsonschema:"nome da regra como o time a chama"`
		// Scope/ScopeID: a rule with no scope is the account's fallback. The
		// most specific rule wins, so a seller rule overrides the global one
		// without anybody having to delete anything.
		Scope   string `json:"scope,omitempty" jsonschema:"global (padrão), seller, team ou product"`
		ScopeID string `json:"scope_id,omitempty" jsonschema:"id do vendedor/time/produto quando o escopo não é global"`

		Base       string `json:"base,omitempty" jsonschema:"margin (padrão, valor menos custo), gross (valor cheio) ou net"`
		DeductTax  bool   `json:"deduct_tax,omitempty" jsonschema:"só vale para base net: desconta o imposto antes do percentual"`
		DeductCost bool   `json:"deduct_cost,omitempty" jsonschema:"só vale para base net: desconta o custo antes do percentual"`

		RatePercent float64  `json:"rate_percent,omitempty" jsonschema:"percentual fixo (2.5 = 2,5%); ignorado quando há escalões"`
		Tiers       []tierIn `json:"tiers,omitempty" jsonschema:"escalões por meta; substituem o percentual fixo"`
		// RepurchaseRatePercent is a pointer so "not provided" (same as a new
		// sale) stays different from 0 ("não pagamos nada na recompra").
		RepurchaseRatePercent *float64      `json:"repurchase_rate_percent,omitempty" jsonschema:"percentual só para recompra; ausente = igual a venda nova, 0 = não paga nada"`
		Milestones            []milestoneIn `json:"milestones,omitempty"`
		Splits                []splitIn     `json:"splits,omitempty"`
		Active                *bool         `json:"active,omitempty" jsonschema:"false desativa a regra sem apagá-la"`
	}

	toDomain := func(in ruleIn) *domain.CommissionRule {
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		rule := &domain.CommissionRule{
			ID:                    in.ID,
			Name:                  in.Name,
			Scope:                 domain.CommissionScopeKind(in.Scope),
			ScopeID:               in.ScopeID,
			Base:                  domain.CommissionBaseKind(in.Base),
			Deductions:            domain.CommissionDeductions{Tax: in.DeductTax, Cost: in.DeductCost},
			RatePercent:           in.RatePercent,
			RepurchaseRatePercent: in.RepurchaseRatePercent,
			Active:                active,
		}
		for _, t := range in.Tiers {
			rule.Tiers = append(rule.Tiers, domain.CommissionTier{
				FromCents: t.FromCents, RatePercent: t.RatePercent,
			})
		}
		for _, m := range in.Milestones {
			rule.Milestones = append(rule.Milestones, domain.CommissionMilestone{
				TargetCents: m.TargetCents, BonusCents: m.BonusCents, Label: m.Label,
			})
		}
		for _, sp := range in.Splits {
			rule.Splits = append(rule.Splits, domain.CommissionSplit{
				Role: sp.Role, SharePercent: sp.SharePercent,
			})
		}
		return rule
	}

	type ruleOut struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Scope string `json:"scope"`
		// ScopeID is echoed so the co-pilot can say WHOSE rule it just read
		// instead of "uma regra de vendedor".
		ScopeID     string  `json:"scope_id,omitempty"`
		Base        string  `json:"base"`
		RatePercent float64 `json:"rate_percent"`
		Tiers       int     `json:"tiers"`
		Splits      int     `json:"splits"`
		Active      bool    `json:"active"`
	}
	summarize := func(r *domain.CommissionRule) ruleOut {
		return ruleOut{
			ID: r.ID, Name: r.Name, Scope: string(r.Scope), ScopeID: r.ScopeID,
			Base: string(r.Base), RatePercent: r.RatePercent,
			Tiers: len(r.Tiers), Splits: len(r.Splits), Active: r.Active,
		}
	}

	type listOut struct {
		Rules []ruleOut `json:"rules"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_commission_rules",
		Description: "Lista as regras de comissão da conta: escopo, base de cálculo, percentual e se " +
			"está ativa. A regra mais específica vence a global.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *listOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		rules, err := deps.CommissionRuleUC.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listOut{Rules: make([]ruleOut, 0, len(rules))}
		for _, r := range rules {
			out.Rules = append(out.Rules, summarize(r))
		}
		return nil, out, nil
	})

	type simulateInput struct {
		Rule          ruleIn  `json:"rule"`
		GrossCents    int64   `json:"gross_cents" jsonschema:"venda média em CENTAVOS (R$ 10.000,00 = 1000000)"`
		MarginPercent float64 `json:"margin_percent,omitempty" jsonschema:"margem bruta típica em % (padrão 30)"`
		SalesPerMonth int     `json:"sales_per_month,omitempty" jsonschema:"quantas vendas assim por mês (padrão 1)"`
	}
	type simulateOut struct {
		PerSaleCents int64 `json:"per_sale_cents"`
		MonthlyCents int64 `json:"monthly_cents"`
		// EffectiveRatePercent is the commission over the month's GROSS — what
		// the rule costs the company, whichever base it was written against.
		EffectiveRatePercent float64 `json:"effective_rate_percent"`
		BaseCents            int64   `json:"base_cents"`
		RatePercent          float64 `json:"rate_percent"`
		Explanation          string  `json:"explanation"`
		Warning              string  `json:"warning,omitempty"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "simulate_commission",
		Description: "Simula quanto uma regra pagaria ANTES de salvá-la, com o mesmo cálculo do extrato. " +
			"Use isto para responder \"e se eu pagar X%?\" — nada é gravado. Mostre o valor por " +
			"venda, o do mês e o percentual efetivo sobre o faturamento: comissão sobre o BRUTO " +
			"pode passar da margem inteira.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in simulateInput) (*mcpsdk.CallToolResult, *simulateOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		sim, err := deps.CommissionRuleUC.Simulate(ctx, id.AccountID, toDomain(in.Rule),
			in.GrossCents, in.MarginPercent, in.SalesPerMonth)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &simulateOut{
			PerSaleCents: sim.PerSaleCents, MonthlyCents: sim.MonthlyCents,
			EffectiveRatePercent: sim.EffectiveRatePercent,
		}
		if sim.Calc != nil {
			out.BaseCents = sim.Calc.BaseCents
			out.RatePercent = sim.Calc.RatePercent
			out.Explanation = sim.Calc.Explanation
			out.Warning = sim.Calc.Warning
		}
		return nil, out, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upsert_commission_rule",
		Description: "Cria ou edita uma regra de comissão. SIMULE primeiro com simulate_commission e " +
			"mostre o resultado ao usuário: isto define o que uma pessoa recebe no fim do mês. " +
			"Só existe UMA regra global ativa por conta — para uma exceção, use escopo de " +
			"vendedor, time ou produto, que vence a global.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ruleIn) (*mcpsdk.CallToolResult, *ruleOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		saved, err := deps.CommissionRuleUC.Save(ctx, id.AccountID, toDomain(in))
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := summarize(saved)
		return nil, &out, nil
	})
}

// The monthly close (§6.6, G2).
//
// mark_commission_paid is the one tool here that moves money in the record, and
// it is the reason the pair exists: the transfer happens in a banking app, and
// the moment somebody will actually record it is right after they send it, out
// loud, not later in a form.
func registerCommissionStatementTools(s *mcpsdk.Server, deps Deps) {
	if deps.CommissionStmtUC == nil {
		return
	}

	type statementOut struct {
		SellerUserID string `json:"seller_user_id"`
		SellerName   string `json:"seller_name,omitempty"`
		SalesCount   int    `json:"sales_count"`
		// PendingCents is what is still owed — the number about to be
		// transferred. PaidCents is what already left.
		PendingCents    int64    `json:"pending_cents"`
		PaidCents       int64    `json:"paid_cents"`
		CommissionCents int64    `json:"commission_cents"`
		MilestoneCents  int64    `json:"milestone_cents"`
		AdjustmentCents int64    `json:"adjustment_cents"`
		Warnings        []string `json:"warnings,omitempty"`
	}
	type cycleOut struct {
		Cycle             string         `json:"cycle"`
		Statements        []statementOut `json:"statements"`
		TotalPendingCents int64          `json:"total_pending_cents"`
	}

	collect := func(cycle string, list []*domain.CommissionStatement) *cycleOut {
		out := &cycleOut{Cycle: cycle, Statements: make([]statementOut, 0, len(list))}
		for _, st := range list {
			out.TotalPendingCents += st.PendingCents
			out.Statements = append(out.Statements, statementOut{
				SellerUserID: st.SellerUserID, SellerName: st.SellerName,
				SalesCount: st.SalesCount, PendingCents: st.PendingCents,
				PaidCents: st.PaidCents, CommissionCents: st.CommissionCents,
				MilestoneCents: st.MilestoneCents, AdjustmentCents: st.AdjustmentCents,
				Warnings: st.Warnings,
			})
		}
		return out
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_commission_statement",
		Description: "Extrato de comissão de um mês (AAAA-MM, padrão o mês atual): quanto cada vendedor " +
			"tem a receber, o que já foi pago, e os avisos a conferir antes de pagar. " +
			"Use recompute=true para refazer o cálculo a partir das vendas e das regras — " +
			"lançamentos já pagos NUNCA são reescritos; diferença vira ajuste visível.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		Cycle        string `json:"cycle,omitempty" jsonschema:"mês no formato AAAA-MM (padrão: mês atual da conta)"`
		SellerUserID string `json:"seller_user_id,omitempty" jsonschema:"id de um vendedor específico"`
		Recompute    bool   `json:"recompute,omitempty" jsonschema:"refaz o cálculo do ciclo antes de responder"`
	}) (*mcpsdk.CallToolResult, *cycleOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		cycle := in.Cycle
		if cycle == "" {
			cycle = deps.CommissionStmtUC.CurrentCycle(ctx, id.AccountID)
		}

		var list []*domain.CommissionStatement
		var err error
		if in.Recompute {
			list, err = deps.CommissionStmtUC.Recompute(ctx, id.AccountID, cycle)
			if err == nil && in.SellerUserID != "" {
				list, err = deps.CommissionStmtUC.List(ctx, id.AccountID, cycle, in.SellerUserID)
			}
		} else {
			list, err = deps.CommissionStmtUC.List(ctx, id.AccountID, cycle, in.SellerUserID)
		}
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, collect(cycle, list), nil
	})

	type payOut struct {
		Cycle        string `json:"cycle"`
		SellerUserID string `json:"seller_user_id"`
		PaidCents    int64  `json:"paid_cents"`
		PendingCents int64  `json:"pending_cents"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "mark_commission_paid",
		Description: "Marca a comissão de um vendedor num ciclo como PAGA. O pagamento em si acontece " +
			"fora do sistema (PIX/TED) — isto registra que saiu, e a partir daí o valor não é " +
			"mais recalculado. Mostre o valor e o nome do vendedor e peça confirmação antes de " +
			"chamar. Se o mês ainda não terminou, é preciso allow_open_cycle=true: vendas " +
			"depois do pagamento vão gerar novos lançamentos.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		SellerUserID   string `json:"seller_user_id" jsonschema:"id do vendedor"`
		Cycle          string `json:"cycle,omitempty" jsonschema:"mês no formato AAAA-MM (padrão: mês atual)"`
		Note           string `json:"note,omitempty" jsonschema:"como foi pago (ex.: PIX 12/08)"`
		AllowOpenCycle bool   `json:"allow_open_cycle,omitempty" jsonschema:"confirma adiantar comissão de um mês que não terminou"`
	}) (*mcpsdk.CallToolResult, *payOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		cycle := in.Cycle
		if cycle == "" {
			cycle = deps.CommissionStmtUC.CurrentCycle(ctx, id.AccountID)
		}
		st, err := deps.CommissionStmtUC.MarkPaid(ctx, id.AccountID, cycle,
			in.SellerUserID, in.Note, in.AllowOpenCycle)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &payOut{
			Cycle: cycle, SellerUserID: st.SellerUserID,
			PaidCents: st.PaidCents, PendingCents: st.PendingCents,
		}, nil
	})
}
