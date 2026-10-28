package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// "Manda uma foto do equipamento" is a question the distributor already has
// the answer to — it is on page 4 of the PDF they handed us. These pin that
// the pictures reach the product, and the three ways the job must degrade
// without ever costing the catalogue.

type imageSources struct {
	fakeSources
	added     []*domain.ProductSourceImage
	byPage    map[int][]*domain.ProductSourceImage
	pageAsked []string // "4" or "all", in order
	addErr    error
}

func (s *imageSources) AddImage(_ context.Context, img *domain.ProductSourceImage) error {
	if s.addErr != nil {
		return s.addErr
	}
	s.added = append(s.added, img)
	return nil
}

func (s *imageSources) ListImages(
	_ context.Context, _, _ string, page *int,
) ([]*domain.ProductSourceImage, error) {
	if page == nil {
		s.pageAsked = append(s.pageAsked, "all")
		var all []*domain.ProductSourceImage
		for _, v := range s.byPage {
			all = append(all, v...)
		}
		return all, nil
	}
	s.pageAsked = append(s.pageAsked, "page")
	return s.byPage[*page], nil
}

type recordingStore struct {
	paths []string
	err   error
}

func (s *recordingStore) UploadBytes(_ context.Context, path string, _ []byte, _ string) error {
	if s.err != nil {
		return s.err
	}
	s.paths = append(s.paths, path)
	return nil
}

func pics() []PDFImage {
	return []PDFImage{
		{Page: 4, Width: 600, Height: 600, ContentType: "image/jpeg", SHA256: strings.Repeat("a", 64), Data: []byte("jpegbytes")},
		{Page: 7, Width: 800, Height: 400, ContentType: "image/png", SHA256: strings.Repeat("b", 64), Data: []byte("pngbytes")},
	}
}

// inlineDispatch runs the detached job synchronously so a test can observe it.
func imageUC(sources *imageSources, store *recordingStore, extract pdfImageExtractor) *ImportProductsPDFUseCase {
	uc := &ImportProductsPDFUseCase{sources: sources, store: store}
	uc.SetImageExtractor(extract)
	uc.SetImageDispatch(func(fn func()) { fn() })
	return uc
}

func TestStoreBrochureImages_UploadsAndRecordsEachPicture(t *testing.T) {
	sources := &imageSources{}
	store := &recordingStore{}
	uc := imageUC(sources, store, func([]byte) ([]PDFImage, error) { return pics(), nil })

	uc.storeBrochureImages(context.Background(), "acc1", "src1", []byte("%PDF"))

	if len(store.paths) != 2 || len(sources.added) != 2 {
		t.Fatalf("uploaded %d, recorded %d, want 2 and 2", len(store.paths), len(sources.added))
	}
	// The object name carries page and hash: legible in a bucket listing and
	// impossible to collide.
	if !strings.Contains(store.paths[0], "accounts/acc1/product-images/src1/p004-") ||
		!strings.HasSuffix(store.paths[0], ".jpg") {
		t.Fatalf("object path is not legible: %s", store.paths[0])
	}
	if !strings.HasSuffix(store.paths[1], ".png") {
		t.Fatalf("png stored under the wrong extension: %s", store.paths[1])
	}
	got := sources.added[0]
	if got.Page != 4 || got.Width != 600 || got.ContentType != "image/jpeg" || got.ByteSize == 0 {
		t.Fatalf("row lost the picture's facts: %+v", got)
	}
}

// The catalogue is the point; pictures are a convenience on top of it. A
// bucket that refuses must not cost rows, and must not leave the caller
// believing pictures exist.
func TestStoreBrochureImages_UploadFailureCostsNoRows(t *testing.T) {
	sources := &imageSources{}
	store := &recordingStore{err: errors.New("bucket unreachable")}
	uc := imageUC(sources, store, func([]byte) ([]PDFImage, error) { return pics(), nil })

	uc.storeBrochureImages(context.Background(), "acc1", "src1", []byte("%PDF"))

	if len(sources.added) != 0 {
		t.Fatalf("recorded %d rows for objects that were never stored", len(sources.added))
	}
}

// An extractor that blows up on a malformed document must not take the import
// with it — by the time this runs, the person already has their preview.
func TestStoreBrochureImages_ExtractionFailureIsSurvivable(t *testing.T) {
	sources := &imageSources{}
	store := &recordingStore{}
	uc := imageUC(sources, store, func([]byte) ([]PDFImage, error) {
		return nil, errors.New("not a pdf")
	})

	uc.storeBrochureImages(context.Background(), "acc1", "src1", []byte("junk"))

	if len(store.paths) != 0 || len(sources.added) != 0 {
		t.Fatal("a failed extraction stored something")
	}
}

// Unwired seams are a no-op, not a panic: the picture job is optional.
func TestStoreBrochureImages_UnwiredIsANoOp(t *testing.T) {
	uc := &ImportProductsPDFUseCase{}
	uc.storeBrochureImages(context.Background(), "acc1", "src1", []byte("%PDF"))
}

func TestProductImages_ScopedToThePageTheProductCameFrom(t *testing.T) {
	sources := &imageSources{byPage: map[int][]*domain.ProductSourceImage{
		4: {{ID: "i1", Page: 4, ObjectPath: "accounts/acc1/product-images/src1/p004-a.jpg", Width: 600, Height: 600}},
	}}
	products := &fakeProductGetter{p: &domain.Product{
		ID: "p1", AccountID: "acc1", SourceID: "src1", SourcePage: 4,
	}}
	signer := &fakeSigner{}
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: signer}

	got, err := uc.Images(context.Background(), "acc1", "p1")
	if err != nil {
		t.Fatalf("Images: %v", err)
	}
	if len(got) != 1 || got[0].Page != 4 || got[0].URL == "" {
		t.Fatalf("got %+v", got)
	}
	// The narrow question first — a forty-page catalogue under one product is
	// a scroll, not an answer.
	if len(sources.pageAsked) != 1 || sources.pageAsked[0] != "page" {
		t.Fatalf("asked %v, want the page only", sources.pageAsked)
	}
}

// A page with no artwork of its own falls back to the brochure: a picture in
// the wrong order still beats an empty gallery.
func TestProductImages_FallsBackToTheWholeBrochure(t *testing.T) {
	sources := &imageSources{byPage: map[int][]*domain.ProductSourceImage{
		9: {{ID: "i9", Page: 9, ObjectPath: "p009.jpg"}},
	}}
	products := &fakeProductGetter{p: &domain.Product{
		ID: "p1", AccountID: "acc1", SourceID: "src1", SourcePage: 4,
	}}
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: &fakeSigner{}}

	got, err := uc.Images(context.Background(), "acc1", "p1")
	if err != nil {
		t.Fatalf("Images: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("fallback returned %d images", len(got))
	}
	if len(sources.pageAsked) != 2 || sources.pageAsked[1] != "all" {
		t.Fatalf("asked %v, want the page then the brochure", sources.pageAsked)
	}
}

func TestProductImages_AnotherAccountsProductIsNotFound(t *testing.T) {
	sources := &imageSources{}
	products := &fakeProductGetter{p: &domain.Product{
		ID: "p1", AccountID: "acc1", SourceID: "src1", SourcePage: 4,
	}}
	uc := &ProductSourceLinkUseCase{products: products, sources: sources, signer: &fakeSigner{}}

	_, err := uc.Images(context.Background(), "someone-else", "p1")
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("got %v, want ErrProductNotFound", err)
	}
	if len(sources.pageAsked) != 0 {
		t.Fatal("listed another account's pictures")
	}
}
