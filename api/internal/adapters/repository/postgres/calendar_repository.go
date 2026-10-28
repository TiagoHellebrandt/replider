package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- CalendarRepository ---

type CalendarRepositoryPG struct {
	queries *Queries
}

func NewCalendarRepository(db DBTX) *CalendarRepositoryPG {
	return &CalendarRepositoryPG{queries: New(db)}
}

func (r *CalendarRepositoryPG) Create(ctx context.Context, cal *domain.Calendar) error {
	q := getQueries(ctx, r.queries)
	return q.CreateCalendar(ctx, CreateCalendarParams{
		ID:                  cal.ID,
		AgentID:             cal.AgentID,
		Name:                cal.Name,
		AvailableDays:       cal.AvailableDays,
		TimeSlots:           toBytes(cal.TimeSlots),
		SlotDurationMinutes: cal.SlotDurationMinutes,
		GoogleCalendarID:    cal.GoogleCalendarID,
		IsActive:            &cal.IsActive,
		CreatedAt:           pgtype.Timestamptz{Time: cal.CreatedAt, Valid: true},
		UpdatedAt:           pgtype.Timestamptz{Time: cal.UpdatedAt, Valid: true},
	})
}

func (r *CalendarRepositoryPG) GetByID(ctx context.Context, id string) (*domain.Calendar, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetCalendarByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapCalendarToDomain(row), nil
}

func (r *CalendarRepositoryPG) ListByAgentID(ctx context.Context, agentID string) ([]*domain.Calendar, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListCalendarsByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	cals := make([]*domain.Calendar, len(rows))
	for i, row := range rows {
		cals[i] = mapCalendarToDomain(row)
	}
	return cals, nil
}

func (r *CalendarRepositoryPG) Update(ctx context.Context, cal *domain.Calendar) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateCalendar(ctx, UpdateCalendarParams{
		ID:                  cal.ID,
		Name:                cal.Name,
		AvailableDays:       cal.AvailableDays,
		TimeSlots:           toBytes(cal.TimeSlots),
		SlotDurationMinutes: cal.SlotDurationMinutes,
		GoogleCalendarID:    cal.GoogleCalendarID,
		IsActive:            &cal.IsActive,
		UpdatedAt:           pgtype.Timestamptz{Time: cal.UpdatedAt, Valid: true},
	})
}

func (r *CalendarRepositoryPG) Delete(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteCalendar(ctx, id)
}

func mapCalendarToDomain(row Calendar) *domain.Calendar {
	// JSONB column comes back as []byte from pgx. Wrap it in
	// json.RawMessage so the API serializes the stored JSON verbatim
	// instead of base64-encoding the raw bytes — the dashboard was
	// parsing the response as a string ("W10=") and failing to render
	// the calendar list.
	var ts interface{}
	if len(row.TimeSlots) > 0 {
		ts = json.RawMessage(row.TimeSlots)
	} else {
		ts = json.RawMessage("[]")
	}
	return &domain.Calendar{
		ID:                  row.ID,
		AgentID:             row.AgentID,
		Name:                row.Name,
		AvailableDays:       row.AvailableDays,
		TimeSlots:           ts,
		SlotDurationMinutes: row.SlotDurationMinutes,
		GoogleCalendarID:    row.GoogleCalendarID,
		IsActive:            *row.IsActive,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}
