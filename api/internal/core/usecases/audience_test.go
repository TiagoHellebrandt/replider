package usecases

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The value of this feature is the SECOND and third audience. Somebody who
// sells patient monitors to hospitals also sells them to dialysis clinics,
// and the reason they do not prospect those is that nobody wrote it down.

const suggestionJSON = `[
  {"name":"Hospitais gerais","segment":"hospitais gerais","decision_maker":"gerente de compras","keywords":["hospital"],"unexplored":false},
  {"name":"Clínicas de diálise","segment":"clínicas de diálise","decision_maker":"diretor clínico","keywords":["clínica de diálise"],"unexplored":true},
  {"name":"Home care","segment":"empresas de home care","decision_maker":"coordenador","keywords":["home care"],"unexplored":false}
]`

func TestParseAudienceSuggestions_ReadsThePlainAnswer(t *testing.T) {
	got := parseAudienceSuggestions(suggestionJSON)

	require.Len(t, got, 3)
	assert.Equal(t, "hospitais gerais", got[0].Segment)
	assert.True(t, got[0].Suggested, "a proposal is labelled as one")
}

// Exactly one "você talvez não viu" card. Two would be a list, and a list of
// things you have not considered is just noise.
func TestParseAudienceSuggestions_KeepsOnlyOneAdjacentSuggestion(t *testing.T) {
	greedy := `[
	  {"segment":"a","unexplored":true},
	  {"segment":"b","unexplored":true},
	  {"segment":"c","unexplored":true}
	]`

	got := parseAudienceSuggestions(greedy)

	var flagged int
	for _, a := range got {
		if a.Unexplored {
			flagged++
		}
	}
	assert.Equal(t, 1, flagged)
}

// Models add fences no matter how firmly they are asked not to.
func TestParseAudienceSuggestions_SurvivesCodeFences(t *testing.T) {
	got := parseAudienceSuggestions("Claro!\n```json\n" + suggestionJSON + "\n```")

	assert.Len(t, got, 3)
}

// A garbage answer produces nothing rather than a half-parsed audience the
// customer would then prospect.
func TestParseAudienceSuggestions_GarbageYieldsNothing(t *testing.T) {
	for _, raw := range []string{"", "desculpe, não consegui", "{}", "[{]"} {
		assert.Empty(t, parseAudienceSuggestions(raw), "input %q", raw)
	}
}

// An entry with no segment has nothing to search for.
func TestParseAudienceSuggestions_DropsEntriesWithNoSegment(t *testing.T) {
	got := parseAudienceSuggestions(`[{"name":"vazio"},{"segment":"clínicas"}]`)

	require.Len(t, got, 1)
	assert.Equal(t, "clínicas", got[0].Segment)
}

// Never more than three, whatever the model decides to be helpful with: a
// list of ten audiences is a list nobody chooses from.
func TestParseAudienceSuggestions_CapsAtThree(t *testing.T) {
	many := `[{"segment":"a"},{"segment":"b"},{"segment":"c"},{"segment":"d"},{"segment":"e"}]`

	assert.Len(t, parseAudienceSuggestions(many), domain.AudienceSuggestionCount)
}

// A nameless audience is unpickable in a dropdown; the segment is what the
// person would have called it anyway.
func TestParseAudienceSuggestions_FallsBackToTheSegmentAsAName(t *testing.T) {
	got := parseAudienceSuggestions(`[{"segment":"clínicas de imagem"}]`)

	require.Len(t, got, 1)
	assert.Equal(t, "clínicas de imagem", got[0].Name)
}

// ── What reaches the prospecting source ───────────────────────────────────

// Google Places indexes businesses, not job titles: "gerente de compras" in
// the query finds recruitment agencies.
func TestDeriveKeywords_LeavesTheRoleOut(t *testing.T) {
	got := deriveKeywords(&domain.Audience{
		Segment: "clínicas de diálise", Size: "10 a 50 leitos",
		DecisionMaker: "gerente de compras",
	})

	assert.Equal(t, []string{"clínicas de diálise", "10 a 50 leitos"}, got)
}

func TestAudience_SearchQueryJoinsIntoOnePhrase(t *testing.T) {
	a := &domain.Audience{
		Keywords: []string{"clínica de diálise", "nefrologia"},
		Regions:  []string{"Sorocaba", "Campinas"},
	}

	keyword, location := a.SearchQuery()

	// Sources take one phrase, not a boolean expression.
	assert.Equal(t, "clínica de diálise nefrologia", keyword)
	assert.Equal(t, "Sorocaba", location)
}

// An audience with no keywords still has a segment, which is the sentence a
// human would have typed anyway.
func TestAudience_SearchQueryFallsBackToTheSegment(t *testing.T) {
	keyword, _ := (&domain.Audience{Segment: "hospitais gerais"}).SearchQuery()

	assert.Equal(t, "hospitais gerais", keyword)
}
