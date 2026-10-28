package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// rawDBError mimics what a pgx/driver error string looks like when it
// bubbles up unhandled: it names a table and column. None of this may
// ever reach the client (US-010).
const rawDBError = `ERROR: duplicate key value violates unique constraint "users_email_key" (SQLSTATE 23505) DETAIL: Key (email)=(victim@example.com) already exists.`

// runRespondError spins a one-route gin engine, sets a request ID in the
// context (as the RequestID middleware would), invokes respondError with
// the given error, and returns the recorded response.
func runRespondError(err error) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/boom", func(c *gin.Context) {
		c.Set("requestID", "01HXREQIDFORTEST0000000000")
		respondError(c, err)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)
	return w
}

// TestRespondError_RawDBErrorSanitized: a raw driver error returned as a
// plain error must come back as a generic 500 with no schema, constraint
// name, table, column, SQLSTATE, or PII in the body.
func TestRespondError_RawDBErrorSanitized(t *testing.T) {
	w := runRespondError(assertErr(rawDBError))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	bodyStr := w.Body.String()
	for _, leak := range []string{"users_email_key", "constraint", "SQLSTATE", "23505", "victim@example.com", "email", "DETAIL"} {
		assert.NotContainsf(t, strings.ToLower(bodyStr), strings.ToLower(leak),
			"5xx body leaked %q: %s", leak, bodyStr)
	}

	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL_ERROR", body.Code)
	assert.Equal(t, "internal server error", body.Message)
	// Request ID echoed so the user can quote it to support.
	assert.Equal(t, "01HXREQIDFORTEST0000000000", body.Metadata["request_id"])
}

// TestRespondError_AppError5xxSanitized: an AppError that maps to a 5xx
// status (here AI_PROVIDER_UNAVAILABLE → 502) must also have its Message
// scrubbed even though it is a typed domain error — a developer may have
// stuffed err.Error() into Message.
func TestRespondError_AppError5xxSanitized(t *testing.T) {
	appErr := &domain.AppError{
		Code:    "AI_PROVIDER_UNAVAILABLE",
		Message: rawDBError, // pretend someone leaked a raw error in here
	}
	w := runRespondError(appErr)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	bodyStr := strings.ToLower(w.Body.String())
	assert.NotContains(t, bodyStr, "users_email_key")
	assert.NotContains(t, bodyStr, "victim@example.com")

	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "AI_PROVIDER_UNAVAILABLE", body.Code)
	assert.Equal(t, "internal server error", body.Message)
}

// TestRespondError_4xxMessagePreserved: client-fault errors keep their
// Message — that text is deliberate, user-facing validation feedback and
// must NOT be scrubbed (regression guard so the sanitizer doesn't go too
// far).
func TestRespondError_4xxMessagePreserved(t *testing.T) {
	appErr := &domain.AppError{
		Code:    "AGENT_NOT_FOUND",
		Message: "agent not found",
	}
	w := runRespondError(appErr)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "AGENT_NOT_FOUND", body.Code)
	assert.Equal(t, "agent not found", body.Message)
}

// TestRespondError_OwnerEmailExistsIs409: creating a contract with an
// email that already belongs to an account is a conflict, not a generic
// 400. Regression guard — the code used to be absent from
// httpStatusForCode and fell through to the default 400, giving the
// operator no signal to switch to the "existing account" flow.
func TestRespondError_OwnerEmailExistsIs409(t *testing.T) {
	appErr := &domain.AppError{
		Code:    "OWNER_EMAIL_EXISTS",
		Message: "a user with this email already exists",
	}
	w := runRespondError(appErr)

	assert.Equal(t, http.StatusConflict, w.Code)
	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "OWNER_EMAIL_EXISTS", body.Code)
	assert.Equal(t, "a user with this email already exists", body.Message)
}

// TestRespondError_OwnerValidationStaysBadRequest: the pure input-
// validation owner codes are NOT in the map and must keep defaulting to
// 400 (the conflict/not-found mappings are deliberately the only ones
// promoted).
func TestRespondError_OwnerValidationStaysBadRequest(t *testing.T) {
	w := runRespondError(&domain.AppError{
		Code:    "OWNER_NAME_REQUIRED",
		Message: "company name is required",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRespondError_USDBillingCodes: the USD billing conflict codes must
// be registered in httpStatusForCode (US-017 review fixes). Before the
// mapping, PLAN_CURRENCY_MISMATCH and ADDON_CURRENCY_UNSUPPORTED fell
// through to the generic 400, so the app couldn't distinguish "your
// account bills in the other currency" from a malformed request.
func TestRespondError_USDBillingCodes(t *testing.T) {
	cases := []struct {
		err  *domain.AppError
		want int
	}{
		{domain.ErrPlanCurrencyMismatch, http.StatusConflict},
		{domain.ErrAddonCurrencyUnsupported, http.StatusConflict},
		{domain.ErrPlanNotFound, http.StatusNotFound},
		{domain.ErrCouponExhausted, http.StatusConflict},
	}
	for _, c := range cases {
		w := runRespondError(c.err)
		assert.Equalf(t, c.want, w.Code, "code %s", c.err.Code)

		var body domain.AppError
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, c.err.Code, body.Code)
		// 4xx keeps its deliberate user-facing message.
		assert.Equal(t, c.err.Message, body.Message)
	}
}

// assertErr is a tiny error wrapper so tests can build a plain (non
// AppError) error from a string without importing errors in every test.
type assertErr string

func (e assertErr) Error() string { return string(e) }
