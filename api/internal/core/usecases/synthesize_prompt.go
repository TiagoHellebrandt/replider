package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// synthesisMetaPrompt is the system role sent to the LLM when turning
// the wizard answers into a production system_prompt. v2 enforces 9
// sections (vs 5), 700-word cap (vs 350), and conditional rules driven
// by compliance_flags / regional_slang / data_collection.
const synthesisMetaPrompt = `You produce production-grade system prompts for a customer-service AI agent.

Output ONLY the final system prompt body, in the user's language — no preamble, no explanation, no markdown headings, no fenced code blocks. The body must read as direct instructions the model will follow at runtime.

The body must contain these sections, in this exact order, separated by a blank line. Use the section name as a plain uppercase line (no markdown), followed by content:

IDENTITY
PERSONA
KNOWLEDGE
TONE
SCOPE
STRATEGY
ESCALATION
EXAMPLES
EDGE-CASES

Section guidance:
- IDENTITY — name the company, location and one-line value proposition. If a field is missing, reuse the vertical generically (e.g. "a clinic in Brazil") without inventing facts.
- PERSONA — bot name + pronoun + role. State that the bot acts on the company's behalf.
- KNOWLEDGE — operating hours (with timezone), service catalog with price ranges when supplied, policies (cancellation, refund, payment, delivery, LGPD/privacy when applicable). When data is missing, instruct the model to defer to a human rather than invent.
- TONE — tone keyword + audience + emoji posture + message length cap + customer addressing + regional slang policy.
- SCOPE — bullets of what the bot SHOULD answer + what it MUST NOT do. Compliance flags map to hard MUST-NOTs (medical_no_diagnosis → "never diagnose, never prescribe"; financial_no_advice → "never recommend specific investments"; legal_no_consult → "never give legal counsel").
- STRATEGY — service strategy (consultative / direct / educational / soft_sell), upsell rule, primary goal, data-collection list with timing. If LGPD is required or CPF appears in data_collection, require explicit opt-in before asking.
- ESCALATION — triggers + handoff channels. State exact phrasing for the handoff message.
- EXAMPLES — 1 to 5 Q→A pairs the user supplied. Format: "Q: …" then "A: …".
- EDGE-CASES — anti-examples (with explanation), common-scenario directives, personal-question response policy.

Rules:
- Maximum 700 words total.
- Imperative, unambiguous instructions ("Always …", "Never …").
- Never reveal you are an AI or mention the underlying model.
- Do not leave placeholders such as {{COMPANY}} or [name] — produce a body the operator can use as-is.
- Reuse the user's wording when possible; do not invent business facts.
- When a field is empty, omit it gracefully — don't write "not provided".`

// SynthesizePromptDTO is the input for the synthesizer.
// SaveAs (when non-empty) persists the result to the prompt library
// under that name; otherwise the body is returned without saving so
// the wizard can preview before commit.
type SynthesizePromptDTO struct {
	AccountID string
	UserID    string
	Answers   domain.WizardAnswers
	SaveAs    string
}

type SynthesizePromptResponse struct {
	Body         string         `json:"body"`
	Prompt       *domain.Prompt `json:"prompt,omitempty"`
	TokensUsed   int64          `json:"tokens_used"`
	TokensRemain int64          `json:"tokens_remain"`
}

type SynthesizePromptUseCase struct {
	accountRepo  domain.AccountRepository
	planRepo     domain.PlanRepository
	usageRepo    domain.UsageRepository
	auditLogRepo domain.AuditLogRepository
	promptRepo   domain.PromptRepository
	aiFactory    domain.AIFactory
	usageGuard   services.UsageChecker
}

func NewSynthesizePromptUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditLogRepo domain.AuditLogRepository,
	promptRepo domain.PromptRepository,
	aiFactory domain.AIFactory,
) *SynthesizePromptUseCase {
	return &SynthesizePromptUseCase{
		accountRepo:  accountRepo,
		planRepo:     planRepo,
		usageRepo:    usageRepo,
		auditLogRepo: auditLogRepo,
		promptRepo:   promptRepo,
		aiFactory:    aiFactory,
	}
}

func (uc *SynthesizePromptUseCase) SetUsageGuard(g services.UsageChecker) {
	uc.usageGuard = g
}

func (uc *SynthesizePromptUseCase) Execute(ctx context.Context, dto SynthesizePromptDTO) (*SynthesizePromptResponse, error) {
	if err := validateAnswers(&dto.Answers); err != nil {
		return nil, err
	}

	// Merge vertical defaults into any v2 fields the user left blank.
	// The wizard already pre-fills these client-side; this is the
	// server-side safety net so direct API users (or older clients)
	// still get the enriched output.
	mergeVerticalDefaults(&dto.Answers)

	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}

	plan := domain.TrialPlan()
	if account.PlanID != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}

	usage, _ := uc.usageRepo.GetByAccountID(ctx, dto.AccountID)
	if usage != nil && uc.usageGuard != nil {
		if err := uc.usageGuard.Check(account, plan, usage, "tokens"); err != nil {
			return nil, err
		}
	}

	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("failed to initialize AI provider: %v", err),
		}
	}

	userPayload := buildSynthesisUserPayload(&dto.Answers)
	resp, err := provider.GenerateResponse(ctx, synthesisMetaPrompt, []domain.ChatMessage{
		{Role: "user", Content: userPayload},
	}, nil)
	if err != nil || resp == nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("AI provider failed to synthesize prompt: %v", err),
		}
	}

	body := strings.TrimSpace(resp.Content)
	if body == "" {
		return nil, &domain.AppError{Code: "PROMPT_SYNTHESIS_EMPTY", Message: "synthesizer returned an empty body"}
	}
	if len(body) > domain.MaxPromptBodyBytes {
		body = body[:domain.MaxPromptBodyBytes]
	}

	tokensCost := int64((float64(resp.Usage.PromptTokens) * plan.InputMultiplier) +
		(float64(resp.Usage.CompletionTokens) * plan.OutputMultiplier))
	if tokensCost == 0 {
		tokensCost = 100
	}

	var tokensRemain int64
	if usage != nil {
		current := int64(0)
		if usage.AITokensUsed != nil {
			current = *usage.AITokensUsed
		}
		addon := int64(0)
		if usage.AddonTokens != nil {
			addon = *usage.AddonTokens
		}
		newTotal := current + tokensCost
		usage.AITokensUsed = &newTotal
		usage.UpdatedAt = time.Now().UTC()
		_ = uc.usageRepo.Update(ctx, usage)
		tokensRemain = (plan.MaxAITokensPerMonth + addon) - newTotal
		if tokensRemain < 0 {
			tokensRemain = 0
		}
	}

	out := &SynthesizePromptResponse{
		Body:         body,
		TokensUsed:   tokensCost,
		TokensRemain: tokensRemain,
	}

	if name := strings.TrimSpace(dto.SaveAs); name != "" {
		answers := dto.Answers
		p := &domain.Prompt{
			ID:            ulid.Make().String(),
			AccountID:     dto.AccountID,
			Name:          name,
			Body:          body,
			Source:        domain.PromptSourceWizard,
			WizardAnswers: &answers,
			CreatedBy:     ptrIfNotEmpty(dto.UserID),
		}
		if err := uc.promptRepo.Create(ctx, p); err != nil {
			return nil, err
		}
		out.Prompt = p
	}

	_ = uc.auditLogRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  dto.AccountID,
		Action:     "prompt.synthesize",
		EntityType: "prompt",
		CreatedAt:  time.Now().UTC(),
	})

	return out, nil
}

// mergeVerticalDefaults fills the v2 fields the user left blank with
// the canonical vertical seeds. Lists/structs are only filled when the
// user-provided slice is empty / pointer is nil — never overwriting.
func mergeVerticalDefaults(a *domain.WizardAnswers) {
	d := domain.DefaultsFor(a.Vertical)
	if a.BusinessHours == nil &&
		(d.BusinessHours.WeekdaysFrom != "" || d.BusinessHours.Timezone != "") {
		bh := d.BusinessHours
		a.BusinessHours = &bh
	}
	if len(a.ScopeOut) == 0 && len(d.ScopeOutBullets) > 0 {
		a.ScopeOut = append([]string{}, d.ScopeOutBullets...)
	}
	if len(a.ComplianceFlags) == 0 && len(d.ComplianceFlags) > 0 {
		a.ComplianceFlags = append([]string{}, d.ComplianceFlags...)
	}
	if len(a.CommonScenarios) == 0 && len(d.CommonScenarios) > 0 {
		a.CommonScenarios = append([]string{}, d.CommonScenarios...)
	}
	if len(a.DataCollection) == 0 && len(d.DataCollection) > 0 {
		a.DataCollection = append([]string{}, d.DataCollection...)
	}
	if a.EmojiUsage == "" && d.EmojiUsage != "" {
		a.EmojiUsage = d.EmojiUsage
	}
	if a.PrimaryGoal == "" && d.PrimaryGoal != "" {
		a.PrimaryGoal = d.PrimaryGoal
	}
}

func validateAnswers(a *domain.WizardAnswers) error {
	if strings.TrimSpace(a.Vertical) == "" {
		return &domain.AppError{Code: "WIZARD_VERTICAL_REQUIRED", Message: "vertical is required"}
	}
	if strings.TrimSpace(a.Tone) == "" {
		return &domain.AppError{Code: "WIZARD_TONE_REQUIRED", Message: "tone is required"}
	}
	if len(a.ScopeIn) == 0 {
		return &domain.AppError{Code: "WIZARD_SCOPE_IN_REQUIRED", Message: "at least one scope-in item is required"}
	}
	return nil
}

// buildSynthesisUserPayload renders the wizard answers as a labeled
// section list grouped by the same 5 wizard phases the user filled in.
// Empty fields are omitted so the synthesizer doesn't waste tokens
// reading blanks. Vertical defaults are merged before this point.
func buildSynthesisUserPayload(a *domain.WizardAnswers) string {
	var b strings.Builder
	b.WriteString("Build the system prompt from these answers. Verb tense: imperative.\n")
	fmt.Fprintf(&b, "Vertical hint: %s\n\n", a.Vertical)

	// --- Phase 1: Identity -----------------------------------------
	b.WriteString("[IDENTITY]\n")
	if a.CompanyName != "" {
		fmt.Fprintf(&b, "Company name: %s\n", a.CompanyName)
	}
	if a.CompanyTagline != "" {
		fmt.Fprintf(&b, "Tagline / value prop: %s\n", a.CompanyTagline)
	}
	if a.CompanyLocation != "" {
		fmt.Fprintf(&b, "Location: %s\n", a.CompanyLocation)
	}
	b.WriteString("\n[PERSONA]\n")
	if a.BotName != "" {
		fmt.Fprintf(&b, "Bot name: %s\n", a.BotName)
	}
	if a.BotPronoun != "" {
		fmt.Fprintf(&b, "Pronoun: %s\n", a.BotPronoun)
	}
	if a.BotRole != "" {
		fmt.Fprintf(&b, "Role: %s\n", a.BotRole)
	}

	// --- Phase 2: Knowledge / operational --------------------------
	b.WriteString("\n[KNOWLEDGE]\n")
	if a.BusinessHours != nil {
		writeBusinessHours(&b, a.BusinessHours)
	}
	if len(a.Services) > 0 {
		b.WriteString("Services / catalog:\n")
		for _, s := range a.Services {
			fmt.Fprintf(&b, "  - %s", s.Name)
			if s.PriceRange != "" {
				fmt.Fprintf(&b, " — %s", s.PriceRange)
			}
			if s.Description != "" {
				fmt.Fprintf(&b, " (%s)", s.Description)
			}
			b.WriteString("\n")
		}
	}
	if a.Policies != nil {
		writePolicies(&b, a.Policies)
	}
	if a.HumanResponseEta != "" {
		fmt.Fprintf(&b, "Human response ETA: %s\n", a.HumanResponseEta)
	}
	if len(a.EscalationChannels) > 0 {
		fmt.Fprintf(&b, "Handoff channels: %s\n", strings.Join(a.EscalationChannels, ", "))
	}

	// --- Phase 3: Tone ---------------------------------------------
	b.WriteString("\n[TONE]\n")
	fmt.Fprintf(&b, "Tone: %s\n", a.Tone)
	if a.Audience != "" {
		fmt.Fprintf(&b, "Audience: %s\n", a.Audience)
	}
	if a.LanguageFormality != "" {
		fmt.Fprintf(&b, "Language formality: %s\n", a.LanguageFormality)
	}
	if a.EmojiUsage != "" {
		fmt.Fprintf(&b, "Emoji posture: %s\n", a.EmojiUsage)
	}
	if a.MessageLength != "" {
		fmt.Fprintf(&b, "Message length: %s\n", a.MessageLength)
	}
	if a.CustomerAddressStyle != "" {
		fmt.Fprintf(&b, "Customer addressing: %s\n", a.CustomerAddressStyle)
	}
	if a.RegionalSlang != "" {
		fmt.Fprintf(&b, "Regional slang: %s\n", a.RegionalSlang)
	}

	// --- Phase 4: Scope + strategy ---------------------------------
	b.WriteString("\n[SCOPE-IN]\n")
	for _, s := range a.ScopeIn {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	if len(a.ScopeOut) > 0 {
		b.WriteString("\n[SCOPE-OUT]\n")
		for _, s := range a.ScopeOut {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	if len(a.ComplianceFlags) > 0 {
		fmt.Fprintf(&b, "\nCompliance flags (HARD must-not): %s\n", strings.Join(a.ComplianceFlags, ", "))
	}
	b.WriteString("\n[STRATEGY]\n")
	if a.ServiceStrategy != "" {
		fmt.Fprintf(&b, "Service strategy: %s\n", a.ServiceStrategy)
	}
	if a.UpsellWhen != "" {
		fmt.Fprintf(&b, "Upsell when: %s\n", a.UpsellWhen)
	}
	if a.PrimaryGoal != "" {
		fmt.Fprintf(&b, "Primary goal: %s\n", a.PrimaryGoal)
	}
	if len(a.DataCollection) > 0 {
		fmt.Fprintf(&b, "Collect data: %s", strings.Join(a.DataCollection, ", "))
		if a.DataCollectionWhen != "" {
			fmt.Fprintf(&b, " (when: %s)", a.DataCollectionWhen)
		}
		b.WriteString("\n")
	}

	// --- Escalation triggers ---------------------------------------
	if len(a.EscalationTriggers) > 0 {
		b.WriteString("\n[ESCALATION TRIGGERS]\n")
		for _, s := range a.EscalationTriggers {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}

	// --- Phase 5: Examples + edge cases ----------------------------
	if len(a.Examples) > 0 {
		b.WriteString("\n[EXAMPLES]\n")
		for _, e := range a.Examples {
			fmt.Fprintf(&b, "Q: %s\nA: %s\n\n", e.Question, e.Answer)
		}
	}
	if len(a.AntiExamples) > 0 {
		b.WriteString("\n[ANTI-EXAMPLES — never reply like this]\n")
		for _, e := range a.AntiExamples {
			fmt.Fprintf(&b, "Q: %s\nBad A: %s\nWhy: %s\n\n", e.Question, e.BadAnswer, e.Why)
		}
	}
	if len(a.CommonScenarios) > 0 {
		b.WriteString("\n[COMMON SCENARIOS — make sure the prompt covers them]\n")
		for _, s := range a.CommonScenarios {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	if a.PersonalQuestionsResponse != "" {
		fmt.Fprintf(&b, "\nPersonal questions about the bot: %s\n", a.PersonalQuestionsResponse)
	}

	// JSON tail kept as a structured fallback if the model wants it.
	if raw, err := json.Marshal(a); err == nil {
		fmt.Fprintf(&b, "\n[RAW ANSWERS — JSON for reference]\n%s\n", string(raw))
	}
	return b.String()
}

func writeBusinessHours(b *strings.Builder, h *domain.BusinessHours) {
	any := false
	if h.WeekdaysFrom != "" || h.WeekdaysTo != "" {
		fmt.Fprintf(b, "Weekdays: %s–%s\n", h.WeekdaysFrom, h.WeekdaysTo)
		any = true
	}
	if h.SaturdayFrom != "" || h.SaturdayTo != "" {
		fmt.Fprintf(b, "Saturday: %s–%s\n", h.SaturdayFrom, h.SaturdayTo)
		any = true
	}
	if h.SundayFrom != "" || h.SundayTo != "" {
		fmt.Fprintf(b, "Sunday: %s–%s\n", h.SundayFrom, h.SundayTo)
		any = true
	}
	if h.Timezone != "" && any {
		fmt.Fprintf(b, "Timezone: %s\n", h.Timezone)
	}
}

func writePolicies(b *strings.Builder, p *domain.Policies) {
	if p.Cancellation != "" {
		fmt.Fprintf(b, "Cancellation policy: %s\n", p.Cancellation)
	}
	if p.Refund != "" {
		fmt.Fprintf(b, "Refund policy: %s\n", p.Refund)
	}
	if len(p.PaymentMethods) > 0 {
		fmt.Fprintf(b, "Payment methods: %s\n", strings.Join(p.PaymentMethods, ", "))
	}
	if p.Delivery != "" {
		fmt.Fprintf(b, "Delivery policy: %s\n", p.Delivery)
	}
	if p.LgpdRequired {
		b.WriteString("LGPD/GDPR: REQUIRED — request explicit opt-in before collecting PII.\n")
	}
}
