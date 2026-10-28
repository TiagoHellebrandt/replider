package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A handler that already answered, followed by a gate that wants to deny.
// Appending a second document produces one 200 carrying two JSON bodies —
// the client gets a parse error instead of a permission error, so the
// failure reads as "the app is broken" rather than "you cannot do this".
// Observed in the wild on POST /funnels/:id/leads/draft-batch:
//
//	{"drafted":4,...}{"code":"UNAUTHORIZED","message":"account not found"}

type noAccountRepo struct{ domain.AccountRepository }

func (noAccountRepo) GetByID(context.Context, string) (*domain.Account, error) {
	return nil, nil // account vanished — the gate wants to deny
}

func TestSubscriptionGate_NeverAppendsToAnAnsweredRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	gate := RequireActiveSubscription(noAccountRepo{})
	r.POST("/x", func(c *gin.Context) {
		c.Set("accountID", "acc-1")
		// The handler answers first — whatever ordering bug puts the gate
		// after it, the response is already on the wire.
		c.JSON(http.StatusOK, gin.H{"drafted": 4})
		gate(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))

	// Exactly one JSON document, decodable in full.
	var body map[string]any
	dec := json.NewDecoder(w.Body)
	require.NoError(t, dec.Decode(&body), "corpo: %s", w.Body.String())
	assert.Equal(t, float64(4), body["drafted"])
	assert.False(t, dec.More(), "resposta trouxe um segundo documento JSON: %s", w.Body.String())
}

// The gate must still deny normally when it runs before any answer.
func TestSubscriptionGate_StillDeniesWhenItRunsFirst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", func(c *gin.Context) { c.Set("accountID", "acc-1") },
		RequireActiveSubscription(noAccountRepo{}),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"drafted": 4}) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "account not found")
	assert.NotContains(t, w.Body.String(), "drafted", "o handler rodou mesmo negado")
}
