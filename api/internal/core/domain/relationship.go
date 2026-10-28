package domain

import (
	"context"
	"strings"
	"time"
)

// Relationship routines (§6.5, F4).
//
// The cheapest revenue in B2B is the customer you already have, and the
// cheapest way to lose them is silence. This is the small, unglamorous half
// of that: remembering a birthday, noticing that nobody has spoken to
// somebody in two months, and calling before the equipment breaks rather
// than after.
//
// Everything here goes through the approval queue, with ONE exception —
// see GreetingIsLowRisk.

// RelationshipDateKind is what a date means.
type RelationshipDateKind string

const (
	// DateBirthday is the contact's own. Recurring: only the day and month
	// matter, and storing a year would make a 60-year-old's birthday fire
	// once in 1966.
	DateBirthday RelationshipDateKind = "birthday"
	// DateFounded is the company's opening date. In Brazil it comes free
	// with the CNPJ, and "parabéns pelos 12 anos" from a supplier is a
	// message people actually answer.
	DateFounded RelationshipDateKind = "founded"
	// DateSector is a professional day: Dia do Médico, da Enfermagem, do
	// Dentista. Set per ACCOUNT rather than per contact, because a
	// distributor knows who they sell to and nobody wants to tag six
	// hundred contacts by profession.
	DateSector RelationshipDateKind = "sector"
	// DateMaintenance is a one-off: the equipment sold in March needs
	// servicing in September. Not recurring — it moves every time it is
	// done.
	DateMaintenance RelationshipDateKind = "maintenance"
	// DateCustom is whatever the salesperson wants to remember.
	DateCustom RelationshipDateKind = "custom"
)

// RelationshipDate is one thing worth remembering.
type RelationshipDate struct {
	ID        string               `json:"id"`
	AccountID string               `json:"-"`
	Kind      RelationshipDateKind `json:"kind"`

	// ContactID is empty for an account-wide date — a sector day applies to
	// the whole portfolio, and duplicating it per contact would mean six
	// hundred rows saying the same thing.
	ContactID string `json:"contact_id,omitempty"`

	// Month and Day drive a RECURRING date. Year is deliberately absent:
	// what matters about a birthday is that it comes round.
	Month int `json:"month"`
	Day   int `json:"day"`

	// DueAt drives a ONE-OFF date (maintenance). Nil on a recurring one.
	DueAt *time.Time `json:"due_at,omitempty"`

	Label string `json:"label"`
	Notes string `json:"notes,omitempty"`

	// LastFiredYear stops a recurring date from firing twice in one year —
	// a worker that ticks daily would otherwise send a birthday greeting
	// every day of the lead-time window.
	LastFiredYear int `json:"last_fired_year,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Recurring reports a date that comes round every year.
func (d *RelationshipDate) Recurring() bool {
	return d != nil && d.DueAt == nil && d.Month > 0 && d.Day > 0
}

// GreetingLeadDays is how far ahead a date is picked up.
//
// Two days, not ten: "parabéns pelo aniversário" a week early is worse than
// nothing, and the window exists only so a worker that misses a tick still
// catches the day.
const GreetingLeadDays = 2

// DueWithin reports a date worth acting on now.
//
// A recurring date is due when today is inside the window AND it has not
// already fired this year. A one-off is due once its date arrives and stays
// due until somebody handles it — a missed maintenance does not stop
// mattering because the day passed.
func (d *RelationshipDate) DueWithin(now time.Time) bool {
	if d == nil {
		return false
	}
	if d.DueAt != nil {
		return !now.Before(d.DueAt.AddDate(0, 0, -GreetingLeadDays))
	}
	if !d.Recurring() || d.LastFiredYear == now.Year() {
		return false
	}
	target := time.Date(now.Year(), time.Month(d.Month), d.Day, 0, 0, 0, 0, now.Location())
	diff := int(target.Sub(now).Hours() / 24)
	return diff >= 0 && diff <= GreetingLeadDays
}

// GreetingIsLowRisk reports a routine that autopilot may send unattended.
//
// ONLY a greeting, and only because of what it is: "parabéns pelo Dia do
// Médico" cannot misquote a price, promise a delivery date, or commit the
// company to anything. A maintenance reminder can — it implies a service the
// company may not be able to schedule — and a check-in is a sales message
// wearing a friendly hat.
//
// The channel rules still apply on top of this: autopilot is email-only, and
// that is enforced in domain.Autopilot, not here.
func GreetingIsLowRisk(kind RelationshipDateKind) bool {
	return kind == DateBirthday || kind == DateFounded || kind == DateSector
}

// SectorDate is one professional day in the Brazilian health calendar.
//
// Tagged, like everything that crosses the wire: an untagged domain struct
// serialises with Go field names and the client silently reads nulls.
type SectorDate struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Month int    `json:"month"`
	Day   int    `json:"day"`
}

// BrazilianHealthSectorDates is the catalogue an account picks from.
//
// Shipped rather than typed: nobody remembers that Dia do Fonoaudiólogo is
// the 9th of December, and a list somebody has to research is a list that
// stays empty.
var BrazilianHealthSectorDates = []SectorDate{
	{Key: "enfermagem", Label: "Dia da Enfermagem", Month: 5, Day: 12},
	{Key: "medico", Label: "Dia do Médico", Month: 10, Day: 18},
	{Key: "dentista", Label: "Dia do Dentista", Month: 10, Day: 25},
	{Key: "farmaceutico", Label: "Dia do Farmacêutico", Month: 1, Day: 20},
	{Key: "fisioterapeuta", Label: "Dia do Fisioterapeuta", Month: 10, Day: 13},
	{Key: "nutricionista", Label: "Dia do Nutricionista", Month: 8, Day: 31},
	{Key: "psicologo", Label: "Dia do Psicólogo", Month: 8, Day: 27},
	{Key: "veterinario", Label: "Dia do Veterinário", Month: 9, Day: 9},
	{Key: "biomedico", Label: "Dia do Biomédico", Month: 11, Day: 20},
	{Key: "fonoaudiologo", Label: "Dia do Fonoaudiólogo", Month: 12, Day: 9},
	{Key: "tecnico_enfermagem", Label: "Dia do Técnico de Enfermagem", Month: 5, Day: 20},
	{Key: "hospital", Label: "Dia Nacional do Hospital", Month: 7, Day: 2},
}

// SectorDateByKey looks one up. Returns nil for an unknown key rather than a
// zero value, so a typo cannot silently become a 1 January greeting.
func SectorDateByKey(key string) *SectorDate {
	k := strings.ToLower(strings.TrimSpace(key))
	for i := range BrazilianHealthSectorDates {
		if BrazilianHealthSectorDates[i].Key == k {
			return &BrazilianHealthSectorDates[i]
		}
	}
	return nil
}

// CheckInAfterDays is when silence becomes worth a message.
//
// Forty-five days: shorter and a quarterly customer gets checked on between
// every order, which reads as pestering. It is deliberately shorter than
// StaleRelationshipDays (60), because that flags a problem to a human and
// this proposes a message — the message should go out before the
// relationship is already cold.
const CheckInAfterDays = 45

// RelationshipRepository persists the dates.
type RelationshipRepository interface {
	List(ctx context.Context, accountID string) ([]*RelationshipDate, error)
	Upsert(ctx context.Context, d *RelationshipDate) error
	Delete(ctx context.Context, id, accountID string) error
	// DueDates spans accounts, for the worker.
	DueDates(ctx context.Context, now time.Time, limit int) ([]*RelationshipDate, error)
	// MarkFired stamps a recurring date so it cannot fire twice in a year,
	// or clears a one-off. Takes the instant rather than a year: the year is
	// the ACCOUNT's, and only the repository knows its timezone.
	MarkFired(ctx context.Context, id string, at time.Time) error
	// DueForCheckIn lists customers nobody has spoken to in a while.
	DueForCheckIn(ctx context.Context, silentSince time.Time, limit int) ([]*CheckInCandidate, error)
	RecordCheckIn(ctx context.Context, id, accountID, contactID string, at time.Time) error
}

// CheckInCandidate is one customer who has gone quiet.
type CheckInCandidate struct {
	AccountID     string     `json:"-"`
	ContactID     string     `json:"contact_id"`
	ContactName   string     `json:"contact_name"`
	LastContactAt *time.Time `json:"last_contact_at,omitempty"`
}
