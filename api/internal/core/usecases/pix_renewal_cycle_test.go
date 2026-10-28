package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M014. This ladder can stop serving a paying customer, so the tests
// are about restraint: one rung per run, never twice, never a backlog
// dumped at once, and never a suspension for someone who was never asked.

type fakePixRepo struct {
	accounts []domain.PixRenewalAccount
	stamps   []struct {
		accountID string
		stage     domain.PixDunningStage
	}
	chargeIDs map[string]string
}

func (f *fakePixRepo) ListDue(context.Context, time.Time, int) ([]domain.PixRenewalAccount, error) {
	return f.accounts, nil
}

func (f *fakePixRepo) MarkStage(_ context.Context, accountID string, stage domain.PixDunningStage, periodEnd time.Time) error {
	f.stamps = append(f.stamps, struct {
		accountID string
		stage     domain.PixDunningStage
	}{accountID, stage})
	for i := range f.accounts {
		if f.accounts[i].AccountID == accountID {
			f.accounts[i].StageSent = stage
			pe := periodEnd
			f.accounts[i].StagePeriodEnd = &pe
		}
	}
	return nil
}

func (f *fakePixRepo) SetChargeID(_ context.Context, accountID, chargeID string) error {
	if f.chargeIDs == nil {
		f.chargeIDs = map[string]string{}
	}
	f.chargeIDs[accountID] = chargeID
	return nil
}

type fakePixNotifier struct {
	sent []int // daysUntil per send
	err  error
}

func (f *fakePixNotifier) SendPrebillingEmail(_, _, _ string, daysUntil int, _, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, daysUntil)
	return nil
}

type fakeSuspender struct {
	MockAccountRepo
	suspended []string
	err       error
}

func (f *fakeSuspender) Suspend(_ context.Context, accountID, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.suspended = append(f.suspended, accountID)
	return nil
}

type pixAdminResolver struct{}

func (pixAdminResolver) GetAdminByAccountID(_ context.Context, accountID string) (*domain.User, error) {
	return &domain.User{ID: "u", Email: accountID + "@test.com", Name: "Admin"}, nil
}

func pixUC(repo *fakePixRepo, accounts domain.AccountRepository, notifier PixRenewalNotifier, now time.Time) *SendPixRenewalCycleUseCase {
	uc := NewSendPixRenewalCycleUseCase(repo, accounts, nil, pixAdminResolver{}, "https://app.replider.com")
	uc.SetDelivery(nil, notifier)
	return uc.WithClock(func() time.Time { return now })
}

func dueAccount(periodEnd time.Time, sent domain.PixDunningStage) domain.PixRenewalAccount {
	acc := domain.PixRenewalAccount{
		AccountID: "acc1",
		PlanID:    "plan1",
		PeriodEnd: periodEnd,
		StageSent: sent,
	}
	if sent != domain.PixStageNone {
		pe := periodEnd
		acc.StagePeriodEnd = &pe
	}
	return acc
}

func TestPixCycle_DeliversOneRungPerRunAndNeverRepeats(t *testing.T) {
	renewal := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakePixRepo{accounts: []domain.PixRenewalAccount{
		dueAccount(renewal, domain.PixStageNone),
	}}
	notifier := &fakePixNotifier{}

	// D-3: the charge rung.
	sent, err := pixUC(repo, &fakeSuspender{}, notifier, renewal.AddDate(0, 0, -3)).
		Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, domain.PixStageCharge, repo.stamps[0].stage)

	// Same day, second run: nothing new.
	again, err := pixUC(repo, &fakeSuspender{}, notifier, renewal.AddDate(0, 0, -3)).
		Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, again, "a rung already delivered must not be re-sent")
	assert.Len(t, repo.stamps, 1)
}

// A worker down for a week must not walk the customer through the whole
// ladder on its way back up.
func TestPixCycle_BacklogDoesNotDumpEveryRung(t *testing.T) {
	renewal := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakePixRepo{accounts: []domain.PixRenewalAccount{
		dueAccount(renewal, domain.PixStageNone),
	}}
	suspender := &fakeSuspender{}

	sent, err := pixUC(repo, suspender, &fakePixNotifier{}, renewal.AddDate(0, 0, 9)).
		Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, domain.PixStageSuspended, repo.stamps[0].stage)
	assert.Equal(t, []string{"acc1"}, suspender.suspended)
}

// Suspension is the one rung with a real-world consequence, so a failed
// suspend must not be stamped as done.
func TestPixCycle_FailedSuspensionStaysDue(t *testing.T) {
	renewal := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakePixRepo{accounts: []domain.PixRenewalAccount{
		dueAccount(renewal, domain.PixStageOverdueReminder),
	}}
	suspender := &fakeSuspender{err: errors.New("db down")}

	sent, err := pixUC(repo, suspender, &fakePixNotifier{}, renewal.AddDate(0, 0, 7)).
		Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sent)
	assert.Empty(t, repo.stamps, "a failed rung must be retried, not skipped")
}

// A reminder that could not be delivered is not stamped either: otherwise
// the account marches to suspension having been told nothing.
func TestPixCycle_FailedNotificationStaysDue(t *testing.T) {
	renewal := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakePixRepo{accounts: []domain.PixRenewalAccount{
		dueAccount(renewal, domain.PixStageCharge),
	}}

	sent, err := pixUC(repo, &fakeSuspender{}, &fakePixNotifier{err: errors.New("smtp down")}, renewal).
		Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sent)
	assert.Empty(t, repo.stamps)
}

// Nothing is due before the ladder starts.
func TestPixCycle_NothingHappensEarly(t *testing.T) {
	renewal := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakePixRepo{accounts: []domain.PixRenewalAccount{
		dueAccount(renewal, domain.PixStageNone),
	}}

	sent, err := pixUC(repo, &fakeSuspender{}, &fakePixNotifier{}, renewal.AddDate(0, 0, -10)).
		Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sent)
	assert.Empty(t, repo.stamps)
}
