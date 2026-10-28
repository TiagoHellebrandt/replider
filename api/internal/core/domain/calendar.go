package domain

import (
	"context"
	"time"
)

// Calendar represents a scheduling calendar linked to an agent
type Calendar struct {
	ID                  string      `json:"id"`
	AgentID             string      `json:"agent_id"`
	Name                string      `json:"name"`
	AvailableDays       []int32     `json:"available_days"`
	TimeSlots           interface{} `json:"time_slots" swaggertype:"object"`
	SlotDurationMinutes int32       `json:"slot_duration_minutes"`
	GoogleCalendarID    *string     `json:"google_calendar_id,omitempty"`
	IsActive            bool        `json:"is_active"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

// CalendarRepository defines persistence operations for Calendars
type CalendarRepository interface {
	Create(ctx context.Context, cal *Calendar) error
	GetByID(ctx context.Context, id string) (*Calendar, error)
	ListByAgentID(ctx context.Context, agentID string) ([]*Calendar, error)
	Update(ctx context.Context, cal *Calendar) error
	Delete(ctx context.Context, id string) error
}
