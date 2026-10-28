package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-F015. Media is the dependency whose failure is invisible in normal
// traffic: signed URLs are minted locally and only fail when the CUSTOMER
// clicks them, so a revoked key or deleted bucket shows up hours later as
// "images don't load". These tests run against the real DB pool so the
// database leg of the check is genuine, and drive the storage leg through
// an injected probe.

type fakeStorageProbe struct {
	err   error
	calls int
}

func (f *fakeStorageProbe) Ping(context.Context) error {
	f.calls++
	return f.err
}

func healthRouter(t *testing.T, cfg *config.Config, probe handlers.StorageProbe) *gin.Engine {
	t.Helper()
	h := handlers.NewHealthHandler(testPool, cfg)
	if probe != nil {
		h.SetStorageProbe(probe)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", h.HandleHealth)
	// The breakdown moved behind the operator token (hardening 2026-09-01);
	// /health itself answers one word.
	r.GET("/health/details", h.HandleHealthDetails)
	r.GET("/ready", h.HandleReady)
	return r
}

func configuredStorageCfg() *config.Config {
	return &config.Config{
		MetricsToken:            "t0k",
		GeminiAPIKey:            "test-key",
		StripeSecretKey:         "sk_test",
		GCSBucketName:           "replider-media-test",
		GoogleCredentialsBase64: "e30=", // "{}" — presence is what matters here
	}
}

func getJSON(t *testing.T, r *gin.Engine, path string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

func TestHealth_StorageHealthyWithRealDB(t *testing.T) {
	probe := &fakeStorageProbe{}
	r := healthRouter(t, configuredStorageCfg(), probe)

	code, body := getJSON(t, r, "/health/details?token=t0k")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "healthy", body["status"])
	checks := body["checks"].(map[string]any)
	assert.Equal(t, "healthy", checks["database"].(map[string]any)["status"])
	assert.Equal(t, "healthy", checks["storage"].(map[string]any)["status"])
	assert.Positive(t, probe.calls, "a configured bucket must be probed for real")
}

// The failure this story exists for: bucket unreachable → the endpoint an
// external monitor watches goes 503, instead of reporting healthy while
// customer media is broken.
func TestHealth_StorageFailureDegradesEndpoint(t *testing.T) {
	probe := &fakeStorageProbe{err: errors.New("storage: permission denied")}
	r := healthRouter(t, configuredStorageCfg(), probe)

	code, body := getJSON(t, r, "/health/details?token=t0k")

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "degraded", body["status"])
	checks := body["checks"].(map[string]any)
	storage := checks["storage"].(map[string]any)
	assert.Equal(t, "unhealthy", storage["status"])
	assert.Contains(t, storage["detail"], "permission denied")
	// The DB is fine — the report must say which dependency broke.
	assert.Equal(t, "healthy", checks["database"].(map[string]any)["status"])
}

// A dev/self-hosted install with no bucket is a missing feature, not an
// outage: /health must stay green or every local run looks broken.
func TestHealth_UnconfiguredStorageDoesNotDegrade(t *testing.T) {
	cfg := &config.Config{GeminiAPIKey: "test-key", MetricsToken: "t0k"}
	probe := &fakeStorageProbe{err: errors.New("must not be called")}
	r := healthRouter(t, cfg, probe)

	code, body := getJSON(t, r, "/health/details?token=t0k")

	assert.Equal(t, http.StatusOK, code)
	checks := body["checks"].(map[string]any)
	assert.Equal(t, "not_configured", checks["storage"].(map[string]any)["status"])
	assert.Zero(t, probe.calls, "an unconfigured bucket must not be dialled")
}

// The probe is cached: liveness is scraped continuously and each real probe
// is a GCS API call.
func TestHealth_StorageProbeIsCached(t *testing.T) {
	probe := &fakeStorageProbe{}
	r := healthRouter(t, configuredStorageCfg(), probe)

	for i := 0; i < 5; i++ {
		code, _ := getJSON(t, r, "/health/details?token=t0k")
		require.Equal(t, http.StatusOK, code)
	}
	assert.Equal(t, 1, probe.calls, "repeat scrapes must reuse the cached result")
}

func TestReady_UsesTheSameStorageProbe(t *testing.T) {
	probe := &fakeStorageProbe{err: errors.New("bucket gone")}
	r := healthRouter(t, configuredStorageCfg(), probe)

	code, body := getJSON(t, r, "/ready")

	assert.Equal(t, http.StatusServiceUnavailable, code)
	checks := body["checks"].(map[string]any)
	assert.Equal(t, "unreachable", checks["gcs"].(map[string]any)["status"])
	assert.Equal(t, "ready", checks["database"].(map[string]any)["status"])
}
