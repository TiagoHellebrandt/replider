package e2e

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestE2E_RBACAttendantCannotCreateAgents(t *testing.T) {
	cleanDB(t)

	// Register admin
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	adminToken := login(t, "admin@test.com", "pass1234")

	// Get the account ID
	var accountID string
	err := testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "admin@test.com").Scan(&accountID)
	require.NoError(t, err)

	// Insert attendant user directly into DB
	attendantID := ulid.Make().String()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.DefaultCost)
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO users (id, account_id, name, email, password_hash, role, is_email_verified)
		VALUES ($1, $2, 'Attendant', 'attendant@test.com', $3, 'attendant', true)
	`, attendantID, accountID, string(hash))
	require.NoError(t, err)

	attendantToken := login(t, "attendant@test.com", "pass1234")

	// Admin can create agents (201)
	w := doRequest("POST", "/v1/agents", map[string]string{
		"name":     "Bot",
		"provider": "google",
	}, adminToken)
	assert.Equal(t, 201, w.Code, "admin should create agent: %s", w.Body.String())

	// Attendant cannot create agents (403)
	w = doRequest("POST", "/v1/agents", map[string]string{
		"name":     "Hacked Bot",
		"provider": "google",
	}, attendantToken)
	assert.Equal(t, 403, w.Code, "attendant should be blocked from creating agent")
}

func TestE2E_RBACAttendantCanListConversations(t *testing.T) {
	cleanDB(t)

	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")

	var accountID string
	err := testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "admin@test.com").Scan(&accountID)
	require.NoError(t, err)

	// Insert attendant
	attendantID := ulid.Make().String()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.DefaultCost)
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO users (id, account_id, name, email, password_hash, role, is_email_verified)
		VALUES ($1, $2, 'Attendant', 'attendant@test.com', $3, 'attendant', true)
	`, attendantID, accountID, string(hash))
	require.NoError(t, err)

	attendantToken := login(t, "attendant@test.com", "pass1234")

	// Attendant can list conversations (200)
	w := doRequest("GET", "/v1/conversations", nil, attendantToken)
	assert.Equal(t, 200, w.Code, "attendant should be able to list conversations")
}

func TestE2E_RBACAttendantCannotDeleteAccount(t *testing.T) {
	cleanDB(t)

	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")

	var accountID string
	err := testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "admin@test.com").Scan(&accountID)
	require.NoError(t, err)

	// Insert attendant
	attendantID := ulid.Make().String()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.DefaultCost)
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO users (id, account_id, name, email, password_hash, role, is_email_verified)
		VALUES ($1, $2, 'Attendant', 'attendant@test.com', $3, 'attendant', true)
	`, attendantID, accountID, string(hash))
	require.NoError(t, err)

	attendantToken := login(t, "attendant@test.com", "pass1234")

	// Attendant cannot delete account (403)
	w := doRequest("DELETE", "/v1/accounts", nil, attendantToken)
	assert.Equal(t, 403, w.Code, "attendant should not be able to delete account")
}

func TestE2E_RBACAttendantCannotAccessChannels(t *testing.T) {
	cleanDB(t)

	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")

	var accountID string
	err := testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "admin@test.com").Scan(&accountID)
	require.NoError(t, err)

	// Insert attendant
	attendantID := ulid.Make().String()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.DefaultCost)
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO users (id, account_id, name, email, password_hash, role, is_email_verified)
		VALUES ($1, $2, 'Attendant', 'attendant@test.com', $3, 'attendant', true)
	`, attendantID, accountID, string(hash))
	require.NoError(t, err)

	attendantToken := login(t, "attendant@test.com", "pass1234")

	// Attendant cannot access channels (403)
	w := doRequest("GET", "/v1/channels?agent_id=someID", nil, attendantToken)
	assert.Equal(t, 403, w.Code, "attendant should not access channels")
}

func TestE2E_UnauthenticatedAccessDenied(t *testing.T) {
	// Protected routes without token should return 401
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/agents"},
		{"GET", "/v1/conversations"},
		{"GET", "/v1/me"},
		{"GET", "/v1/accounts/usage"},
		{"GET", "/v1/channels?agent_id=x"},
	}

	for _, ep := range endpoints {
		w := doRequest(ep.method, ep.path, nil, "")
		assert.Equal(t, 401, w.Code, "%s %s should require auth", ep.method, ep.path)
	}
}
