package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type queuedRoutine struct {
	accountID, contactID, source, subject, text string
	lowRisk                                     bool
}

type fakeRelationshipDrafter struct {
	queued []queuedRoutine
	err    error
}

func (f *fakeRelationshipDrafter) QueueRelationshipApproach(_ context.Context,
	accountID, contactID, source, subject, text string, lowRisk bool) error {
	if f.err != nil {
		return f.err
	}
	f.queued = append(f.queued, queuedRoutine{accountID, contactID, source, subject, text, lowRisk})
	return nil
}

type fakeRelationshipRepo struct {
	dates      []*domain.RelationshipDate
	checkIns   []*domain.CheckInCandidate
	fired      []string
	recorded   []string
	datesErr   error
	markErr    error
	checkInErr error
}

func (f *fakeRelationshipRepo) List(context.Context, string) ([]*domain.RelationshipDate, error) {
	return f.dates, nil
}
func (f *fakeRelationshipRepo) Upsert(context.Context, *domain.RelationshipDate) error { return nil }
func (f *fakeRelationshipRepo) Delete(context.Context, string, string) error           { return nil }
func (f *fakeRelationshipRepo) DueDates(_ context.Context, _ time.Time, _ int) ([]*domain.RelationshipDate, error) {
	return f.dates, f.datesErr
}
func (f *fakeRelationshipRepo) MarkFired(_ context.Context, id string, _ time.Time) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.fired = append(f.fired, id)
	return nil
}
func (f *fakeRelationshipRepo) DueForCheckIn(_ context.Context, _ time.Time, _ int) ([]*domain.CheckInCandidate, error) {
	return f.checkIns, f.checkInErr
}
func (f *fakeRelationshipRepo) RecordCheckIn(_ context.Context, _, _, contactID string, _ time.Time) error {
	f.recorded = append(f.recorded, contactID)
	return nil
}

func worker(repo *fakeRelationshipRepo, drafter RelationshipDrafter) *RelationshipWorker {
	return NewRelationshipWorker(repo, drafter).
		WithClock(func() time.Time { return time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC) })
}

func TestRelationshipWorker_QueuesABirthdayAsLowRisk(t *testing.T) {
	repo := &fakeRelationshipRepo{dates: []*domain.RelationshipDate{
		{ID: "d1", AccountID: "acc", ContactID: "c1", Kind: domain.DateBirthday, Month: 5, Day: 12},
	}}
	drafter := &fakeRelationshipDrafter{}
	worker(repo, drafter).RunOnce(context.Background())

	if len(drafter.queued) != 1 {
		t.Fatalf("expected one queued routine, got %d", len(drafter.queued))
	}
	if !drafter.queued[0].lowRisk {
		t.Fatal("a birthday greeting is the one routine autopilot may send")
	}
	if len(repo.fired) != 1 {
		t.Fatal("the date must be stamped so it cannot fire twice this year")
	}
}

// A maintenance reminder implies a service the company may not be able to
// schedule. It never goes out unread.
func TestRelationshipWorker_MaintenanceIsNeverLowRisk(t *testing.T) {
	due := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	repo := &fakeRelationshipRepo{dates: []*domain.RelationshipDate{
		{ID: "d1", AccountID: "acc", ContactID: "c1", Kind: domain.DateMaintenance,
			DueAt: &due, Label: "autoclave"},
	}}
	drafter := &fakeRelationshipDrafter{}
	worker(repo, drafter).RunOnce(context.Background())

	if drafter.queued[0].lowRisk {
		t.Fatal("a maintenance reminder must wait for a person")
	}
	// And it asks rather than schedules: the company may not have a
	// technician free, and a message that promises one has to be walked back.
	if !strings.Contains(drafter.queued[0].text, "Quer que eu veja uma data") {
		t.Fatalf("expected the message to ask, got %q", drafter.queued[0].text)
	}
}

// Firing an account-wide date would message the entire portfolio from a
// background worker. That is a broadcast, and a broadcast belongs where
// somebody sees the count before it goes out.
func TestRelationshipWorker_SkipsAccountWideDates(t *testing.T) {
	repo := &fakeRelationshipRepo{dates: []*domain.RelationshipDate{
		{ID: "d1", AccountID: "acc", Kind: domain.DateSector, Month: 5, Day: 12,
			Label: "Dia da Enfermagem"},
	}}
	drafter := &fakeRelationshipDrafter{}
	worker(repo, drafter).RunOnce(context.Background())

	if len(drafter.queued) != 0 {
		t.Fatalf("an account-wide date must not fan out, got %+v", drafter.queued)
	}
	if len(repo.fired) != 0 {
		t.Fatal("nothing was queued, so nothing may be stamped as fired")
	}
}

// A date marked as fired that never reached the queue is a birthday nobody
// ever wishes.
func TestRelationshipWorker_DoesNotStampWhatItCouldNotQueue(t *testing.T) {
	repo := &fakeRelationshipRepo{dates: []*domain.RelationshipDate{
		{ID: "d1", AccountID: "acc", ContactID: "c1", Kind: domain.DateBirthday, Month: 5, Day: 12},
	}}
	drafter := &fakeRelationshipDrafter{err: errors.New("no funnel")}
	worker(repo, drafter).RunOnce(context.Background())

	if len(repo.fired) != 0 {
		t.Fatal("a failed queue must leave the date due")
	}
}

// One customer failing is a fact about that customer, not about the batch.
func TestRelationshipWorker_OneFailureDoesNotCostTheBatch(t *testing.T) {
	repo := &fakeRelationshipRepo{
		dates: []*domain.RelationshipDate{
			{ID: "d1", AccountID: "acc", ContactID: "c1", Kind: domain.DateBirthday, Month: 5, Day: 12},
			{ID: "d2", AccountID: "acc", ContactID: "c2", Kind: domain.DateBirthday, Month: 5, Day: 12},
		},
	}
	drafter := &countingDrafter{failOn: "c1"}
	worker(repo, drafter).RunOnce(context.Background())

	if drafter.ok != 1 {
		t.Fatalf("expected the second customer to still be queued, got %d", drafter.ok)
	}
}

type countingDrafter struct {
	failOn string
	ok     int
}

func (d *countingDrafter) QueueRelationshipApproach(_ context.Context,
	_, contactID, _, _, _ string, _ bool) error {
	if contactID == d.failOn {
		return errors.New("boom")
	}
	d.ok++
	return nil
}

func TestRelationshipWorker_QueuesACheckInForASilentCustomer(t *testing.T) {
	last := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeRelationshipRepo{checkIns: []*domain.CheckInCandidate{
		{AccountID: "acc", ContactID: "c1", ContactName: "Maria Silva", LastContactAt: &last},
	}}
	drafter := &fakeRelationshipDrafter{}
	worker(repo, drafter).RunOnce(context.Background())

	if len(drafter.queued) != 1 {
		t.Fatalf("expected one check-in, got %d", len(drafter.queued))
	}
	// A check-in is a sales message wearing a friendly hat.
	if drafter.queued[0].lowRisk {
		t.Fatal("a check-in must never be sent unattended")
	}
	if !strings.Contains(drafter.queued[0].text, "Maria") {
		t.Fatalf("expected the first name, got %q", drafter.queued[0].text)
	}
	if len(repo.recorded) != 1 {
		t.Fatal("the nudge must be recorded so the customer is not asked daily")
	}
}

// "Faz 0 dias" is worse than saying nothing, and a customer whose history
// predates the system must not be told it has been twenty thousand days.
func TestCheckInMessage_OnlyCountsWhenTheCountIsUseful(t *testing.T) {
	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)

	ancient := now.AddDate(-4, 0, 0)
	msg := CheckInMessage(&domain.CheckInCandidate{ContactName: "Ana", LastContactAt: &ancient}, now)
	if strings.Contains(msg, "dias") && strings.Contains(msg, "1461") {
		t.Fatalf("a four-year gap must not be quoted in days: %q", msg)
	}

	none := CheckInMessage(&domain.CheckInCandidate{ContactName: "Ana"}, now)
	if strings.Contains(none, "cerca de") {
		t.Fatalf("with no last contact there is no number to quote: %q", none)
	}
}

// "Parabéns, Maria Aparecida da Silva Santos" is a form letter.
func TestRelationshipMessage_UsesTheFirstNameOnly(t *testing.T) {
	d := &domain.RelationshipDate{Kind: domain.DateBirthday}
	msg := RelationshipMessage(d, "Maria Aparecida da Silva Santos", time.Now())

	if !strings.HasPrefix(msg, "Olá, Maria!") {
		t.Fatalf("expected a first-name greeting, got %q", msg)
	}
}

func TestRelationshipMessage_NoNameStillGreets(t *testing.T) {
	d := &domain.RelationshipDate{Kind: domain.DateBirthday}
	if msg := RelationshipMessage(d, "", time.Now()); !strings.HasPrefix(msg, "Olá!") {
		t.Fatalf("expected a nameless greeting, got %q", msg)
	}
}

// An unwired worker must be inert, not a panic on the first tick.
func TestRelationshipWorker_UnwiredIsInert(t *testing.T) {
	(&RelationshipWorker{}).RunOnce(context.Background())
}
