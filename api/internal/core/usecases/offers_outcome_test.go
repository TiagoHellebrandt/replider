package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

type outcomeSalesEvents struct {
	counts map[domain.SalesEventKind]int64
	since  time.Time
	err    error
}

func (f *outcomeSalesEvents) Record(ctx context.Context, e *domain.SalesEvent) error { return nil }
func (f *outcomeSalesEvents) Metrics(ctx context.Context, accountID string, from, to time.Time) (*domain.CaseMetrics, error) {
	return nil, nil
}
func (f *outcomeSalesEvents) LastApproachAt(ctx context.Context, leadID string) (time.Time, error) {
	return time.Time{}, nil
}
func (f *outcomeSalesEvents) CountSince(ctx context.Context, accountID string, kind domain.SalesEventKind, since time.Time) (int64, error) {
	f.since = since
	if f.err != nil {
		return 0, f.err
	}
	return f.counts[kind], nil
}

type fakeProposalCounter struct {
	n   int64
	err error
}

func (f *fakeProposalCounter) CountCreatedSince(ctx context.Context, accountID string, since time.Time) (int64, error) {
	return f.n, f.err
}

func TestOffersOutcome_WeekStartsMondayInAccountTimezone(t *testing.T) {
	events := &outcomeSalesEvents{counts: map[domain.SalesEventKind]int64{
		domain.SalesEventApproachSent:  12,
		domain.SalesEventReplyReceived: 3,
	}}
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Timezone: "America/Cuiaba"}, nil)

	uc := NewOffersOutcomeUseCase(events, &fakeProposalCounter{n: 1}, accounts)
	// Monday 2026-08-24 01:00 UTC is still SUNDAY 21:00 in Cuiabá (UTC-4):
	// the week shown must be the seller's, anchored Monday the 17th.
	uc.WithClock(func() time.Time {
		return time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC)
	})

	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	cuiaba, _ := time.LoadLocation("America/Cuiaba")
	assert.Equal(t, time.Date(2026, 8, 17, 0, 0, 0, 0, cuiaba), out.WeekStart)
	assert.Equal(t, int64(12), out.Sent)
	assert.Equal(t, int64(3), out.Replies)
	assert.Equal(t, int64(1), out.Proposals)
}

func TestOffersOutcome_BrokenProposalCountDegradesToZero(t *testing.T) {
	events := &outcomeSalesEvents{counts: map[domain.SalesEventKind]int64{}}
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)

	uc := NewOffersOutcomeUseCase(events, &fakeProposalCounter{err: errors.New("boom")}, accounts)
	out, err := uc.Execute(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Equal(t, int64(0), out.Proposals)
}
