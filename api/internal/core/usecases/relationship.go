package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// RelationshipUseCase is the CRUD behind the relationship routines (§6.5,
// F4). The worker acts on what this stores.
type RelationshipUseCase struct {
	repo domain.RelationshipRepository
	now  func() time.Time
}

func NewRelationshipUseCase(repo domain.RelationshipRepository) *RelationshipUseCase {
	return &RelationshipUseCase{repo: repo, now: time.Now}
}

func (uc *RelationshipUseCase) WithClock(now func() time.Time) *RelationshipUseCase {
	uc.now = now
	return uc
}

// List returns the account's dates, soonest first.
func (uc *RelationshipUseCase) List(ctx context.Context, accountID string) ([]*domain.RelationshipDate, error) {
	dates, err := uc.repo.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if dates == nil {
		dates = []*domain.RelationshipDate{}
	}
	return dates, nil
}

// RelationshipDateInput is what a client may set. Deliberately not the domain
// struct: LastFiredYear is the worker's, and letting a client reset it would
// let anybody send the same birthday greeting twice.
type RelationshipDateInput struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	ContactID string     `json:"contact_id"`
	Month     int        `json:"month"`
	Day       int        `json:"day"`
	DueAt     *time.Time `json:"due_at"`
	Label     string     `json:"label"`
	Notes     string     `json:"notes"`
	// SectorKey fills month/day from the shipped catalogue, so nobody has to
	// look up that Dia do Fonoaudiólogo is the 9th of December.
	SectorKey string `json:"sector_key"`
}

// Save creates or updates one date.
func (uc *RelationshipUseCase) Save(ctx context.Context, accountID string, in RelationshipDateInput) (*domain.RelationshipDate, error) {
	kind := domain.RelationshipDateKind(strings.TrimSpace(strings.ToLower(in.Kind)))
	switch kind {
	case domain.DateBirthday, domain.DateFounded, domain.DateSector,
		domain.DateMaintenance, domain.DateCustom:
	default:
		return nil, &domain.AppError{
			Code: "RELATIONSHIP_INVALID_KIND", Message: "tipo de data inválido"}
	}

	month, day := in.Month, in.Day
	label := strings.TrimSpace(in.Label)
	if key := strings.TrimSpace(in.SectorKey); key != "" {
		sector := domain.SectorDateByKey(key)
		if sector == nil {
			return nil, &domain.AppError{
				Code: "RELATIONSHIP_UNKNOWN_SECTOR", Message: "data do setor desconhecida"}
		}
		month, day = sector.Month, sector.Day
		if label == "" {
			label = sector.Label
		}
	}

	// One or the other, never neither — a row with no date is a row the
	// worker can never act on, and the DB constraint would reject it with a
	// message nobody can read.
	if in.DueAt == nil && (month <= 0 || day <= 0) {
		return nil, &domain.AppError{
			Code: "RELATIONSHIP_DATE_REQUIRED", Message: "informe uma data (dia e mês, ou uma data única)"}
	}
	if in.DueAt != nil && (month > 0 || day > 0) {
		// Both would make "when is this due?" ambiguous, and the worker has
		// to pick one — better to make the caller pick.
		return nil, &domain.AppError{
			Code: "RELATIONSHIP_DATE_AMBIGUOUS", Message: "use dia e mês OU uma data única, não os dois"}
	}
	if in.DueAt == nil && !validMonthDay(month, day) {
		return nil, &domain.AppError{
			Code: "RELATIONSHIP_DATE_INVALID", Message: "dia ou mês inválido"}
	}

	date := &domain.RelationshipDate{
		ID:        strings.TrimSpace(in.ID),
		AccountID: accountID,
		Kind:      kind,
		ContactID: strings.TrimSpace(in.ContactID),
		Month:     month,
		Day:       day,
		DueAt:     in.DueAt,
		Label:     label,
		Notes:     strings.TrimSpace(in.Notes),
	}
	if date.ID == "" {
		date.ID = ulid.Make().String()
	}
	if err := uc.repo.Upsert(ctx, date); err != nil {
		return nil, err
	}
	return date, nil
}

func (uc *RelationshipUseCase) Delete(ctx context.Context, accountID, id string) error {
	return uc.repo.Delete(ctx, id, accountID)
}

// Sectors exposes the shipped catalogue so the screen can offer it.
func (uc *RelationshipUseCase) Sectors() []domain.SectorDate {
	return domain.BrazilianHealthSectorDates
}

// validMonthDay rejects the impossible without a calendar.
//
// 29 February is ACCEPTED: a person born on it has a birthday, and refusing
// to store it would be the system telling them they do not. The due-date
// query rolls it to 1 March in a non-leap year (to_date, not make_date —
// see RelationshipRepository.DueDates).
func validMonthDay(month, day int) bool {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return false
	}
	switch time.Month(month) {
	case time.February:
		return day <= 29
	case time.April, time.June, time.September, time.November:
		return day <= 30
	}
	return true
}
