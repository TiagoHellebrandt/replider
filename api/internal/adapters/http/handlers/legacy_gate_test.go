package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With LEGACY_WORKERS off there is no cadence runner and no autopilot hook,
// so the routes that would store an intent nothing honours answer 503 with
// a deliberate message — before touching any repo (nil here on purpose).
func TestSequenceHandler_EnrollUnavailableWhenLegacyWorkersOff(t *testing.T) {
	handler := NewSequenceHandler(nil, nil)
	handler.SetEnrollmentEnabled(false)

	router := setupTestRouter()
	router.POST("/v1/sequences/:id/enroll", handler.HandleEnroll)
	w := performRequest(router, http.MethodPost, "/v1/sequences/seq-1/enroll",
		map[string]any{"lead_id": "lead-1"}, "")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "SEQUENCES_UNAVAILABLE", resp["code"])
	assert.Equal(t, "cadências automáticas desativadas neste plano", resp["message"])
}

func TestAutopilotHandler_UnavailableWhenLegacyWorkersOff(t *testing.T) {
	handler := NewAutopilotHandler(nil)
	handler.SetEnabled(false)

	router := setupTestRouter()
	router.GET("/v1/autopilot", handler.HandleGet)
	router.PUT("/v1/autopilot", handler.HandleUpdate)
	router.GET("/v1/autopilot/sends", handler.HandleListSends)
	router.POST("/v1/autopilot/dismiss-offer", handler.HandleDismissOffer)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/autopilot"},
		{http.MethodPut, "/v1/autopilot"},
		{http.MethodGet, "/v1/autopilot/sends"},
		{http.MethodPost, "/v1/autopilot/dismiss-offer"},
	} {
		w := performRequest(router, tc.method, tc.path, map[string]any{"enabled": true}, "")
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, "%s %s", tc.method, tc.path)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "AUTOPILOT_UNAVAILABLE", resp["code"], "%s %s", tc.method, tc.path)
	}
}

// The default keeps every existing test and the LEGACY_WORKERS=true path
// intact: a handler nobody gated behaves as before.
func TestLegacyGates_DefaultOn(t *testing.T) {
	assert.True(t, NewSequenceHandler(nil, nil).enrollmentEnabled)
	assert.True(t, NewAutopilotHandler(nil).enabled)
}
