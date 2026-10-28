package domain

import (
	"context"
	"strings"
	"time"
)

// EnrichmentData holds the decision-maker + social-profile data discovered for
// a lead via a grounded web search (Gemini + Google Search, US-023). Every
// field is best-effort: any may be empty when the search did not surface it,
// and an empty result is NOT an error.
type EnrichmentData struct {
	DecisionMakerName string `json:"decision_maker_name,omitempty"`
	DecisionMakerRole string `json:"decision_maker_role,omitempty"`
	LinkedInURL       string `json:"linkedin_url,omitempty"`
	InstagramURL      string `json:"instagram_url,omitempty"`

	// Findings is everything the search learned, one entry per fact, each
	// carrying the page it was read from.
	//
	// Sources below is the union of every page the grounded call touched, and
	// that is all it can be — so a dossier citing it attributes ALL of them to
	// EVERY researched fact. When the search read LinkedIn for the decision
	// maker and the company's own site for what it does, both facts came out
	// citing both pages, and a salesperson checking one landed on the wrong
	// one. Per-fact provenance is the difference between a citation and a
	// gesture at a bibliography.
	//
	// Empty from an older answer: the legacy fields above still carry the four
	// original values, so nothing breaks when the model returns the old shape.
	Findings []EnrichmentFinding `json:"findings,omitempty"`

	// Sources is every page the call touched. Kept for the answers that carry
	// no per-fact citation, and as the fallback the panel already renders.
	Sources    []string  `json:"sources,omitempty"`
	EnrichedAt time.Time `json:"enriched_at,omitempty"`
	// Usage is what the provider charged for this call (US-M015).
	// Additive on the DATA rather than the port signature so metering
	// could be added without rewriting every implementation and its
	// tests. Zero means the provider reported nothing — metered as
	// "unknown", never as free.
	Usage AIUsage `json:"-"`
}

// EnrichmentFinding is one fact the research learned, and where it read it.
//
// The point of the shape is that a fact and its evidence travel together. A
// value with no page behind it cannot be checked, and one a model is
// unsure about must not reach a customer worded as certain.
type EnrichmentFinding struct {
	// Field is the canonical key: what_they_do, size, region, recent_event,
	// current_supplier, decision_maker_name, … Free-form on purpose — the model
	// may surface something worth knowing that nobody enumerated, and dropping
	// it because it was not on a list is how research stays shallow.
	Field string `json:"field"`
	Value string `json:"value"`
	// Source is the page this was read from. A finding with no source is kept
	// but marked low-confidence: it means the model asserted without citing,
	// which is exactly what the draft must hedge rather than repeat.
	Source string `json:"source,omitempty"`
	// Confident is the model's own claim about whether it is sure. Used to
	// decide whether the draft may state the fact or must soften it — never to
	// hide it, because a soft fact is still a conversation starter.
	Confident bool `json:"confident"`
}

// Trustworthy reports a finding a message may state rather than hedge: the model
// said it was sure AND named a page. Either alone is not enough — an uncited
// certainty is the shape a hallucination arrives in.
func (f EnrichmentFinding) Trustworthy() bool {
	return f.Confident && strings.HasPrefix(strings.TrimSpace(f.Source), "http")
}

// LeadEnrichmentProvider discovers a lead's decision maker and social profiles
// using a grounded web search. Implementations resolve the company name and
// website from the lead and never fail on missing fields — a sparse result is
// returned as a mostly-empty EnrichmentData with a nil error.
type LeadEnrichmentProvider interface {
	Enrich(ctx context.Context, lead *Lead) (EnrichmentData, error)
}

// LeadEnrichmentFactory builds a LeadEnrichmentProvider bound to an account's
// BYOK Google/Gemini key. It returns ErrEnrichmentRequiresGemini when the
// account's configured AI provider is not Google/Gemini (grounding-with-search
// is a Gemini-only capability) and ErrBYOKNotConfigured when the key is absent.
type LeadEnrichmentFactory interface {
	ProviderForAccount(ctx context.Context, account *Account, plan *Plan) (LeadEnrichmentProvider, error)
}

// ErrEnrichmentRequiresGemini is returned when the account's AI provider is not
// Google/Gemini. Lead enrichment relies on Gemini's Grounding with Google
// Search, which has no OpenAI equivalent, so the feature is gated to Gemini.
var ErrEnrichmentRequiresGemini = &AppError{Code: "ENRICHMENT_REQUIRES_GEMINI", Message: "lead enrichment requires a Google/Gemini AI key"}

// ErrEnrichmentNoResult signals the grounded model returned zero candidates —
// a transient provider hiccup, quota throttle or safety block, NOT a genuine
// "searched and found nothing" (which comes back as a candidate with empty
// fields). The use case treats it as a soft failure and restores the lead's
// prior status so the enrichment can be retried, instead of stamping it
// `enriched` with no data and no way to try again.
var ErrEnrichmentNoResult = &AppError{Code: "ENRICHMENT_NO_RESULT", Message: "enrichment returned no result; try again"}

// ErrEnrichmentNothingToGround signals the lead carries neither a company name
// nor a website, so there is nothing to search FOR and no call was made. This
// used to return empty data with a nil error, which moved the lead to
// `enriched` — the board then claimed a research that never happened, and the
// panel offered the same button again with no hint that it would do nothing.
var ErrEnrichmentNothingToGround = &AppError{Code: "ENRICHMENT_NOTHING_TO_GROUND", Message: "lead has no company name or website to search for"}

// EnrichmentBlockedKey is where the lead's enrichment blob records WHY a
// research attempt did no work, so the UI can say it instead of showing the
// never-researched copy forever.
const EnrichmentBlockedKey = "enrich_blocked"

// EnrichmentBlockedNoIdentity is the only blocked reason so far: no company
// name and no website.
const EnrichmentBlockedNoIdentity = "no_company_or_website"
