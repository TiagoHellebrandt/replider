package mcp

import (
	"context"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Command centre tools (§6.13, I2).
//
// The copilot's answer to "por onde começo?". It reads the same ranked list the
// home screen shows, so the two can never give different advice on the same
// morning.
func registerCommandCenterTools(s *mcpsdk.Server, deps Deps) {
	if deps.CommandCenterUC == nil {
		return
	}

	type pendingInput struct{}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_pending_actions",
		Description: "O que precisa de uma pessoa agora, na MESMA ordem de impacto da tela de início: " +
			"prazo vencido primeiro, depois por valor em jogo, e entre os sem valor os mais antigos. " +
			"Cada item traz o NOME do cliente, o valor quando existe e o link de onde a ação acontece. " +
			"Vem junto: os quatro números do período (leads ativos, esperando aprovação, reuniões, " +
			"ganhos) e o mini-funil. " +
			"Se `warnings` vier preenchido, alguma seção falhou — diga isso em vez de tratar a lista " +
			"como completa. `total_actions` maior que os itens retornados significa que há mais.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ pendingInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		out, err := deps.CommandCenterUC.Execute(ctx, id.Actor())
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, out, nil
	})

	type dismissInput struct {
		Kind        string `json:"kind" jsonschema:"tipo da ação: approve_draft | unowned_lead | signature_viewed | contract_alert | confirm_sale | reorder_due"`
		TargetID    string `json:"target_id" jsonschema:"o id que veio no item da lista"`
		SnoozeHours int    `json:"snooze_hours,omitempty" jsonschema:"por quantas horas adiar (padrão 72, máximo 720)"`
		Note        string `json:"note,omitempty" jsonschema:"por que foi adiado"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "snooze_pending_action",
		Description: "Adia um item de 'precisa de você' por um tempo, só para o usuário atual. " +
			"NÃO marca como feito e não existe adiar para sempre: os itens são derivados do estado " +
			"real, então o que faz a linha desaparecer de vez é o trabalho ser feito. Use quando a " +
			"pessoa disser explicitamente que aquele item não é para agora — nunca para 'limpar' a lista.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in dismissInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		dismissal := &domain.PendingActionDismissal{
			Kind:     domain.PendingActionKind(strings.TrimSpace(in.Kind)),
			TargetID: strings.TrimSpace(in.TargetID),
			Note:     strings.TrimSpace(in.Note),
		}
		if in.SnoozeHours > 0 {
			dismissal.SnoozedUntil = deps.CommandCenterUC.SnoozeFrom(in.SnoozeHours)
		}
		if err := deps.CommandCenterUC.Dismiss(ctx, id.Actor(), dismissal); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, map[string]any{
			"snoozed_until": dismissal.SnoozedUntil,
			// Said back explicitly so the model does not report the item as
			// resolved to the user.
			"completed": false,
		}, nil
	})
}
