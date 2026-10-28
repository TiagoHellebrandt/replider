package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/config"
)

// livenessDBTimeout bounds the `SELECT 1` liveness probe so a hung
// connection can't keep `/health` blocked and produce a liveness
// false-positive (US-068).
const livenessDBTimeout = 2 * time.Second

// readinessProbeTimeout bounds each individual `/ready` dependency probe.
// readinessProbeTimeout bounds the storage probe. 5s, not 2: the probe pays
// TWO Google round-trips (a fresh client's token exchange, then the bucket
// read), and on 2026-08-19 the VPS→Google route degraded to ~1.4s per trip —
// 2.8s of honest latency read as "unreachable", the container went unhealthy
// and a deploy was refused while media actually worked. The docker
// healthcheck's timeout is raised to 10s alongside this (compose), because
// the checks run sequentially and a hung DB (2s) plus a slow bucket (4s)
// must still answer instead of being killed mid-reply.
const readinessProbeTimeout = 4 * time.Second

// ReadinessProbe is a single named dependency check used by `/ready`.
// Check must return nil when the dependency is reachable. Probes are
// injectable so tests can exercise degraded states without real
// external services.
type ReadinessProbe struct {
	Name  string
	Check func(ctx context.Context) error
}

// HealthHandler serves two endpoints:
//   - /health (liveness): a real `SELECT 1` against the DB with a 2s timeout.
//   - /ready  (readiness): probes critical dependencies (DB, AI, GCS, Stripe).
type HealthHandler struct {
	dbPool *pgxpool.Pool
	cfg    *config.Config
	probes []ReadinessProbe // optional override; defaults derived from cfg when nil
	// storage probes the media bucket for real (US-F015). Optional: when
	// unwired the storage check degrades to "is it configured", which is
	// what every check here used to be.
	storage StorageProbe
	// storageCache memoizes the last real probe result so a scrape every
	// few seconds doesn't turn into a GCS API call every few seconds.
	storageCacheMu   sync.Mutex
	storageCachedAt  time.Time
	storageCachedErr error
}

// StorageProbe is the narrow port the health endpoints use to verify the
// media bucket. Satisfied by *storage.GCSAdapter.
type StorageProbe interface {
	Ping(ctx context.Context) error
}

// storageProbeTTL bounds how often the real bucket round-trip runs. Liveness
// is scraped continuously; media outages are measured in minutes, not
// milliseconds, so a short cache keeps the signal without the cost.
const storageProbeTTL = 30 * time.Second

func NewHealthHandler(dbPool *pgxpool.Pool, cfg *config.Config) *HealthHandler {
	return &HealthHandler{dbPool: dbPool, cfg: cfg}
}

// SetStorageProbe wires the real media-bucket check (US-F015). Optional
// setter so the constructor and its call sites stay untouched.
func (h *HealthHandler) SetStorageProbe(p StorageProbe) {
	h.storage = p
}

// storageConfigured reports whether this deployment has media storage at
// all. An unconfigured bucket cannot be "down".
func (h *HealthHandler) storageConfigured() bool {
	return h.cfg.GCSBucketName != "" && h.cfg.GoogleCredentialsBase64 != ""
}

// checkStorage returns the storage health, reusing a result younger than
// storageProbeTTL. Unwired storage falls back to the config-presence check.
func (h *HealthHandler) checkStorage(ctx context.Context) error {
	if h.storage == nil {
		if h.cfg.GCSBucketName == "" || h.cfg.GoogleCredentialsBase64 == "" {
			return errors.New("GCS bucket or credentials not configured")
		}
		return nil
	}

	h.storageCacheMu.Lock()
	defer h.storageCacheMu.Unlock()
	if !h.storageCachedAt.IsZero() && time.Since(h.storageCachedAt) < storageProbeTTL {
		return h.storageCachedErr
	}
	err := h.storage.Ping(ctx)
	h.storageCachedAt = time.Now()
	h.storageCachedErr = err
	return err
}

// SetReadinessProbes overrides the default cfg-derived readiness probes.
// Used by tests to inject deterministic passing/failing probes.
func (h *HealthHandler) SetReadinessProbes(probes ...ReadinessProbe) {
	h.probes = probes
}

// selectOne runs a real `SELECT 1` round-trip so the check fails when the
// connection is established but queries hang — unlike a bare TCP Ping.
func (h *HealthHandler) selectOne(ctx context.Context) error {
	var one int
	if err := h.dbPool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return err
	}
	if one != 1 {
		return errors.New("unexpected SELECT 1 result")
	}
	return nil
}

// HandleHealth godoc
// @Summary      Liveness Health Check
// @Description  Runs the liveness probes (DB `SELECT 1` 2s, media bucket) and answers only {"status":"ok"|"degraded"}. The per-dependency breakdown is on /health/details (METRICS_TOKEN).
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "All systems operational"
// @Failure      503  {object}  map[string]interface{}  "One or more subsystems unhealthy"
// @Router       /health [get]
func (h *HealthHandler) HandleHealth(c *gin.Context) {
	// The public answer is the status code and one word. The breakdown
	// (which AI vendor is configured, the SMTP host, a storage error string)
	// used to be served here unauthenticated — an inventory of the stack for
	// anyone who cared to GET it.
	status, _ := h.runChecks(c)
	body := gin.H{"status": "ok"}
	if status != http.StatusOK {
		body["status"] = "degraded"
	}
	c.JSON(status, body)
}

// HandleHealthDetails godoc
// @Summary      Detailed health (operators)
// @Description  The per-dependency breakdown behind /health. Requires METRICS_TOKEN as `Authorization: Bearer` or `?token=`; refuses every caller when the token is not configured.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]interface{}
// @Failure      503  {object}  map[string]interface{}
// @Router       /health/details [get]
func (h *HealthHandler) HandleHealthDetails(c *gin.Context) {
	if !h.detailsAuthorized(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
		return
	}
	status, checks := h.runChecks(c)
	overallStatus := "healthy"
	if status != http.StatusOK {
		overallStatus = "degraded"
	}
	c.JSON(status, gin.H{
		"status": overallStatus,
		"time":   time.Now().UTC(),
		"checks": checks,
	})
}

// detailsAuthorized checks the operator token. Constant-time compare; an
// empty configured token fails CLOSED (there is no "open by default" for a
// page that lists vendors and hosts).
func (h *HealthHandler) detailsAuthorized(c *gin.Context) bool {
	want := ""
	if h.cfg != nil {
		want = strings.TrimSpace(h.cfg.MetricsToken)
	}
	if want == "" {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if got == "" {
		got = strings.TrimSpace(c.Query("token"))
	}
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// runChecks executes the liveness probes and returns (status, breakdown).
func (h *HealthHandler) runChecks(c *gin.Context) (int, map[string]interface{}) {
	status := http.StatusOK
	checks := make(map[string]interface{})

	// 1. Database liveness — real `SELECT 1` with a 2s timeout so a hung
	//    query (not just a dead socket) surfaces as unhealthy.
	ctx, cancel := context.WithTimeout(c.Request.Context(), livenessDBTimeout)
	defer cancel()
	if err := h.selectOne(ctx); err != nil {
		slog.Error("health check: database SELECT 1 failed", "error", err)
		checks["database"] = map[string]string{"status": "unhealthy"}
		status = http.StatusServiceUnavailable
	} else {
		checks["database"] = map[string]string{"status": "healthy"}
	}

	// 2. Storage (US-F015). Media failures are otherwise invisible until a
	//    customer clicks a signed URL that doesn't resolve, so liveness owns
	//    this check. Degrades the endpoint: an API that cannot serve media
	//    is not fully healthy.
	if !h.storageConfigured() {
		// Dev and self-hosted installs run without a bucket; that is a
		// missing feature, not an outage. Same treatment as SMTP.
		checks["storage"] = map[string]string{"status": "not_configured"}
	} else {
		storageCtx, storageCancel := context.WithTimeout(c.Request.Context(), readinessProbeTimeout)
		storageErr := h.checkStorage(storageCtx)
		storageCancel()
		if storageErr != nil {
			slog.Error("health check: storage unreachable", "error", storageErr)
			checks["storage"] = map[string]string{"status": "unhealthy", "detail": storageErr.Error()}
			status = http.StatusServiceUnavailable
		} else {
			checks["storage"] = map[string]string{"status": "healthy"}
		}
	}

	// 3. Stripe Key Check (non-critical — billing endpoints won't work but API is still up)
	if h.cfg.StripeSecretKey != "" {
		checks["stripe"] = map[string]string{"status": "configured"}
	} else {
		checks["stripe"] = map[string]string{"status": "missing", "detail": "STRIPE_SECRET_KEY not set"}
	}

	// 3. AI Key Check (at least one provider must be configured)
	aiStatus := "missing"
	aiDetail := "No AI API key configured"
	if h.cfg.GeminiAPIKey != "" {
		aiStatus = "configured"
		aiDetail = "gemini"
	} else if h.cfg.OpenAIAPIKey != "" {
		aiStatus = "configured"
		aiDetail = "openai"
	}
	if aiStatus == "configured" {
		checks["ai"] = map[string]string{"status": aiStatus, "provider": aiDetail}
	} else {
		checks["ai"] = map[string]string{"status": aiStatus, "detail": aiDetail}
		status = http.StatusServiceUnavailable
	}

	// 4. SMTP Check
	if h.cfg.SMTPHost != "" {
		checks["smtp"] = map[string]string{"status": "configured", "host": h.cfg.SMTPHost}
	} else {
		checks["smtp"] = map[string]string{"status": "not_configured"}
		// SMTP is not critical — don't degrade health
	}

	return status, checks
}

// defaultProbes derives readiness probes from config when none are injected.
// The DB probe is a real `SELECT 1`; the AI/GCS/Stripe probes treat a
// fully-configured dependency as reachable (a live network round-trip per
// readiness scrape would be costly and flaky). Tests inject real probes via
// SetReadinessProbes to exercise unreachable states.
func (h *HealthHandler) defaultProbes() []ReadinessProbe {
	return []ReadinessProbe{
		{Name: "database", Check: h.selectOne},
		{Name: "ai", Check: func(context.Context) error {
			if h.cfg.GeminiAPIKey == "" && h.cfg.OpenAIAPIKey == "" {
				return errors.New("no AI provider configured")
			}
			return nil
		}},
		{Name: "gcs", Check: h.checkStorage},
		{Name: "stripe", Check: func(context.Context) error {
			if h.cfg.StripeSecretKey == "" {
				return errors.New("Stripe secret key not configured")
			}
			return nil
		}},
	}
}

// HandleReady godoc
// @Summary      Readiness Check
// @Description  Probes critical dependencies (DB, AI provider, GCS, Stripe). Returns 503 if any is unreachable.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Ready to serve traffic"
// @Failure      503  {object}  map[string]interface{}  "One or more dependencies unreachable"
// @Router       /ready [get]
func (h *HealthHandler) HandleReady(c *gin.Context) {
	probes := h.probes
	if probes == nil {
		probes = h.defaultProbes()
	}

	status := http.StatusOK
	checks := make(map[string]interface{})

	for _, p := range probes {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessProbeTimeout)
		err := p.Check(ctx)
		cancel()
		if err != nil {
			slog.Warn("readiness check: dependency unreachable", "dependency", p.Name, "error", err)
			checks[p.Name] = map[string]string{"status": "unreachable", "detail": err.Error()}
			status = http.StatusServiceUnavailable
		} else {
			checks[p.Name] = map[string]string{"status": "ready"}
		}
	}

	overallStatus := "ready"
	if status != http.StatusOK {
		overallStatus = "degraded"
	}

	c.JSON(status, gin.H{
		"status": overallStatus,
		"time":   time.Now().UTC(),
		"checks": checks,
	})
}
