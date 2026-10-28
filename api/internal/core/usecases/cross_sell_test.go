package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

type stubCrossSellRepo struct {
	pairs     []domain.CoPurchasePair
	bought    []string
	pairsCall int
	err       error
}

func (s *stubCrossSellRepo) Pairs(context.Context, string) ([]domain.CoPurchasePair, error) {
	s.pairsCall++
	return s.pairs, s.err
}
func (s *stubCrossSellRepo) ItemsBoughtBy(context.Context, string, string) ([]string, error) {
	return s.bought, s.err
}

func TestCrossSellSuggest_ExplainsItself(t *testing.T) {
	repo := &stubCrossSellRepo{
		bought: []string{"luva"},
		pairs:  []domain.CoPurchasePair{{Product: "luva", Partner: "mascara", Customers: 8, ProductCustomers: 10}},
	}
	out, err := NewCrossSellUseCase(repo).Suggest(context.Background(), "acc", "c1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Suggestions) != 1 || out.Suggestions[0].Because != "luva" {
		t.Fatalf("expected a suggestion with its reason, got %+v", out.Suggestions)
	}
}

// "We do not know yet" is the honest answer for a distributor in their first
// month. An error would read as a broken feature.
func TestCrossSellSuggest_NoSalesIsAnEmptyAnswerNotAnError(t *testing.T) {
	repo := &stubCrossSellRepo{}
	out, err := NewCrossSellUseCase(repo).Suggest(context.Background(), "acc", "c1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Suggestions) != 0 {
		t.Fatalf("expected nothing, got %+v", out.Suggestions)
	}
	// And it does not pay for the expensive query it cannot use.
	if repo.pairsCall != 0 {
		t.Fatal("with nothing bought there is no point reading the patterns")
	}
}

func TestCrossSellSuggest_RequiresAContact(t *testing.T) {
	_, err := NewCrossSellUseCase(&stubCrossSellRepo{}).
		Suggest(context.Background(), "acc", "  ", 3)
	if err == nil {
		t.Fatal("a suggestion with no customer is not a suggestion")
	}
}

func TestCrossSellSuggest_SurfacesARepositoryFailure(t *testing.T) {
	repo := &stubCrossSellRepo{err: errors.New("boom")}
	if _, err := NewCrossSellUseCase(repo).Suggest(context.Background(), "acc", "c1", 3); err == nil {
		t.Fatal("a read failure must not be reported as an empty answer")
	}
}

func TestCrossSellPatterns_NeverReturnsNil(t *testing.T) {
	out, err := NewCrossSellUseCase(&stubCrossSellRepo{}).
		Patterns(context.Background(), "acc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("an empty list serialises as [], nil serialises as null")
	}
}
