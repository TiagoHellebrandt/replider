package enrichment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// geminiTextResponse builds a minimal generateContent response whose single
// candidate part carries the given text, plus optional grounding source URIs.
func geminiTextResponse(text string, sources ...string) string {
	resp := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"parts": []map[string]any{{"text": text}},
			},
		}},
	}
	if len(sources) > 0 {
		chunks := make([]map[string]any, 0, len(sources))
		for _, s := range sources {
			chunks = append(chunks, map[string]any{"web": map[string]any{"uri": s}})
		}
		resp["candidates"].([]map[string]any)[0]["groundingMetadata"] = map[string]any{
			"groundingChunks": chunks,
		}
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func newFakeGeminiAdapter(t *testing.T, handler http.HandlerFunc) *GeminiGroundingAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	a := NewGeminiGroundingAdapter("test-key", "")
	a.baseURL = srv.URL
	return a
}

func leadWith(enrichment string) *domain.Lead {
	return &domain.Lead{ID: "lead-1", AccountID: "acc-1", Enrichment: json.RawMessage(enrichment)}
}

func TestGeminiGrounding_PopulatesLinks(t *testing.T) {
	var gotBody map[string]any
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		answer := `{"decision_maker_name":"João Souza","decision_maker_role":"CEO","linkedin_url":"https://linkedin.com/in/joao","instagram_url":"https://instagram.com/joao"}`
		_, _ = w.Write([]byte(geminiTextResponse(answer, "https://example.com/about")))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"Acme","website":"https://acme.com"}`))
	require.NoError(t, err)

	assert.Equal(t, "João Souza", data.DecisionMakerName)
	assert.Equal(t, "CEO", data.DecisionMakerRole)
	assert.Equal(t, "https://linkedin.com/in/joao", data.LinkedInURL)
	assert.Equal(t, "https://instagram.com/joao", data.InstagramURL)
	assert.Equal(t, []string{"https://example.com/about"}, data.Sources)
	assert.False(t, data.EnrichedAt.IsZero())

	// The request must enable the Google Search grounding tool.
	tools, _ := gotBody["tools"].([]any)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	_, hasSearch := tool["google_search"]
	assert.True(t, hasSearch, "google_search grounding tool must be enabled")
}

func TestGeminiGrounding_HandlesMarkdownFences(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := "```json\n{\"decision_maker_name\":\"Ana\",\"linkedin_url\":\"https://linkedin.com/in/ana\"}\n```"
		_, _ = w.Write([]byte(geminiTextResponse(answer)))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"X"}`))
	require.NoError(t, err)
	assert.Equal(t, "Ana", data.DecisionMakerName)
	assert.Equal(t, "https://linkedin.com/in/ana", data.LinkedInURL)
}

func TestGeminiGrounding_NoDecisionMakerDoesNotError(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := `{"decision_maker_name":"","decision_maker_role":"","linkedin_url":"","instagram_url":""}`
		_, _ = w.Write([]byte(geminiTextResponse(answer)))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"Ghost Co"}`))
	require.NoError(t, err)
	assert.Empty(t, data.DecisionMakerName)
	assert.Empty(t, data.LinkedInURL)
	assert.Empty(t, data.InstagramURL)
}

func TestGeminiGrounding_RejectsNonHTTPURLs(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := `{"decision_maker_name":"Bob","linkedin_url":"linkedin.com/in/bob","instagram_url":"javascript:alert(1)"}`
		_, _ = w.Write([]byte(geminiTextResponse(answer)))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"Y"}`))
	require.NoError(t, err)
	assert.Equal(t, "Bob", data.DecisionMakerName)
	assert.Empty(t, data.LinkedInURL, "non-http URL must be dropped")
	assert.Empty(t, data.InstagramURL, "unsafe scheme must be dropped")
}

func TestGeminiGrounding_NoCompanyOrSiteSkipsCall(t *testing.T) {
	called := false
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	// Skipping the call is right; reporting success is not. A nil error here
	// moved the lead to `enriched`, so the board claimed a research that had
	// never run and the panel kept offering the same do-nothing button.
	data, err := a.Enrich(context.Background(), leadWith(`{}`))
	require.ErrorIs(t, err, domain.ErrEnrichmentNothingToGround)
	assert.False(t, called, "must not call Gemini when there is nothing to search on")
	assert.Empty(t, data.DecisionMakerName)
}

func TestGeminiGrounding_HTTPErrorSurfaces(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"quota"}}`))
	})

	_, err := a.Enrich(context.Background(), leadWith(`{"company_name":"Z"}`))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "429"))
}

func TestGeminiGrounding_MissingKey(t *testing.T) {
	a := NewGeminiGroundingAdapter("", "")
	_, err := a.Enrich(context.Background(), leadWith(`{"company_name":"Z"}`))
	assert.ErrorIs(t, err, domain.ErrBYOKNotConfigured)
}

// A 200 with zero candidates is a transient/safety failure, not "found
// nothing" — it must surface ErrEnrichmentNoResult so the lead stays retryable
// instead of being stamped enriched with no data.
func TestGeminiGrounding_EmptyCandidatesIsRetryableError(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	})

	_, err := a.Enrich(context.Background(), leadWith(`{"company_name":"Z"}`))
	assert.ErrorIs(t, err, domain.ErrEnrichmentNoResult)
}

// --- Full investigation shape ----------------------------------------------

// The deeper research: one grounded call, many facts, each with the page it was
// read from and the model's own claim about certainty.
func TestGeminiGrounding_ReadsFindingsWithTheirOwnSources(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := `{"findings":[
			{"field":"decision_maker_name","value":"Ana Souza","source":"https://www.linkedin.com/in/ana","confident":true},
			{"field":"what_they_do","value":"Clínica de fisioterapia","source":"https://clinica.com.br/sobre","confident":true},
			{"field":"recent_event","value":"abriu segunda unidade","source":"https://jornal.com.br/x","confident":false}
		]}`
		_, _ = w.Write([]byte(geminiTextResponse(answer,
			"https://www.linkedin.com/in/ana", "https://clinica.com.br/sobre")))
	})

	data, err := a.Enrich(context.Background(),
		leadWith(`{"company_name":"Clínica","website":"https://clinica.com.br"}`))

	require.NoError(t, err)
	require.Len(t, data.Findings, 3)
	assert.Equal(t, "https://www.linkedin.com/in/ana", data.Findings[0].Source)
	assert.True(t, data.Findings[0].Trustworthy())
	// The model's own hedge survives the parse: a fact it doubted must not reach
	// a stranger worded as established.
	assert.False(t, data.Findings[2].Trustworthy())
	// The four legacy fields are back-filled, so every existing reader keeps
	// working without knowing findings exist.
	assert.Equal(t, "Ana Souza", data.DecisionMakerName)
}

// A model that omitted `confident` did not claim certainty; defaulting to true
// would let silence become an assertion.
func TestGeminiGrounding_AbsentConfidenceIsNotCertainty(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := `{"findings":[{"field":"size","value":"cerca de 40 funcionários","source":"https://x.com.br"}]}`
		_, _ = w.Write([]byte(geminiTextResponse(answer)))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"X"}`))

	require.NoError(t, err)
	require.Len(t, data.Findings, 1)
	assert.False(t, data.Findings[0].Confident)
}

// An uncited finding is kept — research already paid for, and a good question
// even when it cannot be stated.
func TestGeminiGrounding_KeepsAnUncitedFinding(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := `{"findings":[{"field":"current_supplier","value":"Distribuidora X","confident":true}]}`
		_, _ = w.Write([]byte(geminiTextResponse(answer)))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"X"}`))

	require.NoError(t, err)
	require.Len(t, data.Findings, 1)
	assert.False(t, data.Findings[0].Trustworthy())
}

// Still ONE grounded call. Grounding runs several searches internally; four
// calls would quadruple the cost against a plan allowance that counts
// enrichments.
func TestGeminiGrounding_AsksOnceForEverything(t *testing.T) {
	var calls int
	var body map[string]any
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(geminiTextResponse(`{"findings":[]}`)))
	})

	_, err := a.Enrich(context.Background(), leadWith(`{"company_name":"X"}`))

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	prompt, _ := json.Marshal(body)
	for _, want := range []string{
		"what_they_do", "recent_event", "current_supplier", "pain_signal", "source",
	} {
		assert.Contains(t, string(prompt), want, "prompt must ask for %s", want)
	}
}

// The old four-field answer still parses: an account whose model returns the
// previous shape must not lose its enrichment.
func TestGeminiGrounding_StillReadsTheLegacyShape(t *testing.T) {
	a := newFakeGeminiAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		answer := `{"decision_maker_name":"Ana","decision_maker_role":"Sócia"}`
		_, _ = w.Write([]byte(geminiTextResponse(answer, "https://x.com.br")))
	})

	data, err := a.Enrich(context.Background(), leadWith(`{"company_name":"X"}`))

	require.NoError(t, err)
	assert.Equal(t, "Ana", data.DecisionMakerName)
	assert.Equal(t, "Sócia", data.DecisionMakerRole)
	assert.Equal(t, []string{"https://x.com.br"}, data.Sources)
}
