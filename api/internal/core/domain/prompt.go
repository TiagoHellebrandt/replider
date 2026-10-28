package domain

import "time"

// PromptSource enumerates how a Prompt entry was authored.
const (
	PromptSourceWizard     = "wizard"
	PromptSourceManual     = "manual"
	PromptSourceDuplicated = "duplicated"
	PromptSourceTemplate   = "template"
)

// MaxPromptBodyBytes is the soft cap for a synthesized/applied system
// prompt body. Keeps BuildSystemPrompt responses bounded and avoids
// runaway token costs from a runaway prompt. Bumped from 4 KB to 8 KB
// in the v2 builder which adds 9 sections (vs 5) and richer payloads.
const MaxPromptBodyBytes = 8192

// Prompt is a row in the user's "Meus Prompts" library.
type Prompt struct {
	ID            string         `json:"id"`
	AccountID     string         `json:"account_id"`
	Name          string         `json:"name"`
	Description   *string        `json:"description,omitempty"`
	Body          string         `json:"body"`
	Source        string         `json:"source"`
	WizardAnswers *WizardAnswers `json:"wizard_answers,omitempty"`
	CreatedBy     *string        `json:"created_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// WizardAnswers is the typed shape of the prompt-builder wizard
// payload. Persisted as JSONB on `prompts.wizard_answers` so future
// re-synthesis can run without re-asking the user.
//
// All v2 fields are pointers/optional — backward-compat with prompts
// stored under the v1 schema. Synthesis falls back to the matching
// VerticalDefaults entry when a v2 field is absent.
type WizardAnswers struct {
	// --- v1 (kept) -----------------------------------------------------
	Vertical           string      `json:"vertical"`
	Audience           string      `json:"audience,omitempty"`
	Tone               string      `json:"tone"`
	ScopeIn            []string    `json:"scope_in"`
	ScopeOut           []string    `json:"scope_out"`
	EscalationTriggers []string    `json:"escalation_triggers"`
	Examples           []QAExample `json:"examples,omitempty"`
	LanguageFormality  string      `json:"language_formality,omitempty"`

	// --- v2 phase 1 — Identity ----------------------------------------
	CompanyName     string `json:"company_name,omitempty"`
	CompanyTagline  string `json:"company_tagline,omitempty"`
	CompanyLocation string `json:"company_location,omitempty"`
	BotName         string `json:"bot_name,omitempty"`
	BotPronoun      string `json:"bot_pronoun,omitempty"` // ela | ele | neutro | first_person
	BotRole         string `json:"bot_role,omitempty"`

	// --- v2 phase 2 — Knowledge / operational -------------------------
	BusinessHours      *BusinessHours `json:"business_hours,omitempty"`
	Services           []ServiceItem  `json:"services,omitempty"`
	Policies           *Policies      `json:"policies,omitempty"`
	HumanResponseEta   string         `json:"human_response_eta,omitempty"` // minutes | hours | business_day
	EscalationChannels []string       `json:"escalation_channels,omitempty"`

	// --- v2 phase 3 — Tone specifics ----------------------------------
	EmojiUsage           string `json:"emoji_usage,omitempty"`            // none | sparse | contextual | liberal
	MessageLength        string `json:"message_length,omitempty"`         // short | medium | long
	CustomerAddressStyle string `json:"customer_address_style,omitempty"` // first_name | formal | generic
	RegionalSlang        string `json:"regional_slang,omitempty"`

	// --- v2 phase 4 — Strategy ----------------------------------------
	ServiceStrategy    string   `json:"service_strategy,omitempty"`     // consultative | direct | educational | soft_sell
	UpsellWhen         string   `json:"upsell_when,omitempty"`          // never | on_question | on_close | always
	DataCollection     []string `json:"data_collection,omitempty"`      // name | email | phone | cpf | address
	DataCollectionWhen string   `json:"data_collection_when,omitempty"` // upfront | on_close | on_handoff
	PrimaryGoal        string   `json:"primary_goal,omitempty"`         // book_meeting | qualify_lead | close_sale | support | educate
	ComplianceFlags    []string `json:"compliance_flags,omitempty"`     // medical_no_diagnosis | financial_no_advice | legal_no_consult

	// --- v2 phase 5 — Edge cases --------------------------------------
	AntiExamples              []AntiExample `json:"anti_examples,omitempty"`
	CommonScenarios           []string      `json:"common_scenarios,omitempty"`
	PersonalQuestionsResponse string        `json:"personal_questions_response,omitempty"` // deflect_role | friendly_redirect | strict_refuse
}

type QAExample struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// AntiExample is a "do not answer like this" pair — the Q is shown to
// the LLM along with the bad reply and the reason, so the model learns
// to avoid the pattern. Kept short (max 3 per wizard).
type AntiExample struct {
	Question  string `json:"question"`
	BadAnswer string `json:"bad_answer"`
	Why       string `json:"why"`
}

// BusinessHours captures the operational window. Times are local to
// Timezone (IANA, e.g. "America/Sao_Paulo"). Empty timezone means the
// account default applies.
type BusinessHours struct {
	WeekdaysFrom string `json:"weekdays_from,omitempty"` // HH:MM
	WeekdaysTo   string `json:"weekdays_to,omitempty"`
	SaturdayFrom string `json:"saturday_from,omitempty"`
	SaturdayTo   string `json:"saturday_to,omitempty"`
	SundayFrom   string `json:"sunday_from,omitempty"`
	SundayTo     string `json:"sunday_to,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
}

// ServiceItem is one row in the catalog. PriceRange is free-text so
// "R$ 200-450" or "from $30" both work. Description is optional but
// recommended — keeps the synthesized prompt grounded.
type ServiceItem struct {
	Name        string `json:"name"`
	PriceRange  string `json:"price_range,omitempty"`
	Description string `json:"description,omitempty"`
}

// Policies are the operator's commercial rules surfaced to the LLM so
// it can answer "como funciona o cancelamento?" without inventing.
type Policies struct {
	Cancellation   string   `json:"cancellation,omitempty"`
	Refund         string   `json:"refund,omitempty"`
	PaymentMethods []string `json:"payment_methods,omitempty"`
	Delivery       string   `json:"delivery,omitempty"`
	LgpdRequired   bool     `json:"lgpd_required,omitempty"`
}

// PromptVersion is the per-agent audit row created on each apply.
// Body holds the *previous* system_prompt so rollback is a 1-tap revert.
type PromptVersion struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	PromptID  *string   `json:"prompt_id,omitempty"` // nullable: free-text applies have no library row
	Body      string    `json:"body"`
	AppliedBy *string   `json:"applied_by,omitempty"`
	AppliedAt time.Time `json:"applied_at"`
	Note      *string   `json:"note,omitempty"`
}

var (
	ErrPromptNotFound        = &AppError{Code: "PROMPT_NOT_FOUND", Message: "prompt not found"}
	ErrPromptNameExists      = &AppError{Code: "PROMPT_NAME_EXISTS", Message: "a prompt with this name already exists"}
	ErrPromptBodyTooLarge    = &AppError{Code: "PROMPT_BODY_TOO_LARGE", Message: "prompt body exceeds the maximum allowed size"}
	ErrPromptVersionNotFound = &AppError{Code: "PROMPT_VERSION_NOT_FOUND", Message: "prompt version not found"}
)
