package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Conversational onboarding (§6.20, K5).
//
// The machine walks a fixed ladder and the model only ever does two things: turn
// prose into fields, and propose audiences. Everything has a manual path, because
// an onboarding that breaks when a provider is down is an onboarding that loses
// signups — and the provider IS down sometimes.

// OnboardingChatUseCase drives the conversation.
type OnboardingChatUseCase struct {
	repo     domain.OnboardingChatRepository
	accounts domain.AccountRepository

	// audiences is optional: without it the audiences step offers nothing and
	// is skipped, which is a worse onboarding but a working one.
	audiences *AudienceUseCase
	// products writes what the product step gathered. Optional for the same
	// reason.
	products domain.ProductRepository

	// extractor turns free text into fields. Optional AND cheap: this is the
	// "modelo barato" of the story — it is reading a paragraph, not reasoning.
	extractor OnboardingExtractor

	now func() time.Time
}

func NewOnboardingChatUseCase(
	repo domain.OnboardingChatRepository, accounts domain.AccountRepository,
) *OnboardingChatUseCase {
	return &OnboardingChatUseCase{repo: repo, accounts: accounts, now: time.Now}
}

func (uc *OnboardingChatUseCase) SetAudiences(a *AudienceUseCase) { uc.audiences = a }
func (uc *OnboardingChatUseCase) SetProducts(p domain.ProductRepository) {
	uc.products = p
}
func (uc *OnboardingChatUseCase) SetExtractor(e OnboardingExtractor) { uc.extractor = e }

// OnboardingExtractor turns a sentence into structured fields.
//
// A narrow port with ONE method, because that is the whole job: the model is not
// deciding anything here, it is parsing. A failure returns the zero value and the
// caller keeps whatever the person typed.
type OnboardingExtractor interface {
	ExtractProduct(ctx context.Context, accountID, text string) (name, description string, priceCents int64)
}

// OnboardingChatView is what the screen renders.
type OnboardingChatView struct {
	Step     domain.OnboardingStep      `json:"step"`
	Prompt   domain.OnboardingPrompt    `json:"prompt"`
	Answers  domain.OnboardingAnswers   `json:"answers"`
	Messages []domain.OnboardingMessage `json:"messages"`
	// Suggestions are the audience chips, only on that step.
	Suggestions []string `json:"suggestions,omitempty"`
	// Progress is how far along, for a bar that means something: the ladder has
	// a known length, so this is a fact rather than an estimate.
	Progress float64 `json:"progress"`
	Done     bool    `json:"done"`
}

// Start returns the conversation, creating it on first call.
func (uc *OnboardingChatUseCase) Start(ctx context.Context, accountID string) (*OnboardingChatView, error) {
	chat, err := uc.repo.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		chat = &domain.OnboardingChat{
			AccountID: accountID,
			Step:      domain.StepCompany,
			StartedAt: uc.now(),
		}
		// The opening question is stored as a message so a resume shows the
		// conversation rather than an empty box.
		chat.Messages = append(chat.Messages, domain.OnboardingMessage{
			Role: "assistant",
			Text: domain.PromptFor(domain.StepCompany, chat.Answers).Text,
			At:   uc.now(),
		})
		if err := uc.repo.Save(ctx, chat); err != nil {
			return nil, err
		}
	}
	return uc.view(ctx, chat), nil
}

// AnswerInput is one turn from the person.
type AnswerInput struct {
	AccountID string
	// Text is what they typed, for the steps that take prose.
	Text string
	// Choices are the chips they kept or the channel they picked.
	Choices []string
	// Skip moves on without answering. Refused on the company step.
	Skip bool
}

// Answer records a turn and advances the ladder.
func (uc *OnboardingChatUseCase) Answer(ctx context.Context, in AnswerInput) (*OnboardingChatView, error) {
	chat, err := uc.repo.Get(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return uc.Start(ctx, in.AccountID)
	}
	if chat.Done() {
		return uc.view(ctx, chat), nil
	}

	if text := strings.TrimSpace(in.Text); text != "" {
		chat.Messages = append(chat.Messages, domain.OnboardingMessage{
			Role: "user", Text: text, At: uc.now(),
		})
	}

	if !in.Skip {
		uc.apply(ctx, chat, in)
	} else if chat.Step == domain.StepCompany {
		// The one step that cannot be skipped: everything downstream is
		// written in this company's voice, and an agent with no company is an
		// agent that introduces itself as nobody.
		return nil, &domain.AppError{
			Code:    "ONBOARDING_COMPANY_REQUIRED",
			Message: "preciso ao menos do nome da empresa para continuar",
		}
	}

	if !chat.CanAdvance() {
		return nil, &domain.AppError{
			Code:    "ONBOARDING_COMPANY_REQUIRED",
			Message: "preciso ao menos do nome da empresa para continuar",
		}
	}

	chat.Step = domain.NextOnboardingStep(chat.Step)
	next := domain.PromptFor(chat.Step, chat.Answers)
	chat.Messages = append(chat.Messages, domain.OnboardingMessage{
		Role: "assistant", Text: next.Text, At: uc.now(),
	})
	if chat.Step == domain.StepDone && chat.CompletedAt == nil {
		completed := uc.now()
		chat.CompletedAt = &completed
	}

	if err := uc.repo.Save(ctx, chat); err != nil {
		return nil, err
	}
	return uc.view(ctx, chat), nil
}

// apply writes the step's answer into the draft and, where the step owns a real
// record, into the real table.
func (uc *OnboardingChatUseCase) apply(ctx context.Context, chat *domain.OnboardingChat, in AnswerInput) {
	text := strings.TrimSpace(in.Text)

	switch chat.Step {
	case domain.StepCompany:
		name, site := splitCompanyAnswer(text, in.Choices)
		if name != "" {
			chat.Answers.CompanyName = name
			// Written straight through to the account: the company name is
			// what every prompt downstream introduces the agent as, and
			// leaving it in a draft would mean the first preview chat says
			// "sou o assistente virtual de ".
			uc.renameAccount(ctx, chat.AccountID, name)
		}
		if site != "" {
			chat.Answers.Website = site
		}

	case domain.StepProduct:
		chat.Answers.ProductDescription = text
		// The model's ONLY job here: turn a sentence into a name and a price.
		// It failing costs the structure, not the answer — the description the
		// person typed is kept either way.
		if uc.extractor != nil && text != "" {
			name, description, price := uc.extractor.ExtractProduct(ctx, chat.AccountID, text)
			if name != "" {
				chat.Answers.ProductName = name
			}
			if description != "" {
				chat.Answers.ProductDescription = description
			}
			if price > 0 {
				chat.Answers.ProductPriceCents = price
			}
		}
		if chat.Answers.ProductName == "" {
			// No model, or nothing recognisable: the first line of what they
			// wrote is a better product name than an empty one, and they can
			// rename it on the products screen.
			chat.Answers.ProductName = firstLine(text)
		}
		uc.createProduct(ctx, chat)

	case domain.StepAudiences:
		chat.Answers.AudienceNames = trimAll(in.Choices)

	case domain.StepChannel:
		if len(in.Choices) > 0 {
			chat.Answers.ChannelType = strings.TrimSpace(in.Choices[0])
		}
	}
}

// view assembles what the screen needs, including the audience suggestions when
// the conversation has reached that step.
func (uc *OnboardingChatUseCase) view(ctx context.Context, chat *domain.OnboardingChat) *OnboardingChatView {
	out := &OnboardingChatView{
		Step:     chat.Step,
		Prompt:   domain.PromptFor(chat.Step, chat.Answers),
		Answers:  chat.Answers,
		Messages: chat.Messages,
		Done:     chat.Done(),
		Progress: onboardingProgress(chat.Step),
	}

	if chat.Step == domain.StepAudiences && uc.audiences != nil {
		suggested, err := uc.audiences.Suggest(ctx, chat.AccountID)
		if err != nil {
			// No suggestions is a worse step, not a broken one: the person
			// types their own, and the conversation continues. Logged because
			// this is the one place a provider outage is visible to a new user.
			slog.WarnContext(ctx, "onboarding: could not suggest audiences",
				"account_id", chat.AccountID, "error", err)
		}
		for _, audience := range suggested {
			out.Suggestions = append(out.Suggestions, audience.Name)
		}
	}
	return out
}

// onboardingProgress is a fact, not an estimate: the ladder has a known length.
func onboardingProgress(step domain.OnboardingStep) float64 {
	switch step {
	case domain.StepCompany:
		return 0
	case domain.StepProduct:
		return 0.25
	case domain.StepAudiences:
		return 0.5
	case domain.StepChannel:
		return 0.75
	default:
		return 1
	}
}

// renameAccount writes the company name through. Best-effort: the draft already
// has it, and failing here must not stop the conversation.
func (uc *OnboardingChatUseCase) renameAccount(ctx context.Context, accountID, name string) {
	if uc.accounts == nil {
		return
	}
	if err := uc.accounts.UpdateName(ctx, accountID, name); err != nil {
		slog.WarnContext(ctx, "onboarding: could not set the company name",
			"account_id", accountID, "error", err)
	}
}

// createProduct records what they sell. Best-effort for the same reason.
func (uc *OnboardingChatUseCase) createProduct(ctx context.Context, chat *domain.OnboardingChat) {
	if uc.products == nil || strings.TrimSpace(chat.Answers.ProductName) == "" {
		return
	}
	product := &domain.Product{
		AccountID:   chat.AccountID,
		Name:        chat.Answers.ProductName,
		Description: chat.Answers.ProductDescription,
		PriceCents:  chat.Answers.ProductPriceCents,
		Currency:    domain.CommissionLedgerCurrency,
		Attributes:  json.RawMessage("{}"),
	}
	if err := uc.products.Create(ctx, product); err != nil {
		slog.WarnContext(ctx, "onboarding: could not create the product",
			"account_id", chat.AccountID, "error", err)
	}
}

// StampFirstLead records the activation moment. Called from the lead pipeline;
// idempotent in SQL.
func (uc *OnboardingChatUseCase) StampFirstLead(ctx context.Context, accountID string) {
	if uc == nil || uc.repo == nil {
		return
	}
	if err := uc.repo.StampFirstLead(ctx, accountID, uc.now()); err != nil {
		slog.WarnContext(ctx, "onboarding: could not stamp the first lead",
			"account_id", accountID, "error", err)
	}
}

// splitCompanyAnswer reads a name and a site out of one sentence.
//
// Deterministic, no model: "Clínica Bem Viver, clinicabemviver.com.br" is the
// shape people actually type, and spending a provider call to split a comma
// would be spending money to be slower and less reliable than a loop.
func splitCompanyAnswer(text string, choices []string) (name, site string) {
	candidates := append([]string{}, choices...)
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '|'
	}) {
		candidates = append(candidates, part)
	}

	for _, raw := range candidates {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		if looksLikeSite(part) {
			if site == "" {
				site = part
			}
			continue
		}
		if name == "" {
			name = part
		}
	}
	return name, site
}

// looksLikeSite is deliberately loose: a false positive costs a name in the
// website field, which the person sees and fixes, while a false negative loses
// the single most useful input the conversation gets.
func looksLikeSite(s string) bool {
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "www.") {
		return true
	}
	// A dot with no spaces around it, and a plausible TLD tail.
	if strings.Contains(lower, " ") || !strings.Contains(lower, ".") {
		return false
	}
	tail := lower[strings.LastIndex(lower, ".")+1:]
	return len(tail) >= 2 && len(tail) <= 6
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if len(trimmed) > 80 {
				return trimmed[:80]
			}
			return trimmed
		}
	}
	return ""
}

func trimAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
