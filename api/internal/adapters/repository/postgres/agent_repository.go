package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- AgentRepository ---

type AgentRepository struct {
	queries *Queries
}

func NewAgentRepository(db DBTX) *AgentRepository {
	return &AgentRepository{queries: New(db)}
}

func (r *AgentRepository) Create(ctx context.Context, agent *domain.Agent) error {
	q := getQueries(ctx, r.queries)
	return q.CreateAgent(ctx, CreateAgentParams{
		ID:              agent.ID,
		AccountID:       agent.AccountID,
		Name:            agent.Name,
		SystemPrompt:    agent.SystemPrompt,
		Provider:        agent.Provider,
		Model:           agent.Model,
		Timezone:        agent.Timezone,
		IsActive:        &agent.IsActive,
		CreatedAt:       pgtype.Timestamptz{Time: agent.CreatedAt, Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: agent.UpdatedAt, Valid: true},
		HideAiIdentity:  agent.HideAIIdentity,
		SilentOnUnknown: agent.SilentOnUnknown,
	})
}

func (r *AgentRepository) GetByID(ctx context.Context, id string) (*domain.Agent, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAgentByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	learnFromHumans := false
	if row.LearnFromHumans != nil {
		learnFromHumans = *row.LearnFromHumans
	}
	return &domain.Agent{
		ID:                 row.ID,
		AccountID:          row.AccountID,
		Name:               row.Name,
		SystemPrompt:       row.SystemPrompt,
		PersonalityPrompt:  row.PersonalityPrompt,
		LearnFromHumans:    learnFromHumans,
		Provider:           row.Provider,
		Model:              row.Model,
		Timezone:           row.Timezone,
		BusinessHoursStart: row.BusinessHoursStart,
		BusinessHoursEnd:   row.BusinessHoursEnd,
		BusinessHoursDays:  int(row.BusinessHoursDays),
		HideAIIdentity:     row.HideAiIdentity,
		SilentOnUnknown:    row.SilentOnUnknown,
		IsActive:           *row.IsActive,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (r *AgentRepository) Update(ctx context.Context, agent *domain.Agent) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateAgent(ctx, UpdateAgentParams{
		ID:                 agent.ID,
		Name:               agent.Name,
		SystemPrompt:       agent.SystemPrompt,
		PersonalityPrompt:  agent.PersonalityPrompt,
		LearnFromHumans:    &agent.LearnFromHumans,
		Provider:           agent.Provider,
		Model:              agent.Model,
		Timezone:           agent.Timezone,
		IsActive:           &agent.IsActive,
		BusinessHoursStart: agent.BusinessHoursStart,
		BusinessHoursEnd:   agent.BusinessHoursEnd,
		BusinessHoursDays:  int16(agent.BusinessHoursDays),
		HideAiIdentity:     agent.HideAIIdentity,
		SilentOnUnknown:    agent.SilentOnUnknown,
		UpdatedAt:          pgtype.Timestamptz{Time: agent.UpdatedAt, Valid: true},
	})
}

func (r *AgentRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Agent, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAgentsByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	agents := make([]*domain.Agent, len(rows))
	for i, row := range rows {
		lfh := false
		if row.LearnFromHumans != nil {
			lfh = *row.LearnFromHumans
		}
		agents[i] = &domain.Agent{
			ID:                 row.ID,
			AccountID:          row.AccountID,
			Name:               row.Name,
			SystemPrompt:       row.SystemPrompt,
			PersonalityPrompt:  row.PersonalityPrompt,
			LearnFromHumans:    lfh,
			Provider:           row.Provider,
			Model:              row.Model,
			Timezone:           row.Timezone,
			BusinessHoursStart: row.BusinessHoursStart,
			BusinessHoursEnd:   row.BusinessHoursEnd,
			BusinessHoursDays:  int(row.BusinessHoursDays),
			HideAIIdentity:     row.HideAiIdentity,
			SilentOnUnknown:    row.SilentOnUnknown,
			IsActive:           *row.IsActive,
			CreatedAt:          row.CreatedAt.Time,
			UpdatedAt:          row.UpdatedAt.Time,
		}
	}
	return agents, nil
}

func (r *AgentRepository) Delete(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteAgent(ctx, id)
}

func (r *AgentRepository) ListAllTimezones(ctx context.Context) ([]*domain.AgentTimezone, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAllAgentTimezones(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AgentTimezone, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.AgentTimezone{
			ID:        row.ID,
			AccountID: row.AccountID,
			Name:      row.Name,
			Timezone:  row.Timezone,
		})
	}
	return out, nil
}
