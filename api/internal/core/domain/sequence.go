package domain

import (
	"context"
	"strings"
	"time"
)

// Cadences (§1.5, E1).
//
// A cadence is a plan for following up: write, wait, write again, and at some
// point put it in front of a human. It exists because the single biggest
// reason a B2B deal dies is that nobody followed up a third time — and the
// second biggest is that somebody followed up eleven times.
//
// Two rules shape everything here, and both are enforced in the domain rather
// than in the editor:
//
//   - It STOPS when the lead replies. A cadence that keeps firing after a
//     human answered is not persistence, it is a machine talking over
//     somebody — and it is the fastest way to lose a deal that was working.
//   - Cold WhatsApp needs an approved template. Meta bans numbers for this,
//     and the ban takes the channel the customer's whole business runs on.

// SequenceStepKind is what one step does.
type SequenceStepKind string

const (
	StepEmail    SequenceStepKind = "email"
	StepWhatsApp SequenceStepKind = "whatsapp"
	// StepSMS is opt-in only. Cold SMS in Brazil is both ineffective and a
	// Procon complaint waiting to happen.
	StepSMS SequenceStepKind = "sms"
	// StepHumanTask is the step that makes a cadence worth having: "ligar",
	// "pedir intro no LinkedIn". The machine schedules it and a person does
	// it, which is the only honest way to model the part that works best.
	StepHumanTask SequenceStepKind = "human_task"
	// StepWait is a pause. Explicit rather than an interval on the previous
	// step, because "espere 3 dias" is a decision somebody makes and should
	// be able to see.
	StepWait SequenceStepKind = "wait"
)

// SequenceStep is one entry in the plan.
type SequenceStep struct {
	ID    string           `json:"id"`
	Order int              `json:"order"`
	Kind  SequenceStepKind `json:"kind"`

	// DelayHours is how long AFTER the previous step this one fires. On a
	// wait step it is the whole content of the step.
	DelayHours int `json:"delay_hours"`

	// TemplateID points at an email template, or at an approved WhatsApp
	// template name for a WhatsApp step. Empty on a human task or a wait.
	TemplateID string `json:"template_id,omitempty"`
	// Subject and Body let a step carry its own text instead of a template.
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`

	// TaskTitle is what the human is asked to do. Required on a human task:
	// "tarefa" with no instruction is a notification nobody can action.
	TaskTitle string `json:"task_title,omitempty"`
}

// Sequence is the whole plan.
type Sequence struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`
	FunnelID  string `json:"funnel_id,omitempty"`

	Name  string         `json:"name"`
	Steps []SequenceStep `json:"steps"`

	// StopOnReply defaults TRUE and there is deliberately no UI to turn it
	// off. It is exposed as a field only so the rule is visible in the
	// payload the editor renders at the top of the screen.
	StopOnReply bool `json:"stop_on_reply"`
	// BusinessHoursOnly holds sends until the next working hour. A cold
	// email at 03:14 reads as a machine, which is exactly what it must not.
	BusinessHoursOnly bool `json:"business_hours_only"`

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SequenceViolation is one reason a cadence cannot run.
//
// Returned as a list rather than the first failure: an editor that reports
// one problem per save turns a five-step cadence into five round trips.
type SequenceViolation struct {
	StepOrder int    `json:"step_order"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Violation codes. Stable strings because the editor highlights by them.
const (
	ViolationWhatsAppNeedsTemplate = "WHATSAPP_COLD_NEEDS_TEMPLATE"
	ViolationEmptyStep             = "STEP_HAS_NO_CONTENT"
	ViolationTaskNeedsTitle        = "TASK_NEEDS_TITLE"
	ViolationNoSteps               = "SEQUENCE_HAS_NO_STEPS"
	ViolationOpensWithWhatsApp     = "OPENS_WITH_WHATSAPP"
	ViolationWaitIsLast            = "ENDS_WITH_WAIT"
)

// ColdWhatsAppMaxPosition is how far into a cadence WhatsApp may appear
// without the lead having engaged.
//
// The §1.5 rule: steps 1 and 2 are email, because email costs nothing and
// burns no reputation. WhatsApp is for when the lead has already replied or
// their score has risen — spending a template send (and a slice of the
// number's standing with Meta) on somebody who has not reacted to two emails
// is paying the most for the least likely.
const ColdWhatsAppMaxPosition = 2

// Validate reports everything wrong with a cadence.
//
// Run on SAVE, not only on activation. A cadence saved broken and discovered
// broken a week later, when it silently did nothing, is worse than one that
// refuses to save.
func (s *Sequence) Validate(approvedWhatsAppTemplates map[string]bool) []SequenceViolation {
	var out []SequenceViolation
	if len(s.Steps) == 0 {
		return []SequenceViolation{{
			Code:    ViolationNoSteps,
			Message: "adicione ao menos um passo",
		}}
	}

	for i, step := range s.Steps {
		position := i + 1
		switch step.Kind {
		case StepWhatsApp:
			// The rule that protects the customer's number. A WhatsApp step
			// this early is by definition cold — the lead has not replied to
			// anything yet — so it needs an approved template or it cannot
			// be delivered at all.
			if position <= ColdWhatsAppMaxPosition {
				out = append(out, SequenceViolation{
					StepOrder: position,
					Code:      ViolationOpensWithWhatsApp,
					Message: "os primeiros passos devem ser e-mail: WhatsApp frio gasta " +
						"template e reputação do número com quem ainda não reagiu a nada",
				})
			}
			if !approvedWhatsAppTemplates[strings.TrimSpace(step.TemplateID)] {
				out = append(out, SequenceViolation{
					StepOrder: position,
					Code:      ViolationWhatsAppNeedsTemplate,
					Message: "escolha um template aprovado pela Meta — sem ele a mensagem " +
						"não é entregue e o número corre risco de banimento",
				})
			}
		case StepEmail, StepSMS:
			if strings.TrimSpace(step.TemplateID) == "" &&
				strings.TrimSpace(step.Body) == "" {
				out = append(out, SequenceViolation{
					StepOrder: position,
					Code:      ViolationEmptyStep,
					Message:   "escreva a mensagem ou escolha um modelo",
				})
			}
		case StepHumanTask:
			if strings.TrimSpace(step.TaskTitle) == "" {
				out = append(out, SequenceViolation{
					StepOrder: position,
					Code:      ViolationTaskNeedsTitle,
					Message:   "descreva o que a pessoa deve fazer",
				})
			}
		}
	}

	// A cadence that ends on a wait ends by doing nothing, forever. Almost
	// always a step somebody deleted and did not notice.
	if last := s.Steps[len(s.Steps)-1]; last.Kind == StepWait {
		out = append(out, SequenceViolation{
			StepOrder: len(s.Steps),
			Code:      ViolationWaitIsLast,
			Message:   "a cadência termina numa espera — o último passo não faz nada",
		})
	}
	return out
}

// Runnable reports a cadence with nothing blocking it.
func (s *Sequence) Runnable(approvedWhatsAppTemplates map[string]bool) bool {
	return len(s.Validate(approvedWhatsAppTemplates)) == 0
}

// DefaultSequence is the §1.5 shape, offered when somebody creates their
// first cadence.
//
// Two emails then a human task, and no WhatsApp at all: the channel that
// costs nothing goes first, and the one that costs a number's reputation is
// something a person decides to use after seeing a reaction.
func DefaultSequence(name string) *Sequence {
	return &Sequence{
		Name: name, StopOnReply: true, BusinessHoursOnly: true, IsActive: false,
		Steps: []SequenceStep{
			{Order: 1, Kind: StepEmail, DelayHours: 0},
			{Order: 2, Kind: StepWait, DelayHours: 72},
			{Order: 3, Kind: StepEmail, DelayHours: 0},
			{Order: 4, Kind: StepWait, DelayHours: 120},
			{Order: 5, Kind: StepHumanTask, TaskTitle: "Ligar para o contato"},
		},
	}
}

// ── Enrollment ────────────────────────────────────────────────────────────

// EnrollmentStatus is where a lead is in a cadence.
type EnrollmentStatus string

const (
	EnrollmentActive EnrollmentStatus = "active"
	// EnrollmentReplied is the good ending. Recorded distinctly from
	// "finished" because a cadence that stops early because somebody
	// answered is the outcome it exists for, and counting it as completion
	// would hide the only number that matters.
	EnrollmentReplied   EnrollmentStatus = "replied"
	EnrollmentFinished  EnrollmentStatus = "finished"
	EnrollmentCancelled EnrollmentStatus = "cancelled"
	// EnrollmentPaused is a human hitting stop, on the sequence or the lead.
	EnrollmentPaused EnrollmentStatus = "paused"
)

// SequenceEnrollment is one lead walking through one cadence.
type SequenceEnrollment struct {
	ID         string `json:"id"`
	AccountID  string `json:"-"`
	SequenceID string `json:"sequence_id"`
	LeadID     string `json:"lead_id"`

	// CurrentStep is the NEXT step to run, 1-based. Zero means nothing has
	// run yet.
	CurrentStep int              `json:"current_step"`
	Status      EnrollmentStatus `json:"status"`

	// NextRunAt is when the worker should look at this again. Nil for a
	// finished or paused enrollment.
	NextRunAt *time.Time `json:"next_run_at,omitempty"`

	EnrolledAt time.Time  `json:"enrolled_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
}

// Active reports an enrollment the worker should still advance.
func (e *SequenceEnrollment) Active() bool {
	return e != nil && e.Status == EnrollmentActive
}

// BusinessHoursStart and End bound when a cadence may send.
//
// 9 to 18 local: a cold email at 03:14 reads as a machine, which is exactly
// what it must not, and one at 19:30 lands under tomorrow's inbox anyway.
const (
	BusinessHoursStart = 9
	BusinessHoursEnd   = 18
)

// NextSendWindow moves a send time into working hours.
//
// Returns the time unchanged when it is already inside the window, and never
// moves a send EARLIER — a cadence that fires ahead of schedule to catch the
// window would arrive before the wait somebody configured.
func NextSendWindow(at time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	local := at.In(loc)

	for {
		switch {
		case local.Weekday() == time.Saturday:
			local = startOfNextDay(local).AddDate(0, 0, 1)
		case local.Weekday() == time.Sunday:
			local = startOfNextDay(local)
		case local.Hour() < BusinessHoursStart:
			local = time.Date(local.Year(), local.Month(), local.Day(),
				BusinessHoursStart, 0, 0, 0, loc)
		case local.Hour() >= BusinessHoursEnd:
			local = startOfNextDay(local)
		default:
			return local
		}
	}
}

// startOfNextDay returns 09:00 tomorrow, in the same location.
func startOfNextDay(t time.Time) time.Time {
	next := t.AddDate(0, 0, 1)
	return time.Date(next.Year(), next.Month(), next.Day(),
		BusinessHoursStart, 0, 0, 0, t.Location())
}

// SequenceRepository persists cadences and who is walking through them.
type SequenceRepository interface {
	List(ctx context.Context, accountID string) ([]*Sequence, error)
	Get(ctx context.Context, id, accountID string) (*Sequence, error)
	Upsert(ctx context.Context, s *Sequence) error
	Delete(ctx context.Context, id, accountID string) error

	Enroll(ctx context.Context, e *SequenceEnrollment) error
	// GetEnrollment finds a lead's active enrollment, if any. One at a time:
	// a lead in two cadences gets two follow-ups a week from the same
	// company, which reads as disorganised at best.
	GetEnrollment(ctx context.Context, accountID, leadID string) (*SequenceEnrollment, error)
	// DueEnrollments lists what the worker should act on now, across
	// accounts.
	DueEnrollments(ctx context.Context, now time.Time, limit int) ([]*SequenceEnrollment, error)
	UpdateEnrollment(ctx context.Context, e *SequenceEnrollment) error
	// StopForLead ends every active enrollment for a lead. Called when they
	// reply — the rule that makes a cadence a follow-up rather than a
	// machine talking over somebody.
	StopForLead(ctx context.Context, accountID, leadID string, status EnrollmentStatus, at time.Time) error

	// StepStats is the per-step funnel: how many reached each step and how
	// many replied after it.
	StepStats(ctx context.Context, sequenceID string) ([]SequenceStepStat, error)
}

// SequenceStepStat is one step's performance.
type SequenceStepStat struct {
	StepOrder int `json:"step_order"`
	// Reached counts enrollments that got this far.
	Reached int `json:"reached"`
	// Replied counts the ones that answered AFTER this step and before the
	// next — which is what "este passo funciona" actually means.
	Replied int `json:"replied"`
}

// ReplyRate is nil when nothing reached this step: zero replies out of zero
// is unknown, not zero, and rendering 0% would condemn a step nobody has run.
func (s SequenceStepStat) ReplyRate() *float64 {
	if s.Reached == 0 {
		return nil
	}
	r := float64(s.Replied) / float64(s.Reached)
	return &r
}
