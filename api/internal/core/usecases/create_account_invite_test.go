package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The invite redeem path (docs/fluxo-criacao-conta.md). The governing rule is
// the same as every guard in this codebase: FAIL CLOSED. A token that cannot
// be proven redeemable — wrong e-mail, lost race, unreadable storage, unwired
// repo — is refused with ONE uniform error, and nothing is created.

type fakeInviteRepo struct {
	invite     *domain.AccountInvite
	hash       string
	findErr    error
	consumeOK  bool
	consumed   bool
	unconsumed bool
}

func (f *fakeInviteRepo) Create(context.Context, *domain.AccountInvite, string) error { return nil }
func (f *fakeInviteRepo) FindValid(_ context.Context, hash string) (*domain.AccountInvite, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.invite != nil && hash == f.hash {
		return f.invite, nil
	}
	return nil, nil
}
func (f *fakeInviteRepo) Consume(context.Context, string, string) (bool, error) {
	f.consumed = true
	return f.consumeOK, nil
}
func (f *fakeInviteRepo) Unconsume(context.Context, string) error {
	f.unconsumed = true
	return nil
}
func (f *fakeInviteRepo) Revoke(context.Context, string) (bool, error) { return false, nil }
func (f *fakeInviteRepo) ListOpen(context.Context) ([]*domain.AccountInvite, error) {
	return nil, nil
}

func invitedFixture(t *testing.T, consumeOK bool) (*CreateAccountUseCase, *fakeInviteRepo, *MockAccountRepo, string) {
	t.Helper()
	secret, hash, err := domain.NewInviteToken()
	require.NoError(t, err)
	invites := &fakeInviteRepo{
		hash:      hash,
		consumeOK: consumeOK,
		invite: &domain.AccountInvite{
			ID:        "inv-1",
			Email:     "compras@clinica.com.br",
			PlanID:    "plan-sdr",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTxManager)
	userRepo.On("GetByEmail", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	accountRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	userRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	userRepo.On("SetVerificationToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	usageRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	accountRepo.On("SetPlan", mock.Anything, mock.Anything, "plan-sdr").Return(nil).Maybe()
	accountRepo.On("SetLimitOverrides", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	accountRepo.On("SetTrialBypass", mock.Anything, mock.Anything, true, "account_invite").Return(nil).Maybe()
	accountRepo.On("SetSubscriptionStatus", mock.Anything, mock.Anything, "active").Return(nil).Maybe()

	emailSvc := new(MockEmailService)
	emailSvc.On("SendVerificationEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
	uc.SetInviteRepo(invites)
	// The invite exists to pass a CLOSED door — every scenario runs paused.
	uc.SetRegistrationPaused(true)
	return uc, invites, accountRepo, secret
}

func inviteDTO(email, token string) CreateAccountDTO {
	return CreateAccountDTO{
		AccountName: "Clínica", AdminName: "Compras", AdminEmail: email,
		Password: "senha-forte", TermsAccepted: true, InviteToken: token,
	}
}

func TestCreateAccount_PausedWithoutInviteRefuses(t *testing.T) {
	uc, invites, _, _ := invitedFixture(t, true)
	err := uc.Execute(context.Background(), inviteDTO("compras@clinica.com.br", ""))
	assert.ErrorIs(t, err, domain.ErrRegistrationPaused)
	assert.False(t, invites.consumed)
}

func TestCreateAccount_InvitePassesTheClosedDoorAndAppliesThePlan(t *testing.T) {
	uc, invites, accountRepo, secret := invitedFixture(t, true)

	err := uc.Execute(context.Background(), inviteDTO("compras@clinica.com.br", secret))
	require.NoError(t, err)
	assert.True(t, invites.consumed, "the invite is burned exactly once")
	// Born contracted: the same four writes apply-contract makes.
	accountRepo.AssertCalled(t, "SetPlan", mock.Anything, mock.Anything, "plan-sdr")
	accountRepo.AssertCalled(t, "SetSubscriptionStatus", mock.Anything, mock.Anything, "active")
	accountRepo.AssertCalled(t, "SetTrialBypass", mock.Anything, mock.Anything, true, "account_invite")
}

func TestCreateAccount_InviteBoundToAnotherEmailRefuses(t *testing.T) {
	uc, invites, accountRepo, secret := invitedFixture(t, true)

	err := uc.Execute(context.Background(), inviteDTO("outro@lugar.com.br", secret))
	assert.ErrorIs(t, err, domain.ErrInviteInvalid,
		"link encaminhado para terceiro não funciona — o binding é a defesa")
	assert.False(t, invites.consumed)
	accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateAccount_UnreadableInviteStorageFailsClosed(t *testing.T) {
	uc, invites, accountRepo, secret := invitedFixture(t, true)
	invites.findErr = errors.New("connection reset")

	err := uc.Execute(context.Background(), inviteDTO("compras@clinica.com.br", secret))
	assert.ErrorIs(t, err, domain.ErrInviteInvalid,
		"não conseguir checar a permissão é não ter a permissão")
	accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateAccount_LostConsumeRaceRefusesBeforeCreatingAnything(t *testing.T) {
	uc, invites, accountRepo, secret := invitedFixture(t, false) // consume loses

	err := uc.Execute(context.Background(), inviteDTO("compras@clinica.com.br", secret))
	assert.ErrorIs(t, err, domain.ErrInviteInvalid)
	assert.True(t, invites.consumed, "the burn was attempted")
	accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything,
		"consume-first: the loser of a race never creates a duplicate account")
}

func TestCreateAccount_FailedCreationUnburnsTheInvite(t *testing.T) {
	uc, invites, accountRepo, secret := invitedFixture(t, true)
	// Overwrite: creation inside the tx fails.
	accountRepo.ExpectedCalls = nil
	accountRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("disk full"))

	err := uc.Execute(context.Background(), inviteDTO("compras@clinica.com.br", secret))
	require.Error(t, err)
	assert.True(t, invites.unconsumed,
		"a clean failure must not cost the customer their invite")
}

func TestCreateAccount_TokenWithUnwiredRepoRefuses(t *testing.T) {
	uc, _, _, secret := invitedFixture(t, true)
	uc.SetInviteRepo(nil)

	err := uc.Execute(context.Background(), inviteDTO("compras@clinica.com.br", secret))
	assert.ErrorIs(t, err, domain.ErrInviteInvalid,
		"uma permissão que não dá para verificar é uma permissão que não existe")
}
