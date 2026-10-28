package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Kanban tools (US-M008).
//
// Parity is structural, not aspirational: these call the SAME use case the
// HTTP endpoints do, so the ownership scoping from US-F009 and the
// stage↔status coupling apply identically. An agent cannot move a card the
// logged-in user could not move by hand.

type getFunnelStagesInput struct {
	FunnelID string `json:"funnel_id" jsonschema:"ID do funil"`
}

type stageOut struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Position     int    `json:"position"`
	MapsToStatus string `json:"maps_to_status" jsonschema:"status canônico que esta coluna representa"`
	IsTerminal   bool   `json:"is_terminal" jsonschema:"true para colunas de fim de funil (ganho/perdido)"`
	LeadCount    int    `json:"lead_count"`
}

type getFunnelStagesOutput struct {
	Stages []stageOut `json:"stages"`
}

type moveLeadInput struct {
	LeadID  string `json:"lead_id"`
	StageID string `json:"stage_id" jsonschema:"ID da coluna destino — use get_funnel_stages para descobrir"`
}

type moveLeadOutput struct {
	LeadID  string `json:"lead_id"`
	StageID string `json:"stage_id"`
	// Status is echoed because moving a card CHANGES it — an agent that
	// only tracked the column would report a stale pipeline state.
	Status string `json:"status"`
}

func registerKanbanTools(s *mcpsdk.Server, deps Deps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_funnel_stages",
		Description: "Lista as colunas (etapas) do Kanban de um funil, com a contagem de leads em cada uma. " +
			"As colunas são definidas pelo cliente e mapeiam para os status canônicos do pipeline. " +
			"Use antes de move_lead para descobrir o stage_id de destino.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in getFunnelStagesInput) (*mcpsdk.CallToolResult, *getFunnelStagesOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		board, err := deps.LeadUC.Board(ctx, id.Actor(), in.FunnelID, 500, 0)
		if err != nil {
			return nil, nil, toolError(err)
		}

		counts := map[string]int{}
		for _, card := range board.Cards {
			counts[card.StageID]++
		}
		out := &getFunnelStagesOutput{Stages: make([]stageOut, 0, len(board.Stages))}
		for _, st := range board.Stages {
			out.Stages = append(out.Stages, stageOut{
				ID:           st.ID,
				Name:         st.Name,
				Position:     st.Position,
				MapsToStatus: string(st.MapsToStatus),
				IsTerminal:   st.IsTerminal,
				LeadCount:    counts[st.ID],
			})
		}
		return nil, out, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "move_lead",
		Description: "Move um lead para outra coluna do Kanban. Isso TAMBÉM altera o status canônico do lead " +
			"(a coluna define o status), então mover para uma coluna de 'ganho'/'perdido' encerra o lead no funil. " +
			"Um vendedor só move os próprios leads. NÃO envia nenhuma mensagem ao cliente.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in moveLeadInput) (*mcpsdk.CallToolResult, *moveLeadOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		// No drop-slot neighbors over MCP: the card lands in the column's
		// automatic order (manual ranking is a board gesture, not an agent's).
		lead, err := deps.LeadUC.MoveLead(ctx, id.Actor(), in.LeadID, in.StageID, "", "")
		if err != nil {
			return nil, nil, toolError(err)
		}
		stageID := ""
		if lead.StageID != nil {
			stageID = *lead.StageID
		}
		return nil, &moveLeadOutput{
			LeadID:  lead.ID,
			StageID: stageID,
			Status:  string(lead.Status),
		}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_lead",
		Description: "Retorna um lead pelo ID: status, etapa, dados coletados, enrichment e rascunho de abordagem. " +
			"Respeita a visibilidade: um vendedor não lê o lead privado de um colega.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		LeadID string `json:"lead_id"`
	}) (*mcpsdk.CallToolResult, *domain.Lead, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		lead, err := deps.LeadUC.GetForActor(ctx, id.Actor(), in.LeadID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, lead, nil
	})
}
