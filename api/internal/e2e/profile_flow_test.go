package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_GetMe(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	w := doRequest("GET", "/v1/me", nil, token)
	assert.Equal(t, 200, w.Code, "get me failed: %s", w.Body.String())

	resp := parseJSON(t, w)
	user := resp["user"].(map[string]interface{})
	assert.Equal(t, "admin@test.com", user["email"])
	assert.Equal(t, "Corp", user["name"])
}

func TestE2E_UpdateProfile(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Update name
	w := doRequest("PUT", "/v1/me", map[string]string{
		"name":  "New Name",
		"email": "admin@test.com",
	}, token)
	assert.Equal(t, 200, w.Code, "update profile failed: %s", w.Body.String())

	// Verify
	w = doRequest("GET", "/v1/me", nil, token)
	resp := parseJSON(t, w)
	user := resp["user"].(map[string]interface{})
	assert.Equal(t, "New Name", user["name"])
}

func TestE2E_ChangePassword(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Change password
	w := doRequest("PATCH", "/v1/me/password", map[string]string{
		"current_password": "pass1234",
		"new_password":     "newpass456",
	}, token)
	assert.Equal(t, 200, w.Code, "change password failed: %s", w.Body.String())

	// Login with new password
	newToken := login(t, "admin@test.com", "newpass456")
	require.NotEmpty(t, newToken)

	// Old password should fail
	w = doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "pass1234",
	}, "")
	assert.Equal(t, 401, w.Code)
}

func TestE2E_ChangePasswordWrongCurrent(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Wrong current password
	w := doRequest("PATCH", "/v1/me/password", map[string]string{
		"current_password": "wrongpass",
		"new_password":     "newpass456",
	}, token)
	assert.NotEqual(t, 200, w.Code, "should reject wrong current password")
}

func TestE2E_DeleteAccount(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// Delete account
	w := doRequest("DELETE", "/v1/accounts", nil, token)
	assert.Equal(t, 200, w.Code, "delete account failed: %s", w.Body.String())

	// Login should now fail
	w = doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "pass1234",
	}, "")
	assert.NotEqual(t, 200, w.Code, "login should fail after account deletion")
}
