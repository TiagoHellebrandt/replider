package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// ConversationSummarizer compresses an arbitrarily long thread into a
// single sentence the human attendant can read in 2 seconds before
// hitting "assumir conversa". Different from SmartReplier (suggests
// what to say next) and SentimentAnalyzer (rates the tone) — this
// answers "what is this conversation actually about?".
//
// Two design choices:
//   - One sentence, ≤200 chars. Anything longer defeats the purpose
//     (operator would just scroll the thread).
//   - Inherits the agent's persona prompt so the summary uses
//     domain-aware vocabulary ("compra do produto X" vs "purchase
//     intent class A"). Cheap on tokens since the system prompt
//     reuses what's already configured.
type ConversationSummarizer struct {
	aiFactory domain.AIFactory
	provider  string
	scope     accountAIScope
	meter     UsageMeter
}

// WithAccounts wires BYOK resolution (the account's own key; a keyless
// account gets domain.ErrBYOKKeyMissing back, surfaced to the caller).
func (s *ConversationSummarizer) WithAccounts(accounts domain.AccountRepository, plans domain.PlanRepository) *ConversationSummarizer {
	s.scope = accountAIScope{accounts: accounts, plans: plans}
	return s
}

// WithMeter lands every summary in the cost ledger ("summary").
func (s *ConversationSummarizer) WithMeter(m UsageMeter) *ConversationSummarizer {
	s.meter = m
	return s
}

func NewConversationSummarizer(aiFactory domain.AIFactory) *ConversationSummarizer {
	return &ConversationSummarizer{aiFactory: aiFactory, provider: "google"}
}

// SummaryHistoryItem is the conversation slice the summarizer needs.
// Mirrors SmartReplyHistoryItem so handlers can convert once and
// reuse for both surfaces.
type SummaryHistoryItem struct {
	From    string // "customer" or "agent"
	Content string
}

// Summarize returns a one-sentence pt-BR/en/es summary of the given
// thread. Returns "" + error on LLM failure or empty input — callers
// fall back to a "Sem resumo disponível" placeholder rather than
// retry hot.
func (s *ConversationSummarizer) Summarize(
	ctx context.Context,
	accountID string,
	history []SummaryHistoryItem,
	language, agentSystemPrompt string,
) (string, error) {
	if len(history) == 0 {
		return "", fmt.Errorf("summarizer: empty history")
	}
	provider, plan, err := s.scope.providerFor(ctx, s.aiFactory, s.provider, accountID)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		return "", domain.ErrBYOKKeyMissing
	}
	if err != nil {
		return "", fmt.Errorf("summarizer: get provider: %w", err)
	}

	persona := strings.TrimSpace(agentSystemPrompt)
	if persona == "" {
		persona = "Você é um assistente de atendimento ao cliente."
	}
	// pt-BR is the ANCHOR, not a guess: "use the conversation's language"
	// over a three-word thread had the model answering in Spanish. The
	// caller can still pin another language explicitly.
	langHint := "\n\nEscreva em português do Brasil, a menos que a conversa " +
		"esteja claramente em outro idioma."
	if language != "" {
		langHint = fmt.Sprintf("\n\nResponda em %s.", language)
	}
	systemPrompt := persona + langHint + `

Sua tarefa: resumir a conversa abaixo em UMA ÚNICA FRASE, máximo 200 caracteres.

Regras:
- Foque no objetivo do cliente + estado atual ("quer comprar X, aguarda código de rastreio").
- Sem prefixos do tipo "O cliente está...". Comece direto pelo fato.
- Sem markdown, sem aspas, sem emoji.
- Em dúvida sobre o idioma, escreva em português do Brasil.
- Responda APENAS com o texto da frase. Sem JSON, sem explicação.`

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
		return "", fmt.Errorf("summarizer: generate: %w", err)
	}
	recordUsage(ctx, s.meter, accountID, "summary", plan, resp.Usage)
	out := strings.TrimSpace(resp.Content)
	out = strings.Trim(out, "\"") // model sometimes wraps
	// Hard cap so a chatty model doesn't blow up the dashboard tile.
	if len(out) > 240 {
		out = out[:240] + "…"
	}
	return out, nil
}
