package captcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewTurnstileVerifier_EmptySecretReturnsNil: no secret → nil verifier so
// callers can treat "unconfigured" as "captcha disabled".
func TestNewTurnstileVerifier_EmptySecretReturnsNil(t *testing.T) {
	assert.Nil(t, NewTurnstileVerifier("", nil))
	assert.Nil(t, NewTurnstileVerifier("   ", nil))
	assert.NotNil(t, NewTurnstileVerifier("secret", nil))
}

// TestTurnstileVerifier_Verify drives the verifier against a stub siteverify
// endpoint covering the success, failure, empty-token, and HTTP-error paths.
func TestTurnstileVerifier_Verify(t *testing.T) {
	var lastForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		lastForm = r.Form
		// Echo success only for the magic "valid" token.
		if r.FormValue("response") == "valid" {
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("test-secret", srv.Client())
	v.url = srv.URL

	t.Run("valid token passes and forwards fields", func(t *testing.T) {
		ok, err := v.Verify(context.Background(), "valid", "8.8.8.8")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "test-secret", lastForm.Get("secret"))
		assert.Equal(t, "valid", lastForm.Get("response"))
		assert.Equal(t, "8.8.8.8", lastForm.Get("remoteip"))
	})

	t.Run("invalid token fails", func(t *testing.T) {
		ok, err := v.Verify(context.Background(), "nope", "")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("empty token short-circuits without a call", func(t *testing.T) {
		ok, err := v.Verify(context.Background(), "", "")
		assert.NoError(t, err)
		assert.False(t, ok)
	})
}

// TestTurnstileVerifier_Verify_HTTPError: a non-200 from siteverify surfaces an
// error so the middleware can fail closed.
func TestTurnstileVerifier_Verify_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("test-secret", srv.Client())
	v.url = srv.URL

	ok, err := v.Verify(context.Background(), "valid", "")
	assert.Error(t, err)
	assert.False(t, ok)
}
