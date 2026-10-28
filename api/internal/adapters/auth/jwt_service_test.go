package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUser(role *string) *domain.User {
	return &domain.User{
		ID:        "user_01JTEST000000000000000001",
		AccountID: "acc_01JTEST000000000000000001",
		Name:      "Test User",
		Email:     "test@example.com",
		Role:      role,
		IsActive:  true,
	}
}

func strPtr(s string) *string { return &s }

func TestGenerateAndValidateToken_RoundTrip(t *testing.T) {
	svc := NewJWTService("test-secret-key", 24)
	role := "admin"
	user := newTestUser(&role)

	tokenStr, err := svc.GenerateToken(user)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.AccountID, claims.AccountID)
	assert.Equal(t, user.Name, claims.UserName)
	assert.Equal(t, "admin", claims.Role)
}

func TestGenerateToken_ClaimsPreserved(t *testing.T) {
	svc := NewJWTService("my-secret", 1)

	tests := []struct {
		name         string
		role         *string
		expectedRole string
	}{
		{"admin role", strPtr("admin"), "admin"},
		{"attendant role", strPtr("attendant"), "attendant"},
		{"nil role defaults to attendant", nil, "attendant"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := newTestUser(tc.role)
			tokenStr, err := svc.GenerateToken(user)
			require.NoError(t, err)

			claims, err := svc.ValidateToken(tokenStr)
			require.NoError(t, err)

			assert.Equal(t, user.ID, claims.UserID)
			assert.Equal(t, user.AccountID, claims.AccountID)
			assert.Equal(t, user.Name, claims.UserName)
			assert.Equal(t, tc.expectedRole, claims.Role)
		})
	}
}

func TestValidateToken_Expired(t *testing.T) {
	// Create a service with 0-hour lifespan, then manually craft an expired token
	secret := "test-secret"
	svc := NewJWTService(secret, 24)

	// Create an already-expired token manually
	now := time.Now().Add(-2 * time.Hour)
	claims := &domain.UserClaims{
		UserID:    "user_123",
		UserName:  "Expired User",
		AccountID: "acc_123",
		Role:      "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // expired 1h ago
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	result, err := svc.ValidateToken(tokenStr)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestValidateToken_InvalidMalformed(t *testing.T) {
	svc := NewJWTService("test-secret", 24)

	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"random garbage", "not-a-jwt-token"},
		{"partial jwt", "eyJhbGciOiJIUzI1NiJ9.bad.bad"},
		{"truncated", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := svc.ValidateToken(tc.token)
			assert.Nil(t, claims)
			assert.ErrorIs(t, err, domain.ErrUnauthorized)
		})
	}
}

func TestValidateToken_RotationAcceptsPreviousSecret(t *testing.T) {
	// Old service signs a token under the previous key.
	oldSvc := NewJWTService("old-secret", 24)
	user := newTestUser(strPtr("admin"))
	oldToken, err := oldSvc.GenerateToken(user)
	require.NoError(t, err)

	// Rotated service signs with the new key but still accepts the old one.
	rotated := NewJWTService("new-secret", 24, "old-secret")

	// Token minted under the old key still validates during the grace window.
	claims, err := rotated.ValidateToken(oldToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, "admin", claims.Role)

	// Freshly minted tokens are signed with the new key and also validate.
	newToken, err := rotated.GenerateToken(user)
	require.NoError(t, err)
	claims, err = rotated.ValidateToken(newToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
}

func TestValidateToken_RotationRejectsAfterPreviousDropped(t *testing.T) {
	oldSvc := NewJWTService("old-secret", 24)
	user := newTestUser(strPtr("admin"))
	oldToken, err := oldSvc.GenerateToken(user)
	require.NoError(t, err)

	// Rotation complete: only the new key remains, the old grace key is gone.
	current := NewJWTService("new-secret", 24)
	claims, err := current.ValidateToken(oldToken)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestValidateToken_DifferentSecretRejects(t *testing.T) {
	svc1 := NewJWTService("secret-one", 24)
	svc2 := NewJWTService("secret-two", 24)

	user := newTestUser(strPtr("admin"))
	tokenStr, err := svc1.GenerateToken(user)
	require.NoError(t, err)

	// Validate with a different secret — must fail
	claims, err := svc2.ValidateToken(tokenStr)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}
