package services

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math/rand"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// A manufacturer's brochure is full of pictures that are not OF anything: the
// brand mark on every page, bullet glyphs, rules, gradients. Shipping those
// makes a gallery nobody can use, so these pin the filters — and pin them
// against a real PDF built from real JPEGs, not a mock of the library.

// noiseJPEG encodes a w×h image of random pixels. Random on purpose: a solid
// colour compresses to almost nothing, so it would fail the size floor for a
// reason the test did not intend to exercise.
func noiseJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(1))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)),
				B: uint8(rng.Intn(256)), A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

// brochureWith builds a real PDF carrying one image per page, so the filters
// are exercised against pdfcpu's actual output rather than a mock of it.
func brochureWith(t *testing.T, images [][]byte) []byte {
	t.Helper()
	readers := make([]io.Reader, 0, len(images))
	for _, b := range images {
		readers = append(readers, bytes.NewReader(b))
	}
	var out bytes.Buffer
	// nil source: import into a new document rather than onto an existing one.
	if err := api.ImportImages(nil, &out, readers,
		pdfcpu.DefaultImportConfig(), model.NewDefaultConfiguration()); err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return out.Bytes()
}

func TestExtractPDFImages_KeepsPhotographsAndDropsMarks(t *testing.T) {
	product := noiseJPEG(t, 600, 600) // a product shot
	logo := noiseJPEG(t, 80, 80)      // a brand mark, too small to be a product

	got, err := ExtractPDFImages(brochureWith(t, [][]byte{product, logo}))
	if err != nil {
		t.Fatalf("ExtractPDFImages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("kept %d images, want 1 (the mark must be dropped)", len(got))
	}
	img := got[0]
	if img.Width < minImageSide || img.Height < minImageSide {
		t.Fatalf("kept an image below the floor: %dx%d", img.Width, img.Height)
	}
	if img.ContentType != "image/jpeg" {
		t.Fatalf("content type = %q", img.ContentType)
	}
	if img.Page < 1 {
		t.Fatalf("page = %d, want the 1-based page it was read from", img.Page)
	}
	if len(img.Data) == 0 || img.SHA256 == "" {
		t.Fatal("image carries no bytes or no hash")
	}
}

// The same brand mark on forty pages is one picture, not forty. Here: the same
// photograph placed on two pages.
func TestExtractPDFImages_DeduplicatesRepeatedArtwork(t *testing.T) {
	shot := noiseJPEG(t, 600, 600)

	got, err := ExtractPDFImages(brochureWith(t, [][]byte{shot, shot}))
	if err != nil {
		t.Fatalf("ExtractPDFImages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("kept %d copies of one image", len(got))
	}
}

func TestExtractPDFImages_EmptyDocumentIsAnError(t *testing.T) {
	if _, err := ExtractPDFImages(nil); err == nil {
		t.Fatal("empty input must not read as a brochure with no pictures")
	}
}

// A document that is not a PDF must fail loudly rather than answer "no
// images" — the caller logs the difference.
func TestExtractPDFImages_GarbageIsAnError(t *testing.T) {
	if _, err := ExtractPDFImages([]byte("this is not a pdf")); err == nil {
		t.Fatal("garbage read as a valid brochure")
	}
}
