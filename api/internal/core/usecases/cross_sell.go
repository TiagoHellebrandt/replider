package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// CrossSellUseCase answers "what else should I offer this customer?" from the
// account's own sales (§6.5, F5).
type CrossSellUseCase struct {
	repo domain.CrossSellRepository
}

func NewCrossSellUseCase(repo domain.CrossSellRepository) *CrossSellUseCase {
	return &CrossSellUseCase{repo: repo}
}

// CrossSellResult is what the panel renders.
type CrossSellResult struct {
	Suggestions []domain.CrossSellSuggestion `json:"suggestions"`
	// Bought is what the customer already buys, so the screen can show the
	// evidence next to the suggestion instead of an unexplained product name.
	Bought []string `json:"bought"`
}

// Suggest picks what to offer one customer.
//
// Returns an EMPTY list rather than an error when the account has too few
// sales to see a pattern. "We do not know yet" is the honest answer for a
// distributor in their first month, and an error would read as a broken
// feature.
func (uc *CrossSellUseCase) Suggest(ctx context.Context, accountID, contactID string, limit int) (*CrossSellResult, error) {
	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return nil, domain.ErrContactNotFound
	}

	bought, err := uc.repo.ItemsBoughtBy(ctx, accountID, contactID)
	if err != nil {
		return nil, err
	}
	result := &CrossSellResult{Suggestions: []domain.CrossSellSuggestion{}, Bought: bought}
	if len(bought) == 0 {
		// Nothing bought means nothing to anchor a suggestion on. Guessing
		// from the account's most popular product would be a recommendation
		// with no reason, which is the kind the salesperson learns to ignore.
		return result, nil
	}

	pairs, err := uc.repo.Pairs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result.Suggestions = domain.SuggestCrossSell(bought, pairs, limit)
	return result, nil
}

// Patterns exposes what the account has learned, for the screen that explains
// where a suggestion came from.
func (uc *CrossSellUseCase) Patterns(ctx context.Context, accountID string) ([]domain.CoPurchasePair, error) {
	pairs, err := uc.repo.Pairs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if pairs == nil {
		pairs = []domain.CoPurchasePair{}
	}
	return pairs, nil
}
