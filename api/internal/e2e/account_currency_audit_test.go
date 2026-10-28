package e2e

import (
	"context"
	"testing"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/require"
)

// US-015 (review fixes): locale-driven currency arbitrage must be
// auditable and correctable. These run against the real schema so the
// 000138 migration (signup_ip/signup_country/signup_locale) and the
// SetAccountCurrency query are exercised, not mocked.

// TestE2E_SignupPersistsOriginAudit FAILS on the pre-US-015 schema:
// the signup_* columns don't exist, so the INSERT blows up.
func TestE2E_SignupPersistsOriginAudit(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	repo := postgres.NewAccountRepository(testPool, nil)
	ip, country, locale := "203.0.113.7", "BR", "en-US"
	acc := &domain.Account{
		ID:                 "01SIGNUP0AUDIT0ACCOUNT0000",
		Name:               "Arbitrage Co",
		Currency:           domain.CurrencyUSD,
		SubscriptionStatus: "trialing",
		SignupIP:           &ip,
		SignupCountry:      &country,
		SignupLocale:       &locale,
	}
	require.NoError(t, repo.Create(ctx, acc))

	var gotIP, gotCountry, gotLocale, gotCurrency string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT signup_ip, signup_country, signup_locale, currency FROM accounts WHERE id = $1`,
		acc.ID).Scan(&gotIP, &gotCountry, &gotLocale, &gotCurrency))
	require.Equal(t, "203.0.113.7", gotIP)
	require.Equal(t, "BR", gotCountry)
	require.Equal(t, "en-US", gotLocale)
	require.Equal(t, "usd", gotCurrency)
}

// TestE2E_OwnerCurrencyCorrection exercises the repo half of the owner
// escape hatch: the BR-origin account that forced USD gets corrected
// back to BRL.
func TestE2E_OwnerCurrencyCorrection(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	repo := postgres.NewAccountRepository(testPool, nil)
	acc := &domain.Account{
		ID:                 "01CURRENCY0FIX0ACCOUNT0000",
		Name:               "Corrigida Ltda",
		Currency:           domain.CurrencyUSD,
		SubscriptionStatus: "trialing",
	}
	require.NoError(t, repo.Create(ctx, acc))

	require.NoError(t, repo.SetCurrency(ctx, acc.ID, domain.CurrencyBRL))

	got, err := repo.GetByID(ctx, acc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.CurrencyBRL, got.Currency)
}
