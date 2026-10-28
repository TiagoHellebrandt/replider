package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

type fakeScheduleRepo struct {
	upserted    []*domain.ApproachSchedule
	due         []*domain.ApproachSchedule
	sent        []string
	rescheduled map[string]time.Time
	failed      map[string]string
	touched     map[string]string
}

func newFakeScheduleRepo() *fakeScheduleRepo {
	return &fakeScheduleRepo{
		rescheduled: map[string]time.Time{},
		failed:      map[string]string{},
		touched:     map[string]string{},
	}
}

func (f *fakeScheduleRepo) Upsert(ctx context.Context, s *domain.ApproachSchedule) error {
	f.upserted = append(f.upserted, s)
	return nil
}
func (f *fakeScheduleRepo) ClaimDue(ctx context.Context, limit int32) ([]*domain.ApproachSchedule, error) {
	return f.due, nil
}
func (f *fakeScheduleRepo) MarkSent(ctx context.Context, id string, at time.Time) error {
	f.sent = append(f.sent, id)
	return nil
}
func (f *fakeScheduleRepo) Reschedule(ctx context.Context, id string, at time.Time) error {
	f.rescheduled[id] = at
	return nil
}
func (f *fakeScheduleRepo) MarkFailed(ctx context.Context, id, reason string) error {
	f.failed[id] = reason
	return nil
}
func (f *fakeScheduleRepo) TouchAttempt(ctx context.Context, id, reason string) error {
	f.touched[id] = reason
	return nil
}
func (f *fakeScheduleRepo) PendingByLeadIDs(ctx context.Context, accountID string, leadIDs []string) (map[string]time.Time, error) {
	return map[string]time.Time{}, nil
}

func TestScheduleApproach_TomorrowMorningInAccountTimezone(t *testing.T) {
	uc, leadRepo, funnelRepo, _, _, accountRepo, _, _, _, _, _, _ := newApproachUC()
	repo := newFakeScheduleRepo()
	uc.SetScheduleRepo(repo)
	// 2026-08-28 23:30 UTC = 19:30 in Cuiabá — the schedule must land on
	// the 29th at 08:00 LOCAL, not on UTC's idea of tomorrow (the 30th).
	uc.now = func() time.Time { return time.Date(2026, 8, 28, 23, 30, 0, 0, time.UTC) }

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc1", FunnelID: "f1",
			Status: domain.LeadStatusPendingApproval}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").
		Return(&domain.Account{ID: "acc1", Timezone: "America/Cuiaba"}, nil)

	sendAt, err := uc.ScheduleApproach(context.Background(), "acc1", "lead-1",
		ApproachSendInput{Text: "texto final", Subject: "Proposta"})

	assert.NoError(t, err)
	cuiaba, _ := time.LoadLocation("America/Cuiaba")
	assert.Equal(t, time.Date(2026, 8, 29, 8, 0, 0, 0, cuiaba), sendAt)
	if assert.Len(t, repo.upserted, 1) {
		assert.Equal(t, "lead-1", repo.upserted[0].LeadID)
		assert.Equal(t, "Proposta", repo.upserted[0].Subject)
	}
	// The human-edited text was persisted as the draft the worker will send.
	leadRepo.AssertCalled(t, "Upsert", mock.Anything, mock.MatchedBy(
		func(l *domain.Lead) bool { return l.ApproachDraft == "texto final" }))
}

func TestScheduleApproach_RefusesLeadNotInQueue(t *testing.T) {
	uc, leadRepo, funnelRepo, _, _, _, _, _, _, _, _, _ := newApproachUC()
	uc.SetScheduleRepo(newFakeScheduleRepo())

	leadRepo.On("GetByID", mock.Anything, "lead-sent").
		Return(&domain.Lead{ID: "lead-sent", AccountID: "acc1", FunnelID: "f1",
			Status: domain.LeadStatusContacted}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)

	_, err := uc.ScheduleApproach(context.Background(), "acc1", "lead-sent",
		ApproachSendInput{Text: "x"})
	assert.Equal(t, domain.ErrLeadNotPendingApproval, err)
}

func TestFireScheduled_SupersededLeadDiesQuietly(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()
	repo := newFakeScheduleRepo()
	repo.due = []*domain.ApproachSchedule{{ID: "s1", AccountID: "acc1", LeadID: "lead-1"}}
	uc.SetScheduleRepo(repo)

	// Discarded while the row waited: the human's later decision wins.
	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc1",
			Status: domain.LeadStatusLost}, nil)

	sent, err := uc.ProcessDueScheduledApproaches(context.Background(), 10)

	assert.NoError(t, err)
	assert.Equal(t, 0, sent)
	assert.Contains(t, repo.failed["s1"], "no longer pending")
}
