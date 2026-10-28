package usecases

import (
	"context"
	"fmt"

	"github.com/replider/api/internal/core/domain"
)

type PurchaseAddonDTO struct {
	AccountID  string
	PackID     string // "addon_p", "addon_m", "addon_g"
	SuccessURL string
	CancelURL  string
}

type AddonPack struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Tokens   int64  `json:"tokens"`
	Price    int    `json:"price"`    // cents, in Currency
	Currency string `json:"currency"` // "brl" | "usd"
}

// addonPacksBRL — one-time credit packs for BRL accounts. Prices in
// centavos BRL, aligned with the landing page (pricing.addon_note).
var addonPacksBRL = []AddonPack{
	{ID: "addon_p", Name: "Recarga P", Tokens: 25_000, Price: 4700, Currency: domain.CurrencyBRL},   // R$ 47
	{ID: "addon_m", Name: "Recarga M", Tokens: 75_000, Price: 14700, Currency: domain.CurrencyBRL},  // R$ 147
	{ID: "addon_g", Name: "Recarga G", Tokens: 200_000, Price: 29700, Currency: domain.CurrencyBRL}, // R$ 297
}

// addonPacksUSD — the same three tiers priced for USD accounts. Charged
// via Stripe ad-hoc price_data (currency + unit_amount), so no Stripe
// price id is needed — the amounts here are the source of truth.
var addonPacksUSD = []AddonPack{
	{ID: "addon_p", Name: "Top-up S", Tokens: 25_000, Price: 900, Currency: domain.CurrencyUSD},   // $9
	{ID: "addon_m", Name: "Top-up M", Tokens: 75_000, Price: 2500, Currency: domain.CurrencyUSD},  // $25
	{ID: "addon_g", Name: "Top-up L", Tokens: 200_000, Price: 4900, Currency: domain.CurrencyUSD}, // $49
}

// AddonPacks is the BRL default set, kept for callers/tests that don't
// carry a currency. Prefer AddonPacksForCurrency.
var AddonPacks = addonPacksBRL

// AddonPacksForCurrency returns the pack set for the given account
// currency. Anything other than USD falls back to the BRL ladder (empty
// currency = legacy BRL account).
func AddonPacksForCurrency(currency string) []AddonPack {
	if currency == domain.CurrencyUSD {
		return addonPacksUSD
	}
	return addonPacksBRL
}

type PurchaseAddonUseCase struct {
	accountRepo domain.AccountRepository
	paymentAdpt domain.PaymentAdapter
}

func NewPurchaseAddonUseCase(accountRepo domain.AccountRepository, paymentAdpt domain.PaymentAdapter) *PurchaseAddonUseCase {
	return &PurchaseAddonUseCase{
		accountRepo: accountRepo,
		paymentAdpt: paymentAdpt,
	}
}

func (uc *PurchaseAddonUseCase) Execute(ctx context.Context, dto PurchaseAddonDTO) (string, error) {
	// 1. Verify account exists — its currency picks the pack set and
	// prices the Stripe charge, so it must resolve before pack lookup
	// (the price is currency-specific, not a relabelled cents figure).
	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return "", domain.ErrAccountNotFound
	}
	currency := account.Currency
	if currency == "" {
		currency = domain.CurrencyBRL
	}

	// 2. Find the pack by ID within the account's currency set.
	packs := AddonPacksForCurrency(currency)
	var pack *AddonPack
	for i := range packs {
		if packs[i].ID == dto.PackID {
			pack = &packs[i]
			break
		}
	}
	if pack == nil {
		return "", &domain.AppError{Code: "INVALID_PACK", Message: "addon pack not found"}
	}

	// 3. Create a Stripe checkout session for the one-time pack purchase.
	// The price is in the account's own currency (BRL or USD), so no
	// cross-currency mislabelling is possible.
	url, err := uc.paymentAdpt.CreateAddonCheckoutSession(pack.ID, pack.Name, int64(pack.Price), account.ID, currency, dto.SuccessURL, dto.CancelURL)
	if err != nil {
		return "", fmt.Errorf("could not create stripe addon checkout session: %w", err)
	}

	return url, nil
}
