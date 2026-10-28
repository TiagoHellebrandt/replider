package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginLockout_LocksAfterThresholdWithinWindow(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	l := newLoginLockout()
	l.now = func() time.Time { return now }

	for i := 0; i < loginLockoutThreshold-1; i++ {
		l.fail("Ana@Clinica.com.br")
	}
	assert.False(t, l.locked("ana@clinica.com.br"), "one under the threshold is still open")
	l.fail("ana@clinica.com.br ")
	assert.True(t, l.locked("ANA@clinica.com.br"), "the key is the normalised e-mail")
	assert.False(t, l.locked("outra@clinica.com.br"), "other e-mails are untouched")

	// The window ends and the slate is clean.
	now = now.Add(loginLockoutWindow)
	assert.False(t, l.locked("ana@clinica.com.br"))

	// A success clears an open window at once.
	l.fail("ana@clinica.com.br")
	l.reset("ana@clinica.com.br")
	l.mu.Lock()
	_, present := l.entries["ana@clinica.com.br"]
	l.mu.Unlock()
	assert.False(t, present)
}

// Ten wrong passwords lock the e-mail; the eleventh attempt gets AUTH_LOCKED
// even with the right password, and the repository is not consulted.
func TestAuthenticateUser_LocksEmailAfterRepeatedFailures(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtService := new(MockJWTService)
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo.On("GetByEmail", mock.Anything, "ana@clinica.com.br").Return(&domain.User{
		ID: "u1", AccountID: "a1", Email: "ana@clinica.com.br", PasswordHash: &hashStr,
	}, nil)

	uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)

	for i := 0; i < loginLockoutThreshold; i++ {
		_, err := uc.Execute(context.Background(), AuthenticateUserDTO{Email: "ana@clinica.com.br", Password: "wrong"})
		assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
	}
	calls := len(userRepo.Calls)

	_, err := uc.Execute(context.Background(), AuthenticateUserDTO{Email: "ana@clinica.com.br", Password: "right-password"})
	assert.ErrorIs(t, err, ErrAuthLocked)
	assert.Equal(t, calls, len(userRepo.Calls), "a locked e-mail never reaches the repository")
}

func TestAuthenticateUser_SuccessClearsFailureCount(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtService := new(MockJWTService)
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo.On("GetByEmail", mock.Anything, "ana@clinica.com.br").Return(&domain.User{
		ID: "u1", AccountID: "a1", Email: "ana@clinica.com.br", PasswordHash: &hashStr,
	}, nil)
	jwtService.On("GenerateToken", mock.Anything).Return("jwt", nil)
	accountRepo.On("GetByID", mock.Anything, "a1").Return(nil, assert.AnError)

	uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
	for i := 0; i < loginLockoutThreshold-1; i++ {
		_, _ = uc.Execute(context.Background(), AuthenticateUserDTO{Email: "ana@clinica.com.br", Password: "wrong"})
	}
	resp, err := uc.Execute(context.Background(), AuthenticateUserDTO{Email: "ana@clinica.com.br", Password: "right-password"})
	assert.NoError(t, err)
	assert.Equal(t, "jwt", resp.Token)
	assert.False(t, uc.lockout.locked("ana@clinica.com.br"))
}
