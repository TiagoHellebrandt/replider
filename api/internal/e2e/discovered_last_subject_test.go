package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mailbox history as per-contact memory (issue #292). The discovery walk goes
// BACKWARDS through the Sent folder — newest page first — so last_subject's
// newest-wins rule is the whole correctness property: an older sighting
// arriving later must never overwrite the current thread's subject.
func TestDiscoveredContact_LastSubjectFollowsTheNewestSighting(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewDiscoveredContactRepository(testPool)
	accountID := seedIdentityAccount(t)

	newest := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	older := newest.AddDate(-1, 0, 0)

	require.NoError(t, repo.Upsert(ctx, accountID, "", domain.SentRecipient{
		Address: "compras@hospital.com.br",
		SentAt:  newest,
		Subject: "Proposta tomógrafo 2026",
	}))
	// The backwards walk now reaches last year's thread.
	require.NoError(t, repo.Upsert(ctx, accountID, "", domain.SentRecipient{
		Address: "compras@hospital.com.br",
		SentAt:  older,
		Subject: "Orçamento antigo",
	}))

	got, err := repo.GetByAddresses(ctx, accountID, []string{"compras@hospital.com.br"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].SentCount)
	assert.Equal(t, "Proposta tomógrafo 2026", got[0].LastSubject,
		"an older sighting must not overwrite the newest thread's subject")

	// A genuinely newer message updates it.
	require.NoError(t, repo.Upsert(ctx, accountID, "", domain.SentRecipient{
		Address: "compras@hospital.com.br",
		SentAt:  newest.AddDate(0, 1, 0),
		Subject: "Re: Proposta tomógrafo 2026 — parcelamento",
	}))
	got, err = repo.GetByAddresses(ctx, accountID, []string{"compras@hospital.com.br"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Re: Proposta tomógrafo 2026 — parcelamento", got[0].LastSubject)
}
