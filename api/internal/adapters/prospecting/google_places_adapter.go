// Package prospecting provides outbound-sales lead discovery by querying
// third-party business directories. The concrete implementation here talks to
// the official Google Places API (New) — both Text Search and Nearby Search —
// returning normalized business candidates (name, phone, address, website,
// place_id) that the prospecting use case (US-022) turns into leads.
//
// The use case depends only on the small ProspectingProvider interface, so a
// fake (or an alternative directory) can swap in without touching the HTTP
// layer.
package prospecting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// Places API (New) endpoints. The New API returns phone + website directly in
// a single search response (via the X-Goog-FieldMask header), so no per-place
// Place Details round-trip is needed to satisfy the candidate shape.
const (
	defaultPlacesBaseURL = "https://places.googleapis.com"
	textSearchPath       = "/v1/places:searchText"
	nearbySearchPath     = "/v1/places:searchNearby"

	// fieldMask requests exactly the fields a candidate needs plus the paging
	// token. Asking for fewer fields is also cheaper on Google's SKU-based
	// billing, so this is deliberately narrow.
	placesFieldMask = "places.id,places.displayName,places.formattedAddress,places.internationalPhoneNumber,places.nationalPhoneNumber,places.websiteUri,nextPageToken"

	// maxPlacesPageSize is the per-request cap imposed by the Places API (New).
	maxPlacesPageSize = 20
)

// ErrAPIKeyMissing is returned by every search method when no Google Places API
// key is configured. The use case maps it to a clear, actionable API error.
var ErrAPIKeyMissing = errors.New("google places api key not configured")

// ErrRateLimited signals Google answered 429 (RESOURCE_EXHAUSTED). The caller
// can surface a "try again later" message rather than a generic failure.
var ErrRateLimited = errors.New("google places api rate limit exceeded")

// Candidate is a normalized business returned by a prospecting search. Phone
// and Website may be empty — not every listed business publishes them.
type Candidate struct {
	PlaceID string `json:"place_id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	Website string `json:"website"`
}

// TextSearchParams describes a free-text business search ("dentistas em São
// Paulo"). MaxResults caps how many candidates to collect across pages (the
// adapter pages until it reaches the cap or Google runs out of results).
type TextSearchParams struct {
	Query        string
	LanguageCode string // e.g. "pt-BR"; empty lets Google infer
	MaxResults   int
}

// NearbySearchParams describes a radius search around a coordinate. Nearby
// Search (New) does not page, so it returns at most one page (≤ MaxResults,
// capped at 20). IncludedTypes uses Google place types (e.g. "restaurant").
type NearbySearchParams struct {
	Latitude      float64
	Longitude     float64
	RadiusMeters  float64
	IncludedTypes []string
	LanguageCode  string
	MaxResults    int
}

// ProspectingProvider is the narrow port the prospecting use case depends on.
type ProspectingProvider interface {
	TextSearch(ctx context.Context, p TextSearchParams) ([]Candidate, error)
	NearbySearch(ctx context.Context, p NearbySearchParams) ([]Candidate, error)
}

// GooglePlacesAdapter implements ProspectingProvider against the Places API
// (New). A nil/empty key does not stop construction — the wiring stays simple
// and the methods fail with a clear ErrAPIKeyMissing instead.
type GooglePlacesAdapter struct {
	apiKey  string
	client  *http.Client
	baseURL string
	// limiter throttles outbound calls so a tight paging loop or a busy
	// prospecting batch can't trip Google's per-second quota. Defaults to
	// 10 req/s with a small burst; nil disables throttling (tests).
	limiter *rate.Limiter
}

// NewGooglePlacesAdapter builds an adapter bound to the given API key. A nil
// client falls back to a 15s-timeout client so a slow Google never hangs a
// prospecting request indefinitely.
func NewGooglePlacesAdapter(apiKey string, client *http.Client) *GooglePlacesAdapter {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &GooglePlacesAdapter{
		apiKey:  strings.TrimSpace(apiKey),
		client:  client,
		baseURL: defaultPlacesBaseURL,
		limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 5),
	}
}

// Configured reports whether an API key is present, so callers can decide
// up front (e.g. hide a prospecting button) without making a request.
func (a *GooglePlacesAdapter) Configured() bool {
	return a != nil && a.apiKey != ""
}

// --- Google wire types (subset we consume) ---

type placesLocalizedText struct {
	Text string `json:"text"`
}

type placesPlace struct {
	ID                 string              `json:"id"`
	DisplayName        placesLocalizedText `json:"displayName"`
	FormattedAddress   string              `json:"formattedAddress"`
	InternationalPhone string              `json:"internationalPhoneNumber"`
	NationalPhone      string              `json:"nationalPhoneNumber"`
	WebsiteURI         string              `json:"websiteUri"`
}

type placesSearchResponse struct {
	Places        []placesPlace `json:"places"`
	NextPageToken string        `json:"nextPageToken"`
}

type placesErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// TextSearch runs a paged Text Search, collecting candidates until MaxResults
// is reached or Google returns no further page token.
func (a *GooglePlacesAdapter) TextSearch(ctx context.Context, p TextSearchParams) ([]Candidate, error) {
	if !a.Configured() {
		return nil, ErrAPIKeyMissing
	}
	if strings.TrimSpace(p.Query) == "" {
		return nil, errors.New("text search query is required")
	}

	limit := normalizeMaxResults(p.MaxResults)
	var out []Candidate
	pageToken := ""

	for {
		body := map[string]any{
			"textQuery": p.Query,
			"pageSize":  pageSizeFor(limit - len(out)),
		}
		if p.LanguageCode != "" {
			body["languageCode"] = p.LanguageCode
		}
		if pageToken != "" {
			body["pageToken"] = pageToken
		}

		resp, err := a.doSearch(ctx, textSearchPath, body)
		if err != nil {
			return nil, err
		}

		out = appendCandidates(out, resp.Places, limit)
		pageToken = resp.NextPageToken

		if pageToken == "" || len(out) >= limit {
			break
		}
		// The Places API (New) needs a brief delay before a page token is
		// valid; the limiter's pacing doubles as that grace period.
	}

	return out, nil
}

// NearbySearch runs a single Nearby Search (the New API does not page nearby
// results) and returns up to MaxResults candidates.
func (a *GooglePlacesAdapter) NearbySearch(ctx context.Context, p NearbySearchParams) ([]Candidate, error) {
	if !a.Configured() {
		return nil, ErrAPIKeyMissing
	}
	if p.RadiusMeters <= 0 {
		return nil, errors.New("nearby search radius must be positive")
	}

	limit := normalizeMaxResults(p.MaxResults)
	body := map[string]any{
		"maxResultCount": pageSizeFor(limit),
		"locationRestriction": map[string]any{
			"circle": map[string]any{
				"center": map[string]any{
					"latitude":  p.Latitude,
					"longitude": p.Longitude,
				},
				"radius": p.RadiusMeters,
			},
		},
	}
	if len(p.IncludedTypes) > 0 {
		body["includedTypes"] = p.IncludedTypes
	}
	if p.LanguageCode != "" {
		body["languageCode"] = p.LanguageCode
	}

	resp, err := a.doSearch(ctx, nearbySearchPath, body)
	if err != nil {
		return nil, err
	}
	return appendCandidates(nil, resp.Places, limit), nil
}

// doSearch performs one rate-limited, authenticated POST and decodes the
// response (or maps the error envelope to a clear error).
func (a *GooglePlacesAdapter) doSearch(ctx context.Context, path string, body map[string]any) (*placesSearchResponse, error) {
	if a.limiter != nil {
		if err := a.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("places rate limiter: %w", err)
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal places request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("build places request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", a.apiKey)
	req.Header.Set("X-Goog-FieldMask", placesFieldMask)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call places api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		var env placesErrorEnvelope
		_ = json.NewDecoder(resp.Body).Decode(&env)
		msg := strings.TrimSpace(env.Error.Message)
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		slog.Warn("google places search failed", "status", resp.StatusCode, "api_status", env.Error.Status, "message", msg)
		return nil, fmt.Errorf("places api error: %s", msg)
	}

	var out placesSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode places response: %w", err)
	}
	return &out, nil
}

// --- helpers ---

func normalizeMaxResults(n int) int {
	if n <= 0 {
		return maxPlacesPageSize // one page by default
	}
	return n
}

// pageSizeFor clamps a desired count to the Places API per-request bounds.
func pageSizeFor(remaining int) int {
	if remaining > maxPlacesPageSize {
		return maxPlacesPageSize
	}
	if remaining < 1 {
		return 1
	}
	return remaining
}

// appendCandidates normalizes Google places onto out, stopping at limit.
func appendCandidates(out []Candidate, places []placesPlace, limit int) []Candidate {
	for _, pl := range places {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(pl.ID) == "" {
			continue
		}
		phone := pl.InternationalPhone
		if phone == "" {
			phone = pl.NationalPhone
		}
		out = append(out, Candidate{
			PlaceID: pl.ID,
			Name:    pl.DisplayName.Text,
			Phone:   strings.TrimSpace(phone),
			Address: pl.FormattedAddress,
			Website: pl.WebsiteURI,
		})
	}
	return out
}
