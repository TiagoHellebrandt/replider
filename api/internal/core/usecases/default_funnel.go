package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// ensureDefaultFunnel gives a freshly created account its implicit funnel.
//
// Called from every place an account is born (public signup, Google/Meta
// signup, the owner panel's contract accounts) so the Clientes tab has
// somewhere to put customers before the person has done anything. Optional
// wiring: a nil repo is a no-op, and FunnelUseCase.List self-heals a missing
// default on first read anyway — this just spares the first read a write.
//
// Inside a transaction it joins it (getQueries reads the tx from ctx), so a
// failure rolls the whole signup back rather than leaving an account without
// its funnel. That is deliberate: the insert is trivial and the alternative
// is a best-effort write with no test or metric watching it.
func ensureDefaultFunnel(ctx context.Context, repo domain.SalesFunnelRepository, accountID string) error {
	if repo == nil {
		return nil
	}
	_, err := repo.EnsureDefault(ctx, accountID)
	return err
}
