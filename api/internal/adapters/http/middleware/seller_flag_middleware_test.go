package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type fakeSellerFlags struct {
	sellers map[string]bool
	err     error
	calls   int
}

func (f *fakeSellerFlags) IsSeller(_ context.Context, userID string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.sellers[userID], nil
}

// performSellerRequest routes through RequireSeller with a handler that
// records whether it ran — the mint-a-ref-code stand-in.
func performSellerRequest(checker SellerFlagChecker, userID string) (*httptest.ResponseRecorder, *bool) {
	gin.SetMode(gin.TestMode)
	reached := false
	r := gin.New()
	r.POST("/seller/ref-code", func(c *gin.Context) {
		if userID != "" {
			c.Set("userID", userID)
		}
	}, RequireSeller(checker), func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"code": "VNDR1234"})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/seller/ref-code", nil)
	r.ServeHTTP(w, req)
	return w, &reached
}

func TestRequireSeller_AllowsFlaggedSeller(t *testing.T) {
	w, reached := performSellerRequest(&fakeSellerFlags{sellers: map[string]bool{"u1": true}}, "u1")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, *reached)
}

// The US-014 core: a regular platform-account user opening /seller must
// get 403 and the downstream handler (which would mint a ref code and
// make them payable) must never run.
func TestRequireSeller_403ForUnflaggedUser_NoMint(t *testing.T) {
	flags := &fakeSellerFlags{sellers: map[string]bool{}}
	w, reached := performSellerRequest(flags, "u-regular")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "SELLER_NOT_FLAGGED")
	assert.False(t, *reached, "handler ran — an unflagged user would have minted a ref code")
}

// Fail-closed: a lookup error denies, never lets the request through.
func TestRequireSeller_FailClosedOnLookupError(t *testing.T) {
	w, reached := performSellerRequest(&fakeSellerFlags{err: errors.New("db down")}, "u1")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.False(t, *reached)
}

// Nil checker (wiring gap) and missing userID also fail closed.
func TestRequireSeller_FailClosedOnNilCheckerOrNoUser(t *testing.T) {
	w, reached := performSellerRequest(nil, "u1")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, *reached)

	w2, reached2 := performSellerRequest(&fakeSellerFlags{sellers: map[string]bool{"u1": true}}, "")
	assert.Equal(t, http.StatusForbidden, w2.Code)
	assert.False(t, *reached2)
}
