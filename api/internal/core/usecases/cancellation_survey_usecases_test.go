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

type MockCancellationSurveyRepo struct{ mock.Mock }

func (m *MockCancellationSurveyRepo) Create(ctx context.Context, s *domain.CancellationSurvey) error {
	return m.Called(ctx, s).Error(0)
}

func (m *MockCancellationSurveyRepo) Breakdown(ctx context.Context, since time.Time) ([]domain.CancellationReasonCount, error) {
	args := m.Called(ctx, since)
	return args.Get(0).([]domain.CancellationReasonCount), args.Error(1)
}

func (m *MockCancellationSurveyRepo) RecentComments(ctx context.Context, since time.Time, limit int) ([]domain.CancellationSurveyComment, error) {
	args := m.Called(ctx, since, limit)
	return args.Get(0).([]domain.CancellationSurveyComment), args.Error(1)
}

func TestSubmitCancellationSurvey_PersistsValidReason(t *testing.T) {
	repo := new(MockCancellationSurveyRepo)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(s *domain.CancellationSurvey) bool {
		return s.AccountID == "acc-1" && s.UserID == "user-1" &&
			s.Reason == domain.CancellationReasonTooExpensive &&
			s.Comment == "too pricey" && s.ID != ""
	})).Return(nil)

	uc := NewSubmitCancellationSurveyUseCase(repo)
	err := uc.Execute(context.Background(), "acc-1", "user-1", "  too_expensive  ", "  too pricey  ")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSubmitCancellationSurvey_RejectsUnknownReason(t *testing.T) {
	repo := new(MockCancellationSurveyRepo)
	uc := NewSubmitCancellationSurveyUseCase(repo)

	for _, reason := range []string{"", "lol", "TOO_EXPENSIVE", "robux"} {
		err := uc.Execute(context.Background(), "acc-1", "user-1", reason, "")
		assert.ErrorIs(t, err, domain.ErrInvalidCancellationReason)
	}
	// Create must never be reached for an invalid reason.
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSubmitCancellationSurvey_TruncatesLongComment(t *testing.T) {
	repo := new(MockCancellationSurveyRepo)
	long := make([]byte, cancellationCommentMaxLen+500)
	for i := range long {
		long[i] = 'a'
	}
	repo.On("Create", mock.Anything, mock.MatchedBy(func(s *domain.CancellationSurvey) bool {
		return len(s.Comment) == cancellationCommentMaxLen
	})).Return(nil)

	uc := NewSubmitCancellationSurveyUseCase(repo)
	require.NoError(t, uc.Execute(context.Background(), "acc-1", "user-1", domain.CancellationReasonOther, string(long)))
	repo.AssertExpectations(t)
}

func TestCancellationSurveyMetrics_SumsTotalAndDefaultsWindow(t *testing.T) {
	repo := new(MockCancellationSurveyRepo)
	breakdown := []domain.CancellationReasonCount{
		{Reason: domain.CancellationReasonTooExpensive, Count: 7},
		{Reason: domain.CancellationReasonNotUsing, Count: 3},
	}
	comments := []domain.CancellationSurveyComment{
		{AccountID: "acc-1", Reason: domain.CancellationReasonTooExpensive, Comment: "price"},
	}

	// Default window is 90 days; RecentComments capped at 50.
	repo.On("Breakdown", mock.Anything, mock.Anything).Return(breakdown, nil)
	repo.On("RecentComments", mock.Anything, mock.Anything, 50).Return(comments, nil)

	uc := NewCancellationSurveyMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 0)

	require.NoError(t, err)
	assert.Equal(t, 90, res.WindowDays)
	assert.Equal(t, 10, res.Total)
	require.Len(t, res.Breakdown, 2)
	assert.Equal(t, domain.CancellationReasonTooExpensive, res.Breakdown[0].Reason)
	require.Len(t, res.Recent, 1)
	assert.Equal(t, "price", res.Recent[0].Comment)
	repo.AssertExpectations(t)
}

func TestCancellationSurveyMetrics_HonorsRequestedWindow(t *testing.T) {
	repo := new(MockCancellationSurveyRepo)
	repo.On("Breakdown", mock.Anything, mock.Anything).Return([]domain.CancellationReasonCount{}, nil)
	repo.On("RecentComments", mock.Anything, mock.Anything, 50).Return([]domain.CancellationSurveyComment{}, nil)

	uc := NewCancellationSurveyMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 30)
	require.NoError(t, err)
	assert.Equal(t, 30, res.WindowDays)
	assert.Equal(t, 0, res.Total)
	repo.AssertExpectations(t)
}
