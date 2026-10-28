package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Autopilot sends cold email without a human reading it first. Against a real
// database the thing worth proving is that it cannot be switched on by
// asking nicely — the streak is EARNED, and a direct API call must not be the
// way around earning it.

func setStreak(t *testing.T, accountID string, streak int) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO autopilot_settings (account_id, unedited_streak)
		 VALUES ($1,$2)
		 ON CONFLICT (account_id) DO UPDATE SET unedited_streak = EXCLUDED.unedited_streak`,
		accountID, streak)
	require.NoError(t, err)
}

func TestAutopilot_StartsOffWithNothingEarned(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	got := parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))

	assert.Equal(t, false, got["enabled"])
	assert.Equal(t, false, got["eligible_to_offer"])
	// A goal, not a wall: the UI can say "faltam 20" instead of "não
	// elegível".
	assert.EqualValues(t, 20, got["streak_needed"])
}

// The whole premise is that this is earned. Turning it on with a bare PUT
// would make the streak decorative.
func TestAutopilot_CannotBeTurnedOnWithoutTheStreak(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	setStreak(t, accountID, 19)

	w := doRequest("PUT", "/v1/autopilot", map[string]any{"enabled": true}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "AUTOPILOT_NOT_EARNED")
	assert.Equal(t, false,
		parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))["enabled"])
}

func TestAutopilot_TurnsOnOnceEarned(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	setStreak(t, accountID, 20)

	require.Equal(t, http.StatusOK,
		doRequest("PUT", "/v1/autopilot", map[string]any{"enabled": true}, token).Code)

	got := parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))
	assert.Equal(t, true, got["enabled"])
	// Defaults are conservative and explicit, never zero.
	assert.EqualValues(t, 20, got["effective_daily_cap"])
	assert.EqualValues(t, 80, got["effective_min_score"])
}

// A safety switch that argues is not a safety switch.
func TestAutopilot_CanAlwaysBeTurnedOff(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	setStreak(t, accountID, 20)
	require.Equal(t, http.StatusOK,
		doRequest("PUT", "/v1/autopilot", map[string]any{"enabled": true}, token).Code)

	// Streak wiped by an edit, as a real edit would.
	setStreak(t, accountID, 0)

	require.Equal(t, http.StatusOK,
		doRequest("PUT", "/v1/autopilot", map[string]any{"enabled": false}, token).Code)
	assert.Equal(t, false,
		parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))["enabled"])
}

// Whether cold email leaves without anyone reading it is an account-level
// risk, not a personal preference.
func TestAutopilot_OnlyAnAdminCanChangeIt(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	setStreak(t, accountID, 20)
	attendant := tokenForSeededAccount(t, accountID, "attendant")

	w := doRequest("PUT", "/v1/autopilot", map[string]any{"enabled": true}, attendant)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// The salesperson whose queue it drains has every right to see what went out
// in their name.
func TestAutopilot_AnAttendantCanReadTheAudit(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendant := tokenForSeededAccount(t, accountID, "attendant")

	w := doRequest("GET", "/v1/autopilot/sends", nil, attendant)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, parseJSONArray(t, w))
}

// The invitation is made once and not nagged.
func TestAutopilot_TheOfferIsNotRepeated(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	setStreak(t, accountID, 25)

	assert.Equal(t, true,
		parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))["eligible_to_offer"])

	require.Equal(t, http.StatusNoContent,
		doRequest("POST", "/v1/autopilot/dismiss-offer", nil, token).Code)

	assert.Equal(t, false,
		parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))["eligible_to_offer"])
}

// A settings save must not be able to grant the streak that a settings save
// is gated on.
func TestAutopilot_SavingSettingsDoesNotGrantTheStreak(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	setStreak(t, accountID, 5)

	require.Equal(t, http.StatusOK,
		doRequest("PUT", "/v1/autopilot", map[string]any{"daily_cap": 50}, token).Code)

	got := parseJSON(t, doRequest("GET", "/v1/autopilot", nil, token))
	assert.EqualValues(t, 5, got["unedited_streak"])
	assert.Equal(t, false, got["enabled"])
}
