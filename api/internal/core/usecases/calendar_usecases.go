package usecases

import (
	"context"
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// --- CreateCalendarUseCase ---

type CreateCalendarDTO struct {
	AgentID             string
	AccountID           string
	SkillID             *string         `json:"skill_id"`
	Name                string          `json:"name"`
	AvailableDays       []int32         `json:"available_days"`
	TimeSlots           json.RawMessage `json:"time_slots"`
	SlotDurationMinutes *int32          `json:"slot_duration_minutes"`
	GoogleCalendarID    *string         `json:"google_calendar_id"`
}

type CreateCalendarUseCase struct {
	agentRepo    domain.AgentRepository
	calendarRepo domain.CalendarRepository
}

func NewCreateCalendarUseCase(agentRepo domain.AgentRepository, calendarRepo domain.CalendarRepository) *CreateCalendarUseCase {
	return &CreateCalendarUseCase{agentRepo: agentRepo, calendarRepo: calendarRepo}
}

func (uc *CreateCalendarUseCase) Execute(ctx context.Context, dto CreateCalendarDTO) (*domain.Calendar, error) {
	agent, err := uc.agentRepo.GetByID(ctx, dto.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != dto.AccountID {
		return nil, domain.ErrAgentNotFound
	}

	now := time.Now()
	slotDuration := int32(30)
	if dto.SlotDurationMinutes != nil {
		slotDuration = *dto.SlotDurationMinutes
	}
	availDays := dto.AvailableDays
	if availDays == nil {
		availDays = []int32{1, 2, 3, 4, 5}
	}
	timeSlots := dto.TimeSlots
	if timeSlots == nil {
		timeSlots = json.RawMessage("[]")
	}

	cal := &domain.Calendar{
		ID:                  ulid.Make().String(),
		AgentID:             dto.AgentID,
		Name:                dto.Name,
		AvailableDays:       availDays,
		TimeSlots:           timeSlots,
		SlotDurationMinutes: slotDuration,
		GoogleCalendarID:    dto.GoogleCalendarID,
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := uc.calendarRepo.Create(ctx, cal); err != nil {
		return nil, err
	}
	return cal, nil
}

// --- UpdateCalendarUseCase ---

type UpdateCalendarDTO struct {
	CalendarID          string
	AgentID             string
	AccountID           string
	Name                *string         `json:"name"`
	AvailableDays       []int32         `json:"available_days"`
	TimeSlots           json.RawMessage `json:"time_slots"`
	SlotDurationMinutes *int32          `json:"slot_duration_minutes"`
	GoogleCalendarID    *string         `json:"google_calendar_id"`
	IsActive            *bool           `json:"is_active"`
}

type UpdateCalendarUseCase struct {
	agentRepo    domain.AgentRepository
	calendarRepo domain.CalendarRepository
}

func NewUpdateCalendarUseCase(agentRepo domain.AgentRepository, calendarRepo domain.CalendarRepository) *UpdateCalendarUseCase {
	return &UpdateCalendarUseCase{agentRepo: agentRepo, calendarRepo: calendarRepo}
}

func (uc *UpdateCalendarUseCase) Execute(ctx context.Context, dto UpdateCalendarDTO) (*domain.Calendar, error) {
	agent, err := uc.agentRepo.GetByID(ctx, dto.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != dto.AccountID {
		return nil, domain.ErrAgentNotFound
	}

	cal, err := uc.calendarRepo.GetByID(ctx, dto.CalendarID)
	if err != nil {
		return nil, err
	}
	if cal == nil || cal.AgentID != dto.AgentID {
		return nil, domain.ErrCalendarNotFound
	}

	if dto.Name != nil {
		cal.Name = *dto.Name
	}
	if dto.AvailableDays != nil {
		cal.AvailableDays = dto.AvailableDays
	}
	if dto.TimeSlots != nil {
		cal.TimeSlots = dto.TimeSlots
	}
	if dto.SlotDurationMinutes != nil {
		cal.SlotDurationMinutes = *dto.SlotDurationMinutes
	}
	if dto.GoogleCalendarID != nil {
		cal.GoogleCalendarID = dto.GoogleCalendarID
	}
	if dto.IsActive != nil {
		cal.IsActive = *dto.IsActive
	}
	cal.UpdatedAt = time.Now()

	if err := uc.calendarRepo.Update(ctx, cal); err != nil {
		return nil, err
	}
	return cal, nil
}

// --- DeleteCalendarUseCase ---

type DeleteCalendarUseCase struct {
	agentRepo    domain.AgentRepository
	calendarRepo domain.CalendarRepository
}

func NewDeleteCalendarUseCase(agentRepo domain.AgentRepository, calendarRepo domain.CalendarRepository) *DeleteCalendarUseCase {
	return &DeleteCalendarUseCase{agentRepo: agentRepo, calendarRepo: calendarRepo}
}

func (uc *DeleteCalendarUseCase) Execute(ctx context.Context, accountID, agentID, calendarID string) error {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.AccountID != accountID {
		return domain.ErrAgentNotFound
	}

	cal, err := uc.calendarRepo.GetByID(ctx, calendarID)
	if err != nil {
		return err
	}
	if cal == nil || cal.AgentID != agentID {
		return domain.ErrCalendarNotFound
	}

	return uc.calendarRepo.Delete(ctx, calendarID)
}

// --- ListCalendarsUseCase ---

type ListCalendarsUseCase struct {
	agentRepo    domain.AgentRepository
	calendarRepo domain.CalendarRepository
}

func NewListCalendarsUseCase(agentRepo domain.AgentRepository, calendarRepo domain.CalendarRepository) *ListCalendarsUseCase {
	return &ListCalendarsUseCase{agentRepo: agentRepo, calendarRepo: calendarRepo}
}

func (uc *ListCalendarsUseCase) Execute(ctx context.Context, accountID, agentID string) ([]*domain.Calendar, error) {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != accountID {
		return nil, domain.ErrAgentNotFound
	}

	return uc.calendarRepo.ListByAgentID(ctx, agentID)
}
