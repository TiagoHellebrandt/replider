package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The brochure was archived on every import for months and orphaned in the
// same breath — a random object name, no row, nothing that could say which
// file a product came from. These pin the door that makes it reachable, and
// the two ways it must refuse.

type fakeProductGetter struct{ p *domain.Product }

func (f *fakeProductGetter) GetByID(_ context.Context, id string) (*domain.Product, error) {
	if f.p == nil || f.p.ID != id {
		return nil, nil
	}
	return f.p, nil
}

type fakeSources struct {
	src  *domain.ProductSource
	seen []string
}

func (f *fakeSources) Upsert(context.Context, *domain.ProductSource) (string, error) {
	return "src1", nil
}

// The port grew image methods; this fake answers "no pictures" so the tests
// about the FILE stay about the file.
func (f *fakeSources) AddImage(context.Context, *domain.ProductSourceImage) error { return nil }

func (f *fakeSources) ListImages(
	context.Context, string, string, *int,
) ([]*domain.ProductSourceImage, error) {
	return nil, nil
}

func (f *fakeSources) GetByID(_ context.Context, id, accountID string) (*domain.ProductSource, error) {
	f.seen = append(f.seen, id+"|"+accountID)
	if f.src == nil || f.src.ID != id || f.src.AccountID != accountID {
		return nil, nil
	}
	return f.src, nil
}

type fakeSigner struct {
	path   string
	expiry time.Duration
	err    error
}

func (f *fakeSigner) GenerateReadSignedURL(_ context.Context, path string, expiry time.Duration) (string, error) {
	f.path, f.expiry = path, expiry
	if f.err != nil {
		return "", f.err
	}
	return "https://signed.example/" + path, nil
}

func linkFixture() (*fakeProductGetter, *fakeSources, *fakeSigner) {
	return &fakeProductGetter{p: &domain.Product{
			ID: "p1", AccountID: "acc1", Name: "Incubadora",
			SourceID: "src1", SourcePage: 4,
		}},
		&fakeSources{src: &domain.ProductSource{
			ID: "src1", AccountID: "acc1", Filename: "Line Duo G.pdf",
			ObjectPath: "accounts/acc1/product-imports/01ABC.pdf", Pages: 12,
		}},
		&fakeSigner{}
}

func TestProductSourceLink_ReturnsTheBrochureAndThePage(t *testing.T) {
	products, sources, signer := linkFixture()
	// The repo port is wider than this use case needs; the narrow fake stands
	// in through the same interface the production repo satisfies.
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: signer}

	link, err := uc.Execute(context.Background(), "acc1", "p1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// The name the person recognises, never the object path.
	if link.Filename != "Line Duo G.pdf" {
		t.Fatalf("filename = %q", link.Filename)
	}
	if link.Page != 4 || link.Pages != 12 {
		t.Fatalf("page %d of %d, want 4 of 12", link.Page, link.Pages)
	}
	if signer.path != "accounts/acc1/product-imports/01ABC.pdf" {
		t.Fatalf("signed the wrong object: %s", signer.path)
	}
	// Short-lived: a link for opening a file now, not for pasting into a
	// message. A long-lived object URL is how a private bucket stops being one.
	if signer.expiry != sourceLinkTTL || signer.expiry > time.Hour {
		t.Fatalf("expiry = %s", signer.expiry)
	}
}

func TestProductSourceLink_ProductOfAnotherAccountIsNotFound(t *testing.T) {
	products, sources, signer := linkFixture()
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: signer}

	_, err := uc.Execute(context.Background(), "someone-else", "p1")
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("got %v, want ErrProductNotFound", err)
	}
	// And it must not have gone looking for the file at all.
	if len(sources.seen) != 0 {
		t.Fatalf("read the source of another account's product: %v", sources.seen)
	}
	if signer.path != "" {
		t.Fatal("signed a URL for another account's file")
	}
}

func TestProductSourceLink_CSVProductHasNoBrochure(t *testing.T) {
	products, sources, signer := linkFixture()
	products.p.SourceID = "" // imported from a spreadsheet, or typed by hand
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: signer}

	_, err := uc.Execute(context.Background(), "acc1", "p1")
	if !errors.Is(err, ErrProductSourceNotFound) {
		t.Fatalf("got %v, want ErrProductSourceNotFound", err)
	}
}

// A source id pointing at a row that is gone must read as "no brochure", not
// as a crash: the archive is best-effort and the product survives it.
func TestProductSourceLink_MissingRowDegradesToNotFound(t *testing.T) {
	products, sources, signer := linkFixture()
	sources.src = nil
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: signer}

	if _, err := uc.Execute(context.Background(), "acc1", "p1"); !errors.Is(err, ErrProductSourceNotFound) {
		t.Fatalf("got %v, want ErrProductSourceNotFound", err)
	}
}
