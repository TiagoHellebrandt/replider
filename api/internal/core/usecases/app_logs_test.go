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

// Whoever reads the production log sees EVERY account. The gate is the operator
// panel's — the email list — and it fails closed: a deploy that forgot the variable
// must not become an open door onto everyone's log.

type fakeAppLogRepo struct {
	queried domain.AppLogFilter
	logs    []*domain.AppLog
	groups  []*domain.AppLogGroup
}

func (f *fakeAppLogRepo) InsertBatch(context.Context, []*domain.AppLog) error { return nil }

func (f *fakeAppLogRepo) Query(_ context.Context, filter domain.AppLogFilter) ([]*domain.AppLog, error) {
	f.queried = filter
	return f.logs, nil
}

func (f *fakeAppLogRepo) Summarize(context.Context, time.Time, time.Time, int) ([]*domain.AppLogGroup, error) {
	return f.groups, nil
}

func (f *fakeAppLogRepo) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func operatorRepo(t *testing.T, email string) *MockUserRepo {
	t.Helper()
	users := new(MockUserRepo)
	users.On("GetByID", mock.Anything, "u1").
		Return(&domain.User{ID: "u1", Email: email}, nil)
	return users
}

func TestAppLogs_OnlyTheAllowlistReads(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAppLogRepo{logs: []*domain.AppLog{{Msg: "boom"}}}

	uc := NewAppLogUseCase(repo, operatorRepo(t, "op@replider.com"),
		[]string{" OP@Replider.com "})
	got, err := uc.Query(ctx, "u1", domain.AppLogFilter{})
	require.NoError(t, err, "e-mail da lista, com espaço e caixa diferentes, entra")
	assert.Len(t, got, 1)

	uc = NewAppLogUseCase(repo, operatorRepo(t, "cliente@empresa.com"),
		[]string{"op@replider.com"})
	_, err = uc.Query(ctx, "u1", domain.AppLogFilter{})
	assert.Equal(t, ErrLogsForbidden, err)
}

// With no list configured, nobody reads. A deploy missing the variable would be
// the worst possible moment for the system to decide to be permissive.
func TestAppLogs_NoAllowlistIsClosed(t *testing.T) {
	uc := NewAppLogUseCase(&fakeAppLogRepo{}, operatorRepo(t, "op@replider.com"), nil)

	_, err := uc.Query(context.Background(), "u1", domain.AppLogFilter{})
	assert.Equal(t, ErrLogsForbidden, err)

	_, err = uc.Summarize(context.Background(), "u1", time.Time{}, time.Time{}, 10)
	assert.Equal(t, ErrLogsForbidden, err)
}

// With no window given, the question is "what is breaking right now" — and the
// right answer is the last hour, not the whole table.
func TestAppLogs_DefaultWindowIsTheLastHour(t *testing.T) {
	repo := &fakeAppLogRepo{}
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	uc := NewAppLogUseCase(repo, operatorRepo(t, "op@replider.com"),
		[]string{"op@replider.com"}).WithClock(func() time.Time { return now })

	_, err := uc.Query(context.Background(), "u1", domain.AppLogFilter{})
	require.NoError(t, err)
	assert.Equal(t, now.Add(-time.Hour), repo.queried.Since)
}

// An explicitly requested window wins — the default is a default, not a cap.
func TestAppLogs_AnExplicitWindowWins(t *testing.T) {
	repo := &fakeAppLogRepo{}
	since := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	uc := NewAppLogUseCase(repo, operatorRepo(t, "op@replider.com"),
		[]string{"op@replider.com"})

	_, err := uc.Query(context.Background(), "u1", domain.AppLogFilter{Since: since})
	require.NoError(t, err)
	assert.Equal(t, since, repo.queried.Since)
}
