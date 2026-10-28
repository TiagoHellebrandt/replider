package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A lead re-drafted must land in the SAME arm. Random assignment would let
// someone regenerate until they got the variant they liked, quietly turning
// the experiment into a preference poll.
func TestAssignVariant_IsStableForTheSameLead(t *testing.T) {
	for _, id := range []string{"01KYMQ98DQC505XXD7GVJG8663", "abc", ""} {
		first := AssignVariant(id, 50)
		for i := 0; i < 20; i++ {
			assert.Equal(t, first, AssignVariant(id, 50), "lead %q trocou de braço", id)
		}
	}
}

// A 50/50 split must actually split. A hash that clumps would silently
// starve one arm and the experiment would never reach significance.
func TestAssignVariant_SplitsRoughlyEvenly(t *testing.T) {
	b := 0
	const n = 2000
	for i := 0; i < n; i++ {
		if AssignVariant(ulidish(i), 50) == "B" {
			b++
		}
	}
	assert.Greater(t, b, n*40/100, "braço B ficou faminto: %d de %d", b, n)
	assert.Less(t, b, n*60/100, "braço B ficou inchado: %d de %d", b, n)
}

func TestAssignVariant_HonoursTheExtremes(t *testing.T) {
	assert.Equal(t, "A", AssignVariant("qualquer", 0))
	assert.Equal(t, "B", AssignVariant("qualquer", 100))
}

// Sent verbatim is the signal the whole experiment turns on.
func TestEditRatio_ZeroWhenUntouched(t *testing.T) {
	texto := "Olá Dra. Marlene, aqui é a Rita da AssistMed. Podemos conversar?"
	assert.Equal(t, float32(0), EditRatio(texto, texto))
	assert.Equal(t, float32(0), EditRatio(texto, "  "+texto+"  "), "espaço nas bordas não é edição")
}

// A fixed typo and a full rewrite are opposite verdicts on the draft, and a
// boolean "edited" would score them the same.
func TestEditRatio_SeparatesATypoFromARewrite(t *testing.T) {
	draft := "Olá Douglas, cuidamos de autoclaves e bombas de infusão em Cuiabá."
	typo := "Olá Douglas, cuidamos de autoclaves e bombas de infusao em Cuiabá."
	rewrite := "Bom dia! Tudo certo por aí? Queria falar de outro assunto totalmente diferente."

	r1, r2 := EditRatio(draft, typo), EditRatio(draft, rewrite)
	assert.Less(t, r1, float32(0.05), "trocar um acento não é reescrever")
	assert.Greater(t, r2, float32(0.5), "reescrita inteira deveria pontuar alto")
	assert.Less(t, r1, r2)
}

func TestEditRatio_HandlesEmpties(t *testing.T) {
	assert.Equal(t, float32(0), EditRatio("", ""))
	assert.Equal(t, float32(1), EditRatio("algo", ""))
	assert.Equal(t, float32(1), EditRatio("", "algo"))
}

func ulidish(i int) string {
	const alfabeto = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	b := make([]byte, 26)
	for j := range b {
		b[j] = alfabeto[(i*7+j*13)%len(alfabeto)]
	}
	return string(b)
}
