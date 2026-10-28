package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testPlansFixture() []*domain.Plan {
	return []*domain.Plan{
		{
			ID:                       "plan-brl-1",
			Name:                     "Solo",
			Currency:                 "brl",
			Price:                    19900,
			AIProvider:               "google",
			AIModel:                  "gemini-2.5-flash",
			MaxAgents:                1,
			MaxChannels:              2,
			MaxConversationsPerMonth: 200,
			MaxAITokensPerMonth:      100000,
			IsListed:                 true,
			CreatedAt:                time.Now(),
		},
		{
			ID:                       "plan-usd-1",
			Name:                     "Core",
			Currency:                 "usd",
			Price:                    9900,
			AIProvider:               "google",
			AIModel:                  "gemini-3.1-flash-lite",
			MaxAgents:                3,
			MaxChannels:              3,
			MaxConversationsPerMonth: 1000,
			MaxAITokensPerMonth:      35000000,
			IsListed:                 true,
			CreatedAt:                time.Now(),
		},
	}
}

// Unlisted plans (is_listed = false, e.g. the retired Partner plan)
// never surface in the public catalogue, regardless of currency (US-001
// vendedores v3).
func TestListPlans_HidesUnlistedPlans(t *testing.T) {
	plans := testPlansFixture()
	plans = append(plans, &domain.Plan{
		ID:       "01PLAN_PARTNER_00000000",
		Name:     "Partner",
		Currency: "brl",
		IsListed: false,
	})

	planRepo := new(MockPlanRepo)
	planRepo.On("ListAll", mock.Anything).Return(plans, nil)

	handler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Solo", resp[0].Name)

	planRepo.AssertExpectations(t)
}

func TestListPlans_DefaultsToBRL(t *testing.T) {
	planRepo := new(MockPlanRepo)
	planRepo.On("ListAll", mock.Anything).Return(testPlansFixture(), nil)

	handler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Solo", resp[0].Name)
	assert.Equal(t, "brl", resp[0].Currency)

	planRepo.AssertExpectations(t)
}

func TestListPlans_CurrencyParamUSD(t *testing.T) {
	planRepo := new(MockPlanRepo)
	planRepo.On("ListAll", mock.Anything).Return(testPlansFixture(), nil)

	handler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans?currency=usd", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Core", resp[0].Name)
	assert.Equal(t, "usd", resp[0].Currency)

	planRepo.AssertExpectations(t)
}

func TestListPlans_InvalidCurrency(t *testing.T) {
	planRepo := new(MockPlanRepo)

	handler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans?currency=eur", nil, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_CURRENCY", resp["code"])

	planRepo.AssertNotCalled(t, "ListAll", mock.Anything)
}

// An authenticated caller is pinned to the account's currency — even
// when the query param asks for the other one (US-002).
func TestListPlans_AuthenticatedUsesAccountCurrency(t *testing.T) {
	planRepo := new(MockPlanRepo)
	planRepo.On("ListAll", mock.Anything).Return(testPlansFixture(), nil)

	accountRepo := new(MockAccountRepo)
	accountRepo.On("GetByID", mock.Anything, "acc_usd").
		Return(&domain.Account{ID: "acc_usd", Currency: "usd"}, nil)

	handler := NewPlansHandler(planRepo, accountRepo)

	router := setupTestRouter()
	router.GET("/v1/plans", func(c *gin.Context) {
		c.Set("accountID", "acc_usd")
	}, handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans?currency=brl", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "usd", resp[0].Currency)

	planRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}

// An account-currency lookup failure degrades to the anonymous path
// instead of failing the listing.
func TestListPlans_AccountLookupErrorFallsBack(t *testing.T) {
	planRepo := new(MockPlanRepo)
	planRepo.On("ListAll", mock.Anything).Return(testPlansFixture(), nil)

	accountRepo := new(MockAccountRepo)
	accountRepo.On("GetByID", mock.Anything, "acc_broken").
		Return((*domain.Account)(nil), errors.New("db error"))

	handler := NewPlansHandler(planRepo, accountRepo)

	router := setupTestRouter()
	router.GET("/v1/plans", func(c *gin.Context) {
		c.Set("accountID", "acc_broken")
	}, handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans?currency=usd", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "usd", resp[0].Currency)
}

func TestListPlans_EmptyList(t *testing.T) {
	planRepo := new(MockPlanRepo)

	planRepo.On("ListAll", mock.Anything).Return([]*domain.Plan{}, nil)

	handler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 0)

	planRepo.AssertExpectations(t)
}

func TestListPlans_InternalError(t *testing.T) {
	planRepo := new(MockPlanRepo)

	planRepo.On("ListAll", mock.Anything).Return(([]*domain.Plan)(nil), errors.New("db error"))

	handler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", handler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans", nil, "")

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", resp["code"])

	planRepo.AssertExpectations(t)
}
