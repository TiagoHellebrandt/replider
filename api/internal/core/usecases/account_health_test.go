package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAccountHealthRepo struct{ mock.Mock }

func (m *mockAccountHealthRepo) ListAccountHealthMetrics(ctx context.Context, now time.Time) ([]domain.AccountHealthMetrics, error) {
	args := m.Called(ctx, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AccountHealthMetrics), args.Error(1)
}

func TestAccountHealth_HealthyAccountScoresHigh(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	seen := now.Add(-12 * time.Hour) // seen today → full login recency
	repo := new(mockAccountHealthRepo)
	repo.On("ListAccountHealthMetrics", mock.Anything, mock.Anything).Return([]domain.AccountHealthMetrics{
		{
			AccountID:             "acc-healthy",
			Name:                  "Healthy",
			MessagesThisWeek:      80, // above full-score volume → full 40
			MessagesPrevWeek:      75,
			ConversationsThisWeek: 10,
			TakenOverThisWeek:     0, // 0% takeover → full 20
			ConversationsPrevWeek: 10,
			TakenOverPrevWeek:     0,
			HasActiveAgent:        true, // → 20
			LastSeenAt:            &seen,
		},
	}, nil)

	uc := NewAccountHealthUseCase(repo)
	uc.now = func() time.Time { return now }

	scores, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, scores, 1)

	s := scores[0]
	assert.Equal(t, 100, s.Score)
	assert.Equal(t, 40, s.Breakdown.Messages)
	assert.Equal(t, 20, s.Breakdown.BotActive)
	assert.Equal(t, 20, s.Breakdown.Takeover)
	assert.Equal(t, 20, s.Breakdown.LoginRecency)
	assert.False(t, s.AtRisk)
	assert.Equal(t, 0, s.TakeoverPct)
	require.NotNil(t, s.DaysSinceLogin)
	assert.Equal(t, 0, *s.DaysSinceLogin)
}

func TestAccountHealth_FlagsWeekOverWeekDrop(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	seen := now.Add(-1 * time.Hour)
	repo := new(mockAccountHealthRepo)
	repo.On("ListAccountHealthMetrics", mock.Anything, mock.Anything).Return([]domain.AccountHealthMetrics{
		{
			AccountID:             "acc-drop",
			Name:                  "Dropping",
			MessagesThisWeek:      5,  // collapsed from 50
			MessagesPrevWeek:      50, // was at full messages weight
			ConversationsThisWeek: 0,
			ConversationsPrevWeek: 0,
			HasActiveAgent:        true,
			LastSeenAt:            &seen,
		},
	}, nil)

	uc := NewAccountHealthUseCase(repo)
	uc.now = func() time.Time { return now }

	scores, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, scores, 1)

	s := scores[0]
	// prev: 40+20+20+20 = 100; now: 4+20+20+20 = 64 → 36% drop ≥ 30%.
	assert.Equal(t, 100, s.PrevScore)
	assert.Equal(t, 64, s.Score)
	assert.GreaterOrEqual(t, s.DropPct, HealthDropAlertThreshold)
	assert.True(t, s.AtRisk)
}

func TestAccountHealth_SortsAtRiskAndWorstFirst(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	seen := now.Add(-1 * time.Hour)
	repo := new(mockAccountHealthRepo)
	repo.On("ListAccountHealthMetrics", mock.Anything, mock.Anything).Return([]domain.AccountHealthMetrics{
		{AccountID: "acc-healthy", Name: "H", MessagesThisWeek: 50, MessagesPrevWeek: 50, HasActiveAgent: true, LastSeenAt: &seen},
		{AccountID: "acc-atrisk", Name: "R", MessagesThisWeek: 0, MessagesPrevWeek: 50, HasActiveAgent: true, LastSeenAt: &seen},
		{AccountID: "acc-mid", Name: "M", MessagesThisWeek: 10, MessagesPrevWeek: 12, HasActiveAgent: false},
	}, nil)

	uc := NewAccountHealthUseCase(repo)
	uc.now = func() time.Time { return now }

	scores, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, scores, 3)

	// At-risk account floats to the top regardless of absolute score.
	assert.Equal(t, "acc-atrisk", scores[0].AccountID)
	assert.True(t, scores[0].AtRisk)
	// Remaining ordered worst-score first.
	assert.LessOrEqual(t, scores[1].Score, scores[2].Score)
}

func TestAccountHealth_NeverLoggedInZeroesRecency(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	repo := new(mockAccountHealthRepo)
	repo.On("ListAccountHealthMetrics", mock.Anything, mock.Anything).Return([]domain.AccountHealthMetrics{
		{AccountID: "acc-cold", Name: "Cold", HasActiveAgent: false, LastSeenAt: nil},
	}, nil)

	uc := NewAccountHealthUseCase(repo)
	uc.now = func() time.Time { return now }

	scores, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, scores, 1)

	s := scores[0]
	// No messages, no agent, no convs (full takeover weight), never seen.
	assert.Equal(t, 0, s.Breakdown.Messages)
	assert.Equal(t, 0, s.Breakdown.BotActive)
	assert.Equal(t, 20, s.Breakdown.Takeover)
	assert.Equal(t, 0, s.Breakdown.LoginRecency)
	assert.Nil(t, s.DaysSinceLogin)
	assert.False(t, s.AtRisk) // no prior-week score → no drop
}
