package mcp

import (
	"errors"
	"fmt"

	"github.com/replider/api/internal/core/domain"
)

// toolError converts a domain error into a Claude-friendly pt-BR error.
// Returned errors from ToolHandlerFor become IsError tool results (not
// protocol errors), so Claude can read them and recover — e.g. suggest
// the next step when the WhatsApp 24h window is closed.
func toolError(err error) error {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		if msg, ok := toolErrorMessages[appErr.Code]; ok {
			return errors.New(msg)
		}
		return fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
	}
	return fmt.Errorf("a operação falhou: %v", err)
}

// toolErrorMessages translates the most actionable domain error codes
// into instructions Claude can act on, in pt-BR.
var toolErrorMessages = map[string]string{
	"CONV_NOT_FOUND":       "Conversa não encontrada. Confira o conversation_id com list_conversations.",
	"CONV_NOT_ASSUMED":     "A conversa ainda está com o bot. Chame assume_conversation antes de enviar mensagem humana.",
	"CONV_OUTSIDE_24H":     "A janela de 24h do WhatsApp/Instagram fechou — não é possível enviar mensagem livre agora. O cliente precisa mandar uma nova mensagem primeiro (ou use um template pelo dashboard).",
	"AGENT_NOT_FOUND":      "Agente não encontrado. Use list_agents para ver os agentes disponíveis.",
	"CHANNEL_NOT_FOUND":    "Canal não encontrado. Use list_channels para ver os canais disponíveis.",
	"PLAN_LIMIT_EXCEEDED":  "Limite do plano atingido para este recurso. Considere sugerir um upgrade (get_billing).",
	"USAGE_LIMIT_EXCEEDED": "O limite de uso do plano foi atingido neste ciclo.",
	"MESSAGE_SEND_FAILED":  "A mensagem foi registrada mas o envio ao canal falhou. Informe o usuário e sugira tentar novamente.",
	"AUTH_UNAUTHORIZED":    "Sem permissão para esta operação com o papel atual.",
	"VALIDATION_ERROR":     "Dados inválidos para esta operação. Revise os parâmetros.",
	"FORBIDDEN":            "Esta operação exige papel de administrador.",

	// Outbound-sales pipeline
	"FUNNEL_NOT_FOUND":           "Funil não encontrado. Use list_funnels para ver os funis da conta.",
	"SALES_FUNNEL_NOT_FOUND":     "Funil não encontrado. Use list_funnels para ver os funis da conta.",
	"LEAD_INVALID_PHONE":         "Telefone inválido — informe em formato E.164, ex.: +5565999998888.",
	"LEAD_FIELD_NOT_IN_SCHEMA":   "Campo de metadata não declarado no metadata_schema do funil — confira o funil com list_funnels ou ajuste com upsert_funnel.",
	"LEAD_NOT_FOUND":             "Lead não encontrado. Use list_funnel_leads para localizar o lead.",
	"LEAD_NOT_PENDING_APPROVAL":  "O lead não está aguardando aprovação — gere um rascunho antes com draft_lead_approach.",
	"CONTACT_OPTED_OUT":          "Este contato pediu para não receber mensagens (opt-out). O envio é proibido.",
	"PROSPECTING_NOT_CONFIGURED": "A busca de empresas (Google Places) não está configurada nesta instalação.",
	"ENRICHMENT_REQUIRES_GEMINI": "O enriquecimento de leads exige provedor Gemini configurado na conta.",
	"BROADCAST_BUDGET_EXCEEDED":  "O orçamento de mensagens ativas do plano foi atingido neste ciclo. Considere sugerir upgrade (get_billing).",
	"TEMPLATE_MISSING_OPT_OUT":   "Template de MARKETING sem botão de descadastro (ex.: 'Parar de receber') — a Meta exige. Ajuste o template no canal.",
}

// errNoIdentity is returned when the request context lost the
// authenticated identity — a wiring bug, not a user mistake.
var errNoIdentity = errors.New("sessão não autenticada — reconecte o Replider ao Claude")
