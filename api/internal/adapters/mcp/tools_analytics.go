package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

func registerAnalyticsTools(s *mcpsdk.Server, deps Deps) {
	type analyticsInput struct {
		Report string `json:"report" jsonschema:"tipo de relatório: overview | sdr | expensive_conversations | attendant_performance | live_supervision"`
		Days   int    `json:"days,omitempty" jsonschema:"janela em dias para o overview: 7, 30 ou 90 (padrão 30)"`
		Period string `json:"period,omitempty" jsonschema:"período do relatório sdr: today | 7d | 30d | mtd (padrão 7d)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_analytics",
		Description: "Métricas de atendimento e de vendas. overview: volume, % resolvido pela IA, tempo " +
			"de primeira resposta, CSAT, canais e perguntas mais comuns. sdr: o funil de conversão do " +
			"período (coorte de leads que entraram, seguidos até onde chegaram) mais os cortes por " +
			"estratégia, por vendedor, por canal (com o custo de IA ao lado) e por sinal público. " +
			"expensive_conversations: top-10 conversas por custo de IA. attendant_performance: desempenho " +
			"por atendente. live_supervision: carga da equipe agora. " +
			"No relatório sdr, taxas sem denominador vêm AUSENTES em vez de 0% — não as trate como zero, " +
			"e o custo por canal cobre só a IA (a tarifa da operadora/Meta é cobrada na conta do próprio " +
			"cliente e a gente não vê essa fatura).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in analyticsInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		switch in.Report {
		case "overview", "":
			days := in.Days
			if days == 0 {
				days = 30
			}
			out, err := deps.OverviewUC.Execute(ctx, id.AccountID, days)
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, out, nil
		case "sdr":
			if deps.SDRAnalyticsUC == nil {
				return nil, nil, fmt.Errorf("relatório sdr não está disponível nesta instalação")
			}
			out, err := deps.SDRAnalyticsUC.Execute(ctx, id.AccountID, in.Period)
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, out, nil
		case "expensive_conversations":
			out, err := deps.ExpensiveConvUC.Execute(ctx, id.AccountID)
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, out, nil
		case "attendant_performance":
			out, err := deps.AttendantPerfUC.Execute(ctx, id.AccountID, id.UserID, usecases.GetAttendantPerformanceDTO{
				Period: usecases.PeriodLast7d,
			})
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, out, nil
		case "live_supervision":
			out, err := deps.LiveSupUC.Execute(ctx, id.AccountID, id.UserID)
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, out, nil
		default:
			return nil, nil, fmt.Errorf("report inválido %q — use overview, sdr, expensive_conversations, attendant_performance ou live_supervision", in.Report)
		}
	})

	registerSDRDrillTool(s, deps)
}

// registerSDRDrillTool is the "abre esse número" half of the SDR report.
//
// A report the copilot can only quote is a report it cannot help with. This is
// what lets it answer "quais são os 12 que responderam e ninguém retomou?" with
// names instead of a number.
func registerSDRDrillTool(s *mcpsdk.Server, deps Deps) {
	if deps.SDRAnalyticsUC == nil {
		return
	}
	type drillInput struct {
		Metric       string `json:"metric" jsonschema:"etapa do funil: entered | researched | approached | replied | qualified | won"`
		Period       string `json:"period,omitempty" jsonschema:"today | 7d | 30d | mtd (padrão 7d)"`
		StrategyID   string `json:"strategy_id,omitempty" jsonschema:"restringe a uma estratégia"`
		SellerUserID string `json:"seller_user_id,omitempty" jsonschema:"restringe a um vendedor; use 'none' para os leads sem dono"`
		Channel      string `json:"channel,omitempty" jsonschema:"restringe a quem foi abordado por este canal (whatsapp, telegram, sms…)"`
		SignalSource string `json:"signal_source,omitempty" jsonschema:"restringe a leads originados deste sinal público (pncp…)"`
		Limit        int    `json:"limit,omitempty" jsonschema:"quantos leads listar (padrão 50, máximo 200)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_sdr_metric_leads",
		Description: "Lista os leads por trás de um número do relatório sdr — a mesma janela e os mesmos " +
			"critérios que produziram a contagem, então a lista tem exatamente o tamanho do número que a " +
			"originou. Use para transformar 'responderam: 12' em quem são os 12.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in drillInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		limit := in.Limit
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		leads, period, err := deps.SDRAnalyticsUC.Drill(ctx, domain.SDRDrillFilter{
			AccountID:    id.AccountID,
			Metric:       strings.TrimSpace(in.Metric),
			StrategyID:   strings.TrimSpace(in.StrategyID),
			SellerUserID: strings.TrimSpace(in.SellerUserID),
			Channel:      strings.TrimSpace(in.Channel),
			SignalSource: strings.TrimSpace(in.SignalSource),
			Limit:        int32(limit), //nolint:gosec // G115: bounded to [1,200] above
		}, in.Period)
		if err != nil {
			return nil, nil, toolError(err)
		}
		// The window travels with the rows: a list quoted without the period
		// it covers is a list somebody will compare against last week's
		// number.
		return nil, map[string]any{
			"metric": strings.TrimSpace(in.Metric),
			"period": period,
			"leads":  leads,
		}, nil
	})
}

// registerCaseReportTool exposes the prospecting case report (US-M012).
//
// Same numbers, same honesty as the HTML page: an average over zero replies
// stays absent rather than becoming "0s", and the work-avoided estimate is
// returned WITH the assumption that produced it, so the copilot can quote
// the arithmetic instead of a bare claim.
func registerCaseReportTool(s *mcpsdk.Server, deps Deps) {
	if deps.CaseMetricsUC == nil {
		return
	}
	type caseInput struct {
		Days               int `json:"days,omitempty" jsonschema:"janela em dias (padrão 30). O período anterior de mesmo tamanho entra como comparativo."`
		MinutesPerApproach int `json:"minutes_per_approach,omitempty" jsonschema:"minutos de trabalho manual por abordagem usados na estimativa (padrão 8)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_case_report",
		Description: "Relatório de prospecção do período: leads gerados por origem, abordagens enviadas " +
			"por canal, respostas, reuniões, taxa de resposta e tempo médio de resposta — mais o período " +
			"anterior de mesmo tamanho como comparativo e uma estimativa (com a premissa explícita) do " +
			"trabalho manual evitado. Métricas sem base de cálculo vêm ausentes, nunca como zero.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in caseInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		days := in.Days
		if days <= 0 {
			days = 30
		}
		to := time.Now()
		from := to.AddDate(0, 0, -days)
		out, err := deps.CaseMetricsUC.Report(ctx, id.AccountID, from, to, in.MinutesPerApproach)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, out, nil
	})
}
