package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Summarising a thread for the person who has to act on it.
//
// The question this answers is the one someone actually has when they open a
// forty-message thread after a week away, or inherit a lead from a colleague:
// what was agreed, what is pending, and who owes the next move.
type ConversationBriefUseCase struct {
	briefs      domain.ConversationBriefRepository
	msgRepo     domain.MessageRepository
	convRepo    domain.ConversationRepository
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	aiFactory   domain.AIFactory
	meter       *AIMeter
	// spend is the budget-only gate derived from the meter. Nil allows.
	spend *spendGate
	model string
}

func NewConversationBriefUseCase(
	briefs domain.ConversationBriefRepository,
	msgRepo domain.MessageRepository,
	convRepo domain.ConversationRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	aiFactory domain.AIFactory,
) *ConversationBriefUseCase {
	return &ConversationBriefUseCase{
		briefs: briefs, msgRepo: msgRepo, convRepo: convRepo,
		accountRepo: accountRepo, planRepo: planRepo, aiFactory: aiFactory,
	}
}

func (uc *ConversationBriefUseCase) SetAIMeter(m *AIMeter) {
	uc.meter = m
	uc.spend = budgetGateFromMeter(uc.accountRepo, m)
}

// SetModel pins the model. Empty keeps the plan's.
//
// Summarising is compression, not writing — the cheap tier does it as well as
// the expensive one, and this runs on every long thread a salesperson opens.
// Pinning it separately means the cost does not follow whatever expensive
// model the customer chose for WRITING approaches.
func (uc *ConversationBriefUseCase) SetModel(m string) { uc.model = strings.TrimSpace(m) }

// briefMaxMessages bounds what is fed to the model. Fifty turns is more than
// any B2B negotiation needs to be understood, and an unbounded thread would
// make the prompt grow without limit on exactly the conversations this
// feature is for.
const briefMaxMessages = 50

const briefPrompt = `Você resume uma conversa comercial para o vendedor que vai continuá-la.

Responda em JSON, em português do Brasil, com exatamente estas chaves:
{"acordado": "...", "pendente": "...", "proximo_passo": "..."}

- "acordado": o que já foi combinado ou entregue na conversa. Uma frase.
- "pendente": o que ficou em aberto. Uma frase.
- "proximo_passo": a próxima ação E de quem ela é ("seu: responder o orçamento" / "do cliente: enviar o CNPJ"). Uma frase.

Regras:
- Só use o que está na conversa. Não deduza valores, prazos ou intenções que ninguém escreveu.
- Se algo importante ficou ambíguo, diga que ficou ambíguo — não escolha uma interpretação.
- Se uma chave não tem conteúdo real, use "" — não invente.
- Nomes e números aparecem como foram escritos.
- Responda APENAS com o JSON. Sem cercas de código, sem explicação.`

// parseBriefJSON reads the model's structured answer, tolerating a code
// fence. A reply that is not the expected JSON degrades to flowing text —
// the reader still gets a summary, just unlabeled.
func parseBriefJSON(raw string) (agreed, pending, next string, ok bool) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	var out struct {
		Acordado    string `json:"acordado"`
		Pendente    string `json:"pendente"`
		ProximoPaso string `json:"proximo_passo"`
	}
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return "", "", "", false
	}
	agreed = strings.TrimSpace(out.Acordado)
	pending = strings.TrimSpace(out.Pendente)
	next = strings.TrimSpace(out.ProximoPaso)
	// All three empty is a non-answer, not a structured one.
	ok = agreed != "" || pending != "" || next != ""
	return agreed, pending, next, ok
}

// briefFlowingText renders the three fields as the copy/paste form.
func briefFlowingText(agreed, pending, next string) string {
	var parts []string
	if agreed != "" {
		parts = append(parts, "Acordado: "+agreed)
	}
	if pending != "" {
		parts = append(parts, "Pendente: "+pending)
	}
	if next != "" {
		parts = append(parts, "Próximo passo: "+next)
	}
	return strings.Join(parts, "\n")
}

// Get returns the brief, generating one when it is missing or stale.
//
// Generation is on read rather than on every inbound message: most
// conversations are never opened again, and summarising them all would pay
// for text nobody reads.
func (uc *ConversationBriefUseCase) Get(ctx context.Context, actor domain.Actor, conversationID string) (*domain.ConversationBrief, error) {
	conv, err := uc.convRepo.GetByID(ctx, conversationID, actor.AccountID)
	if err != nil || conv == nil {
		return nil, domain.ErrConversationNotFound
	}

	messages, err := uc.msgRepo.ListByConversationID(ctx, conversationID, briefMaxMessages, 0)
	if err != nil {
		return nil, err
	}
	if len(messages) < domain.MinMessagesForBrief {
		// Below this the thread fits on one screen and the summary would be
		// longer than what it replaces.
		return nil, nil
	}
	latest := latestMessageAt(messages)

	existing, err := uc.briefs.Get(ctx, conversationID, actor.AccountID)
	if err != nil {
		return nil, err
	}
	if !existing.IsStale(latest) {
		return existing, nil
	}

	text, model, err := uc.generate(ctx, actor.AccountID, messages)
	if err != nil {
		// A failed summary is not an error worth blocking on: the caller has
		// the whole thread anyway. Returning the STALE brief would be worse —
		// it would describe a negotiation that has moved on.
		return nil, err
	}

	brief := &domain.ConversationBrief{
		ConversationID: conversationID,
		AccountID:      actor.AccountID,
		Text:           text,
		Model:          model,
		CoversUntil:    latest,
		// The DB stamps its own NOW() on save, but THIS struct is what the
		// first read returns — a zero time here rendered "gerado há 739855d"
		// on every freshly generated brief.
		GeneratedAt: time.Now(),
	}
	if agreed, pending, next, ok := parseBriefJSON(text); ok {
		brief.Agreed, brief.Pending, brief.NextStep = agreed, pending, next
		brief.Text = briefFlowingText(agreed, pending, next)
	}
	if err := uc.briefs.Save(ctx, brief); err != nil {
		// Generated but not stored: return it anyway. The reader gets their
		// answer and the next call simply pays again.
		return brief, nil
	}
	return brief, nil
}

func (uc *ConversationBriefUseCase) generate(ctx context.Context, accountID string, messages []*domain.Message) (string, string, error) {
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return "", "", domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	effective := plan
	if uc.model != "" && uc.model != plan.AIModel {
		clone := *plan
		clone.AIModel = uc.model
		effective = &clone
	}

	if err := uc.spend.ensureAnyFor(ctx, account); err != nil {
		return "", "", err
	}
	provider, err := uc.aiFactory.GetProviderForAccount(ctx, effective.AIProvider, account, effective)
	if err != nil {
		return "", "", err
	}

	resp, err := provider.GenerateResponse(ctx, briefPrompt,
		[]domain.ChatMessage{{Role: "user", Content: renderThread(messages)}}, nil)
	if err != nil || resp == nil {
		if err == nil {
			err = domain.ErrAIProviderUnavailable
		}
		return "", "", err
	}
	uc.meter.Record(ctx, accountID, "conversation_brief", effective.AIProvider, effective.AIModel, resp.Usage)

	text := strings.TrimSpace(resp.Content)
	if text == "" {
		return "", "", domain.ErrAIProviderUnavailable
	}
	return text, effective.AIModel, nil
}

// renderThread turns the messages into something a model can read, labelled
// by SIDE rather than by sender id: who said what matters, which ULID said it
// does not, and the ids would eat tokens on every summary.
func renderThread(messages []*domain.Message) string {
	var b strings.Builder
	for _, m := range messages {
		if m.Content == nil || strings.TrimSpace(*m.Content) == "" {
			continue
		}
		side := "Cliente"
		if m.Direction == "outbound" {
			side = "Nós"
		}
		fmt.Fprintf(&b, "%s: %s\n", side, strings.TrimSpace(*m.Content))
	}
	return b.String()
}

func latestMessageAt(messages []*domain.Message) time.Time {
	var latest time.Time
	for _, m := range messages {
		if m.Timestamp.After(latest) {
			latest = m.Timestamp
		}
	}
	return latest
}
