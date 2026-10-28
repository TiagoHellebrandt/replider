package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A tender is the strongest intent signal there is — somebody wrote down what
// they intend to buy, by when, for how much. Its value is entirely in being
// acted on BEFORE the deadline.

var signalNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func tender(closesIn time.Duration) *Signal {
	closes := signalNow.Add(closesIn)
	return &Signal{
		Source: SignalPNCP, ExternalID: "pncp-1",
		Title:        "Aquisição de monitores multiparâmetro",
		Organization: "Hospital Municipal de Sorocaba",
		State:        "SP", ValueCents: 45000000,
		PublishedAt: signalNow.Add(-48 * time.Hour),
		ClosesAt:    &closes,
	}
}

// Approaching somebody about a bid they already closed is worse than not
// writing at all: it proves nobody read it.
func TestSignal_ExpiredIsNotActionable(t *testing.T) {
	closed := tender(-1 * time.Hour)

	assert.True(t, closed.Expired(signalNow))
	assert.False(t, closed.Actionable(signalNow))
}

func TestSignal_OpenTenderIsActionable(t *testing.T) {
	assert.True(t, tender(72*time.Hour).Actionable(signalNow))
}

// Guessing a deadline and hiding a live opportunity is a worse error than
// showing an old one, whose date the salesperson can see.
func TestSignal_NoDeadlineNeverExpires(t *testing.T) {
	s := tender(72 * time.Hour)
	s.ClosesAt = nil

	assert.False(t, s.Expired(signalNow.Add(10*365*24*time.Hour)))
	assert.Nil(t, s.DaysLeft(signalNow))
}

// Two people chasing the same tender is worse than one: the buyer gets two
// pitches from the same company.
func TestSignal_AWorkedSignalIsNotOffered(t *testing.T) {
	s := tender(72 * time.Hour)
	s.LeadID = "lead1"

	assert.False(t, s.Actionable(signalNow))
}

func TestSignal_ADismissedSignalStaysDismissed(t *testing.T) {
	s := tender(72 * time.Hour)
	s.DismissedAt = &signalNow

	assert.False(t, s.Actionable(signalNow))
}

// Urgency is the whole point: three days is a reason to write today.
func TestSignal_DaysLeftDrivesTheCard(t *testing.T) {
	left := tender(72 * time.Hour).DaysLeft(signalNow)

	require.NotNil(t, left)
	assert.Equal(t, 3, *left)
}

// ── Watches ───────────────────────────────────────────────────────────────

func watch(mut func(*SignalWatch)) *SignalWatch {
	w := &SignalWatch{
		Source: SignalPNCP, IsActive: true,
		Keywords: []string{"monitor", "monitorização"},
	}
	if mut != nil {
		mut(w)
	}
	return w
}

// ANY keyword counts. A supplier of monitors sells to "monitor
// multiparâmetro" and to "monitorização"; demanding every term finds nothing.
func TestSignalWatch_AnyKeywordMatches(t *testing.T) {
	assert.True(t, watch(nil).Matches(tender(72*time.Hour)))
}

func TestSignalWatch_UnrelatedTenderDoesNotMatch(t *testing.T) {
	s := tender(72 * time.Hour)
	s.Title = "Aquisição de material de escritório"
	s.Organization = "Prefeitura de Sorocaba"

	assert.False(t, watch(nil).Matches(s))
}

// The organization is searched too: "Hospital" in the buyer's name is a
// signal even when the object is written in procurement-speak.
func TestSignalWatch_MatchesOnTheBuyersName(t *testing.T) {
	s := tender(72 * time.Hour)
	s.Title = "Registro de preços — grupo 07"
	w := watch(func(w *SignalWatch) { w.Keywords = []string{"hospital"} })

	assert.True(t, w.Matches(s))
}

// A supplier who cannot deliver to Acre does not want to read about Acre.
func TestSignalWatch_StateFilter(t *testing.T) {
	w := watch(func(w *SignalWatch) { w.States = []string{"SP", "PR"} })
	assert.True(t, w.Matches(tender(72*time.Hour)))

	far := tender(72 * time.Hour)
	far.State = "AC"
	assert.False(t, w.Matches(far))
}

// R$ 800 of office supplies is not worth a salesperson's morning.
func TestSignalWatch_MinimumValue(t *testing.T) {
	// The fixture tender is R$ 450.000; a floor of R$ 1.000.000 excludes it,
	// a floor of R$ 100.000 does not.
	assert.False(t, watch(func(w *SignalWatch) { w.MinValueCents = 1_000_000_00 }).
		Matches(tender(72*time.Hour)))
	assert.True(t, watch(func(w *SignalWatch) { w.MinValueCents = 100_000_00 }).
		Matches(tender(72*time.Hour)))
}

// "Not disclosed" is common in public tenders and is NOT the same as cheap.
// Filtering those out would hide the biggest buys in the country.
func TestSignalWatch_UndisclosedValuePassesTheFloor(t *testing.T) {
	w := watch(func(w *SignalWatch) { w.MinValueCents = 1_000_000_00 })
	s := tender(72 * time.Hour)
	s.ValueCents = 0

	assert.True(t, w.Matches(s))
}

// A watch with no keywords matches NOTHING. "Everything in the state" is
// thousands of tenders a month and, with the auto-offer behind it, a draft
// budget spent on office supplies.
func TestSignalWatch_NoKeywordsMatchesNothing(t *testing.T) {
	w := watch(func(w *SignalWatch) { w.Keywords = nil })
	assert.False(t, w.Matches(tender(72*time.Hour)))

	blank := watch(func(w *SignalWatch) { w.Keywords = []string{"", "  "} })
	assert.False(t, blank.Matches(tender(72*time.Hour)), "blank keywords are no keywords")
}

func TestSignalWatch_InactiveMatchesNothing(t *testing.T) {
	w := watch(func(w *SignalWatch) { w.IsActive = false })

	assert.False(t, w.Matches(tender(72*time.Hour)))
}

func TestSignalWatch_CaseAndAccentsDoNotBreakMatching(t *testing.T) {
	s := tender(72 * time.Hour)
	s.Title = "AQUISIÇÃO DE MONITORES MULTIPARÂMETRO"

	assert.True(t, watch(nil).Matches(s))
}

// Whole-word matching, measured against 417 real MT tenders: plain substring
// matching produced 57 hits where 3 were defensible, and every removed hit
// would have spent a model call drafting an offer for an unrelated tender.

func TestSignalWatchMatches_DoesNotMatchInsideALongerWord(t *testing.T) {
	w := &SignalWatch{IsActive: true, Keywords: []string{"monitor"}}

	assert.False(t, w.Matches(&Signal{
		Title: "Prestação de serviços continuados de monitoramento de alarmes",
	}), "monitoramento is not a monitor")

	assert.True(t, w.Matches(&Signal{
		Title: "Aquisição de monitor multiparamétrico",
	}))
}

func TestSignalWatchMatches_FoldsAccentsOnBothSides(t *testing.T) {
	w := &SignalWatch{IsActive: true, Keywords: []string{"oxigênio"}}

	assert.True(t, w.Matches(&Signal{
		Title: "Aquisicao de gas oxigenio medicinal e fluxometros",
	}), "a tender that dropped the accents is the same tender")
}

func TestSignalWatchMatches_BoundariesAreNotLetters(t *testing.T) {
	w := &SignalWatch{IsActive: true, Keywords: []string{"capuz"}}

	for _, title := range []string{
		"aquisição de capuz de oxigênio",
		"CAPUZ, tamanho médio",
		"itens: capuz/máscara",
		"capuz",
	} {
		assert.True(t, w.Matches(&Signal{Title: title}), title)
	}
	assert.False(t, w.Matches(&Signal{Title: "encapuzamento de fiação"}))
}

func TestSignalWatchMatches_OverlappingOccurrenceStillCounts(t *testing.T) {
	// The first occurrence fails the boundary test and the scan must keep
	// going rather than give up on the word.
	w := &SignalWatch{IsActive: true, Keywords: []string{"ambu"}}

	assert.True(t, w.Matches(&Signal{
		Title: "reanimador ambulatorial tipo ambu",
	}))
}

func TestSignalWatchMatches_TolerantesToThePortuguesePlural(t *testing.T) {
	// The ordinary case: the catalogue names one, the tender buys several.
	w := &SignalWatch{IsActive: true, Keywords: []string{"monitor", "capuz"}}

	assert.True(t, w.Matches(&Signal{Title: "Aquisição de monitores multiparâmetro"}))
	assert.True(t, w.Matches(&Signal{Title: "Aquisição de capuzes de oxigênio"}))
	assert.False(t, w.Matches(&Signal{Title: "serviços de monitoramento"}),
		"the plural rule must not reopen the door to longer words")
}

func TestSignalWatchMatches_APluralWatchWordFindsTheSingular(t *testing.T) {
	w := &SignalWatch{IsActive: true, Keywords: []string{"luvas"}}
	assert.True(t, w.Matches(&Signal{Title: "Aquisição de luva de procedimento"}))
}

// ---- NormalizeAndValidate: the one rule all three writers share ----

// A watch with no UF is not a watch. The poll's scope is the UNION of the
// watches in it, so one unscoped row used to drag every other account's fetch
// across the country — which is exactly how a dogfood watch for the word
// "luva" earned the 429s that emptied the radar.
func TestSignalWatch_RefusesAWatchWithNoState(t *testing.T) {
	w := &SignalWatch{Keywords: []string{"oxigênio"}}

	err := w.NormalizeAndValidate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "estado")
}

// The other half of the same rule: no keyword matches nothing, so saving one
// saves a watch that silently never fires.
func TestSignalWatch_RefusesAWatchWithNoKeyword(t *testing.T) {
	w := &SignalWatch{States: []string{"MT"}, Keywords: []string{"  ", ""}}

	err := w.NormalizeAndValidate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "palavra-chave")
}

// A typo that passes a two-letter shape test ("XX") would produce a feed that
// is quietly empty forever, so the codes are checked against the real 27.
func TestSignalWatch_RefusesAStateThatIsNotAState(t *testing.T) {
	w := &SignalWatch{Keywords: []string{"monitor"}, States: []string{"XX"}}

	assert.Error(t, w.NormalizeAndValidate())
}

// A valid watch comes back tidied: states upper-cased, trimmed and deduped,
// keywords stripped of blanks — so the stored row is the one the poll reads.
func TestSignalWatch_TidiesWhatItAccepts(t *testing.T) {
	w := &SignalWatch{
		Keywords: []string{" oxigênio ", "", "cilindro"},
		States:   []string{" mt ", "MT", "sp"},
	}

	require.NoError(t, w.NormalizeAndValidate())

	assert.Equal(t, []string{"oxigênio", "cilindro"}, w.Keywords)
	assert.Equal(t, []string{"MT", "SP"}, w.States)
}
