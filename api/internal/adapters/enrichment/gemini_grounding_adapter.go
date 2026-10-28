// Package enrichment implements lead decision-maker/social-profile discovery
// via Gemini's Grounding with Google Search (US-023). It deliberately avoids
// expensive B2B data APIs: a single grounded generateContent call asks the
// model to find the decision maker + LinkedIn/Instagram URLs for a company and
// return them as JSON.
package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// defaultGeminiBaseURL is the Generative Language REST host. It is a struct
// field (not a const) so tests can point it at an httptest server.
const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

// defaultEnrichmentModel is the grounded model. Gemini 2.x exposes the
// `google_search` tool (the 1.5-era `google_search_retrieval` is superseded);
// 2.5-flash matches the provider model used elsewhere in the platform.
const defaultEnrichmentModel = "gemini-2.5-flash"

// GeminiGroundingAdapter implements domain.LeadEnrichmentProvider by calling
// the Gemini generateContent REST endpoint with the Google Search grounding
// tool enabled. It is constructed per-account with the resolved BYOK key.
type GeminiGroundingAdapter struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

var _ domain.LeadEnrichmentProvider = (*GeminiGroundingAdapter)(nil)

// NewGeminiGroundingAdapter builds an adapter for the given API key. An empty
// model falls back to the default grounded model.
func NewGeminiGroundingAdapter(apiKey, model string) *GeminiGroundingAdapter {
	if strings.TrimSpace(model) == "" {
		model = defaultEnrichmentModel
	}
	return &GeminiGroundingAdapter{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultGeminiBaseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Enrich resolves the lead's company name + website and asks the grounded model
// for the decision maker and social profiles. Best-effort: a sparse or empty
// model answer returns a mostly-empty EnrichmentData with a nil error; only a
// transport/HTTP failure surfaces an error so the caller can restore status.
func (a *GeminiGroundingAdapter) Enrich(ctx context.Context, lead *domain.Lead) (domain.EnrichmentData, error) {
	company, website := extractCompanySite(lead)

	// Nothing to ground a search on — skip the network call entirely. This is
	// an error, not empty data: returning nil moved the lead to `enriched`, so
	// the board claimed a research that never ran.
	if company == "" && website == "" {
		return domain.EnrichmentData{}, domain.ErrEnrichmentNothingToGround
	}

	prompt := buildEnrichmentPrompt(company, website)
	respText, sources, usage, err := a.generate(ctx, prompt)
	if err != nil {
		return domain.EnrichmentData{}, err
	}

	data := parseEnrichmentJSON(respText)
	if len(data.Sources) == 0 {
		data.Sources = sources
	}
	data.EnrichedAt = time.Now().UTC()
	data.Usage = usage
	return data, nil
}

// extractCompanySite pulls the company name and website from the lead's
// enrichment blob. Prospecting (US-022) seeds `website`; the enrichment use
// case injects `company_name` (from the contact) before calling. Falls back to
// a bare `name` key for resilience.
func extractCompanySite(lead *domain.Lead) (company, website string) {
	if lead == nil || len(lead.Enrichment) == 0 {
		return "", ""
	}
	var m map[string]any
	if err := json.Unmarshal(lead.Enrichment, &m); err != nil {
		return "", ""
	}
	company = firstString(m, "company_name", "name")
	website = firstString(m, "website")
	return company, website
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// buildEnrichmentPrompt asks for the picture, not the four fields.
//
// The old prompt asked one question — who decides, plus two social URLs — and
// that is what the research returned: four values for a message that then had
// to be written from them. Everything a first approach actually leans on was
// missing. What the company DOES, how big it is, where it operates, what
// happened there recently, who they buy from today: each of those is a sentence
// somebody can open a conversation with, and none of them were asked for.
//
// Two things make the wider ask safe rather than just longer:
//
//  1. Every fact must arrive WITH THE PAGE it was read from. Grounding returns a
//     union of pages touched, which cannot say which page supports which fact —
//     so a dossier citing it credited every page for every fact. Asking the
//     model to attribute each one is the only way to get a citation instead of a
//     bibliography.
//
//  2. The model declares its own confidence, per fact. A guess is still worth
//     having — it makes a decent question — but it must arrive marked, so the
//     draft softens it instead of asserting it. An uncited certainty is the
//     shape a hallucination comes in.
//
// Still ONE grounded call. Grounding already runs several searches internally to
// answer a question, and splitting this into four calls would quadruple the cost
// per lead against a plan allowance that counts enrichments, not tokens.
func buildEnrichmentPrompt(company, website string) string {
	var b strings.Builder
	b.WriteString("Você é um investigador de prospecção B2B. Pesquise no Google e monte o retrato desta empresa para embasar uma primeira abordagem comercial.\n\n")
	if company != "" {
		fmt.Fprintf(&b, "Empresa: %s\n", company)
	}
	if website != "" {
		fmt.Fprintf(&b, "Site: %s\n", website)
	}
	b.WriteString(`
Procure, nesta ordem de utilidade:
  - decision_maker_name / decision_maker_role: quem decide compras (dono, sócio, diretor, gerente responsável)
  - linkedin_url / instagram_url: perfis reais dessa pessoa ou da empresa
  - what_they_do: o que a empresa faz, em uma frase, na linguagem do próprio site
  - size: porte — número de funcionários, unidades, leitos, salas, frota, o que fizer sentido
  - region: cidade/estado e onde mais atua
  - recent_event: algo que aconteceu nos últimos 12 meses — expansão, nova unidade, contratação, prêmio, mudança de sócio, obra, aniversário da empresa
  - current_supplier: com quem ela já trabalha hoje no que vendemos, se aparecer
  - pain_signal: dificuldade que ela mesma declarou publicamente (reclamação respondida, vaga aberta, entrevista)

Responda APENAS com um objeto JSON válido (sem markdown, sem comentários):
{"findings":[{"field":"","value":"","source":"","confident":true}]}

Regras, todas obrigatórias:
  - Um item por fato. NÃO devolva campo que você não encontrou: omitir é a resposta correta para o que não existe.
  - "source" é a URL da página onde você LEU aquele fato específico. Não repita a mesma URL para fatos lidos em páginas diferentes.
  - "confident": true só quando a página afirma o fato claramente. Se você inferiu, deduziu ou está supondo, use false — um palpite marcado é útil, um palpite disfarçado de certeza queima a conversa.
  - NUNCA invente nome, número, URL ou evento. Empresa homônima em outra cidade não é esta empresa.
  - "value" curto: uma frase, não um parágrafo.
  - Pode incluir um fato que não está na lista acima se for claramente útil para vender.`)
	return b.String()
}

// --- REST wire types -------------------------------------------------------

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
	Tools    []any           `json:"tools,omitempty"`
}

type geminiGroundingChunk struct {
	Web *struct {
		URI   string `json:"uri"`
		Title string `json:"title"`
	} `json:"web,omitempty"`
}

type geminiCandidate struct {
	Content           geminiContent `json:"content"`
	GroundingMetadata *struct {
		GroundingChunks []geminiGroundingChunk `json:"groundingChunks"`
	} `json:"groundingMetadata,omitempty"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int64 `json:"promptTokenCount"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
	Error         *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// generate POSTs the grounded prompt and returns the concatenated text plus the
// grounding source URIs.
func (a *GeminiGroundingAdapter) generate(ctx context.Context, prompt string) (string, []string, domain.AIUsage, error) {
	if strings.TrimSpace(a.apiKey) == "" {
		return "", nil, domain.AIUsage{}, domain.ErrBYOKNotConfigured
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{{
			Role:  "user",
			Parts: []geminiPart{{Text: prompt}},
		}},
		// Empty `google_search` object enables Grounding with Google Search.
		Tools: []any{map[string]any{"google_search": map[string]any{}}},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, domain.AIUsage{}, err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", a.baseURL, a.model, a.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", nil, domain.AIUsage{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", nil, domain.AIUsage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", nil, domain.AIUsage{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, domain.AIUsage{}, fmt.Errorf("gemini enrichment HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", nil, domain.AIUsage{}, fmt.Errorf("gemini enrichment: parse response: %w", err)
	}
	if parsed.Error != nil {
		return "", nil, domain.AIUsage{}, fmt.Errorf("gemini enrichment API error: %s", parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 {
		// Zero candidates is a transient/quota/safety failure, not a genuine
		// "found nothing" (that returns a candidate with empty fields). Surface
		// it so the use case restores status and the enrichment stays retryable.
		return "", nil, domain.AIUsage{}, domain.ErrEnrichmentNoResult
	}

	var text strings.Builder
	for _, p := range parsed.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}

	var sources []string
	if gm := parsed.Candidates[0].GroundingMetadata; gm != nil {
		for _, ch := range gm.GroundingChunks {
			if ch.Web != nil && isHTTPURL(ch.Web.URI) {
				sources = append(sources, ch.Web.URI)
			}
		}
	}
	// Usage is best-effort: a response without usageMetadata is metered
	// as unknown (zero) and logged upstream, never as free.
	var usage domain.AIUsage
	if um := parsed.UsageMetadata; um != nil {
		usage = domain.AIUsage{
			PromptTokens:     um.PromptTokenCount,
			CompletionTokens: um.CandidatesTokenCount,
		}
	}
	return text.String(), sources, usage, nil
}

// parseEnrichmentJSON extracts the decision-maker fields from the model's text,
// tolerating markdown code fences and surrounding prose. Invalid social URLs
// are dropped (kept empty) — best-effort, never an error.
func parseEnrichmentJSON(text string) domain.EnrichmentData {
	jsonStr := extractJSONObject(text)
	if jsonStr == "" {
		return domain.EnrichmentData{}
	}
	var raw struct {
		// The wider shape: one entry per fact, each with its own page.
		Findings []struct {
			Field     string `json:"field"`
			Value     string `json:"value"`
			Source    string `json:"source"`
			Confident *bool  `json:"confident"`
		} `json:"findings"`
		// The original four. Still read, because a model occasionally answers
		// in the shape it was asked in last month, and dropping the answer
		// because its envelope changed would lose research already paid for.
		DecisionMakerName string `json:"decision_maker_name"`
		DecisionMakerRole string `json:"decision_maker_role"`
		LinkedInURL       string `json:"linkedin_url"`
		InstagramURL      string `json:"instagram_url"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		slog.Warn("gemini enrichment: could not parse JSON answer", "error", err)
		return domain.EnrichmentData{}
	}

	data := domain.EnrichmentData{
		DecisionMakerName: strings.TrimSpace(raw.DecisionMakerName),
		DecisionMakerRole: strings.TrimSpace(raw.DecisionMakerRole),
	}
	if u := strings.TrimSpace(raw.LinkedInURL); isHTTPURL(u) {
		data.LinkedInURL = u
	}
	if u := strings.TrimSpace(raw.InstagramURL); isHTTPURL(u) {
		data.InstagramURL = u
	}

	for _, f := range raw.Findings {
		field := normaliseFindingField(f.Field)
		value := strings.TrimSpace(f.Value)
		if field == "" || value == "" {
			continue
		}
		source := strings.TrimSpace(f.Source)
		if !isHTTPURL(source) {
			// Kept, without the citation. A fact the model would not attribute
			// is exactly the one the draft has to hedge, and dropping it would
			// throw away a usable question.
			source = ""
		}
		// Absent `confident` reads as NOT confident. A model that omitted the
		// field did not claim certainty, and defaulting to true would let
		// silence become an assertion.
		confident := f.Confident != nil && *f.Confident && source != ""
		data.Findings = append(data.Findings, domain.EnrichmentFinding{
			Field: field, Value: value, Source: source, Confident: confident,
		})

		// The four legacy fields stay populated from the findings, so every
		// screen and prompt that reads them keeps working unchanged.
		switch field {
		case "decision_maker_name":
			if data.DecisionMakerName == "" {
				data.DecisionMakerName = value
			}
		case "decision_maker_role":
			if data.DecisionMakerRole == "" {
				data.DecisionMakerRole = value
			}
		case "linkedin_url":
			if data.LinkedInURL == "" && isHTTPURL(value) {
				data.LinkedInURL = value
			}
		case "instagram_url":
			if data.InstagramURL == "" && isHTTPURL(value) {
				data.InstagramURL = value
			}
		}
	}
	return data
}

// normaliseFindingField lowercases and underscores whatever key the model used,
// so `Recent Event`, `recent-event` and `recent_event` land in one bucket.
//
// Unknown keys pass through rather than being dropped: the prompt invites a fact
// nobody enumerated, and discarding it for not matching a list is how research
// stays as shallow as the list.
func normaliseFindingField(field string) string {
	field = strings.ToLower(strings.TrimSpace(field))
	field = strings.NewReplacer(" ", "_", "-", "_").Replace(field)
	// Guard against a model returning a whole sentence as the key.
	if len(field) > 40 {
		return ""
	}
	return field
}

// extractJSONObject returns the first {...} block in s, stripping ```json fences
// the model sometimes wraps the answer in.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return s[start : end+1]
}

func isHTTPURL(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
