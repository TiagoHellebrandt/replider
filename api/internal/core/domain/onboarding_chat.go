package domain

import (
	"context"
	"strings"
	"time"
)

// Conversational onboarding (§6.20, K5).
//
// The wizard asks for a company name, an agent prompt, a knowledge base and a
// channel before anything happens — so somebody evaluating the product supplies
// all of it before seeing a single lead. That ordering is why activation is the
// bottleneck: the value arrives after the work rather than during it.
//
// A conversation inverts it. One question at a time, each answer immediately
// worth something, and the whole thing resumable.
//
// Two rules shape everything below:
//
//  1. The LADDER is fixed and the machine walks it. A model deciding what to ask
//     next is a model that can wander, and the whole point is to finish.
//  2. It works with NO AI at all. The model only ever turns prose into fields and
//     proposes audiences; every step has a manual path, because an onboarding
//     that breaks when a provider is down is an onboarding that loses signups.

// OnboardingStep is where the conversation is.
type OnboardingStep string

const (
	// StepCompany asks who they are. The only genuinely required answer:
	// everything downstream is written in this company's voice.
	StepCompany OnboardingStep = "company"
	// StepProduct asks what they sell.
	StepProduct OnboardingStep = "product"
	// StepAudiences proposes who to sell it to. The one step with a real model
	// call behind it, and the one step that is skippable.
	StepAudiences OnboardingStep = "audiences"
	// StepChannel connects a way to talk.
	StepChannel OnboardingStep = "channel"
	// StepDone means the account can prospect.
	StepDone OnboardingStep = "done"
)

// onboardingLadder is the fixed order. One place, so "what comes next" cannot
// disagree between the server and the screen.
var onboardingLadder = []OnboardingStep{
	StepCompany, StepProduct, StepAudiences, StepChannel, StepDone,
}

// NextOnboardingStep returns the step after this one.
//
// An unknown step resolves to the FIRST rather than to done: a corrupted value
// should restart the conversation, not silently declare it finished and leave an
// account with nothing configured.
func NextOnboardingStep(current OnboardingStep) OnboardingStep {
	for i, step := range onboardingLadder {
		if step == current {
			if i+1 < len(onboardingLadder) {
				return onboardingLadder[i+1]
			}
			return StepDone
		}
	}
	return StepCompany
}

// ValidOnboardingStep guards a value arriving from a client.
func ValidOnboardingStep(v string) bool {
	for _, step := range onboardingLadder {
		if string(step) == v {
			return true
		}
	}
	return false
}

// OnboardingAnswers is what the conversation has gathered.
//
// Every field optional, because at any moment most of them are. The struct is
// the draft; nothing here is written to the real tables until the step that owns
// it completes.
type OnboardingAnswers struct {
	CompanyName string `json:"company_name,omitempty"`
	// Website is the highest-leverage single input: one URL usually answers
	// what they sell and to whom, which is why the company step asks for it.
	Website     string `json:"website,omitempty"`
	CompanyWhat string `json:"company_what,omitempty"`

	ProductName        string `json:"product_name,omitempty"`
	ProductDescription string `json:"product_description,omitempty"`
	ProductPriceCents  int64  `json:"product_price_cents,omitempty"`

	// AudienceNames are the ones the person kept. Kept as names rather than ids
	// because they are created at the end of the step, and a draft that
	// referenced rows that do not exist yet would be a draft that cannot be
	// resumed.
	AudienceNames []string `json:"audience_names,omitempty"`

	ChannelType string `json:"channel_type,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
}

// OnboardingMessage is one turn of the transcript.
type OnboardingMessage struct {
	// Role is "assistant" or "user". Not a model conversation — the assistant
	// turns are the machine's scripted questions, and storing them keeps the
	// screen from having to re-derive what was already asked.
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// OnboardingChat is the whole persisted state.
type OnboardingChat struct {
	AccountID string              `json:"-"`
	Step      OnboardingStep      `json:"step"`
	Answers   OnboardingAnswers   `json:"answers"`
	Messages  []OnboardingMessage `json:"messages"`

	StartedAt   time.Time  `json:"started_at"`
	FirstLeadAt *time.Time `json:"first_lead_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Done reports a finished conversation.
func (c *OnboardingChat) Done() bool {
	return c != nil && c.Step == StepDone
}

// TimeToFirstLead is the activation metric, or nil when no lead has landed yet.
//
// Nil rather than zero, and rather than "time so far": an account still in its
// first hour is not an account with a time-to-first-lead of one hour, and
// averaging the in-progress ones in would make the number improve every time
// somebody signs up and does nothing.
func (c *OnboardingChat) TimeToFirstLead() *time.Duration {
	if c == nil || c.FirstLeadAt == nil {
		return nil
	}
	d := c.FirstLeadAt.Sub(c.StartedAt)
	if d < 0 {
		// A lead that predates the conversation belongs to an account that
		// arrived here after already prospecting. Reporting a negative
		// activation time would be worse than reporting none.
		return nil
	}
	return &d
}

// CanAdvance reports whether the current step has what it needs.
//
// Only the company step genuinely blocks. Everything else can be skipped and
// filled in later from its own screen — an onboarding that refuses to move
// because somebody has not decided on a price yet is an onboarding people
// abandon at the price question.
func (c *OnboardingChat) CanAdvance() bool {
	if c == nil {
		return false
	}
	if c.Step == StepCompany {
		return strings.TrimSpace(c.Answers.CompanyName) != ""
	}
	return true
}

// OnboardingPrompt is the question the machine asks at a step, and what the
// screen should render beside it.
type OnboardingPrompt struct {
	Step OnboardingStep `json:"step"`
	Text string         `json:"text"`
	// Widget tells the client what to draw: a plain input, editable chips, or
	// the channel tiles. The server decides, so a new step does not need a
	// client release to be answerable.
	Widget string `json:"widget"`
	// Skippable is false only for the company step.
	Skippable bool `json:"skippable"`
}

// Onboarding widget kinds.
const (
	WidgetText    = "text"
	WidgetSite    = "site"
	WidgetChips   = "chips"
	WidgetChannel = "channel"
	WidgetDone    = "done"
)

// PromptFor returns the question for a step.
//
// The copy lives here rather than in the client for one reason: the machine owns
// the ladder, and a question that disagrees with the step it belongs to is how a
// conversation asks for something it already has.
func PromptFor(step OnboardingStep, answers OnboardingAnswers) OnboardingPrompt {
	switch step {
	case StepCompany:
		return OnboardingPrompt{
			Step:   StepCompany,
			Text:   "Para começar: qual é o nome da sua empresa, e o site se tiver? Com o site eu já entendo o que vocês vendem.",
			Widget: WidgetSite,
		}
	case StepProduct:
		name := answers.CompanyName
		if name == "" {
			name = "vocês"
		}
		return OnboardingPrompt{
			Step:      StepProduct,
			Text:      "O que " + name + " vende? Pode escrever do seu jeito — eu organizo.",
			Widget:    WidgetText,
			Skippable: true,
		}
	case StepAudiences:
		return OnboardingPrompt{
			Step:      StepAudiences,
			Text:      "Separei alguns públicos que fazem sentido. Tire os que não servem, edite os que quase servem.",
			Widget:    WidgetChips,
			Skippable: true,
		}
	case StepChannel:
		return OnboardingPrompt{
			Step:      StepChannel,
			Text:      "Falta por onde falar com eles. Qual canal você usa hoje?",
			Widget:    WidgetChannel,
			Skippable: true,
		}
	default:
		return OnboardingPrompt{
			Step:   StepDone,
			Text:   "Pronto. Agora é só trazer os primeiros leads — eu te mostro como.",
			Widget: WidgetDone,
		}
	}
}

// OnboardingChatRepository persists the conversation.
type OnboardingChatRepository interface {
	// Get returns the account's chat, or (nil, nil) when it has not started.
	Get(ctx context.Context, accountID string) (*OnboardingChat, error)
	// Save upserts the whole state. One write per turn: the state is small and
	// a partial update would need a column per field, which is exactly what the
	// JSONB draft avoids.
	Save(ctx context.Context, chat *OnboardingChat) error
	// StampFirstLead records the activation moment, once. Idempotent: the first
	// lead is the first lead, and a second write would restart the clock.
	StampFirstLead(ctx context.Context, accountID string, at time.Time) error
}
