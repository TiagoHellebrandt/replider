package domain

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

// Cross-sell (§6.5, F5).
//
// "Whoever buys gloves buys masks" is a fact sitting unused in every
// distributor's own sales table. The point is not a recommendation engine —
// it is one line on the screen when a salesperson opens a customer who buys
// gloves and has never bought masks.
//
// Learned WITHIN the account, deliberately. What a distributor sells to whom
// is their commercial position, and pooling it across accounts would mean one
// customer's product mix informing a competitor's pitch. The PRD imagines an
// anonymised cross-account layer; that needs explicit consent before it is
// built, and every account already has enough of its own sales to learn its
// own patterns.

// CoPurchasePair is "people who bought A also bought B", within one account.
type CoPurchasePair struct {
	// Product and Partner are normalised item labels, not ids: most sales
	// carry free text, and waiting for a catalogue match would mean learning
	// nothing until every product is registered.
	Product string `json:"product"`
	Partner string `json:"partner"`

	// Customers is how many DISTINCT customers bought both. Distinct on
	// purpose: one customer ordering the same pair twelve times is one
	// pattern, not twelve.
	Customers int `json:"customers"`
	// ProductCustomers is how many bought the first item at all — the
	// denominator that turns a count into a rate.
	ProductCustomers int `json:"product_customers"`
}

// Confidence is how often buying the product came with the partner. Nil when
// nobody bought the product: zero over zero is unknown, not zero.
func (p CoPurchasePair) Confidence() *float64 {
	if p.ProductCustomers == 0 {
		return nil
	}
	c := float64(p.Customers) / float64(p.ProductCustomers)
	return &c
}

// MinCoPurchaseCustomers is how much evidence a pattern needs.
//
// Three distinct customers. Two is a coincidence somebody will act on, and
// acting on a coincidence in front of a customer costs more credibility than
// the suggestion was ever worth.
const MinCoPurchaseCustomers = 3

// MinCoPurchaseConfidence is the rate below which a pair is noise.
//
// Thirty percent. Everybody who buys anything also buys gloves, and without a
// floor the suggestion for every customer would be "gloves" — which is
// exactly the useless recommendation that teaches people to ignore the panel.
const MinCoPurchaseConfidence = 0.30

// Strong reports a pattern worth putting in front of somebody.
func (p CoPurchasePair) Strong() bool {
	if p.Customers < MinCoPurchaseCustomers {
		return false
	}
	c := p.Confidence()
	return c != nil && *c >= MinCoPurchaseConfidence
}

// CrossSellSuggestion is what the screen shows for one customer.
type CrossSellSuggestion struct {
	// Suggest is the item they have NOT bought.
	Suggest string `json:"suggest"`
	// Because names the item they DO buy, so the salesperson can say why.
	// A suggestion with no reason is a guess the customer can hear.
	Because string `json:"because"`
	// Confidence is the observed rate among this account's own customers.
	Confidence float64 `json:"confidence"`
	Customers  int     `json:"customers"`
}

// SuggestCrossSell picks what to offer a customer, given what they already
// buy and the account's own co-purchase patterns.
//
// Never suggests something they already buy, and never repeats a suggestion
// for two different reasons — a panel listing "máscara (porque luva)" and
// "máscara (porque avental)" is the same idea twice.
func SuggestCrossSell(bought []string, pairs []CoPurchasePair, limit int) []CrossSellSuggestion {
	if limit <= 0 {
		limit = 3
	}
	owns := map[string]struct{}{}
	for _, item := range bought {
		owns[NormalizeItemLabel(item)] = struct{}{}
	}

	best := map[string]CrossSellSuggestion{}
	for _, pair := range pairs {
		if !pair.Strong() {
			continue
		}
		// Only patterns anchored on something this customer actually buys.
		if _, has := owns[pair.Product]; !has {
			continue
		}
		if _, already := owns[pair.Partner]; already {
			continue
		}
		confidence := *pair.Confidence()
		if prev, seen := best[pair.Partner]; seen && prev.Confidence >= confidence {
			continue
		}
		best[pair.Partner] = CrossSellSuggestion{
			Suggest: pair.Partner, Because: pair.Product,
			Confidence: confidence, Customers: pair.Customers,
		}
	}

	out := make([]CrossSellSuggestion, 0, len(best))
	for _, s := range best {
		out = append(out, s)
	}
	// Strongest first, then alphabetically so the order is stable between
	// two reads — a panel that reshuffles on refresh looks broken.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].Suggest < out[j].Suggest
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

var (
	// itemNoiseRe strips the packaging and sizing that make the same product
	// look like five: "Luvas de procedimento cx 100" and "Luvas de
	// procedimento caixa c/100" are one item to a human and two to a GROUP BY.
	itemNoiseRe = regexp.MustCompile(`(?i)\b(cx|caixa|pct|pacote|un|unid|unidade|c/|com)\b\.?\s*\d*`)
	// digitsRe removes the counts left behind.
	digitsRe = regexp.MustCompile(`\d+`)
	spacesRe = regexp.MustCompile(`\s+`)
	// pluralRe folds "luvas" onto "luva". Naive on purpose — it only strips a
	// trailing s from a word of four letters or more, so "gas" survives. It
	// will mangle a few words ("lapis" → "lapi"), which costs nothing here
	// because BOTH sides of every comparison run through the same rule; what
	// it buys is that "Luva" and "Luvas" stop being two products with half
	// the evidence each.
	pluralRe = regexp.MustCompile(`([a-z]{3,})s\b`)
)

// NormalizeItemLabel collapses the ways one product gets typed.
//
// Deliberately blunt — it lowercases, folds accents, drops packaging words
// and numbers. It will merge two genuinely different sizes of the same
// product, and that is the right trade: for a co-purchase pattern "luva" and
// "luva P" are the same signal, and keeping them apart halves the evidence
// for both.
func NormalizeItemLabel(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	// Reuses the brand-kit folder: one table of Portuguese accents in the
	// package, so the two normalisers cannot disagree about "máscara".
	s = accentFolder.Replace(s)
	s = itemNoiseRe.ReplaceAllString(s, " ")
	s = digitsRe.ReplaceAllString(s, " ")
	s = spacesRe.ReplaceAllString(s, " ")
	s = pluralRe.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// CrossSellRepository reads co-purchase patterns from an account's own sales.
type CrossSellRepository interface {
	// Pairs returns every co-purchase pattern in the account, already
	// filtered to those with enough evidence.
	Pairs(ctx context.Context, accountID string) ([]CoPurchasePair, error)
	// ItemsBoughtBy lists the normalised labels one customer has bought.
	ItemsBoughtBy(ctx context.Context, accountID, contactID string) ([]string, error)
}
