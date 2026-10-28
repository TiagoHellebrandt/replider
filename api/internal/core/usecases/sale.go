package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Sales (§5, F1).
//
// Two ways in, and they are deliberately asymmetric:
//
//   - A person types it. The typing IS the confirmation.
//   - A model reads a conversation and PROPOSES one. It never reaches the
//     table until somebody says yes — the house pattern, and the same one the
//     approach queue runs on.
type SaleUseCase struct {
	repo        domain.SaleRepository
	contactRepo domain.ContactRepository

	// AI extraction is optional. Unwired, the manual path still works —
	// which is the important half, because a sale nobody records is a
	// portfolio nobody has.
	aiFactory   domain.AIFactory
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	msgRepo     domain.MessageRepository
	// convRepo scopes Extract to the caller's account. Without it, Extract
	// would read a conversation by id alone — a cross-tenant IDOR, since
	// ListByConversationID is not account-scoped. Optional only so the manual
	// (non-AI) path keeps working unwired; Extract refuses to run without it.
	convRepo domain.ConversationRepository
	model    string
	// lifecycle emits sale.registered (J2). Optional: a deployment without
	// webhooks records sales exactly as before.
	lifecycle LifecycleEmitter
	// leadRepo links a sale to the deal it closed: resolves the customer
	// when only a lead id comes, and marks the lead won afterwards. Optional.
	leadRepo domain.LeadRepository

	now func() time.Time
}

// SetLeadRepo wires the lead link. Optional setter, same shape as SetAI.
func (uc *SaleUseCase) SetLeadRepo(r domain.LeadRepository) { uc.leadRepo = r }

func NewSaleUseCase(repo domain.SaleRepository, contacts domain.ContactRepository) *SaleUseCase {
	return &SaleUseCase{repo: repo, contactRepo: contacts, now: time.Now}
}

// SetAI wires extraction. All four together: the provider comes from the
// ACCOUNT's plan, the same way every other generation does.
func (uc *SaleUseCase) SetAI(f domain.AIFactory, accounts domain.AccountRepository, plans domain.PlanRepository, messages domain.MessageRepository) {
	uc.aiFactory, uc.accountRepo, uc.planRepo, uc.msgRepo = f, accounts, plans, messages
}

// SetConvRepo wires the account-ownership check for Extract. Optional setter,
// same shape as SetAI; extraction refuses to run without it.
func (uc *SaleUseCase) SetConvRepo(c domain.ConversationRepository) { uc.convRepo = c }

func (uc *SaleUseCase) SetModel(m string) { uc.model = strings.TrimSpace(m) }

func (uc *SaleUseCase) WithClock(now func() time.Time) *SaleUseCase {
	uc.now = now
	return uc
}

// ErrSaleInvalid carries every problem at once, so a form reports them
// together rather than one per submit.
type ErrSaleInvalid struct {
	Problems []string
}

func (e *ErrSaleInvalid) Error() string { return strings.Join(e.Problems, "; ") }

// Register records a sale.
func (uc *SaleUseCase) Register(ctx context.Context, accountID string, s *domain.Sale) (*domain.Sale, error) {
	s.AccountID = accountID
	if s.SoldAt.IsZero() {
		// Today, not zero: a sale somebody records without picking a date
		// happened now, and a zero date would sort it before every real one
		// forever.
		s.SoldAt = uc.now()
	}
	if s.Currency == "" {
		s.Currency = uc.accountCurrency(ctx, accountID)
	}
	if s.Source == "" {
		s.Source = domain.SaleManual
	}

	// A sale filed from a lead's card: the lead knows who the customer is,
	// and asking the form for the contact id a second time is how the app
	// ended up sending the lead's own id as the customer and getting
	// CONTACT_NOT_FOUND back.
	lead, err := uc.leadFor(ctx, accountID, s.LeadID)
	if err != nil {
		return nil, err
	}
	if lead != nil && strings.TrimSpace(s.ContactID) == "" {
		s.ContactID = lead.ContactID
	}

	if problems := s.Validate(); len(problems) > 0 {
		return nil, &ErrSaleInvalid{Problems: problems}
	}
	// The customer must exist and be ours. Without this a sale could be
	// filed against another account's contact and appear in their portfolio.
	if uc.contactRepo != nil {
		contact, err := uc.contactRepo.GetByID(ctx, s.ContactID, accountID)
		if err != nil {
			return nil, err
		}
		if contact == nil {
			// Falls back to the external id, the legacy identity shape the
			// approach path uses (lead.ContactID == external_id).
			contact, err = uc.contactRepo.GetByExternalID(ctx, accountID, s.ContactID)
			if err != nil || contact == nil {
				return nil, &domain.AppError{
					Code: "CONTACT_NOT_FOUND", Message: "cliente não encontrado"}
			}
			s.ContactID = contact.ID
		}
	}

	if s.ID == "" {
		s.ID = ulid.Make().String()
	}
	if err := uc.repo.Create(ctx, s); err != nil {
		return nil, err
	}

	// The deal closed. Best-effort AFTER the row commits: a sale recorded
	// with a lead left in "qualified" is a pipeline that lies a little; a
	// sale refused because a status write failed is money nobody recorded.
	uc.markLeadWon(ctx, lead)

	// J2: the sale is the event integrations care most about. Fired AFTER the
	// row commits — a webhook announcing a sale that failed to persist is worse
	// than no webhook.
	emitLifecycle(uc.lifecycle, accountID, domain.EventSaleRegistered, domain.LifecyclePayload{
		ID:          s.ID,
		ContactID:   s.ContactID,
		AmountCents: s.GrossCents,
		Currency:    s.Currency,
		// An AI-extracted sale fires this too, with confirmed=false: the
		// integration decides whether to act on an unvalidated one, and hiding
		// it until somebody confirms would delay every honest integration for
		// the sake of the dishonest reading.
		Confirmed:  s.Source != domain.SaleAIExtracted || s.ConfirmedByUserID != "",
		OccurredAt: s.SoldAt.UTC().Format(time.RFC3339),
	})
	return s, nil
}

func (uc *SaleUseCase) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.Sale, error) {
	list, err := uc.repo.List(ctx, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Sale{}
	}
	return list, nil
}

// leadFor loads the lead a sale names, scoped to the account.
//
// A lead from another account is indistinguishable from a missing one — the
// answer that leaks nothing. Nil lead with nil error when no id was given.
func (uc *SaleUseCase) leadFor(ctx context.Context, accountID, leadID string) (*domain.Lead, error) {
	if uc.leadRepo == nil || strings.TrimSpace(leadID) == "" {
		return nil, nil
	}
	lead, err := uc.leadRepo.GetByID(ctx, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil || lead.AccountID != accountID {
		return nil, &domain.AppError{Code: "LEAD_NOT_FOUND", Message: "lead não encontrado"}
	}
	return lead, nil
}

// markLeadWon closes the deal on the board. Terminal statuses stay: a lead
// already won is won, and a lost one somebody sold to anyway is a decision
// for the person who marked it lost, not for this write.
func (uc *SaleUseCase) markLeadWon(ctx context.Context, lead *domain.Lead) {
	if uc.leadRepo == nil || lead == nil || lead.Status.IsTerminal() {
		return
	}
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, domain.LeadStatusWon); err != nil {
		slog.WarnContext(ctx, "sale: failed to mark lead won",
			"lead_id", lead.ID, "error", err)
	}
}

func (uc *SaleUseCase) accountCurrency(ctx context.Context, accountID string) string {
	if uc.accountRepo == nil {
		return "brl"
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.Currency == "" {
		return "brl"
	}
	return account.Currency
}

var errSaleAIUnavailable = &domain.AppError{
	Code:    "SALE_EXTRACT_UNAVAILABLE",
	Message: "leitura automática indisponível: nenhum provedor de IA configurado",
}

// Extract reads a conversation and PROPOSES a sale.
//
// Records nothing. The answer is a question to put in front of a person —
// "fechou 200 cx de luvas, ciclo ~30d?" — because a model that files sales on
// its own turns a misread message into a commission somebody is owed.
func (uc *SaleUseCase) Extract(ctx context.Context, accountID, conversationID string) (*domain.SaleExtraction, error) {
	if uc.aiFactory == nil || uc.msgRepo == nil || uc.convRepo == nil {
		return nil, errSaleAIUnavailable
	}
	// Ownership gate: the conversation MUST belong to the caller's account
	// before a single message is loaded. ListByConversationID keys on
	// conversation_id alone, so skipping this reads another tenant's messages.
	conv, err := uc.convRepo.GetByID(ctx, conversationID, accountID)
	if err != nil || conv == nil {
		return nil, domain.ErrConversationNotFound
	}
	messages, err := uc.msgRepo.ListByConversationID(ctx, conversationID, saleExtractWindow, 0)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return &domain.SaleExtraction{ConversationID: conversationID, Found: false}, nil
	}

	provider, err := uc.provider(ctx, accountID)
	if err != nil {
		return nil, err
	}
	resp, err := provider.GenerateResponse(ctx, saleExtractSystemPrompt,
		[]domain.ChatMessage{{Role: "user", Content: saleTranscript(messages)}}, nil)
	if err != nil || resp == nil {
		return nil, errSaleAIUnavailable
	}

	out := parseSaleExtraction(resp.Content)
	out.ConversationID = conversationID
	return out, nil
}

// saleExtractWindow is how much of the thread the model reads.
//
// The tail, not the whole history: a deal is closed in the last few messages,
// and feeding a year of conversation costs tokens to find the same sentence.
const saleExtractWindow = 40

func (uc *SaleUseCase) provider(ctx context.Context, accountID string) (domain.AIProvider, error) {
	if uc.accountRepo == nil {
		return nil, errSaleAIUnavailable
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && uc.planRepo != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	if uc.model != "" && uc.model != plan.AIModel {
		clone := *plan
		clone.AIModel = uc.model
		plan = &clone
	}
	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil || provider == nil {
		return nil, errSaleAIUnavailable
	}
	return provider, nil
}

const saleExtractSystemPrompt = "Você lê conversas comerciais brasileiras e identifica " +
	"vendas FECHADAS. Responde só com JSON válido, sem cercas de código."

func saleTranscript(messages []*domain.Message) string {
	var b strings.Builder
	b.WriteString("Leia a conversa e diga se uma venda foi FECHADA.\n\n")
	for _, m := range messages {
		if m == nil || m.Content == nil {
			continue
		}
		who := "Cliente"
		if m.Direction == "outbound" {
			who = "Vendedor"
		}
		b.WriteString(who + ": " + *m.Content + "\n")
	}
	b.WriteString(`
Responda APENAS com JSON:
{"found":true,"gross_cents":0,"cycle_days":0,
 "items":[{"description":"","quantity":0}],
 "question":"fechou 200 cx de luvas, ciclo ~30d?"}

Regras:
- found=false se ninguém fechou nada. Proposta, orçamento e "vou pensar" NÃO
  são venda fechada.
- gross_cents em CENTAVOS (R$ 1.500,00 = 150000). Omita se o valor não
  aparecer — não invente.
- cycle_days é de quanto em quanto tempo esse cliente costuma recomprar, se a
  conversa disser. Omita se não disser.
- question é UMA frase curta, em português, pedindo confirmação do que você
  entendeu. É o que a pessoa vai ler.`)
	return b.String()
}

// parseSaleExtraction reads the model's answer.
//
// Anything unparseable becomes found=false: a half-read sale offered for
// confirmation is worse than no offer, because somebody will confirm it.
func parseSaleExtraction(raw string) *domain.SaleExtraction {
	body := extractJSONObject(raw)
	if body == "" {
		return &domain.SaleExtraction{Found: false}
	}
	var parsed struct {
		Found      bool   `json:"found"`
		GrossCents int64  `json:"gross_cents"`
		CycleDays  int    `json:"cycle_days"`
		Question   string `json:"question"`
		Items      []struct {
			Description string `json:"description"`
			Quantity    int    `json:"quantity"`
		} `json:"items"`
	}
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return &domain.SaleExtraction{Found: false}
	}
	if !parsed.Found || strings.TrimSpace(parsed.Question) == "" {
		// No question means nothing to put in front of a person, and the
		// whole interaction IS the question.
		return &domain.SaleExtraction{Found: false}
	}

	out := &domain.SaleExtraction{
		Found: true, GrossCents: parsed.GrossCents,
		CycleDays: parsed.CycleDays, Question: strings.TrimSpace(parsed.Question),
	}
	for _, item := range parsed.Items {
		if strings.TrimSpace(item.Description) == "" {
			continue
		}
		out.Items = append(out.Items, domain.SaleItem{
			Description: item.Description, Quantity: item.Quantity,
		})
	}
	return out
}

// extractJSONObject finds the object inside whatever the model wrapped it in.
func extractJSONObject(raw string) string {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

// SetLifecycleEmitter wires the sale.registered webhook (J2). Optional.
func (uc *SaleUseCase) SetLifecycleEmitter(e LifecycleEmitter) {
	uc.lifecycle = e
}
