package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// SmartReplier asks an LLM for 3 short reply candidates the human
// agent can pick from. Saves operators 5-10s of typing on each
// reply at the cost of one cheap LLM call. Different from the
// AgentRunner: those are full bot replies with RAG + tool calling;
// these are quick suggestions for a human, ranked by likely fit.
//
// We deliberately bound the output: max 3 candidates, max ~120 chars
// each, plain text only. Long suggestions defeat the purpose (operator
// would rather type their own) and rich formatting confuses the
// channel adapter.
type SmartReplier struct {
	aiFactory domain.AIFactory
	provider  string // "google" / "openai"; defaults to google to keep ¢ low
	scope     accountAIScope
	meter     UsageMeter
}

// WithAccounts wires BYOK resolution (the account's own key; a keyless
// account gets domain.ErrBYOKKeyMissing back, surfaced to the caller).
func (s *SmartReplier) WithAccounts(accounts domain.AccountRepository, plans domain.PlanRepository) *SmartReplier {
	s.scope = accountAIScope{accounts: accounts, plans: plans}
	return s
}

// WithMeter lands every suggestion call in the cost ledger ("smart_reply").
func (s *SmartReplier) WithMeter(m UsageMeter) *SmartReplier {
	s.meter = m
	return s
}

// NewSmartReplier wires the LLM client.
func NewSmartReplier(aiFactory domain.AIFactory) *SmartReplier {
	return &SmartReplier{aiFactory: aiFactory, provider: "google"}
}

// SmartReplyHistoryItem mirrors the minimal shape the LLM needs to
// know "who said what" in this conversation. The handler converts
// domain.Message rows down to this so we don't hand the model
// metadata it can't use.
type SmartReplyHistoryItem struct {
	From    string // "customer" or "agent"
	Content string
}

// Suggest generates up to 3 reply candidates for the given conversation
// snapshot. Returns nil + error if the provider fails or the LLM
// emitted unparseable output — callers fall back to no suggestions.
//
// Language cue is best-effort: the prompt includes the current
// conversation language so the model doesn't reply in en when the
// thread is pt-BR. If language is empty we let the LLM infer.
func (s *SmartReplier) Suggest(
	ctx context.Context,
	accountID string,
	history []SmartReplyHistoryItem,
	language string,
	agentSystemPrompt string,
) ([]string, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("smart_replier: empty history")
	}
	provider, plan, err := s.scope.providerFor(ctx, s.aiFactory, s.provider, accountID)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		return nil, domain.ErrBYOKKeyMissing
	}
	if err != nil {
		return nil, fmt.Errorf("smart_replier: get provider: %w", err)
	}

	// Build the prompt. Two parts: persona + format instructions.
	// Persona inherits the agent's system prompt when one exists so
	// suggestions match the brand voice; otherwise we fall back to a
	// neutral support assistant.
	persona := strings.TrimSpace(agentSystemPrompt)
	if persona == "" {
		persona = "Você é um atendente de suporte ao cliente educado, direto e empático."
	}
	langHint := ""
	if language != "" {
		langHint = fmt.Sprintf("\n\nResponda em %s.", language)
	}
	systemPrompt := persona + langHint + `

Sua tarefa: gerar EXATAMENTE 3 sugestões curtas de resposta para o atendente humano enviar como próxima mensagem.

Regras:
- Máximo 120 caracteres por sugestão.
- Sem formatação markdown, sem emoji exagerado, sem prefixos.
- Variedade: 1ª curta e empática, 2ª pedindo informação útil, 3ª oferecendo próximo passo concreto.
- Use SEMPRE o idioma da conversa.
- Responda APENAS com JSON: {"suggestions":["...","...","..."]}.
- Não inclua nada antes ou depois do JSON.`

	// Convert conversation history into ChatMessage form. We label the
	// agent side as "assistant" so the model treats prior agent turns
	// as its own previous output (consistent voice).
	chat := make([]domain.ChatMessage, 0, len(history))
	for _, h := range history {
		role := "user"
		if h.From == "agent" {
			role = "assistant"
		}
		chat = append(chat, domain.ChatMessage{Role: role, Content: h.Content})
	}

	resp, err := provider.GenerateResponse(ctx, systemPrompt, chat, nil)
	if err != nil {
		return nil, fmt.Errorf("smart_replier: generate: %w", err)
	}
	recordUsage(ctx, s.meter, accountID, "smart_reply", plan, resp.Usage)
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		Suggestions []string `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("smart_replier: parse %q: %w", content, err)
	}
	out := make([]string, 0, len(parsed.Suggestions))
	for _, s := range parsed.Suggestions {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// Hard cap so a runaway model doesn't blow up the chip UI.
		if len(s) > 240 {
			s = s[:240]
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("smart_replier: empty suggestions")
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out, nil
}
