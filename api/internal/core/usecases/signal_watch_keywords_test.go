package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Measured against production, 2026-09-08: a 103-product catalogue seeded a
// watch of fifteen words picked by LENGTH — "antitransbordamento",
// "bilirrubinômetro", "acessórios" — and matched ten of 414 real MT tenders,
// every hit coming from "acessórios" or "instrumentos" (musical instruments,
// clothing, IT supplies). The words naming what the company actually sells
// were shorter and never made the cut.

func TestDeriveWatchKeywords_RanksByWhatTheCompanySellsNotByWordLength(t *testing.T) {
	products := []*domain.Product{
		{Name: "Incubadora neonatal"},
		{Name: "Incubadora de transporte"},
		{Name: "Incubadora UTI"},
		{Name: "Capuz de oxigênio pequeno"},
		{Name: "Capuz de oxigênio grande"},
		// One product carrying the longest word in the catalogue. Under the
		// old length ranking this outranked "incubadora"; it must not now.
		{Name: "Válvula antitransbordamento"},
	}

	kws := deriveWatchKeywords(products)
	require.NotEmpty(t, kws)

	idx := func(w string) int {
		for i, k := range kws {
			if k == w {
				return i
			}
		}
		return -1
	}

	require.NotEqual(t, -1, idx("incubadora"), "the thing it sells must be watched")
	assert.Less(t, idx("incubadora"), idx("antitransbordamento"),
		"three products say incubadora and one says antitransbordamento")
	assert.Less(t, idx("capuz"), idx("antitransbordamento"))
}

func TestDeriveWatchKeywords_DropsWordsThatNameThePackaging(t *testing.T) {
	// Every false positive measured against real tenders came from one of
	// these: they name a category, not a product.
	products := []*domain.Product{
		{Name: "Acessórios para ventilador"},
		{Name: "Instrumentos cirúrgicos reutilizáveis"},
		{Name: "Materiais de consumo"},
	}

	kws := deriveWatchKeywords(products)

	for _, junk := range []string{"acessórios", "instrumentos", "materiais", "reutilizáveis"} {
		assert.NotContains(t, kws, junk, "%q matched musical instruments and clothing", junk)
	}
	assert.Contains(t, kws, "ventilador")
	assert.Contains(t, kws, "cirúrgicos")
}

func TestDeriveWatchKeywords_OneVotePerProduct(t *testing.T) {
	// "Capuz" in three sizes is three products, not three votes from one.
	products := []*domain.Product{
		{Name: "Capuz capuz capuz de oxigênio"},
		{Name: "Incubadora neonatal"},
		{Name: "Incubadora de transporte"},
	}

	kws := deriveWatchKeywords(products)
	require.NotEmpty(t, kws)
	assert.Equal(t, "incubadora", kws[0],
		"two products beat one, however often the one repeats itself")
}

func TestSignalWatchSeeder_RefreshesTheWatchItWroteBefore(t *testing.T) {
	// Without this, a fix to the derivation never reaches an account that
	// was already seeded by the broken one.
	up := &fakeWatchUpserter{}
	seeder := NewSignalWatchSeeder(
		fakeWatchCandidates{list: []domain.SignalWatchCandidate{
			{AccountID: "acc1", UF: "MT", DerivedWatchID: "watch-existing"},
		}},
		fakeProductLister{byAccount: map[string][]*domain.Product{
			"acc1": {{Name: "Incubadora neonatal"}},
		}},
		up,
	)

	seeder.EnsureDefaultWatches(context.Background())

	require.Len(t, up.saved, 1)
	assert.Equal(t, "watch-existing", up.saved[0].ID,
		"same row refreshed — a second watch beside it is a bug the customer sees")
	assert.Contains(t, up.saved[0].Keywords, "incubadora")
	assert.True(t, up.saved[0].DerivedFromCatalog,
		"still ours to refresh next time; an edit through the UI clears this")
}

func TestSignalWatchSeeder_NewAccountStillGetsAFreshID(t *testing.T) {
	up := &fakeWatchUpserter{}
	seeder := NewSignalWatchSeeder(
		fakeWatchCandidates{list: []domain.SignalWatchCandidate{{AccountID: "acc1", UF: "MT"}}},
		fakeProductLister{byAccount: map[string][]*domain.Product{
			"acc1": {{Name: "Incubadora neonatal"}},
		}},
		up,
	)

	seeder.EnsureDefaultWatches(context.Background())

	require.Len(t, up.saved, 1)
	assert.Len(t, up.saved[0].ID, 26, "a ULID, not an empty id")
}
