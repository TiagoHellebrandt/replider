package prospecting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAdapter points an adapter at a fake server and disables the limiter so
// tests don't pay the real per-request pacing delay.
func newTestAdapter(key, baseURL string) *GooglePlacesAdapter {
	a := NewGooglePlacesAdapter(key, nil)
	a.baseURL = baseURL
	a.limiter = nil
	return a
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func TestTextSearch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/places:searchText", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("X-Goog-Api-Key"))
		assert.Contains(t, r.Header.Get("X-Goog-FieldMask"), "places.internationalPhoneNumber")
		body := decodeBody(t, r)
		assert.Equal(t, "dentistas em São Paulo", body["textQuery"])
		assert.Equal(t, "pt-BR", body["languageCode"])

		_, _ = io.WriteString(w, `{
			"places": [
				{"id":"p1","displayName":{"text":"Clínica A"},"formattedAddress":"Rua 1","internationalPhoneNumber":"+55 11 1111-1111","websiteUri":"https://a.example"},
				{"id":"p2","displayName":{"text":"Clínica B"},"formattedAddress":"Rua 2","nationalPhoneNumber":"(11) 2222-2222"}
			]
		}`)
	}))
	defer srv.Close()

	a := newTestAdapter("test-key", srv.URL)
	out, err := a.TextSearch(context.Background(), TextSearchParams{Query: "dentistas em São Paulo", LanguageCode: "pt-BR"})
	require.NoError(t, err)
	require.Len(t, out, 2)

	assert.Equal(t, "p1", out[0].PlaceID)
	assert.Equal(t, "Clínica A", out[0].Name)
	assert.Equal(t, "+55 11 1111-1111", out[0].Phone)
	assert.Equal(t, "Rua 1", out[0].Address)
	assert.Equal(t, "https://a.example", out[0].Website)
	// Falls back to nationalPhoneNumber when international is absent.
	assert.Equal(t, "(11) 2222-2222", out[1].Phone)
	assert.Empty(t, out[1].Website)
}

func TestTextSearch_Pagination(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body := decodeBody(t, r)
		if calls == 1 {
			// First page: no token sent, return a nextPageToken.
			_, hasToken := body["pageToken"]
			assert.False(t, hasToken)
			_, _ = io.WriteString(w, `{
				"places":[{"id":"p1","displayName":{"text":"A"}}],
				"nextPageToken":"TOKEN-2"
			}`)
			return
		}
		// Second page: token echoed back, no further token.
		assert.Equal(t, "TOKEN-2", body["pageToken"])
		_, _ = io.WriteString(w, `{
			"places":[{"id":"p2","displayName":{"text":"B"}}]
		}`)
	}))
	defer srv.Close()

	a := newTestAdapter("test-key", srv.URL)
	out, err := a.TextSearch(context.Background(), TextSearchParams{Query: "q", MaxResults: 50})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	require.Len(t, out, 2)
	assert.Equal(t, "p1", out[0].PlaceID)
	assert.Equal(t, "p2", out[1].PlaceID)
}

func TestTextSearch_MaxResultsStopsPaging(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Always offer another page; the adapter must stop once the cap is hit.
		_, _ = io.WriteString(w, `{
			"places":[{"id":"p1","displayName":{"text":"A"}}],
			"nextPageToken":"MORE"
		}`)
	}))
	defer srv.Close()

	a := newTestAdapter("test-key", srv.URL)
	out, err := a.TextSearch(context.Background(), TextSearchParams{Query: "q", MaxResults: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "should not fetch a second page once MaxResults reached")
	require.Len(t, out, 1)
}

func TestNearbySearch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/places:searchNearby", r.URL.Path)
		body := decodeBody(t, r)
		restriction := body["locationRestriction"].(map[string]any)
		circle := restriction["circle"].(map[string]any)
		assert.Equal(t, float64(1500), circle["radius"])
		assert.Equal(t, []any{"restaurant"}, body["includedTypes"])

		_, _ = io.WriteString(w, `{
			"places":[{"id":"p1","displayName":{"text":"Bar do Zé"},"formattedAddress":"Av 9","internationalPhoneNumber":"+55 11 9999"}]
		}`)
	}))
	defer srv.Close()

	a := newTestAdapter("test-key", srv.URL)
	out, err := a.NearbySearch(context.Background(), NearbySearchParams{
		Latitude:      -23.5,
		Longitude:     -46.6,
		RadiusMeters:  1500,
		IncludedTypes: []string{"restaurant"},
	})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "Bar do Zé", out[0].Name)
	assert.Equal(t, "+55 11 9999", out[0].Phone)
}

func TestSearch_APIKeyMissing(t *testing.T) {
	a := NewGooglePlacesAdapter("   ", nil) // whitespace trims to empty
	assert.False(t, a.Configured())

	_, err := a.TextSearch(context.Background(), TextSearchParams{Query: "q"})
	assert.ErrorIs(t, err, ErrAPIKeyMissing)

	_, err = a.NearbySearch(context.Background(), NearbySearchParams{RadiusMeters: 100})
	assert.ErrorIs(t, err, ErrAPIKeyMissing)
}

func TestTextSearch_QueryRequired(t *testing.T) {
	a := newTestAdapter("test-key", "http://unused")
	_, err := a.TextSearch(context.Background(), TextSearchParams{Query: "  "})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAPIKeyMissing)
}

func TestTextSearch_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"quota"}}`)
	}))
	defer srv.Close()

	a := newTestAdapter("test-key", srv.URL)
	_, err := a.TextSearch(context.Background(), TextSearchParams{Query: "q"})
	assert.ErrorIs(t, err, ErrRateLimited)
}

func TestTextSearch_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"bad field mask"}}`)
	}))
	defer srv.Close()

	a := newTestAdapter("test-key", srv.URL)
	_, err := a.TextSearch(context.Background(), TextSearchParams{Query: "q"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad field mask")
}

// Compile-time assertion: the adapter satisfies the port the use case depends on.
var _ ProspectingProvider = (*GooglePlacesAdapter)(nil)
