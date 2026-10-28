package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerPrompts adds the slash-prompt templates surfaced by MCP
// clients (claude.ai "/" menu, Claude Code /mcp__replider__*).
//
// seller gates the SDR prompts: they name pipeline tools the attendant
// server does not list. legacy re-adds the v1 customer-service prompts
// alongside the v1 tools they drive.
func registerPrompts(s *mcpsdk.Server, seller, legacy bool) {
	textPrompt := func(description, text string) *mcpsdk.GetPromptResult {
		return &mcpsdk.GetPromptResult{
			Description: description,
			Messages: []*mcpsdk.PromptMessage{{
				Role:    "user",
				Content: &mcpsdk.TextContent{Text: text},
			}},
		}
	}

	if seller {
		s.AddPrompt(&mcpsdk.Prompt{
			Name:        "revisar-fila-de-hoje",
			Description: "Revisa as ofertas aguardando aprovação, uma a uma, e o que mais precisa de você hoje.",
		}, func(_ context.Context, _ *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
			return textPrompt("Revisão da fila de hoje",
				"Revise minha fila de hoje. Comece com get_account_health e get_pending_actions. "+
					"Depois liste as ofertas em pending_approval (list_funnel_leads com status=pending_approval, "+
					"ou get_lead para cada item da fila) e me apresente UMA por vez: o hospital, o que "+
					"sabemos dele (parque instalado, sinal público, histórico na carteira), o assunto e o "+
					"texto exato do e-mail. Para cada uma, pergunte se aprovo, ajusto ou descarto — só chame "+
					"approve_send_approach depois da minha confirmação explícita, com o texto final. Termine "+
					"com o que ficou pendente e o que sugere para amanhã."), nil
		})

		s.AddPrompt(&mcpsdk.Prompt{
			Name:        "preparar-oferta",
			Description: "Monta a oferta certa para um hospital: parque instalado, histórico, produto e rascunho para aprovação.",
			Arguments: []*mcpsdk.PromptArgument{{
				Name:        "hospital",
				Description: "Nome do hospital ou clínica (ou o lead_id, se já for um lead)",
				Required:    true,
			}},
		}, func(_ context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
			hospital := "o hospital que eu indicar"
			if req.Params != nil && req.Params.Arguments["hospital"] != "" {
				hospital = req.Params.Arguments["hospital"]
			}
			return textPrompt("Preparar oferta para "+hospital,
				"Prepare uma oferta para "+hospital+". Passos: 1) descubra o que ele tem com "+
					"get_installed_park (o CNES diz quantidade, nunca idade — não invente desgaste) e se há "+
					"licitação aberta dele em list_signals; 2) veja se já é cliente na carteira "+
					"(list_portfolio, upcoming_reorders, suggest_cross_sell) e se já existe lead "+
					"(list_funnel_leads / get_lead); 3) me diga qual produto do catálogo faz sentido e por quê, "+
					"em 3 linhas; 4) com meu OK, gere o rascunho com draft_lead_approach (crie o lead antes se "+
					"não existir) e me mostre o assunto e o texto exato. NÃO envie: o envio é uma decisão "+
					"minha, na fila de aprovação."), nil
		})

		s.AddPrompt(&mcpsdk.Prompt{
			Name:        "o-que-mudou-na-carteira",
			Description: "Resumo do que aconteceu na carteira e no funil desde a última revisão, com as próximas ações.",
		}, func(_ context.Context, _ *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
			return textPrompt("O que mudou na carteira",
				"Me diga o que mudou na minha carteira e no funil. Use get_pending_actions, list_inbox "+
					"(respostas de hospitais que ninguém retomou), upcoming_reorders (quem está perto de "+
					"recomprar), list_signals (licitações novas com prazo) e list_relationship_dates (datas "+
					"que vencem esta semana). Se eu for gestor, use get_analytics com report=sdr para o funil "+
					"do período. Produza um resumo curto em português: o que avançou, o que esfriou, quem "+
					"respondeu e está esperando, e as 3 ações de maior impacto para hoje — cada uma com a "+
					"ferramenta que você pode executar por mim (sem enviar nada sem minha aprovação)."), nil
		})
	}

	if !legacy {
		return
	}

	// ── v1 customer-service prompts (MCP_LEGACY_TOOLS=true) ────────────────

	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "analisar-atendimento-semanal",
		Description: "Resumo executivo do atendimento dos últimos 7 dias com as 3 recomendações de maior impacto.",
	}, func(_ context.Context, _ *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
		return textPrompt("Análise semanal de atendimento",
			"Analise meu atendimento da última semana. Use get_analytics (overview com days=7"+
				", attendant_performance e expensive_conversations) e get_usage. Produza um resumo "+
				"executivo curto em português: volume, % resolvido pela IA, tempo de primeira resposta, "+
				"CSAT e custo. Termine com as 3 recomendações de maior impacto, ordenadas, cada uma com a "+
				"ação concreta que você pode executar por mim."), nil
	})

	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "melhorar-prompt-do-agente",
		Description: "Avalia e melhora o prompt de sistema de um agente, com validação via chat de teste.",
		Arguments: []*mcpsdk.PromptArgument{{
			Name:        "agent_id",
			Description: "Agente a melhorar (vazio: escolher o mais fraco pelo relatório de saúde)",
			Required:    false,
		}},
	}, func(_ context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
		agentRef := "o agente indicado como mais fraco no get_account_health"
		if req.Params != nil && req.Params.Arguments["agent_id"] != "" {
			agentRef = "o agente " + req.Params.Arguments["agent_id"]
		}
		return textPrompt("Melhoria de prompt de agente",
			"Melhore o prompt de sistema de "+agentRef+". Passos: 1) leia o prompt atual com list_agents; "+
				"2) avalie contra o checklist — persona e tom de voz, descrição do negócio e produtos, "+
				"escopo (o que não responder), instruções de escalonamento para humano, idioma; "+
				"3) proponha um prompt novo completo e me mostre; 4) só depois da minha aprovação aplique "+
				"com update_agent; 5) valide com preview_agent_chat simulando 2 perguntas típicas de cliente "+
				"e me mostre as respostas."), nil
	})

	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "configurar-conta",
		Description: "Setup guiado da conta: agente, canal, base de conhecimento e teste, passo a passo.",
	}, func(_ context.Context, _ *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
		return textPrompt("Setup guiado da conta",
			"Me guie na configuração da minha conta Replider, um passo por vez. Comece com "+
				"get_account_health para ver o que falta. Na ordem que fizer sentido: criar/ajustar o "+
				"agente de IA (me pergunte sobre o negócio antes de escrever o prompt), conectar um canal "+
				"(Telegram dá pra fazer por aqui; WhatsApp oriente pelo dashboard), adicionar conteúdo à "+
				"base de conhecimento (me peça o FAQ ou o site), e por fim testar com preview_agent_chat. "+
				"Confirme comigo antes de cada ação de escrita."), nil
	})
}
