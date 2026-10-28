package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// agentSummary is a lean, schema-friendly projection of domain.Agent.
type agentSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Active       bool   `json:"active"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
}

func toAgentSummary(a *domain.Agent) agentSummary {
	out := agentSummary{
		ID:       a.ID,
		Name:     a.Name,
		Active:   a.IsActive,
		Provider: a.Provider,
		Model:    a.Model,
	}
	if a.SystemPrompt != nil {
		out.SystemPrompt = *a.SystemPrompt
	}
	return out
}

type listAgentsOutput struct {
	Agents []agentSummary `json:"agents"`
}

// findAgent resolves one of the account's agents by id (tenant-scoped
// via ListAgentsUC, which filters by account).
func findAgent(ctx context.Context, deps Deps, accountID, agentID string) (*domain.Agent, error) {
	agents, err := deps.ListAgentsUC.Execute(ctx, accountID)
	if err != nil {
		return nil, toolError(err)
	}
	for _, a := range agents {
		if a.ID == agentID {
			return a, nil
		}
	}
	return nil, toolError(domain.ErrAgentNotFound)
}

func registerAgentTools(s *mcpsdk.Server, deps Deps, admin bool) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_agents",
		Description: "Lista os agentes de IA da conta, com status e o prompt de sistema completo de cada um.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, *listAgentsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		agents, err := deps.ListAgentsUC.Execute(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listAgentsOutput{Agents: make([]agentSummary, 0, len(agents))}
		for _, a := range agents {
			out.Agents = append(out.Agents, toAgentSummary(a))
		}
		return nil, out, nil
	})

	if !admin {
		return
	}

	type createAgentInput struct {
		Name         string `json:"name" jsonschema:"nome do agente"`
		SystemPrompt string `json:"system_prompt" jsonschema:"prompt de sistema: negócio, tom de voz, escopo, quando escalar para humano"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_agent",
		Description: "Cria um novo agente de IA. Um bom prompt descreve o negócio, produtos, tom de voz, o que não responder e quando transferir para um humano.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in createAgentInput) (*mcpsdk.CallToolResult, *agentSummary, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		agent, err := deps.CreateAgentUC.Execute(ctx, usecases.CreateAgentDTO{
			AccountID:    id.AccountID,
			Name:         in.Name,
			SystemPrompt: &in.SystemPrompt,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := toAgentSummary(agent)
		return nil, &out, nil
	})

	type updateAgentInput struct {
		AgentID      string  `json:"agent_id"`
		Name         *string `json:"name,omitempty" jsonschema:"novo nome (opcional)"`
		SystemPrompt *string `json:"system_prompt,omitempty" jsonschema:"novo prompt de sistema completo (opcional)"`
		Provider     *string `json:"provider,omitempty" jsonschema:"provedor de IA: openai ou google (opcional)"`
		Model        *string `json:"model,omitempty" jsonschema:"modelo do provedor, ex.: gemini-3.7-flash, gpt-4o (opcional)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "update_agent",
		Description: "Atualiza um agente: nome, prompt de sistema, provedor e/ou modelo de IA. Campos omitidos não mudam. " +
			"Ao melhorar um prompt, mostre o texto novo ao usuário e obtenha aprovação antes de aplicar.",
		Annotations: &mcpsdk.ToolAnnotations{IdempotentHint: true},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in updateAgentInput) (*mcpsdk.CallToolResult, *agentSummary, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if in.Provider != nil && *in.Provider != "openai" && *in.Provider != "google" {
			return nil, nil, toolError(&domain.AppError{Code: "VALIDATION_ERROR", Message: "provider deve ser openai ou google"})
		}
		// UpdateAgentUseCase overwrites Name/SystemPrompt unconditionally,
		// so omitted fields must be backfilled with the current values.
		// Provider/Model are keep-when-empty in the use case.
		current, err := findAgent(ctx, deps, id.AccountID, in.AgentID)
		if err != nil {
			return nil, nil, err
		}
		dto := usecases.UpdateAgentDTO{
			ID:           in.AgentID,
			AccountID:    id.AccountID,
			Name:         current.Name,
			SystemPrompt: current.SystemPrompt,
		}
		if in.Name != nil {
			dto.Name = *in.Name
		}
		if in.SystemPrompt != nil {
			dto.SystemPrompt = in.SystemPrompt
		}
		if in.Provider != nil {
			dto.Provider = *in.Provider
		}
		if in.Model != nil {
			dto.Model = *in.Model
		}
		agent, err := deps.UpdateAgentUC.Execute(ctx, dto)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := toAgentSummary(agent)
		return nil, &out, nil
	})

	type toggleAgentInput struct {
		AgentID string `json:"agent_id"`
		Active  bool   `json:"active" jsonschema:"true para ativar, false para pausar"`
	}
	type okOutput struct {
		OK bool `json:"ok"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "toggle_agent",
		Description: "Ativa ou pausa um agente de IA. Um agente pausado não responde conversas automaticamente.",
		Annotations: &mcpsdk.ToolAnnotations{IdempotentHint: true},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in toggleAgentInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if err := deps.ToggleAgentUC.Execute(ctx, id.AccountID, in.AgentID, in.Active); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true}, nil
	})

	type deleteAgentInput struct {
		AgentID string `json:"agent_id"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "delete_agent",
		Description: "EXCLUI PERMANENTEMENTE um agente (canais ligados a ele ficam sem agente; a base de " +
			"conhecimento dele é removida). Sempre confirme com o usuário antes de chamar.",
		Annotations: &mcpsdk.ToolAnnotations{DestructiveHint: boolPtr(true)},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in deleteAgentInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if err := deps.DeleteAgentUC.Execute(ctx, id.AccountID, in.AgentID); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true}, nil
	})

	type previewChatInput struct {
		AgentID string `json:"agent_id"`
		Message string `json:"message" jsonschema:"mensagem simulada de um cliente"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "preview_agent_chat",
		Description: "Testa o agente com uma mensagem simulada de cliente, sem criar conversa real. " +
			"Consome tokens do plano. Use para validar mudanças de prompt ou de base de conhecimento.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in previewChatInput) (*mcpsdk.CallToolResult, *usecases.PreviewChatResponse, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		resp, err := deps.PreviewChatUC.Execute(ctx, usecases.PreviewChatDTO{
			AccountID: id.AccountID,
			AgentID:   in.AgentID,
			Message:   in.Message,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, resp, nil
	})
}
