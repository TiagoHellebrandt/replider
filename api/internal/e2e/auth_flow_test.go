package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_RegistrationAndLogin(t *testing.T) {
	cleanDB(t)

	// Register
	registerUser(t, "Test Corp", "admin@test.com", "password123")

	// Email verification is no longer a hard login gate (see
	// AuthenticateUserUseCase): pre-verify login succeeds, and the UI
	// nudges users to verify via /me.is_email_verified.
	w := doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "password123",
	}, "")
	assert.Equal(t, 200, w.Code)

	// Verify email
	verifyEmail(t, "admin@test.com")

	// Login succeeds
	token := login(t, "admin@test.com", "password123")
	assert.NotEmpty(t, token)

	// is_new should be true (no agents)
	w = doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "password123",
	}, "")
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["is_new"])

	// Wrong password
	w = doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "wrong",
	}, "")
	assert.Equal(t, 401, w.Code)
}

func TestE2E_DuplicateRegistration(t *testing.T) {
	cleanDB(t)

	registerUser(t, "Corp", "dup@test.com", "pass1234")

	// Duplicate email should fail
	w := doRequest("POST", "/v1/accounts", map[string]string{
		"account_name": "Corp 2",
		"admin_name":   "Admin 2",
		"admin_email":  "dup@test.com",
		"password":     "pass1234",
	}, "")
	assert.Equal(t, 400, w.Code)
}

func TestE2E_PasswordRecovery(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "user@test.com", "oldpass123")
	verifyEmail(t, "user@test.com")

	// Forgot password (always 200)
	w := doRequest("POST", "/v1/auth/forgot-password", map[string]string{
		"email": "user@test.com",
	}, "")
	assert.Equal(t, 200, w.Code)

	// Reset tokens rest as SHA-256 hashes (migration 000222), so the raw
	// value cannot be read back. Confirm one was issued, then plant a known
	// raw token's hash — the endpoint must look it up by hash.
	var storedHash string
	err := testPool.QueryRow(context.Background(),
		"SELECT reset_token FROM users WHERE email = $1", "user@test.com").Scan(&storedHash)
	require.NoError(t, err)
	require.NotEmpty(t, storedHash)
	resetToken := "e2e-raw-reset-token"
	require.NotEqual(t, resetToken, storedHash)
	_, err = testPool.Exec(context.Background(),
		"UPDATE users SET reset_token = $2 WHERE email = $1", "user@test.com", domain.HashToken(resetToken))
	require.NoError(t, err)

	// Reset password
	w = doRequest("POST", "/v1/auth/reset-password", map[string]string{
		"token":        resetToken,
		"new_password": "newpass123",
	}, "")
	assert.Equal(t, 200, w.Code)

	// Login with new password
	token := login(t, "user@test.com", "newpass123")
	assert.NotEmpty(t, token)

	// Old password should fail
	w = doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "user@test.com",
		"password": "oldpass123",
	}, "")
	assert.Equal(t, 401, w.Code)
}

func TestE2E_ForgotPasswordNonexistentEmail(t *testing.T) {
	cleanDB(t)

	// Forgot password for nonexistent email should still return 200 (no information leak)
	w := doRequest("POST", "/v1/auth/forgot-password", map[string]string{
		"email": "nobody@test.com",
	}, "")
	assert.Equal(t, 200, w.Code)
}
