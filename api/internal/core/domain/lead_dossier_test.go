package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dossier exists because the approach generator saw a fraction of what the
// account already knew. These tests pin the two rules that make it usable
// rather than merely bigger: every fact says where it came from, and every
// pattern says how much evidence it has.

func TestDossier_OrdersByHowMuchAMessageMayLeanOnIt(t *testing.T) {
	var d LeadDossier
	d.Add(DossierFact{Kind: DossierResearch, Label: "Decisor", Value: "Ana"})
	d.Add(DossierFact{Kind: DossierCollected, Label: "Orçamento", Value: "R$ 50k"})
	d.Add(DossierFact{Kind: DossierPattern, Label: "Sugerir", Value: "máscara"})
	d.Add(DossierFact{Kind: DossierOwnSales, Label: "Já compra", Value: "luva"})

	out := d.Render()

	// What the customer said, then what we sold them, then what we inferred,
	// then what a search read. A message that leads with an inference and
	// buries the customer's own words has the ladder upside down.
	order := []string{
		dossierKindLabel[DossierCollected],
		dossierKindLabel[DossierOwnSales],
		dossierKindLabel[DossierPattern],
		dossierKindLabel[DossierResearch],
	}
	last := -1
	for _, label := range order {
		at := strings.Index(out, label)
		require.Greater(t, at, last, "out of order: %s", label)
		last = at
	}
}

// A model handed a sentence with no source cites it as established. The origin
// is announced as an epistemic status, not as a table name.
func TestDossier_AnnouncesHowEachFactMayBeUsed(t *testing.T) {
	var d LeadDossier
	d.Add(DossierFact{Kind: DossierResearch, Label: "Decisor", Value: "Ana",
		Source: "linkedin.com"})

	out := d.Render()

	assert.Contains(t, out, "cite como leitura, não como certeza")
	assert.Contains(t, out, "(fonte: linkedin.com)")
}

// "They probably need masks" is a guess with a confident voice. "8 of 11
// customers who buy gloves also buy masks" is a sentence the salesperson can
// say out loud and defend.
func TestDossier_APatternCarriesItsEvidence(t *testing.T) {
	var d LeadDossier
	d.Add(DossierFact{
		Kind: DossierPattern, Label: "Costuma acompanhar", Value: "máscara",
		Evidence: "8 de 11 clientes que compram luva",
	})

	out := d.Render()

	assert.Contains(t, out, "8 de 11 clientes que compram luva")
	assert.Contains(t, out, "cite o número")
}

// An absent section reads as "nothing to say"; a named gap reads as "this is
// unknown". Those are different instructions to a model deciding whether to
// make a claim.
func TestDossier_NamesWhatIsMissingInsteadOfOmittingIt(t *testing.T) {
	var d LeadDossier
	d.Add(DossierFact{Kind: DossierOwnSales, Label: "Já compra", Value: "luva"})
	d.MarkMissing("quem decide a compra")

	out := d.Render()

	assert.Contains(t, out, "Ainda não sabemos")
	assert.Contains(t, out, "quem decide a compra")
	assert.Contains(t, out, "não invente")
}

func TestDossier_MissingIsRecordedOnce(t *testing.T) {
	var d LeadDossier
	d.MarkMissing("quem decide")
	d.MarkMissing("quem decide")
	d.MarkMissing("  ")

	assert.Equal(t, []string{"quem decide"}, d.Missing)
}

// A label with an empty value teaches the model that the field exists and is
// unknown — which is what Missing says, better.
func TestDossier_DropsFactsWithNothingInThem(t *testing.T) {
	var d LeadDossier
	d.Add(DossierFact{Kind: DossierOwnSales, Label: "Já compra", Value: "   "})
	d.Add(DossierFact{Kind: DossierOwnSales, Label: "", Value: "luva"})

	assert.True(t, d.Empty())
	assert.Empty(t, d.Render())
}

// Nothing known and nothing missing renders nothing: a heading over silence is
// worse than no section.
func TestDossier_EmptyRendersNothing(t *testing.T) {
	var d LeadDossier

	assert.Empty(t, d.Render())
	assert.True(t, d.Empty())
}

// Same dossier, same bytes — so a re-draft is comparable to the previous one
// and the provider's prefix cache is not thrown away by a map's iteration
// order.
func TestDossier_RenderIsStable(t *testing.T) {
	build := func() LeadDossier {
		var d LeadDossier
		for _, k := range []DossierFactKind{
			DossierSignal, DossierPattern, DossierRegistry,
			DossierKnowledge, DossierResearch,
		} {
			d.Add(DossierFact{Kind: k, Label: "l", Value: "v"})
		}
		return d
	}

	first := build().Render()
	for i := 0; i < 20; i++ {
		assert.Equal(t, first, build().Render())
	}
}

// Signal and pattern share a weight; the tie breaks on the kind so the order
// never depends on insertion.
func TestDossier_TiesBreakDeterministically(t *testing.T) {
	var a LeadDossier
	a.Add(DossierFact{Kind: DossierSignal, Label: "s", Value: "v"})
	a.Add(DossierFact{Kind: DossierPattern, Label: "p", Value: "v"})

	var b LeadDossier
	b.Add(DossierFact{Kind: DossierPattern, Label: "p", Value: "v"})
	b.Add(DossierFact{Kind: DossierSignal, Label: "s", Value: "v"})

	assert.Equal(t, a.Render(), b.Render())
}

// --- Per-fact provenance ---------------------------------------------------

// Either half alone is not enough. An uncited certainty is the shape a
// hallucination arrives in, and a cited guess is still a guess.
func TestFinding_TrustworthyNeedsBothHalves(t *testing.T) {
	cases := []struct {
		name string
		f    EnrichmentFinding
		want bool
	}{
		{"sure and cited", EnrichmentFinding{
			Confident: true, Source: "https://clinica.com.br/sobre"}, true},
		{"sure, uncited", EnrichmentFinding{Confident: true}, false},
		{"cited guess", EnrichmentFinding{
			Confident: false, Source: "https://clinica.com.br"}, false},
		{"neither", EnrichmentFinding{}, false},
		{"source that is not a URL", EnrichmentFinding{
			Confident: true, Source: "o site deles"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.f.Trustworthy())
		})
	}
}
