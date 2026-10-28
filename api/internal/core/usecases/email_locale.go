package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// emailLocaleFor resolves the transactional-email locale for an account
// (US-018 review fixes): USD accounts get English, everything else
// pt-BR. Nil repo, empty id or a failed lookup all degrade to "" —
// email.resolveLocale treats that as pt-BR, the pre-US-018 behavior —
// so a missing wire never blocks a send.
func emailLocaleFor(ctx context.Context, accounts domain.AccountRepository, accountID string) string {
	if accounts == nil || accountID == "" {
		return ""
	}
	acc, err := accounts.GetByID(ctx, accountID)
	if err != nil || acc == nil {
		return ""
	}
	return domain.EmailLocaleForCurrency(acc.Currency)
}
