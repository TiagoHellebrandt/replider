package resend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient("re_test_key")
	c.SetBaseURL(srv.URL)
	return c
}

func TestCreateDomain_ReturnsRecords(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/domains", r.URL.Path)
		require.Equal(t, "Bearer re_test_key", r.Header.Get("Authorization"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "cuiabamed.com.br", body["name"])
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "dom_1", "name": "cuiabamed.com.br", "status": "not_started",
			"records": []map[string]any{
				{"record": "DKIM", "name": "resend._domainkey", "type": "TXT", "value": "p=abc"},
			},
		})
	})

	dom, err := c.CreateDomain(context.Background(), " Cuiabamed.com.br ")
	require.NoError(t, err)
	assert.Equal(t, "dom_1", dom.ID)
	assert.False(t, dom.Verified())
	require.Len(t, dom.Records, 1)
	assert.Equal(t, "resend._domainkey", dom.Records[0].Name)
}

// A domain already registered must be REUSED, not errored: enabling volume
// twice (or re-opening the screen after a half-finished setup) is normal.
func TestCreateDomain_ConflictReusesExisting(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /domains":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"statusCode":409,"name":"validation_error","message":"already exists"}`))
		case "GET /domains":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "dom_9", "name": "cuiabamed.com.br", "status": "pending"}},
			})
		case "GET /domains/dom_9":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "dom_9", "name": "cuiabamed.com.br", "status": "pending",
				"records": []map[string]any{{"record": "SPF", "name": "send", "type": "TXT", "value": "v=spf1"}},
			})
		default:
			t.Fatalf("unexpected call %s %s", r.Method, r.URL.Path)
		}
	})

	dom, err := c.CreateDomain(context.Background(), "cuiabamed.com.br")
	require.NoError(t, err)
	assert.Equal(t, "dom_9", dom.ID)
	require.Len(t, dom.Records, 1)
}

func TestSendMessageAndGetID_BulkSend(t *testing.T) {
	var got map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/emails", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "email_1"})
	})

	text := "Olá, vi que o hospital opera dois tomógrafos."
	msg := &domain.Message{Content: &text, Metadata: map[string]any{"subject": "Manutenção preventiva"}}
	ch := &domain.Channel{Name: "Email", Type: "email", Config: map[string]any{
		"from_address": "vendas@cuiabamed.com.br", "from_name": "Cuiabamed",
	}}

	id, err := c.SendMessageAndGetID(context.Background(), ch, "compras@hospital.com.br", msg)
	require.NoError(t, err)
	assert.Equal(t, "email_1", id)
	assert.Equal(t, "Cuiabamed <vendas@cuiabamed.com.br>", got["from"])
	assert.Equal(t, "vendas@cuiabamed.com.br", got["reply_to"])
	assert.Equal(t, "Manutenção preventiva", got["subject"])
	assert.Equal(t, text, got["text"])
}

// The mail still claims the customer's identity: no from_address, no send.
func TestSendMessageAndGetID_RequiresFromAddress(t *testing.T) {
	c := NewClient("re_test_key")
	text := "corpo"
	msg := &domain.Message{Content: &text}
	_, err := c.SendMessageAndGetID(context.Background(), &domain.Channel{Type: "email", Config: map[string]any{}}, "a@b.com", msg)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	assert.Equal(t, "CHANNEL_MISSING_CONFIG", appErr.Code)
}

// No key = fail closed, loudly, before any network call.
func TestUnconfiguredFailsClosed(t *testing.T) {
	c := NewClient("")
	_, err := c.CreateDomain(context.Background(), "x.com")
	assert.ErrorIs(t, err, domain.ErrBulkSendingUnavailable)
	text := "oi"
	_, err = c.SendMessageAndGetID(context.Background(), &domain.Channel{Type: "email"}, "a@b.com", &domain.Message{Content: &text})
	assert.ErrorIs(t, err, domain.ErrBulkSendingUnavailable)
}

func TestProviderErrorSurfacesMessage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"statusCode":422,"name":"validation_error","message":"invalid domain name"}`))
	})
	_, err := c.CreateDomain(context.Background(), "not a domain")
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	assert.Equal(t, "BULK_PROVIDER_ERROR", appErr.Code)
	assert.Contains(t, appErr.Message, "invalid domain name")
}
