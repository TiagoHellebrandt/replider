package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/usecases"
)

type knowledgeSourceSummary struct {
	ID          string `json:"id"`
	AgentID     string `json:"agent_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type listKnowledgeInput struct {
	AgentID string `json:"agent_id,omitempty" jsonschema:"filtra por agente; vazio lista de todos os agentes"`
}

type listKnowledgeOutput struct {
	Sources []knowledgeSourceSummary `json:"sources"`
}

func registerKnowledgeTools(s *mcpsdk.Server, deps Deps, admin bool) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_knowledge_sources",
		Description: "Lista as fontes da base de conhecimento (o material que o agente usa para responder via RAG).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listKnowledgeInput) (*mcpsdk.CallToolResult, *listKnowledgeOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		agentIDs := []string{}
		if in.AgentID != "" {
			agentIDs = append(agentIDs, in.AgentID)
		} else {
			agents, err := deps.ListAgentsUC.Execute(ctx, id.AccountID)
			if err != nil {
				return nil, nil, toolError(err)
			}
			for _, a := range agents {
				agentIDs = append(agentIDs, a.ID)
			}
		}
		out := &listKnowledgeOutput{Sources: []knowledgeSourceSummary{}}
		for _, agentID := range agentIDs {
			sources, err := deps.KnowledgeUC.ListSources(ctx, id.AccountID, agentID)
			if err != nil {
				return nil, nil, toolError(err)
			}
			for _, src := range sources {
				summary := knowledgeSourceSummary{ID: src.ID, AgentID: src.AgentID, Name: src.Name}
				if src.Description != nil {
					summary.Description = *src.Description
				}
				out.Sources = append(out.Sources, summary)
			}
		}
		return nil, out, nil
	})

	if !admin {
		return
	}

	type addKnowledgeInput struct {
		AgentID string `json:"agent_id"`
		Type    string `json:"type" jsonschema:"origem do conteúdo: text ou url"`
		Title   string `json:"title" jsonschema:"nome da fonte, ex.: FAQ, Política de trocas"`
		Content string `json:"content,omitempty" jsonschema:"conteúdo em texto (obrigatório quando type=text)"`
		URL     string `json:"url,omitempty" jsonschema:"página a importar (obrigatório quando type=url)"`
	}
	type addKnowledgeOutput struct {
		SourceID string `json:"source_id"`
		Chunks   int    `json:"chunks"`
		Message  string `json:"message"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "add_knowledge",
		Description: "Adiciona conteúdo à base de conhecimento de um agente — texto direto (FAQ, políticas, " +
			"catálogo) ou importado de uma URL. O agente passa a usar esse material nas respostas.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in addKnowledgeInput) (*mcpsdk.CallToolResult, *addKnowledgeOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		switch in.Type {
		case "url":
			result, err := deps.KnowledgeUC.CreateSourceFromURL(ctx, usecases.CreateSourceFromURLDTO{
				AccountID: id.AccountID,
				AgentID:   in.AgentID,
				URL:       in.URL,
				Name:      in.Title,
			})
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, &addKnowledgeOutput{
				SourceID: result.Source.ID,
				Chunks:   result.ChunksCreated,
				Message:  "Conteúdo importado da URL e indexado.",
			}, nil
		default: // "text"
			source, err := deps.KnowledgeUC.CreateSource(ctx, usecases.CreateSourceDTO{
				AccountID: id.AccountID,
				AgentID:   in.AgentID,
				Name:      in.Title,
			})
			if err != nil {
				return nil, nil, toolError(err)
			}
			if _, err := deps.KnowledgeUC.Ingest(ctx, usecases.IngestKnowledgeDTO{
				AccountID: id.AccountID,
				SourceID:  source.ID,
				Content:   in.Content,
			}); err != nil {
				return nil, nil, toolError(err)
			}
			return nil, &addKnowledgeOutput{
				SourceID: source.ID,
				Chunks:   1,
				Message:  "Conteúdo adicionado e indexado.",
			}, nil
		}
	})
}
