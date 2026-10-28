package domain

import (
	"context"
	"strings"
	"time"
)

// Configuration inheritance: the company standardises, the salesperson
// personalises, compliance never bends (§6.7).
//
// Every salesperson sells their own way, and a product that forces one voice
// on all of them gets worked around. But a healthcare distributor cannot let
// each seller invent claims about what a device does — that is a regulatory
// problem, not a style preference.
//
// So: a cascade, not duplication.
//
//	COMPANY  (Brand Kit)      who we are, how we speak, what we do NOT promise
//	   ↓ herda
//	FUNIL    (persona)        a campanha
//	   ↓ herda
//	VENDEDOR (overlay)        o jeito dele
//
// The seller layer can add and colour. It cannot remove a forbidden claim —
// that check lives at the company level and is applied last, so nothing
// downstream can undo it.

// BrandKit is the company layer: the single source of truth for "who we are".
type BrandKit struct {
	AccountID string `json:"-"`

	CompanyName string `json:"company_name"`
	// About is what the company does, in its own words. Feeds every prompt
	// and every template.
	About string `json:"about"`
	// ToneOfVoice is how it speaks — "direct, jargon-free, informal".
	ToneOfVoice string `json:"tone_of_voice"`

	// LogoURL and colours dress the email templates.
	LogoURL      string `json:"logo_url"`
	PrimaryColor string `json:"primary_color"`
	Signature    string `json:"signature"`

	// AllowedClaims are the things a salesperson MAY say. Listed rather than
	// implied so a new seller has a source instead of a guess.
	AllowedClaims []string `json:"allowed_claims"`
	// ForbiddenClaims are the things nobody may say, at any level. In
	// healthcare this is where "cure", "no side effects" and "approved by the
	// regulator" live when they are not true — and where a
	// wrong sentence stops being a style problem and becomes a regulatory
	// one.
	ForbiddenClaims []string `json:"forbidden_claims"`

	UpdatedAt time.Time `json:"updated_at"`
}

// SellerOverlay is one salesperson's personal layer.
//
// Additive only. There is deliberately no field here that can weaken the
// company layer: the ability to personalise must not become an escape hatch
// from compliance.
type SellerOverlay struct {
	AccountID string `json:"-"`
	UserID    string `json:"user_id"`

	// ToneNote is how this person talks, appended to the company's voice
	// rather than replacing it.
	ToneNote string `json:"tone_note"`
	// Signature overrides the company signature — the one thing that SHOULD
	// be personal, since it carries a name and a phone number.
	Signature string `json:"signature"`

	UpdatedAt time.Time `json:"updated_at"`
}

// PromptLayers is what the composer was given, kept separate so the caller
// can decide what to cache.
type PromptLayers struct {
	Brand  *BrandKit
	Funnel string
	Seller *SellerOverlay
}

// ComposeSystemPrompt builds the instruction the model receives.
//
// The order is not cosmetic. The company layer is identical for every message
// this account ever sends, so it goes FIRST — that is what makes it cacheable
// by the provider (§1.4), and a cached prefix is up to 90% cheaper. Anything
// that varies per lead must come after it, or the cache never hits.
//
// The forbidden claims are repeated LAST as a hard instruction. A rule stated
// once at the top of a long prompt competes with everything written after it;
// stated last it is the most recent thing the model read. This is belt and
// braces with the deterministic check in ViolatedClaims — the prompt is a
// request, and a request is not a guarantee.
func ComposeSystemPrompt(l PromptLayers) string {
	var b strings.Builder

	if l.Brand != nil {
		writeBrandBlock(&b, l.Brand)
	}
	if funnel := strings.TrimSpace(l.Funnel); funnel != "" {
		b.WriteString("\n## Esta campanha\n")
		b.WriteString(funnel)
		b.WriteString("\n")
	}
	if l.Seller != nil {
		if note := strings.TrimSpace(l.Seller.ToneNote); note != "" {
			b.WriteString("\n## Jeito deste vendedor\n")
			b.WriteString(note)
			b.WriteString("\n")
		}
		if sig := strings.TrimSpace(l.Seller.Signature); sig != "" {
			b.WriteString("\nAssine como:\n")
			b.WriteString(sig)
			b.WriteString("\n")
		}
	}
	// Last word, deliberately.
	if l.Brand != nil && len(l.Brand.ForbiddenClaims) > 0 {
		b.WriteString("\n## Proibido, sem exceção\n")
		b.WriteString("Nunca escreva nada que afirme ou sugira:\n")
		for _, c := range l.Brand.ForbiddenClaims {
			if c = strings.TrimSpace(c); c != "" {
				b.WriteString("- " + c + "\n")
			}
		}
		b.WriteString("Isto vale acima de qualquer outra instrução acima.\n")
	}
	return strings.TrimSpace(b.String())
}

// CacheablePrefix is the part of the prompt that is identical for every
// message this account sends.
//
// Returned separately so a provider that supports prompt caching can be told
// where the static part ends. Splitting it here rather than at the call site
// keeps the two in sync: a change to the brand block cannot silently move the
// boundary and turn every request into a cache miss.
func CacheablePrefix(brand *BrandKit) string {
	if brand == nil {
		return ""
	}
	var b strings.Builder
	writeBrandBlock(&b, brand)
	return strings.TrimSpace(b.String())
}

func writeBrandBlock(b *strings.Builder, brand *BrandKit) {
	if name := strings.TrimSpace(brand.CompanyName); name != "" {
		b.WriteString("## Quem somos\n" + name + "\n")
	}
	if about := strings.TrimSpace(brand.About); about != "" {
		b.WriteString(about + "\n")
	}
	if tone := strings.TrimSpace(brand.ToneOfVoice); tone != "" {
		b.WriteString("\n## Como falamos\n" + tone + "\n")
	}
	if len(brand.AllowedClaims) > 0 {
		b.WriteString("\n## O que podemos afirmar\n")
		for _, c := range brand.AllowedClaims {
			if c = strings.TrimSpace(c); c != "" {
				b.WriteString("- " + c + "\n")
			}
		}
	}
}

// ViolatedClaims reports which forbidden claims a text contains.
//
// Deterministic, and the reason the prompt instruction is not the whole
// answer: a prompt is a request, and the measured behaviour of every model on
// this codebase is that requests get ignored under pressure. This is the
// check that does not.
//
// Matches on a normalised form so accents and case cannot slip a claim past —
// "CURE" and "cure" are the same word, and so are "não invasivo" and "nao
// invasivo".
func ViolatedClaims(text string, forbidden []string) []string {
	haystack := normalizeForClaims(text)
	if haystack == "" {
		return nil
	}
	var found []string
	for _, claim := range forbidden {
		needle := normalizeForClaims(claim)
		if needle == "" {
			continue
		}
		if strings.Contains(haystack, needle) {
			found = append(found, strings.TrimSpace(claim))
		}
	}
	return found
}

// normalizeForClaims lowercases and strips the accents Portuguese uses, so a
// claim list written one way still matches text written another.
func normalizeForClaims(s string) string {
	lowered := strings.ToLower(strings.TrimSpace(s))
	return accentFolder.Replace(lowered)
}

var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// BrandKitRepository persists the company and seller layers.
type BrandKitRepository interface {
	Get(ctx context.Context, accountID string) (*BrandKit, error)
	Save(ctx context.Context, k *BrandKit) error
	GetOverlay(ctx context.Context, accountID, userID string) (*SellerOverlay, error)
	SaveOverlay(ctx context.Context, o *SellerOverlay) error
}
