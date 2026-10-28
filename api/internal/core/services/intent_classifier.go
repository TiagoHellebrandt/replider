package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// IntentClassifier maps inbound text to one of a fixed set of support
// intents. Used to auto-tag conversations so the dashboard can filter
// by "show me cobrança / cancelamento / compras" without making the
// attendant manually tag every thread.
//
// The label set is deliberately small (6 buckets) so:
//   - LLM accuracy stays high — too many classes and the model gets
//     wishy-washy on edge cases.
//   - The dashboard filter UI doesn't bloat into a 30-tag dropdown.
//   - We can hard-code a sensible color per label.
type IntentClassifier struct {
	aiFactory domain.AIFactory
	provider  string
	scope     accountAIScope
	meter     UsageMeter
}

// WithAccounts wires BYOK resolution: the classification runs on the
// account's own key and is skipped (ErrHelperSkipped) when it has none.
func (c *IntentClassifier) WithAccounts(accounts domain.AccountRepository, plans domain.PlanRepository) *IntentClassifier {
	c.scope = accountAIScope{accounts: accounts, plans: plans}
	return c
}

// WithMeter lands every classification in the cost ledger ("intent").
func (c *IntentClassifier) WithMeter(m UsageMeter) *IntentClassifier {
	c.meter = m
	return c
}

// IntentLabels lists the canonical buckets the model is constrained
// to. Update both here AND the prompt below in sync.
var IntentLabels = []string{
	"cobranca",
	"compra",
	"cancelamento",
	"suporte",
	"duvida",
	"outro",
}

// IntentColors maps each label to a stable hex color. Picked once so
// the dashboard always renders the same color for the same intent
// across accounts. Hex without '#' to match the existing tag schema.
var IntentColors = map[string]string{
	"cobranca":     "ef4444", // red — escalation-y
	"compra":       "22c55e", // green — money in
	"cancelamento": "f59e0b", // orange — at-risk
	"suporte":      "3b82f6", // blue — neutral
	"duvida":       "a855f7", // purple — pre-sale
	"outro":        "6b7280", // gray — fallback
}

// IntentDisplayNames is what the dashboard shows. Keep separate from
// the canonical key so we can rename labels without breaking the
// existing classifier output.
var IntentDisplayNames = map[string]string{
	"cobranca":     "Cobrança",
	"compra":       "Compra",
	"cancelamento": "Cancelamento",
	"suporte":      "Suporte",
	"duvida":       "Dúvida",
	"outro":        "Outro",
}

func NewIntentClassifier(aiFactory domain.AIFactory) *IntentClassifier {
	return &IntentClassifier{aiFactory: aiFactory, provider: "google"}
}

// Classify returns one of IntentLabels for the given text, plus the
// model's confidence. Returns ("", 0, err) on LLM failure or
// unrecognized output — callers fall back to skipping the tag rather
// than assigning "outro" to everything.
func (c *IntentClassifier) Classify(ctx context.Context, accountID, text string) (string, float64, error) {
	provider, plan, err := c.scope.providerFor(ctx, c.aiFactory, c.provider, accountID)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		return "", 0, ErrHelperSkipped
	}
	if err != nil {
		return "", 0, fmt.Errorf("intent: get provider: %w", err)
	}
	systemPrompt := `Classifique a mensagem do cliente abaixo em UMA das categorias:

- cobranca: o cliente está cobrando uma resposta, fatura, prazo, devolução de dinheiro, ou reclamando de demora.
- compra: o cliente quer comprar, fechar pedido, pedir orçamento, escolher plano.
- cancelamento: o cliente quer cancelar, desistir, encerrar, "parar de receber".
- suporte: problema técnico, defeito, bug, "não funciona", precisa de ajuda pra usar.
- duvida: pergunta de pré-venda, características, preço, prazo, política, formas de pagamento.
- outro: qualquer outro caso (saudação solta, agradecimento, mensagem fora de contexto).

Responda APENAS com JSON: {"intent":"...", "score":0.0-1.0}.
Sem prefixos, sem markdown.`
	chat := []domain.ChatMessage{
		{Role: "user", Content: "Mensagem: " + text},
	}
	resp, err := provider.GenerateResponse(ctx, systemPrompt, chat, nil)
	if err != nil {
		return "", 0, fmt.Errorf("intent: generate: %w", err)
	}
	recordUsage(ctx, c.meter, accountID, "intent", plan, resp.Usage)
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		Intent string  `json:"intent"`
		Score  float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return "", 0, fmt.Errorf("intent: parse %q: %w", content, err)
	}
	label := strings.ToLower(strings.TrimSpace(parsed.Intent))
	// Strip diacritics on the LLM's output ("cobrança" → "cobranca")
	// so it matches our canonical key set without surprises.
	label = stripBasicDiacritics(label)
	if !validIntent(label) {
		return "", 0, fmt.Errorf("intent: unknown label %q", parsed.Intent)
	}
	if parsed.Score < 0 {
		parsed.Score = 0
	}
	if parsed.Score > 1 {
		parsed.Score = 1
	}
	return label, parsed.Score, nil
}

func validIntent(label string) bool {
	for _, l := range IntentLabels {
		if l == label {
			return true
		}
	}
	return false
}

// stripBasicDiacritics handles only the common pt-BR cases the model
// emits (ç → c, ã → a, é → e). Avoids pulling in a full Unicode
// normalization dep for a 6-label classifier.
func stripBasicDiacritics(s string) string {
	r := strings.NewReplacer(
		"ç", "c",
		"ã", "a",
		"á", "a",
		"â", "a",
		"é", "e",
		"ê", "e",
		"í", "i",
		"ó", "o",
		"ô", "o",
		"õ", "o",
		"ú", "u",
	)
	return r.Replace(s)
}
