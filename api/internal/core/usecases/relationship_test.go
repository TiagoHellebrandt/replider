package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type stubRelationshipRepo struct {
	saved   []*domain.RelationshipDate
	deleted [][2]string
}

func (s *stubRelationshipRepo) List(context.Context, string) ([]*domain.RelationshipDate, error) {
	return s.saved, nil
}
func (s *stubRelationshipRepo) Upsert(_ context.Context, d *domain.RelationshipDate) error {
	s.saved = append(s.saved, d)
	return nil
}
func (s *stubRelationshipRepo) Delete(_ context.Context, id, accountID string) error {
	s.deleted = append(s.deleted, [2]string{id, accountID})
	return nil
}
func (s *stubRelationshipRepo) DueDates(context.Context, time.Time, int) ([]*domain.RelationshipDate, error) {
	return nil, nil
}
func (s *stubRelationshipRepo) MarkFired(context.Context, string, time.Time) error { return nil }
func (s *stubRelationshipRepo) DueForCheckIn(context.Context, time.Time, int) ([]*domain.CheckInCandidate, error) {
	return nil, nil
}
func (s *stubRelationshipRepo) RecordCheckIn(context.Context, string, string, string, time.Time) error {
	return nil
}

func TestRelationshipSave_FillsTheDateFromTheSectorCatalogue(t *testing.T) {
	repo := &stubRelationshipRepo{}
	uc := NewRelationshipUseCase(repo)

	date, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "sector", SectorKey: "medico",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Nobody remembers that Dia do Médico is 18 October — that is the point
	// of shipping the catalogue.
	if date.Month != 10 || date.Day != 18 {
		t.Fatalf("expected 18 October, got %d/%d", date.Day, date.Month)
	}
	if date.Label != "Dia do Médico" {
		t.Fatalf("expected the catalogue label, got %q", date.Label)
	}
}

// A typo must not silently become a 1 January greeting to the whole
// portfolio.
func TestRelationshipSave_RejectsAnUnknownSector(t *testing.T) {
	uc := NewRelationshipUseCase(&stubRelationshipRepo{})

	_, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "sector", SectorKey: "medicos",
	})
	if err == nil {
		t.Fatal("an unknown sector key must be refused")
	}
}

func TestRelationshipSave_RequiresADate(t *testing.T) {
	uc := NewRelationshipUseCase(&stubRelationshipRepo{})

	// A row with no date is a row the worker can never act on.
	if _, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "birthday",
	}); err == nil {
		t.Fatal("a date with no date must be refused")
	}
}

// Both would make "when is this due?" ambiguous, and the worker has to pick
// one — better to make the caller pick.
func TestRelationshipSave_RejectsBothKindsOfDate(t *testing.T) {
	uc := NewRelationshipUseCase(&stubRelationshipRepo{})
	at := time.Now()

	if _, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "maintenance", Month: 5, Day: 12, DueAt: &at,
	}); err == nil {
		t.Fatal("a recurring AND one-off date must be refused")
	}
}

func TestRelationshipSave_RejectsAnImpossibleDay(t *testing.T) {
	uc := NewRelationshipUseCase(&stubRelationshipRepo{})

	if _, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "birthday", Month: 2, Day: 30,
	}); err == nil {
		t.Fatal("30 February must be refused")
	}
}

// Somebody born on 29 February has a birthday, and refusing to store it would
// be the system telling them they do not.
func TestRelationshipSave_AcceptsTheTwentyNinthOfFebruary(t *testing.T) {
	uc := NewRelationshipUseCase(&stubRelationshipRepo{})

	if _, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "birthday", Month: 2, Day: 29,
	}); err != nil {
		t.Fatalf("29 February must be accepted: %v", err)
	}
}

func TestRelationshipSave_RejectsAnUnknownKind(t *testing.T) {
	uc := NewRelationshipUseCase(&stubRelationshipRepo{})

	if _, err := uc.Save(context.Background(), "acc", RelationshipDateInput{
		Kind: "whatever", Month: 5, Day: 12,
	}); err == nil {
		t.Fatal("an unknown kind must be refused")
	}
}

func TestRelationshipDelete_IsScopedToTheAccount(t *testing.T) {
	repo := &stubRelationshipRepo{}
	uc := NewRelationshipUseCase(repo)

	if err := uc.Delete(context.Background(), "acc", "d1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.deleted[0] != [2]string{"d1", "acc"} {
		t.Fatalf("delete must carry the account, got %v", repo.deleted[0])
	}
}
