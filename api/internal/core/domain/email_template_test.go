package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A library of short texts with variables, not a drag-and-drop builder: the
// editor everyone asks for produces exactly the emails that land in spam.

func TestTemplateVariables_ListedOnceAndSorted(t *testing.T) {
	got := TemplateVariables(
		"Proposta para {{empresa}}",
		"Olá {{nome}}, sobre a {{ empresa }} — falo com {{nome}}?")

	assert.Equal(t, []string{"empresa", "nome"}, got)
}

func TestTemplateVariables_NoneIsEmpty(t *testing.T) {
	assert.Empty(t, TemplateVariables("Assunto", "Corpo sem variável"))
}

func TestRenderTemplate_SubstitutesWhatItHas(t *testing.T) {
	got := RenderTemplate("Olá {{nome}}, sobre a {{empresa}}",
		map[string]string{"nome": "Dra. Ana", "empresa": "Clínica Vida Nova"})

	assert.Equal(t, "Olá Dra. Ana, sobre a Clínica Vida Nova", got)
}

// "Olá {{nome}}" reaching a customer is embarrassing. "Olá ," reaching them is
// embarrassing AND invisible to whoever approved it — a surviving placeholder
// is caught by the approval queue's check, an empty gap is not.
func TestRenderTemplate_LeavesUnknownVariablesStanding(t *testing.T) {
	got := RenderTemplate("Olá {{nome}}, da {{empresa}}",
		map[string]string{"nome": "Dra. Ana"})

	assert.Contains(t, got, "{{empresa}}")
	assert.NotContains(t, got, "da ,")
}

// An empty value is as dangerous as a missing one, and is treated the same.
func TestRenderTemplate_BlankValueCountsAsMissing(t *testing.T) {
	got := RenderTemplate("Olá {{nome}}", map[string]string{"nome": "   "})

	assert.Contains(t, got, "{{nome}}")
}

func TestMissingVariables_ReportsWhatWouldSurvive(t *testing.T) {
	got := MissingVariables("Para {{empresa}}", "Olá {{nome}}, tudo bem?",
		map[string]string{"nome": "Ana"})

	assert.Equal(t, []string{"empresa"}, got)
}

func TestMissingVariables_NothingMissingIsEmpty(t *testing.T) {
	got := MissingVariables("Para {{empresa}}", "Olá {{nome}}",
		map[string]string{"nome": "Ana", "empresa": "Clínica"})

	assert.Empty(t, got)
}

// ── Metrics ───────────────────────────────────────────────────────────────

// Zero replies out of zero sends is not a zero rate, it is unknown. Showing
// 0% would condemn a template nobody has used yet.
func TestStats_RateIsNilWithoutSends(t *testing.T) {
	s := EmailTemplateStats{}

	assert.Nil(t, s.ReplyRate())
	assert.Nil(t, s.BounceRate())
}

func TestStats_ReplyRate(t *testing.T) {
	r := EmailTemplateStats{Sent: 200, Replied: 24}.ReplyRate()

	require.NotNil(t, r)
	assert.InDelta(t, 0.12, *r, 0.001)
}

// Mailbox providers throttle a sender well before ten percent, and by then
// the damage takes weeks to undo.
func TestStats_DirtyListWarning(t *testing.T) {
	assert.True(t, EmailTemplateStats{Sent: 100, Bounced: 6}.NeedsListWarning())
	assert.False(t, EmailTemplateStats{Sent: 100, Bounced: 2}.NeedsListWarning())
}

// One bounce out of two sends is 50% and means nothing.
func TestStats_NoWarningWithoutASample(t *testing.T) {
	assert.False(t, EmailTemplateStats{Sent: 2, Bounced: 1}.NeedsListWarning())
}

func TestWordCount(t *testing.T) {
	assert.Equal(t, 0, WordCount("   "))
	assert.Equal(t, 6, WordCount("Bom dia, posso mandar duas opções?"))
	assert.Less(t, WordCount("Bom dia"), ColdEmailMaxWords)
}
