package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedSourceName(t *testing.T) {
	assert.Equal(t, "Perguntas frequentes", SeedSourceName(""))
	assert.Equal(t, "Perguntas frequentes", SeedSourceName("pt"))
	assert.Equal(t, "Perguntas frequentes", SeedSourceName("fr"))
	assert.Equal(t, "FAQ", SeedSourceName("en"))
}

func TestLocalizeSeedTitle(t *testing.T) {
	t.Run("pt locale returns original PT", func(t *testing.T) {
		assert.Equal(t, "Horário de atendimento",
			LocalizeSeedTitle("Horário de atendimento", "pt"))
	})

	t.Run("empty locale defaults to PT", func(t *testing.T) {
		assert.Equal(t, "Horário de atendimento",
			LocalizeSeedTitle("Horário de atendimento", ""))
	})

	t.Run("en locale picks EN translation", func(t *testing.T) {
		assert.Equal(t, "Business hours",
			LocalizeSeedTitle("Horário de atendimento", "en"))
		assert.Equal(t, "Refunds", LocalizeSeedTitle("Reembolso", "en"))
	})

	t.Run("unknown title falls back to PT under en", func(t *testing.T) {
		assert.Equal(t, "Algum título novo",
			LocalizeSeedTitle("Algum título novo", "en"))
	})
}

// Every Title literal used in agent_templates.go must have an entry in
// seedTitleEN. Walking the canonical list keeps the map honest as new
// templates land — adding a SeedChunk without an EN gloss will fail
// here instead of silently shipping a PT label to EN users.
func TestSeedTitleEN_CoversEveryTemplateTitle(t *testing.T) {
	missing := map[string]struct{}{}
	for _, tpl := range agentTemplates {
		for _, seed := range tpl.SeedKnowledge {
			if _, ok := seedTitleEN[seed.Title]; !ok {
				missing[seed.Title] = struct{}{}
			}
		}
	}
	if len(missing) > 0 {
		var titles []string
		for k := range missing {
			titles = append(titles, k)
		}
		t.Fatalf("seedTitleEN missing %d title(s): %v", len(missing), titles)
	}
}
