package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateCalendarUseCase_Execute(t *testing.T) {
	t.Run("should successfully create a calendar", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Calendar")).Return(nil)

		uc := NewCreateCalendarUseCase(agentRepo, calendarRepo)
		dto := CreateCalendarDTO{
			AgentID:   "agent-1",
			AccountID: "account-1",
			Name:      "Agenda Principal",
		}

		cal, err := uc.Execute(context.Background(), dto)

		assert.NoError(t, err)
		assert.NotNil(t, cal)
		assert.Equal(t, "Agenda Principal", cal.Name)
		assert.Equal(t, "agent-1", cal.AgentID)
		assert.Equal(t, int32(30), cal.SlotDurationMinutes)
		assert.Equal(t, []int32{1, 2, 3, 4, 5}, cal.AvailableDays)
		assert.True(t, cal.IsActive)
		calendarRepo.AssertExpectations(t)
	})

	t.Run("should fail if agent not found or wrong account", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agentRepo.On("GetByID", mock.Anything, "agent-1").Return((*domain.Agent)(nil), nil)

		uc := NewCreateCalendarUseCase(agentRepo, calendarRepo)
		dto := CreateCalendarDTO{
			AgentID:   "agent-1",
			AccountID: "account-1",
			Name:      "Test",
		}

		_, err := uc.Execute(context.Background(), dto)
		assert.Equal(t, domain.ErrAgentNotFound, err)
	})

	t.Run("should use custom slot duration and available days", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Calendar")).Return(nil)

		customDuration := int32(60)
		uc := NewCreateCalendarUseCase(agentRepo, calendarRepo)
		dto := CreateCalendarDTO{
			AgentID:             "agent-1",
			AccountID:           "account-1",
			Name:                "Custom Calendar",
			SlotDurationMinutes: &customDuration,
			AvailableDays:       []int32{1, 3, 5},
			TimeSlots:           json.RawMessage(`[{"start":"09:00","end":"12:00"}]`),
		}

		cal, err := uc.Execute(context.Background(), dto)

		assert.NoError(t, err)
		assert.Equal(t, int32(60), cal.SlotDurationMinutes)
		assert.Equal(t, []int32{1, 3, 5}, cal.AvailableDays)
	})
}

func TestUpdateCalendarUseCase_Execute(t *testing.T) {
	t.Run("should update calendar fields", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		existing := &domain.Calendar{
			ID:                  "cal-1",
			AgentID:             "agent-1",
			Name:                "Old Name",
			SlotDurationMinutes: 30,
			AvailableDays:       []int32{1, 2, 3, 4, 5},
			IsActive:            true,
		}

		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("GetByID", mock.Anything, "cal-1").Return(existing, nil)
		calendarRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Calendar")).Return(nil)

		newName := "New Name"
		isActive := false

		uc := NewUpdateCalendarUseCase(agentRepo, calendarRepo)
		dto := UpdateCalendarDTO{
			CalendarID: "cal-1",
			AgentID:    "agent-1",
			AccountID:  "account-1",
			Name:       &newName,
			IsActive:   &isActive,
		}

		cal, err := uc.Execute(context.Background(), dto)

		assert.NoError(t, err)
		assert.Equal(t, "New Name", cal.Name)
		assert.False(t, cal.IsActive)
	})

	t.Run("should fail if calendar not found", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("GetByID", mock.Anything, "cal-1").Return((*domain.Calendar)(nil), nil)

		uc := NewUpdateCalendarUseCase(agentRepo, calendarRepo)
		dto := UpdateCalendarDTO{
			CalendarID: "cal-1",
			AgentID:    "agent-1",
			AccountID:  "account-1",
		}

		_, err := uc.Execute(context.Background(), dto)
		assert.Equal(t, domain.ErrCalendarNotFound, err)
	})
}

func TestDeleteCalendarUseCase_Execute(t *testing.T) {
	t.Run("should delete calendar owned by agent", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		cal := &domain.Calendar{ID: "cal-1", AgentID: "agent-1"}

		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("GetByID", mock.Anything, "cal-1").Return(cal, nil)
		calendarRepo.On("Delete", mock.Anything, "cal-1").Return(nil)

		uc := NewDeleteCalendarUseCase(agentRepo, calendarRepo)
		err := uc.Execute(context.Background(), "account-1", "agent-1", "cal-1")

		assert.NoError(t, err)
		calendarRepo.AssertExpectations(t)
	})

	t.Run("should fail if calendar belongs to different agent", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		cal := &domain.Calendar{ID: "cal-1", AgentID: "agent-other"}

		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("GetByID", mock.Anything, "cal-1").Return(cal, nil)

		uc := NewDeleteCalendarUseCase(agentRepo, calendarRepo)
		err := uc.Execute(context.Background(), "account-1", "agent-1", "cal-1")

		assert.Equal(t, domain.ErrCalendarNotFound, err)
	})
}

func TestListCalendarsUseCase_Execute(t *testing.T) {
	t.Run("should list calendars for agent owned by account", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "account-1"}
		cals := []*domain.Calendar{
			{ID: "cal-1", AgentID: "agent-1", Name: "Calendar 1"},
			{ID: "cal-2", AgentID: "agent-1", Name: "Calendar 2"},
		}

		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
		calendarRepo.On("ListByAgentID", mock.Anything, "agent-1").Return(cals, nil)

		uc := NewListCalendarsUseCase(agentRepo, calendarRepo)
		result, err := uc.Execute(context.Background(), "account-1", "agent-1")

		assert.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("should fail if agent owned by different account", func(t *testing.T) {
		agentRepo := new(MockAgentRepo)
		calendarRepo := new(MockCalendarRepo)

		agent := &domain.Agent{ID: "agent-1", AccountID: "other-account"}
		agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)

		uc := NewListCalendarsUseCase(agentRepo, calendarRepo)
		_, err := uc.Execute(context.Background(), "account-1", "agent-1")

		assert.Equal(t, domain.ErrAgentNotFound, err)
	})
}
