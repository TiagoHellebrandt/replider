package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/usecases"
)

type emptyInput struct{}

func registerAccountTools(s *mcpsdk.Server, deps Deps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_account_health",
		Description: "Relatório de saúde da conta Replider: se o ciclo de vendas pode rodar (território " +
			"definido, catálogo e clientes importados, e-mail conectado, fila de aprovação em dia, " +
			"histórico de vendas), uso vs limites do plano e uma lista de issues — com fix_tool quando " +
			"existe ferramenta, e com a tela do app quando não existe. Chame esta ferramenta PRIMEIRO, " +
			"no início de cada conversa.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, *usecases.AccountHealthReport, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		report, err := deps.HealthReportUC.Execute(ctx, id.Actor())
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, report, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_usage",
		Description: "Consumo do ciclo atual: conversas e tokens de IA usados, limites do plano, " +
			"percentuais de uso e excedente (overage).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, *usecases.GetUsageResponse, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		usage, err := deps.GetUsageUC.Execute(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, usage, nil
	})
}
