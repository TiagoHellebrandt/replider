package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// RelationshipWorker keeps existing customers from going quiet (§6.5, F4).
//
// The cheapest revenue in B2B is the customer you already have, and the
// cheapest way to lose them is silence. This is the unglamorous half of
// that: remembering a birthday, noticing that nobody has spoken to somebody
// in six weeks, and calling before the equipment breaks rather than after.
//
// Everything it produces lands in the approval queue. The ONE exception is a
// greeting on an account running autopilot, and even that is decided by
// domain.GreetingIsLowRisk + domain.Autopilot.CanAutoSendGreeting rather than
// here — see RelationshipDrafter.
type RelationshipWorker struct {
	repo    domain.RelationshipRepository
	drafter RelationshipDrafter
	// contacts resolves a name so a greeting can address somebody. Optional:
	// without it the messages simply drop the name rather than not going out.
	contacts RelationshipContactLookup

	interval time.Duration
	batch    int
	now      func() time.Time
	// runLog stamps completed passes so a boot decides honestly whether the
	// 24h tick was missed (daily deploys). Optional — see WithRunLog.
	runLog domain.WorkerRunRepository
}

// relationshipWorkerName keys the worker_runs stamp.
const relationshipWorkerName = "relationship"

// WithRunLog wires the worker_runs stamp (boot pass when the last completed
// pass is older than the interval).
func (w *RelationshipWorker) WithRunLog(r domain.WorkerRunRepository) *RelationshipWorker {
	w.runLog = r
	return w
}

// RelationshipDrafter queues one routine, and sends it when — and only when —
// the domain says an unattended greeting is allowed.
//
// A narrow port: the worker knows nothing about funnels, channels, autopilot
// settings or send budgets. It decides WHAT is worth saying and to whom; the
// use case decides whether saying it needs a person.
type RelationshipDrafter interface {
	QueueRelationshipApproach(ctx context.Context,
		accountID, contactID, source, subject, text string, lowRisk bool) error
}

// RelationshipContactLookup is the one contact field a greeting needs.
type RelationshipContactLookup interface {
	GetByExternalID(ctx context.Context, accountID, externalID string) (*domain.Contact, error)
}

func NewRelationshipWorker(repo domain.RelationshipRepository, drafter RelationshipDrafter) *RelationshipWorker {
	return &RelationshipWorker{
		repo: repo, drafter: drafter,
		// Daily. A birthday is a date, not an event: checking four times a
		// day would find the same one four times and only the LastFiredYear
		// guard would stop it going out four times.
		interval: 24 * time.Hour,
		batch:    200,
		now:      time.Now,
	}
}

func (w *RelationshipWorker) WithInterval(d time.Duration) *RelationshipWorker {
	w.interval = d
	return w
}

func (w *RelationshipWorker) WithClock(now func() time.Time) *RelationshipWorker {
	w.now = now
	return w
}

func (w *RelationshipWorker) WithContacts(c RelationshipContactLookup) *RelationshipWorker {
	w.contacts = c
	return w
}

// Start runs the loop until the context is cancelled.
func (w *RelationshipWorker) Start(ctx context.Context) {
	if w.repo == nil || w.drafter == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// No unconditional immediate pass, same reason as the reorder and
	// autopilot workers: a deploy loop that restarts three times in a minute
	// must not become three rounds of messages. But a 24h ticker under daily
	// deploys never fires at all, so the boot pass runs when the stamped
	// last pass is older than the interval (or there is none).
	if bootPassDue(ctx, w.runLog, relationshipWorkerName, w.interval, w.now()) {
		w.RunOnce(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce does one pass over both routines.
func (w *RelationshipWorker) RunOnce(ctx context.Context) {
	if w.repo == nil || w.drafter == nil {
		return
	}
	w.runDates(ctx)
	w.runCheckIns(ctx)
	stampRun(ctx, w.runLog, relationshipWorkerName, w.now())
}

// runDates handles the calendar half: birthdays, anniversaries, maintenance.
func (w *RelationshipWorker) runDates(ctx context.Context) {
	now := w.now()
	dates, err := w.repo.DueDates(ctx, now, w.batch)
	if err != nil {
		slog.ErrorContext(ctx, "relationship: could not read the dates", "error", err)
		return
	}

	var queued int
	for _, d := range dates {
		if d == nil {
			continue
		}
		// An account-wide date (a sector day) has nobody to address. Firing
		// it would mean messaging the entire portfolio from a background
		// worker, which is a broadcast — and a broadcast belongs to the
		// campaign path, where somebody chooses the audience and sees the
		// count before it goes out. These surface on the screen as upcoming
		// instead.
		if strings.TrimSpace(d.ContactID) == "" {
			continue
		}

		text := RelationshipMessage(d, w.contactName(ctx, d.AccountID, d.ContactID), now)
		lowRisk := domain.GreetingIsLowRisk(d.Kind)
		err := w.drafter.QueueRelationshipApproach(ctx, d.AccountID, d.ContactID,
			"relationship", RelationshipSubject(d), text, lowRisk)
		if err != nil {
			// One customer failing is a fact about that customer — a missing
			// funnel, an opted-out contact — and must not cost the batch.
			slog.WarnContext(ctx, "relationship: could not queue a date",
				"account_id", d.AccountID, "kind", string(d.Kind), "error", err)
			continue
		}
		// Stamped AFTER the queue accepted it: a date marked as fired that
		// never reached the queue is a birthday nobody ever wishes.
		if err := w.repo.MarkFired(ctx, d.ID, now); err != nil {
			slog.WarnContext(ctx, "relationship: queued but could not stamp the date — it may repeat",
				"id", d.ID, "error", err)
		}
		queued++
	}
	if queued > 0 {
		slog.InfoContext(ctx, "relationship: dates queued", "count", queued)
	}
}

// runCheckIns handles the silence half.
func (w *RelationshipWorker) runCheckIns(ctx context.Context) {
	now := w.now()
	silentSince := now.AddDate(0, 0, -domain.CheckInAfterDays)

	candidates, err := w.repo.DueForCheckIn(ctx, silentSince, w.batch)
	if err != nil {
		slog.ErrorContext(ctx, "relationship: could not read the quiet customers", "error", err)
		return
	}

	var queued int
	for _, c := range candidates {
		if c == nil {
			continue
		}
		text := CheckInMessage(c, now)
		// lowRisk is FALSE, always. A check-in is a sales message wearing a
		// friendly hat: "faz um tempo que não conversamos, precisa de algo?"
		// is an offer, and an offer nobody read is how a supplier starts
		// sounding automated.
		err := w.drafter.QueueRelationshipApproach(ctx, c.AccountID, c.ContactID,
			"check_in", "Passando para saber como estão as coisas", text, false)
		if err != nil {
			slog.WarnContext(ctx, "relationship: could not queue a check-in",
				"account_id", c.AccountID, "contact_id", c.ContactID, "error", err)
			continue
		}
		if err := w.repo.RecordCheckIn(ctx, ulid.Make().String(),
			c.AccountID, c.ContactID, now); err != nil {
			slog.WarnContext(ctx, "relationship: queued but could not record the check-in — the customer may be asked twice",
				"account_id", c.AccountID, "contact_id", c.ContactID, "error", err)
		}
		queued++
	}
	if queued > 0 {
		slog.InfoContext(ctx, "relationship: check-ins queued", "count", queued)
	}
}

// contactName resolves who we are writing to, best effort.
func (w *RelationshipWorker) contactName(ctx context.Context, accountID, contactID string) string {
	if w.contacts == nil {
		return ""
	}
	contact, err := w.contacts.GetByExternalID(ctx, accountID, contactID)
	if err != nil || contact == nil || contact.Name == nil {
		return ""
	}
	return strings.TrimSpace(*contact.Name)
}

// RelationshipMessage writes the draft a human reviews.
//
// Deliberately not generated by a model, for the same reason as the reorder
// draft: it is the same one fact every time, and a model would spend a call
// rephrasing it into something less specific — with a chance of inventing a
// detail about somebody's birthday.
//
// The first name only. "Parabéns, Maria Aparecida da Silva Santos" is a form
// letter; "Parabéns, Maria" is a message.
func RelationshipMessage(d *domain.RelationshipDate, contactName string, now time.Time) string {
	name := firstName(contactName)
	greeting := "Olá!"
	if name != "" {
		greeting = fmt.Sprintf("Olá, %s!", name)
	}

	switch d.Kind {
	case domain.DateBirthday:
		return greeting + " Passando só para desejar um feliz aniversário e um ótimo dia. Um abraço da nossa equipe!"

	case domain.DateFounded:
		// The number of years, when the label carries it. Without a founding
		// year we congratulate without counting rather than guess a number
		// the customer will notice is wrong.
		return greeting + " Parabéns pelo aniversário da empresa! É um prazer fazer parte dessa história."

	case domain.DateSector:
		what := strings.TrimSpace(d.Label)
		if what == "" {
			what = "o dia da profissão de vocês"
		}
		return fmt.Sprintf("%s Hoje é %s — parabéns pelo trabalho de vocês. Conte com a gente!",
			greeting, what)

	case domain.DateMaintenance:
		what := strings.TrimSpace(d.Label)
		if what == "" {
			what = "o equipamento que vocês compraram com a gente"
		}
		// Honest about being a reminder, and asks rather than schedules —
		// the company may not have a technician free this week, and a
		// message that promises one is a message somebody has to walk back.
		return fmt.Sprintf(
			"%s Chegou a época da manutenção preventiva de %s. Quer que eu veja uma data com a equipe técnica?",
			greeting, what)

	default:
		what := strings.TrimSpace(d.Label)
		if what == "" {
			return greeting + " Passando para falar com vocês."
		}
		return fmt.Sprintf("%s Passando para falar sobre %s.", greeting, what)
	}
}

// RelationshipSubject is the email subject line for a date.
//
// Email needs one and the body is not it: a greeting whose subject repeats
// the whole message reads as a mailshot.
func RelationshipSubject(d *domain.RelationshipDate) string {
	switch d.Kind {
	case domain.DateBirthday:
		return "Feliz aniversário!"
	case domain.DateFounded:
		return "Parabéns pelo aniversário da empresa"
	case domain.DateSector:
		if label := strings.TrimSpace(d.Label); label != "" {
			return "Parabéns pelo " + label
		}
		return "Parabéns pela data"
	case domain.DateMaintenance:
		return "Manutenção preventiva"
	default:
		if label := strings.TrimSpace(d.Label); label != "" {
			return label
		}
		return "Uma mensagem da nossa equipe"
	}
}

// CheckInMessage writes the "we have not spoken in a while" draft.
func CheckInMessage(c *domain.CheckInCandidate, now time.Time) string {
	name := firstName(c.ContactName)
	greeting := "Olá!"
	if name != "" {
		greeting = fmt.Sprintf("Olá, %s!", name)
	}

	// Say how long only when we know. "Faz 0 dias" is worse than saying
	// nothing, and a customer whose last contact predates the system would
	// otherwise be told it has been twenty thousand days.
	if c.LastContactAt != nil {
		days := int(now.Sub(*c.LastContactAt).Hours() / 24)
		if days > 0 && days < 365 {
			return fmt.Sprintf(
				"%s Vi que faz cerca de %d dias que não conversamos. Está tudo certo por aí? Se precisar de qualquer coisa é só me chamar.",
				greeting, days)
		}
	}
	return greeting + " Faz um tempo que não conversamos. Está tudo certo por aí? Se precisar de qualquer coisa é só me chamar."
}

// firstName takes the first word of a name, and nothing when there isn't one.
func firstName(full string) string {
	fields := strings.Fields(strings.TrimSpace(full))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
