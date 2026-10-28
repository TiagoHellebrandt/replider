package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
)

// fakeAccountRepo is the smallest impl of domain.AccountRepository the
// composite test needs. Subscription status + trial deadline drive the
// branching in RequireActiveSubscription.
type fakeAccountRepo struct {
	account *domain.Account
}

func (f *fakeAccountRepo) GetByID(_ context.Context, _ string) (*domain.Account, error) {
	return f.account, nil
}

// All other AccountRepository methods are no-ops for these tests; we
// only exercise the path RequireActiveSubscription touches.
func (f *fakeAccountRepo) Create(context.Context, *domain.Account) error { return nil }
func (f *fakeAccountRepo) GetAccountByStripeSubscription(context.Context, string) (*domain.Account, error) {
	return nil, nil
}
func (f *fakeAccountRepo) UpdateAccountSubscription(context.Context, string, *string, *string, string) error {
	return nil
}
func (f *fakeAccountRepo) SetStripeCustomerIfAbsent(context.Context, string, string) (string, error) {
	return "", nil
}
func (f *fakeAccountRepo) UpdateName(context.Context, string, string) error { return nil }
func (f *fakeAccountRepo) UpdateBranding(context.Context, string, *string, string) error {
	return nil
}
func (f *fakeAccountRepo) UpdateBusinessProfile(context.Context, string, []domain.BusinessLine, *domain.Territory) error {
	return nil
}
func (f *fakeAccountRepo) UpdateCSATEnabled(context.Context, string, bool) error      { return nil }
func (f *fakeAccountRepo) SetTrialBypass(context.Context, string, bool, string) error { return nil }
func (f *fakeAccountRepo) CompleteOnboarding(context.Context, string) error           { return nil }
func (f *fakeAccountRepo) GetOnboardingProgress(context.Context, string) (*domain.OnboardingProgress, error) {
	return nil, nil
}
func (f *fakeAccountRepo) MarkPreviewChatTested(context.Context, string) error { return nil }
func (f *fakeAccountRepo) MarkFirstInboundReceived(context.Context, string, time.Time) error {
	return nil
}
func (f *fakeAccountRepo) DismissOnboardingChecklist(context.Context, string, time.Time) error {
	return nil
}
func (f *fakeAccountRepo) SetAPIKeys(context.Context, string, string, string) error { return nil }
func (f *fakeAccountRepo) SetLimitOverrides(context.Context, string, domain.AccountLimitOverrides) error {
	return nil
}
func (f *fakeAccountRepo) UpdateOnboardingDraft(context.Context, string, int, json.RawMessage) error {
	return nil
}
func (f *fakeAccountRepo) Suspend(context.Context, string, string) error               { return nil }
func (f *fakeAccountRepo) Reactivate(context.Context, string) error                    { return nil }
func (f *fakeAccountRepo) SetSubscriptionStatus(context.Context, string, string) error { return nil }
func (f *fakeAccountRepo) SetPlan(context.Context, string, string) error               { return nil }
func (f *fakeAccountRepo) ClearTrialExpiresAt(context.Context, string) error           { return nil }
func (f *fakeAccountRepo) SetPlatformKeyAllowed(context.Context, string, bool) error   { return nil }
func (f *fakeAccountRepo) UpdateSubscriptionPeriodEnd(context.Context, string, time.Time) error {
	return nil
}
func (f *fakeAccountRepo) ListAccountsDueForPrebilling(context.Context, time.Time) ([]*domain.PrebillingAccount, error) {
	return nil, nil
}
func (f *fakeAccountRepo) MarkPrebillingNotified(context.Context, string, time.Time) error {
	return nil
}
func (f *fakeAccountRepo) SetReplyReminderSettings(context.Context, string, bool, int) error {
	return nil
}
func (f *fakeAccountRepo) Delete(context.Context, string) error { return nil }

// fakeUserRepo only implements UpdateLastSeen — TrackActivity is the
// only middleware that touches the user repo and only calls that one
// method.
type fakeUserRepo struct{ updateCalls int }

func (f *fakeUserRepo) UpdateLastSeen(context.Context, string) error {
	f.updateCalls++
	return nil
}
func (f *fakeUserRepo) Create(context.Context, *domain.User) error               { return nil }
func (f *fakeUserRepo) GetByEmail(context.Context, string) (*domain.User, error) { return nil, nil }
func (f *fakeUserRepo) GetByID(context.Context, string) (*domain.User, error)    { return nil, nil }
func (f *fakeUserRepo) GetByGoogleID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) GetByMetaID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) UpdatePasswordHash(context.Context, string, string) error { return nil }
func (f *fakeUserRepo) UpdateProfile(context.Context, string, string, string, *string) error {
	return nil
}
func (f *fakeUserRepo) SetResetToken(context.Context, string, string, time.Time) error {
	return nil
}
func (f *fakeUserRepo) GetByResetToken(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) ClearResetToken(context.Context, string) error     { return nil }
func (f *fakeUserRepo) SetGoogleID(context.Context, string, string) error { return nil }
func (f *fakeUserRepo) SetMetaID(context.Context, string, string) error   { return nil }
func (f *fakeUserRepo) SetPicture(context.Context, string, string) error  { return nil }
func (f *fakeUserRepo) SetVerificationToken(context.Context, string, string, time.Time) error {
	return nil
}
func (f *fakeUserRepo) GetByVerificationToken(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) VerifyEmail(context.Context, string) error { return nil }
func (f *fakeUserRepo) ListByAccountID(context.Context, string) ([]*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) UpdateRole(context.Context, string, string) error { return nil }
func (f *fakeUserRepo) Deactivate(context.Context, string) error         { return nil }
func (f *fakeUserRepo) GetAdminByAccountID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) MarkInviteSent(context.Context, string) error        { return nil }
func (f *fakeUserRepo) SetTOTPSecret(context.Context, string, string) error { return nil }
func (f *fakeUserRepo) EnableTOTP(context.Context, string, []string) error  { return nil }
func (f *fakeUserRepo) DisableTOTP(context.Context, string) error           { return nil }
func (f *fakeUserRepo) ConsumeTOTPRecoveryCode(context.Context, string, string) (bool, error) {
	return false, nil
}
func (f *fakeUserRepo) StampTOTPVerified(context.Context, string) error { return nil }

// fakeJWT returns a fixed claim set so the auth middleware accepts
// the request and stores accountID for the subscription check.
type fakeJWT struct{}

func (fakeJWT) GenerateToken(_ *domain.User) (string, error) { return "tok", nil }
func (fakeJWT) ValidateToken(_ string) (*domain.UserClaims, error) {
	return &domain.UserClaims{
		UserID:    "u-1",
		UserName:  "Tester",
		AccountID: "acc-1",
		Role:      "admin",
	}, nil
}
func (fakeJWT) GenerateImpersonationToken(_ domain.ImpersonationClaims) (string, error) {
	return "imp", nil
}
func (fakeJWT) GenerateMfaLoginToken(string) (string, error) { return "mfa", nil }
func (fakeJWT) ValidateMfaLoginToken(string) (*domain.MfaLoginClaims, error) {
	return nil, nil
}

// authPaidComposite mirrors the production wrapper from
// router.Setup. The bug it shields against: invoking AuthMiddleware
// inline used to call c.Next() inside, which ran the whole chain
// (handler included) before control returned to the wrapper — so the
// subscription check that came next wrote a SECOND response body on
// top of the handler's 200, producing the
// "Unexpected non-whitespace character after JSON" parser failure
// the dashboard hit on Apr 26 2026.
func authPaidComposite(authMw, paidMw gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		authMw(c)
		if c.IsAborted() {
			return
		}
		paidMw(c)
	}
}

// authMwWithTracking mirrors router.Setup lines 130-136: rawAuthMw +
// trackMw composed inline. trackMw used to call c.Next() at its tail
// which DID run the whole gin chain inline before returning to
// authPaidComposite — handler wrote 200 first, then paidMw's abort
// appended TRIAL_EXPIRED, producing the concatenated body.
func authMwWithTracking(rawAuth, track gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawAuth(c)
		if c.IsAborted() {
			return
		}
		track(c)
	}
}

// TestAuthPaidComposite_TrialExpiredWritesSingleBody is the regression
// guard. With the old c.Next()-everywhere flow, the handler ran first
// and wrote `[{"agent": ...}]`, then the subscription middleware
// aborted with `{"code":"TRIAL_EXPIRED",...}` and gin appended that to
// the same response — the client got two concatenated JSON values and
// the parser threw at the boundary.
//
// Now the success paths in both middlewares return without c.Next, so
// the composite finishes its checks BEFORE gin advances to the
// handler. A trial-expired POST must return ONE body (the 402) and
// never reach the handler. (GET is a separate test below since reads
// now bypass the subscription check.)
func TestAuthPaidComposite_TrialExpiredWritesSingleBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expired := time.Now().Add(-time.Hour)
	repo := &fakeAccountRepo{
		account: &domain.Account{
			ID:                 "acc-1",
			SubscriptionStatus: domain.SubscriptionStatusTrialing,
			TrialExpiresAt:     &expired,
		},
	}
	authMw := middleware.AuthMiddleware(fakeJWT{})
	paidMw := middleware.RequireActiveSubscription(repo)

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(authPaidComposite(authMw, paidMw))
	handlerCalls := 0
	v1.POST("/agents", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, map[string]any{"id": "ag-1"})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 on POST, got %d", w.Code)
	}
	if handlerCalls != 0 {
		t.Fatalf("handler must NOT run when trial is expired; got %d calls", handlerCalls)
	}
	body := w.Body.String()
	// The body must be a single JSON object — no concatenation.
	// The parser the Flutter client uses (json.JsonCodec.decode) will
	// throw if there's any content after the top-level value.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response body is not a single JSON object: %v\nbody: %q", err, body)
	}
	if parsed["code"] != "TRIAL_EXPIRED" {
		t.Errorf("expected TRIAL_EXPIRED code, got %v", parsed["code"])
	}
}

// TestAuthPaidComposite_TrialExpired_GET_PassesThrough proves the
// read-only escape hatch on the paid gate. Product decision: a
// trial-expired user keeps READ access to their agents/channels/
// knowledge so they can see what they had and decide to upgrade,
// without being able to create/edit/delete or burn AI tokens. The
// "look but can't touch" view is a stronger upgrade pull than the
// previous flat 402-on-everything that yanked them to /billing.
//
// Test method matrix: GET passes through (handler runs, 200), POST/
// PUT/PATCH/DELETE all 402 with TRIAL_EXPIRED.
func TestAuthPaidComposite_TrialExpired_ReadAllowed_WriteBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expired := time.Now().Add(-time.Hour)
	repo := &fakeAccountRepo{
		account: &domain.Account{
			ID:                 "acc-1",
			SubscriptionStatus: domain.SubscriptionStatusTrialing,
			TrialExpiresAt:     &expired,
		},
	}
	authMw := middleware.AuthMiddleware(fakeJWT{})
	paidMw := middleware.RequireActiveSubscription(repo)
	composite := authPaidComposite(authMw, paidMw)

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(composite)
	v1.GET("/agents", func(c *gin.Context) {
		c.JSON(http.StatusOK, []map[string]any{{"id": "ag-1"}})
	})
	v1.POST("/agents", func(c *gin.Context) { c.JSON(http.StatusOK, map[string]any{"ok": true}) })
	v1.PUT("/agents/:id", func(c *gin.Context) { c.JSON(http.StatusOK, map[string]any{"ok": true}) })
	v1.PATCH("/agents/:id/toggle", func(c *gin.Context) { c.JSON(http.StatusOK, map[string]any{"ok": true}) })
	v1.DELETE("/agents/:id", func(c *gin.Context) { c.JSON(http.StatusOK, map[string]any{"ok": true}) })

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/v1/agents", http.StatusOK},
		{http.MethodPost, "/v1/agents", http.StatusPaymentRequired},
		{http.MethodPut, "/v1/agents/ag-1", http.StatusPaymentRequired},
		{http.MethodPatch, "/v1/agents/ag-1/toggle", http.StatusPaymentRequired},
		{http.MethodDelete, "/v1/agents/ag-1", http.StatusPaymentRequired},
	}
	for _, tc := range cases {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("%s %s: expected %d, got %d (body=%q)",
					tc.method, tc.path, tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// TestAuthPaidComposite_FullProductionChain mirrors the EXACT shape
// agent_routes.go uses: a v1 group with authPaidMw + RequireRole, then
// the handler. Earlier the in-memory test only stacked authPaidMw, so
// we wanted to be sure RequireRole's own c.Next() (the only remaining
// c.Next() in the auth path) doesn't reintroduce the double-write. It
// shouldn't — RequireRole is the LAST mw before the handler so its
// c.Next() just runs the handler and returns — but the production-shape
// test removes any doubt.
func TestAuthPaidComposite_FullProductionChain_TrialExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expired := time.Now().Add(-time.Hour)
	repo := &fakeAccountRepo{
		account: &domain.Account{
			ID:                 "acc-1",
			SubscriptionStatus: domain.SubscriptionStatusTrialing,
			TrialExpiresAt:     &expired,
		},
	}
	authMw := middleware.AuthMiddleware(fakeJWT{})
	paidMw := middleware.RequireActiveSubscription(repo)
	composite := authPaidComposite(authMw, paidMw)

	r := gin.New()
	v1 := r.Group("/v1")
	agents := v1.Group("/agents")
	agents.Use(composite, middleware.RequireRole("admin"))
	handlerCalls := 0
	agents.POST("", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, map[string]any{"id": "ag-1"})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d (body=%q)", w.Code, w.Body.String())
	}
	if handlerCalls != 0 {
		t.Fatalf("handler must NOT run when trial is expired; got %d calls", handlerCalls)
	}
	body := w.Body.String()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response body is not a single JSON object: %v\nbody: %q", err, body)
	}
	if parsed["code"] != "TRIAL_EXPIRED" {
		t.Errorf("expected TRIAL_EXPIRED, got %v", parsed["code"])
	}
}

// TestAuthPaidComposite_FullProductionStack reproduces the EXACT
// composition router.Setup uses, end-to-end:
//
//	authMw      = rawAuthMw + trackMw          (router.go:130-136)
//	authPaidMw  = authMw    + paidMw           (router.go:173-179)
//	group       = authPaidMw + RequireRole     (agent_routes.go:21)
//	handler     = HandleListAgents
//
// The originally-shipped fix to AuthMiddleware + RequireActiveSubscription
// removed c.Next() from those two functions. But trackMw's c.Next() was
// missed: when called inline inside the authMw composite, c.Next() ran
// the whole remaining gin chain (RequireRole + handler) before returning
// to authPaidMw. Handler wrote 200 + agents body, then paidMw aborted
// with 402 + TRIAL_EXPIRED, gin appended both — the response body the
// user pasted on Apr 26 2026 was literally
//
//	`[{"id":"01KP2HYP..."}]{"code":"TRIAL_EXPIRED",...}`.
//
// Removing trackMw's c.Next() makes the auth chain correctly sequential.
// This test re-creates the production stack in-memory; it fails (status
// 200 with concatenated body) before the trackMw fix and passes after.
func TestAuthPaidComposite_FullProductionStack_TrialExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expired := time.Now().Add(-time.Hour)
	acctRepo := &fakeAccountRepo{
		account: &domain.Account{
			ID:                 "acc-1",
			SubscriptionStatus: domain.SubscriptionStatusTrialing,
			TrialExpiresAt:     &expired,
		},
	}
	userRepo := &fakeUserRepo{}
	rawAuth := middleware.AuthMiddleware(fakeJWT{})
	trackMw := middleware.TrackActivity(userRepo)
	authMw := authMwWithTracking(rawAuth, trackMw)
	paidMw := middleware.RequireActiveSubscription(acctRepo)
	authPaidMw := authPaidComposite(authMw, paidMw)

	r := gin.New()
	v1 := r.Group("/v1")
	agents := v1.Group("/agents")
	agents.Use(authPaidMw, middleware.RequireRole("admin"))
	handlerCalls := 0
	agents.POST("", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, map[string]any{"id": "ag-1"})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d (body=%q)", w.Code, w.Body.String())
	}
	if handlerCalls != 0 {
		t.Fatalf("handler MUST NOT run on trial expired; got %d", handlerCalls)
	}
	body := w.Body.String()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response body is not a single JSON object (likely concatenation regression): %v\nbody: %q", err, body)
	}
	if parsed["code"] != "TRIAL_EXPIRED" {
		t.Errorf("expected TRIAL_EXPIRED, got %v", parsed["code"])
	}
	// trackMw should have NEVER recorded last_seen for a blocked
	// request — the abort is supposed to happen before the handler
	// runs, and tracking before subscription check would write to the
	// DB on every blocked request (pointless load).
	if userRepo.updateCalls != 1 {
		// Note: trackMw runs inside authMw, so it DOES execute once
		// (auth succeeded). What it must NOT do is run the handler
		// chain. We assert it ran exactly once here.
		t.Errorf("trackMw should run exactly once (auth succeeded); got %d", userRepo.updateCalls)
	}
}

// TestAuthPaidComposite_ActiveLetsHandlerRun proves the success path
// still hands off to the handler — the previous fix MUST NOT
// short-circuit live subscriptions.
func TestAuthPaidComposite_ActiveLetsHandlerRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeAccountRepo{
		account: &domain.Account{
			ID:                 "acc-1",
			SubscriptionStatus: domain.SubscriptionStatusActive,
		},
	}
	authMw := middleware.AuthMiddleware(fakeJWT{})
	paidMw := middleware.RequireActiveSubscription(repo)

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(authPaidComposite(authMw, paidMw))
	handlerCalls := 0
	v1.GET("/agents", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, []map[string]any{{"id": "ag-1"}})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on active sub, got %d", w.Code)
	}
	if handlerCalls != 1 {
		t.Fatalf("handler must run exactly once; got %d", handlerCalls)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response body is not a single JSON array: %v", err)
	}
	if len(parsed) != 1 || parsed[0]["id"] != "ag-1" {
		t.Errorf("unexpected payload: %v", parsed)
	}
}
