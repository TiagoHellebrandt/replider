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

type MockNPSRepo struct{ mock.Mock }

func (m *MockNPSRepo) Create(ctx context.Context, r *domain.NPSResponse) error {
	return m.Called(ctx, r).Error(0)
}

func (m *MockNPSRepo) Summarize(ctx context.Context, since time.Time) (domain.NPSSummary, error) {
	args := m.Called(ctx, since)
	return args.Get(0).(domain.NPSSummary), args.Error(1)
}

func (m *MockNPSRepo) RecentComments(ctx context.Context, since time.Time, limit int) ([]domain.NPSComment, error) {
	args := m.Called(ctx, since, limit)
	return args.Get(0).([]domain.NPSComment), args.Error(1)
}

func (m *MockNPSRepo) IsPromptEligible(ctx context.Context, accountID, quarter string, createdBefore time.Time) (bool, error) {
	args := m.Called(ctx, accountID, quarter, createdBefore)
	return args.Bool(0), args.Error(1)
}

func (m *MockNPSRepo) StampPromptHandled(ctx context.Context, accountID, quarter string) error {
	return m.Called(ctx, accountID, quarter).Error(0)
}

func TestSubmitNPS_PersistsValidScore(t *testing.T) {
	repo := new(MockNPSRepo)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.NPSResponse) bool {
		return r.AccountID == "acc-1" && r.UserID == "user-1" && r.Score == 9 &&
			r.Comment == "loved it" && r.ID != ""
	})).Return(nil)
	repo.On("StampPromptHandled", mock.Anything, "acc-1", mock.Anything).Return(nil)

	uc := NewSubmitNPSUseCase(repo)
	err := uc.Execute(context.Background(), "acc-1", "user-1", 9, "  loved it  ")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	// Submitting stamps the prompt as handled so it isn't shown again.
	repo.AssertCalled(t, "StampPromptHandled", mock.Anything, "acc-1", mock.Anything)
}

func TestSubmitNPS_RejectsOutOfRangeScore(t *testing.T) {
	repo := new(MockNPSRepo)
	uc := NewSubmitNPSUseCase(repo)

	for _, score := range []int{-1, 11, 100} {
		err := uc.Execute(context.Background(), "acc-1", "user-1", score, "")
		assert.ErrorIs(t, err, domain.ErrInvalidNPSScore)
	}
	// Create must never be reached for an invalid score.
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSubmitNPS_TruncatesLongComment(t *testing.T) {
	repo := new(MockNPSRepo)
	long := make([]byte, npsCommentMaxLen+500)
	for i := range long {
		long[i] = 'a'
	}
	repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.NPSResponse) bool {
		return len(r.Comment) == npsCommentMaxLen
	})).Return(nil)
	repo.On("StampPromptHandled", mock.Anything, "acc-1", mock.Anything).Return(nil)

	uc := NewSubmitNPSUseCase(repo)
	require.NoError(t, uc.Execute(context.Background(), "acc-1", "user-1", 5, string(long)))
	repo.AssertExpectations(t)
}

func TestNPSMetrics_ComputesScoreAndDefaultsWindow(t *testing.T) {
	repo := new(MockNPSRepo)
	summary := domain.NPSSummary{
		Total: 10, Promoters: 6, Passives: 2, Detractors: 2, Score: 40, AverageScore: 8.1,
	}
	comments := []domain.NPSComment{{AccountID: "acc-1", Score: 9, Comment: "great"}}

	// Default window is 90 days (one quarter); RecentComments capped at 50.
	repo.On("Summarize", mock.Anything, mock.Anything).Return(summary, nil)
	repo.On("RecentComments", mock.Anything, mock.Anything, 50).Return(comments, nil)

	uc := NewNPSMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 0)

	require.NoError(t, err)
	assert.Equal(t, 90, res.WindowDays)
	assert.Equal(t, 40, res.Summary.Score)
	assert.Equal(t, 6, res.Summary.Promoters)
	require.Len(t, res.Recent, 1)
	assert.Equal(t, "great", res.Recent[0].Comment)
	repo.AssertExpectations(t)
}

func TestNPSMetrics_HonorsRequestedWindow(t *testing.T) {
	repo := new(MockNPSRepo)
	repo.On("Summarize", mock.Anything, mock.Anything).Return(domain.NPSSummary{}, nil)
	repo.On("RecentComments", mock.Anything, mock.Anything, 50).Return([]domain.NPSComment{}, nil)

	uc := NewNPSMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 99999)
	require.NoError(t, err)
	// Use case does not clamp (handler does); it honors the requested window.
	assert.Equal(t, 99999, res.WindowDays)
	repo.AssertExpectations(t)
}

func TestNPSPrompt_EligiblePassesQuarterAndAgeCutoff(t *testing.T) {
	repo := new(MockNPSRepo)
	fixed := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) // Q2
	repo.On("IsPromptEligible", mock.Anything, "acc-1", "2026-Q2",
		fixed.Add(-npsMinAccountAge)).Return(true, nil)

	uc := NewNPSPromptUseCase(repo)
	uc.now = func() time.Time { return fixed }

	ok, err := uc.Eligible(context.Background(), "acc-1")
	require.NoError(t, err)
	assert.True(t, ok)
	repo.AssertExpectations(t)
}

func TestNPSPrompt_DismissStampsCurrentQuarter(t *testing.T) {
	repo := new(MockNPSRepo)
	fixed := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) // Q4
	repo.On("StampPromptHandled", mock.Anything, "acc-1", "2026-Q4").Return(nil)

	uc := NewNPSPromptUseCase(repo)
	uc.now = func() time.Time { return fixed }

	require.NoError(t, uc.Dismiss(context.Background(), "acc-1"))
	repo.AssertExpectations(t)
}
