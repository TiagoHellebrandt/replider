package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every salesperson sells their own way, and a product that forces one voice
// on all of them gets worked around. But a healthcare distributor cannot let
// each seller invent claims about what a device does. The cascade exists to
// allow the first without permitting the second.

func testBrand() *BrandKit {
	return &BrandKit{
		CompanyName:     "Distribuidora Exemplo",
		About:           "Vendemos monitores multiparâmetro e bombas de infusão.",
		ToneOfVoice:     "Direto, sem jargão.",
		AllowedClaims:   []string{"assistência técnica própria em Sorocaba"},
		ForbiddenClaims: []string{"cura", "aprovado pela ANVISA", "sem efeitos colaterais"},
	}
}

func TestCompose_LayersInOrder(t *testing.T) {
	got := ComposeSystemPrompt(PromptLayers{
		Brand:  testBrand(),
		Funnel: "Campanha para clínicas de imagem.",
		Seller: &SellerOverlay{ToneNote: "Falo mais devagar.", Signature: "João, 15 99999-0000"},
	})

	assert.Contains(t, got, "Distribuidora Exemplo")
	assert.Contains(t, got, "clínicas de imagem")
	assert.Contains(t, got, "Falo mais devagar")
	assert.Contains(t, got, "João, 15 99999-0000")

	// The company block leads. That is what makes it cacheable — anything
	// varying per lead placed before it turns every request into a miss.
	assert.Less(t, strings.Index(got, "Distribuidora Exemplo"),
		strings.Index(got, "clínicas de imagem"))
}

// A rule stated once at the top of a long prompt competes with everything
// written after it. Stated last, it is the most recent thing the model read.
func TestCompose_ForbiddenClaimsComeLast(t *testing.T) {
	got := ComposeSystemPrompt(PromptLayers{
		Brand:  testBrand(),
		Funnel: "Campanha X",
		Seller: &SellerOverlay{ToneNote: "Meu jeito"},
	})

	assert.Contains(t, got, "cura")
	assert.Greater(t, strings.Index(got, "Proibido, sem exceção"),
		strings.Index(got, "Meu jeito"),
		"the seller layer must not be the last word on compliance")
}

// The cacheable prefix must be exactly the account-invariant part: if it
// contained anything per-lead, the cache would never hit.
func TestCacheablePrefix_HasNoPerLeadContent(t *testing.T) {
	prefix := CacheablePrefix(testBrand())

	assert.Contains(t, prefix, "Distribuidora Exemplo")
	assert.NotContains(t, prefix, "Campanha")
	assert.NotContains(t, prefix, "vendedor")
	// And it is a real prefix of the full prompt, or the boundary is a lie.
	full := ComposeSystemPrompt(PromptLayers{Brand: testBrand(), Funnel: "Campanha X"})
	assert.True(t, strings.HasPrefix(full, prefix), "prefix must actually prefix")
}

func TestCompose_EmptyLayersDegradeCleanly(t *testing.T) {
	assert.Empty(t, ComposeSystemPrompt(PromptLayers{}))
	assert.Empty(t, CacheablePrefix(nil))

	onlyFunnel := ComposeSystemPrompt(PromptLayers{Funnel: "Só a campanha"})
	assert.Contains(t, onlyFunnel, "Só a campanha")
}

// ── The check that does not depend on the model obeying ───────────────────

// A prompt is a request, and the measured behaviour on this codebase is that
// requests get ignored under pressure. This is the check that does not.
func TestViolatedClaims_CatchesTheForbiddenOnes(t *testing.T) {
	forbidden := testBrand().ForbiddenClaims

	got := ViolatedClaims(
		"Nosso equipamento é aprovado pela ANVISA e não tem efeitos colaterais.",
		forbidden)

	assert.Contains(t, got, "aprovado pela ANVISA")
}

// Accents and case are not a way past a compliance rule.
func TestViolatedClaims_IgnoresAccentsAndCase(t *testing.T) {
	got := ViolatedClaims("APROVADO PELA ANVISA, sem efeitos colaterais",
		[]string{"aprovado pela anvisa", "sem efeitos colaterais"})

	assert.Len(t, got, 2)
}

func TestViolatedClaims_CleanTextPassesQuietly(t *testing.T) {
	got := ViolatedClaims(
		"Trabalho com monitores multiparâmetro e assistência em Sorocaba.",
		testBrand().ForbiddenClaims)

	assert.Empty(t, got)
}

func TestViolatedClaims_EmptyInputsAreSafe(t *testing.T) {
	assert.Empty(t, ViolatedClaims("", []string{"cura"}))
	assert.Empty(t, ViolatedClaims("qualquer texto", nil))
	assert.Empty(t, ViolatedClaims("qualquer texto", []string{"   "}))
}

// The seller layer is additive by construction: there is no field on it that
// can weaken the company layer, and the composed prompt proves it.
func TestCompose_SellerCannotRemoveAForbiddenClaim(t *testing.T) {
	brand := testBrand()
	got := ComposeSystemPrompt(PromptLayers{
		Brand: brand,
		Seller: &SellerOverlay{
			ToneNote: "Pode dizer que cura, o cliente gosta.",
		},
	})

	require.Contains(t, got, "Proibido, sem exceção")
	// And the deterministic check does not care what the seller asked for.
	assert.NotEmpty(t, ViolatedClaims("isso cura tudo", brand.ForbiddenClaims))
}
