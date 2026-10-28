package domain

import (
	"context"
	"strings"
	"time"
)

// Target audiences (§2, D3).
//
// A saved answer to "who are we selling to". Prospecting without one is a
// keyword typed from memory every morning, which is how a supplier ends up
// working the same three cities for a year and never noticing the fourth.
//
// The point is not the record — it is the SECOND and third audience. Somebody
// who sells patient monitors to hospitals usually also sells them to dialysis
// clinics and to home-care operators, and the reason they do not prospect
// those is that nobody ever wrote them down.
type Audience struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	Name string `json:"name"`
	// Segment is the kind of business, in the customer's own words:
	// "clínicas de diagnóstico por imagem", not a CNAE code. The code is a
	// filter; this is what a salesperson would say out loud.
	Segment string `json:"segment"`
	// Size narrows by scale — "de 10 a 50 leitos". Free text, because every
	// market measures size differently and a dropdown would be wrong for
	// most of them.
	Size string `json:"size,omitempty"`
	// DecisionMaker is the ROLE to reach, which decides how the approach is
	// written far more than the segment does. A message to a purchasing
	// manager and one to a clinical director share almost no sentences.
	DecisionMaker string `json:"decision_maker,omitempty"`
	// Regions are cities, states, or whatever the customer thinks in.
	Regions []string `json:"regions,omitempty"`

	// Keywords is what actually reaches the prospecting source. Derived from
	// the fields above when the audience is created, then editable — the
	// person doing the work knows their market's vocabulary better than any
	// generator does.
	Keywords []string `json:"keywords,omitempty"`

	// Suggested marks an audience a model proposed rather than a human
	// typed. Kept visible: "a IA sugeriu" is information, and a suggestion
	// presented as the customer's own decision is a small lie that erodes
	// trust in every other suggestion.
	Suggested bool `json:"suggested"`
	// Unexplored flags the one suggestion deliberately chosen for being
	// adjacent rather than obvious — the "você talvez não viu" slot. Exactly
	// one per suggestion round: two would be a list, and a list of things
	// you have not considered is just noise.
	Unexplored bool `json:"unexplored"`

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchQuery is what the audience becomes when it reaches a prospecting
// source: one keyword string and one location, because that is all Google
// Places and its peers accept.
func (a *Audience) SearchQuery() (keyword, location string) {
	terms := a.Keywords
	if len(terms) == 0 {
		// An audience with no keywords still has a segment, which is the
		// sentence a human would have typed anyway.
		terms = []string{a.Segment}
	}
	// Sources take ONE phrase, not a boolean expression. Joining with spaces
	// widens the match instead of narrowing it, which is the right direction
	// for discovery — a search that returns nothing teaches nobody anything.
	keyword = strings.TrimSpace(strings.Join(terms, " "))
	if len(a.Regions) > 0 {
		location = strings.TrimSpace(a.Regions[0])
	}
	return keyword, location
}

// AudienceSuggestionCount is how many a round proposes.
//
// Three: enough that the adjacent one does not stand alone as "the weird
// suggestion", few enough that a person reads all of them. A list of ten
// audiences is a list nobody chooses from.
const AudienceSuggestionCount = 3

// AudienceRepository persists audiences.
type AudienceRepository interface {
	List(ctx context.Context, accountID string) ([]*Audience, error)
	Get(ctx context.Context, id, accountID string) (*Audience, error)
	Upsert(ctx context.Context, a *Audience) error
	Delete(ctx context.Context, id, accountID string) error
}

// AudienceSuggester proposes audiences from what the company already is.
type AudienceSuggester interface {
	// Suggest returns AudienceSuggestionCount audiences, exactly one of them
	// flagged Unexplored.
	Suggest(ctx context.Context, accountID string, context AudienceContext) ([]*Audience, error)
}

// AudienceContext is everything the suggester gets to reason from. Assembled
// by the caller so the suggester needs no repositories of its own.
type AudienceContext struct {
	// CompanyName and About come from the brand kit — what this company says
	// it does, in its own words.
	CompanyName string
	About       string
	// Products is what they actually sell, which is a better predictor of
	// who else would buy than any description of the company.
	Products []string
	// ExistingAudiences keeps a round from proposing what is already saved.
	ExistingAudiences []string
	// WonSegments are the segments that actually bought. The strongest
	// evidence available, and the reason a suggestion round is worth
	// re-running after a few sales rather than only at onboarding.
	WonSegments []string
}
