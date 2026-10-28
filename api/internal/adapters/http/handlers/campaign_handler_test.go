package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupCampaignRouter(campaignRepo *MockCampaignRepo, channelRepo *MockChannelRepo, jwtMock *MockJWTService) *gin.Engine {
	// The limit-enforcement branch is optional: CreateCampaignUseCase treats
	// accountRepo/planRepo/usageRepo as nil-safe (skipped when nil). Tests in
	// this file focus on tenant/channel/recipient validation and don't need
	// the plan checks, so we pass nils. Dedicated limit tests wire real
	// mocks separately.
	createUC := usecases.NewCreateCampaignUseCase(campaignRepo, channelRepo, nil, nil, nil)
	listUC := usecases.NewListCampaignsUseCase(campaignRepo)
	getUC := usecases.NewGetCampaignUseCase(campaignRepo)
	deleteUC := usecases.NewDeleteCampaignUseCase(campaignRepo)
	cancelUC := usecases.NewCancelCampaignUseCase(campaignRepo)

	h := NewCampaignHandler(createUC, listUC, getUC, deleteUC, cancelUC, campaignRepo)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/campaigns", authMw, middleware.RequireRole("admin"))
	{
		g.GET("", h.HandleListCampaigns)
		g.POST("", h.HandleCreateCampaign)
		g.GET("/:id", h.HandleGetCampaign)
		g.DELETE("/:id", h.HandleDeleteCampaign)
		g.GET("/:id/recipients", h.HandleListRecipients)
		g.POST("/:id/cancel", h.HandleCancelCampaign)
	}
	return router
}

func waChannelOK(id, accountID string) *domain.Channel {
	cfg, _ := json.Marshal(map[string]string{"phone_number_id": "pn", "waba_id": "w", "access_token": "t"})
	return &domain.Channel{
		ID: id, AccountID: accountID, Type: "whatsapp", Name: "WA",
		Config: json.RawMessage(cfg), IsActive: true,
	}
}

// ──────────────────────────────────────────────────────────────────
// Auth + RBAC
// ──────────────────────────────────────────────────────────────────

func TestCampaign_NoAuth_401(t *testing.T) {
	router := setupCampaignRouter(new(MockCampaignRepo), new(MockChannelRepo), new(MockJWTService))
	w := performRequest(router, http.MethodGet, "/v1/campaigns", nil, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCampaign_NonAdmin_403(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")
	router := setupCampaignRouter(new(MockCampaignRepo), new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/campaigns", nil, testToken)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Cross-tenant isolation (IDOR)
// ──────────────────────────────────────────────────────────────────

func TestCampaign_GetCrossTenant_404(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	// Repo enforces account_id in WHERE clause; if no match it returns nil.
	campaignRepo.On("GetByID", mock.Anything, "camp-1", testAccountID).Return(nil, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/campaigns/camp-1", nil, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code) // CAMPAIGN_NOT_FOUND mapped to 400 by AppError default
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CAMPAIGN_NOT_FOUND", body["code"])
}

func TestCampaign_DeleteCrossTenant_404(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "camp-1", testAccountID).Return(nil, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/campaigns/camp-1", nil, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCampaign_RecipientsCrossTenant_404(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "camp-1", testAccountID).Return(nil, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/campaigns/camp-1/recipients", nil, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Create — channel ownership + type validation
// ──────────────────────────────────────────────────────────────────

func TestCampaign_Create_NonOwnedChannel_404(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	// Channel belongs to another account
	channelRepo.On("GetByID", mock.Anything, "ch-x").Return(&domain.Channel{
		ID: "ch-x", AccountID: "other-acc", Type: "whatsapp", IsActive: true,
	}, nil)

	router := setupCampaignRouter(campaignRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-x",
		"name":          "Black Friday",
		"template_name": "promo",
		"recipients":    []string{"5511999999999"},
	}, testToken)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCampaign_Create_NonWhatsAppChannel_400(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-y").Return(&domain.Channel{
		ID: "ch-y", AccountID: testAccountID, Type: "telegram", IsActive: true,
	}, nil)

	router := setupCampaignRouter(campaignRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-y",
		"name":          "n",
		"template_name": "t",
		"recipients":    []string{"r"},
	}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "INVALID_CHANNEL_TYPE", body["code"])
}

func TestCampaign_Create_DisabledChannel_400(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	ch := waChannelOK("ch-z", testAccountID)
	ch.IsActive = false
	channelRepo.On("GetByID", mock.Anything, "ch-z").Return(ch, nil)

	router := setupCampaignRouter(campaignRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-z",
		"name":          "n",
		"template_name": "t",
		"recipients":    []string{"r"},
	}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CHANNEL_DISABLED", body["code"])
}

func TestCampaign_Create_TooManyRecipients_400(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupCampaignRouter(new(MockCampaignRepo), new(MockChannelRepo), jwtMock)

	// Build 50001 recipients
	recs := make([]string, 50001)
	for i := range recs {
		recs[i] = "5511" + strings.Repeat("9", 9)
	}
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch",
		"name":          "n",
		"template_name": "t",
		"recipients":    recs,
	}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "TOO_MANY_RECIPIENTS", body["code"])
}

func TestCampaign_Create_MissingField_400(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupCampaignRouter(new(MockCampaignRepo), new(MockChannelRepo), jwtMock)

	// Missing template_name
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id": "ch",
		"name":       "n",
		"recipients": []string{"r"},
	}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Create — happy path: dedupe, persist, scheduled
// ──────────────────────────────────────────────────────────────────

func TestCampaign_Create_Success_DedupesRecipients(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)

	var capturedCampaign *domain.Campaign
	campaignRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.Campaign) bool {
		capturedCampaign = c
		return true
	})).Return(nil)
	campaignRepo.On("CreateRecipient", mock.Anything, mock.Anything).Return(nil)

	router := setupCampaignRouter(campaignRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "Promo",
		"template_name": "boas_vindas",
		"template_lang": "pt_BR",
		"recipients":    []string{"5511111", "5511222", "5511111", "", "5511222"},
	}, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotNil(t, capturedCampaign)
	// 2 unique non-empty recipients
	assert.Equal(t, 2, capturedCampaign.TotalRecipients)
	assert.Equal(t, domain.CampaignScheduled, capturedCampaign.Status)
	assert.Equal(t, "boas_vindas", capturedCampaign.TemplateName)
	assert.Equal(t, "pt_BR", capturedCampaign.TemplateLang)
	// ULID id
	assert.Len(t, capturedCampaign.ID, 26)

	// Exactly 2 recipient inserts
	campaignRepo.AssertNumberOfCalls(t, "CreateRecipient", 2)
}

func TestCampaign_Create_NoRecipients_400(t *testing.T) {
	// Empty recipients list is now a 400. A campaign with no one to send to
	// is always a user error — we refuse early instead of persisting an
	// orphan draft that the worker will ignore.
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	router := setupCampaignRouter(campaignRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "Empty",
		"template_name": "x",
		"recipients":    []string{},
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "NO_RECIPIENTS", body["code"])
}

func TestCampaign_Create_WithCSVVars_Success(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)

	// Capture the recipients passed in: each must carry its own vars slice.
	var capturedRecipients []*domain.CampaignRecipient
	campaignRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	campaignRepo.On("CreateRecipient", mock.Anything,
		mock.MatchedBy(func(r *domain.CampaignRecipient) bool {
			capturedRecipients = append(capturedRecipients, r)
			return true
		})).Return(nil)

	router := setupCampaignRouter(campaignRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "CSV",
		"template_name": "t",
		"recipients_with_vars": []map[string]interface{}{
			{"phone": "5511111", "vars": []string{"João", "123"}},
			{"phone": "5511222", "vars": []string{"Maria", "456"}},
			{"phone": "5511111", "vars": []string{"Duplicate"}}, // dupe → dropped
		},
	}, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Len(t, capturedRecipients, 2, "duplicate phone should be dropped")
	assert.Equal(t, "João", capturedRecipients[0].Vars[0])
	assert.Equal(t, "Maria", capturedRecipients[1].Vars[0])
}

// ──────────────────────────────────────────────────────────────────
// List + pagination
// ──────────────────────────────────────────────────────────────────

func TestCampaign_List_Success(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("ListByAccount", mock.Anything, testAccountID, int32(50), int32(0)).Return([]*domain.Campaign{
		{ID: "c1", Name: "A", AccountID: testAccountID, Status: domain.CampaignDone},
	}, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/campaigns", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Len(t, body, 1)
	assert.Equal(t, "c1", body[0]["ID"])
}

// ──────────────────────────────────────────────────────────────────
// Cancel — state machine
// ──────────────────────────────────────────────────────────────────

func TestCampaign_Cancel_ScheduledCampaign_OK(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "c1", testAccountID).Return(&domain.Campaign{
		ID: "c1", AccountID: testAccountID, Status: domain.CampaignScheduled,
	}, nil)
	campaignRepo.On("UpdateStatus", mock.Anything, "c1", domain.CampaignCanceled).Return(nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns/c1/cancel", nil, testToken)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCampaign_Cancel_TerminalCampaign_400(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "c1", testAccountID).Return(&domain.Campaign{
		ID: "c1", AccountID: testAccountID, Status: domain.CampaignDone,
	}, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns/c1/cancel", nil, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CAMPAIGN_TERMINAL", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// Delete — state machine (cannot delete while sending)
// ──────────────────────────────────────────────────────────────────

func TestCampaign_Delete_RunningCampaign_400(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "c1", testAccountID).Return(&domain.Campaign{
		ID: "c1", AccountID: testAccountID, Status: domain.CampaignSending,
	}, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/campaigns/c1", nil, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CAMPAIGN_RUNNING", body["code"])
}

func TestCampaign_Delete_DraftCampaign_OK(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "c1", testAccountID).Return(&domain.Campaign{
		ID: "c1", AccountID: testAccountID, Status: domain.CampaignDraft,
	}, nil)
	campaignRepo.On("Delete", mock.Anything, "c1", testAccountID).Return(nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/campaigns/c1", nil, testToken)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Recipients listing
// ──────────────────────────────────────────────────────────────────

// ──────────────────────────────────────────────────────────────────
// Plan-based broadcast limits
// ──────────────────────────────────────────────────────────────────

// setupCampaignRouterWithLimits wires a handler where the CreateCampaign
// usecase has account/plan/usage repos — so we can assert the plan-cap
// branch fires as expected.
func setupCampaignRouterWithLimits(
	campaignRepo *MockCampaignRepo,
	channelRepo *MockChannelRepo,
	accountRepo *MockAccountRepo,
	planRepo *MockPlanRepo,
	usageRepo *MockUsageRepo,
	jwtMock *MockJWTService,
) *gin.Engine {
	createUC := usecases.NewCreateCampaignUseCase(campaignRepo, channelRepo, accountRepo, planRepo, usageRepo)
	listUC := usecases.NewListCampaignsUseCase(campaignRepo)
	getUC := usecases.NewGetCampaignUseCase(campaignRepo)
	deleteUC := usecases.NewDeleteCampaignUseCase(campaignRepo)
	cancelUC := usecases.NewCancelCampaignUseCase(campaignRepo)

	h := NewCampaignHandler(createUC, listUC, getUC, deleteUC, cancelUC, campaignRepo)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/campaigns", authMw, middleware.RequireRole("admin"))
	{
		g.POST("", h.HandleCreateCampaign)
	}
	return router
}

func planWithBroadcast(msgsPerMonth, perCampaign int) *domain.Plan {
	planID := "plan-1"
	// Non-nil StripePriceID = paid (non-contract) plan, so the
	// EffectiveMax accessors honor the cap-of-zero rejection path.
	stripePrice := "price_paid_test"
	return &domain.Plan{
		ID:                           planID,
		StripePriceID:                &stripePrice,
		MaxBroadcastMessagesPerMonth: msgsPerMonth,
		MaxRecipientsPerCampaign:     perCampaign,
	}
}

func accountOnPlan(planID string) *domain.Account {
	id := planID
	return &domain.Account{ID: "acc-1", PlanID: &id}
}

func usageWithBroadcastsUsed(n int32) *domain.AccountUsage {
	return &domain.AccountUsage{AccountID: "acc-1", BroadcastMessagesUsed: &n}
}

func TestCampaign_Create_TrialBlocksBroadcasts(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)

	// Trial plan — MaxBroadcastMessagesPerMonth == 0 → broadcasts disallowed.
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(accountOnPlan("trial-1"), nil)
	planRepo.On("GetByID", mock.Anything, "trial-1").Return(planWithBroadcast(0, 0), nil)

	router := setupCampaignRouterWithLimits(campaignRepo, channelRepo, accountRepo, planRepo, usageRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "n",
		"template_name": "t",
		"recipients":    []string{"5511111"},
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "BROADCAST_NOT_ALLOWED", body["code"])
	campaignRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCampaign_Create_ExceedsPerCampaignCap(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)

	// Plan allows up to 500 per campaign but admin sends 501.
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(accountOnPlan("p"), nil)
	planRepo.On("GetByID", mock.Anything, "p").Return(planWithBroadcast(10000, 500), nil)

	recs := make([]string, 501)
	for i := range recs {
		recs[i] = "5511" + strings.Repeat("9", 5) + strings.Repeat("0", 1)
		// Make each unique
		recs[i] = fmt.Sprintf("r-%04d", i)
	}
	router := setupCampaignRouterWithLimits(campaignRepo, channelRepo, accountRepo, planRepo, usageRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "n",
		"template_name": "t",
		"recipients":    recs,
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "RECIPIENTS_OVER_CAMPAIGN_CAP", body["code"])
	campaignRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCampaign_Create_ExceedsMonthlyBudget(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)

	// Plan 1000/month, already used 990. Requesting 20 → 1010 > 1000.
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(accountOnPlan("p"), nil)
	planRepo.On("GetByID", mock.Anything, "p").Return(planWithBroadcast(1000, 1000), nil)
	usageRepo.On("GetByAccountID", mock.Anything, testAccountID).Return(usageWithBroadcastsUsed(990), nil)

	recs := make([]string, 20)
	for i := range recs {
		recs[i] = fmt.Sprintf("r-%02d", i)
	}
	router := setupCampaignRouterWithLimits(campaignRepo, channelRepo, accountRepo, planRepo, usageRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "n",
		"template_name": "t",
		"recipients":    recs,
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "BROADCAST_BUDGET_EXCEEDED", body["code"])
	assert.Contains(t, body["message"], "990")
}

func TestCampaign_Create_WithinBudget_ReservesUsage(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(accountOnPlan("p"), nil)
	planRepo.On("GetByID", mock.Anything, "p").Return(planWithBroadcast(1000, 1000), nil)
	usageRepo.On("GetByAccountID", mock.Anything, testAccountID).Return(usageWithBroadcastsUsed(10), nil)

	// Campaign persists — mocks for Create + recipient inserts + usage reserve.
	campaignRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	campaignRepo.On("CreateRecipient", mock.Anything, mock.Anything).Return(nil)
	usageRepo.On("AddBroadcastMessages", mock.Anything, testAccountID, int32(3)).Return(nil)

	router := setupCampaignRouterWithLimits(campaignRepo, channelRepo, accountRepo, planRepo, usageRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "ok",
		"template_name": "t",
		"recipients":    []string{"a", "b", "c"},
	}, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)
	usageRepo.AssertCalled(t, "AddBroadcastMessages", mock.Anything, testAccountID, int32(3))
}

func TestCampaign_Create_NoAccount_FallsToTrialPlan_Blocks(t *testing.T) {
	// Edge case: account lookup returns nil (corrupted state) — usecase must
	// NOT crash; falls back to TrialPlan which has cap 0 so broadcasts are
	// refused.
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(nil, nil)

	router := setupCampaignRouterWithLimits(campaignRepo, channelRepo, accountRepo, planRepo, usageRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "n",
		"template_name": "t",
		"recipients":    []string{"a"},
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "BROADCAST_NOT_ALLOWED", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// Opt-out filtering at campaign creation (LGPD)
// ──────────────────────────────────────────────────────────────────

func setupCampaignRouterWithContactRepo(
	campaignRepo *MockCampaignRepo,
	channelRepo *MockChannelRepo,
	contactRepo *MockContactRepo,
	jwtMock *MockJWTService,
) *gin.Engine {
	createUC := usecases.
		NewCreateCampaignUseCase(campaignRepo, channelRepo, nil, nil, nil).
		WithContactRepo(contactRepo)
	listUC := usecases.NewListCampaignsUseCase(campaignRepo)
	getUC := usecases.NewGetCampaignUseCase(campaignRepo)
	deleteUC := usecases.NewDeleteCampaignUseCase(campaignRepo)
	cancelUC := usecases.NewCancelCampaignUseCase(campaignRepo)

	h := NewCampaignHandler(createUC, listUC, getUC, deleteUC, cancelUC, campaignRepo)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/campaigns", authMw, middleware.RequireRole("admin"))
	g.POST("", h.HandleCreateCampaign)
	return router
}

func TestCampaign_Create_FiltersOptedOutRecipients(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	contactRepo := new(MockContactRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)

	// "5511222" is opted out → must be dropped pre-insert.
	contactRepo.On("ListOptedOutExternalIDs", mock.Anything, testAccountID, mock.Anything).
		Return(map[string]bool{"5511222": true}, nil)

	var captured *domain.Campaign
	campaignRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.Campaign) bool {
		captured = c
		return true
	})).Return(nil)
	campaignRepo.On("CreateRecipient", mock.Anything, mock.Anything).Return(nil)

	router := setupCampaignRouterWithContactRepo(campaignRepo, channelRepo, contactRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "Promo",
		"template_name": "t",
		"recipients":    []string{"5511111", "5511222", "5511333"},
	}, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotNil(t, captured)
	// 3 requested − 1 opted out = 2
	assert.Equal(t, 2, captured.TotalRecipients)
	campaignRepo.AssertNumberOfCalls(t, "CreateRecipient", 2)
}

func TestCampaign_Create_AllRecipientsOptedOut_400(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	channelRepo := new(MockChannelRepo)
	contactRepo := new(MockContactRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelOK("ch-1", testAccountID), nil)
	contactRepo.On("ListOptedOutExternalIDs", mock.Anything, testAccountID, mock.Anything).
		Return(map[string]bool{"5511111": true, "5511222": true}, nil)

	router := setupCampaignRouterWithContactRepo(campaignRepo, channelRepo, contactRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/campaigns", map[string]interface{}{
		"channel_id":    "ch-1",
		"name":          "Promo",
		"template_name": "t",
		"recipients":    []string{"5511111", "5511222"},
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "ALL_RECIPIENTS_OPTED_OUT", body["code"])
	campaignRepo.AssertNotCalled(t, "Create")
}

func TestCampaign_ListRecipients_Success(t *testing.T) {
	campaignRepo := new(MockCampaignRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	campaignRepo.On("GetByID", mock.Anything, "c1", testAccountID).Return(&domain.Campaign{
		ID: "c1", AccountID: testAccountID,
	}, nil)
	campaignRepo.On("ListRecipients", mock.Anything, "c1", int32(200), int32(0)).Return([]*domain.CampaignRecipient{
		{ID: "r1", CampaignID: "c1", Recipient: "5511", Status: domain.RecipientSent},
	}, nil)

	router := setupCampaignRouter(campaignRepo, new(MockChannelRepo), jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/campaigns/c1/recipients", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Len(t, body, 1)
}
