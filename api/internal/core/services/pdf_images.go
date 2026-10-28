package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg" // registers the decoder DecodeConfig needs
	_ "image/png"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// Pulling the pictures out of a manufacturer's brochure.
//
// A distributor answering "manda uma foto do equipamento" has the photo — it
// is on page 4 of the PDF they already gave us. Extraction lives here, beside
// PDFPageCount, so the use case stays free of the pdf library import.

// ExtractedImage is one picture, in the bytes the PDF stored it as. No
// re-encoding: a JPEG in the document comes out a JPEG, which keeps the file
// identical to what the manufacturer published.
type ExtractedImage struct {
	Page        int
	Width       int
	Height      int
	ContentType string
	SHA256      string
	Data        []byte
}

// The filters. A brochure is full of pictures that are not OF anything: the
// brand's logo on every page, bullet glyphs, rule lines, background gradients.
// Shipping those makes a gallery nobody can use, so the cheap structural
// signals rule them out before anything is stored.
const (
	// Below this on either side it is a mark, not a photograph. Product shots
	// in a print-ready brochure are far larger.
	minImageSide = 200
	// A 200x200 photograph is tens of KB; anything under this is flat colour.
	minImageBytes = 8 << 10
	// One page's illustration should not be an 8MB scan. Above this we skip
	// rather than truncate: half a picture is worse than none.
	maxImageBytes = 8 << 20
	// A whole catalogue's worth. Past this the gallery is a scroll, and the
	// storage is real money for pictures nobody opens.
	maxImagesPerPDF = 40
)

// ExtractPDFImages returns the pictures worth keeping, in page order.
//
// Deliberately best-effort per image: a brochure with one image in a codec
// pdfcpu cannot decode must still yield the other thirty-nine.
func ExtractPDFImages(data []byte) ([]ExtractedImage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("pdf images: empty document")
	}
	out := make([]ExtractedImage, 0, 16)
	// The same logo on forty pages is one picture, not forty.
	seen := map[string]bool{}

	digest := func(img model.Image, _ bool, _ int) error {
		if len(out) >= maxImagesPerPDF {
			return nil
		}
		// A mask or a thumbnail is machinery, never the product.
		if img.IsImgMask || img.Thumb {
			return nil
		}
		if img.Reader == nil {
			return nil
		}
		raw, err := io.ReadAll(io.LimitReader(img.Reader, maxImageBytes+1))
		if err != nil {
			// One unreadable image must not lose the rest.
			return nil
		}
		if len(raw) < minImageBytes || len(raw) > maxImageBytes {
			return nil
		}
		// Dimensions come from the BYTES, not from the library's struct:
		// pdfcpu leaves Width/Height at zero on this extraction path, and
		// trusting them dropped every image in the document. DecodeConfig
		// reads the header only, so this stays cheap.
		//
		// It also decides the type from the real content rather than from a
		// filename-ish string — and a format we cannot measure (tiff, jpx) is
		// skipped rather than stored at an unknown size.
		cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			return nil
		}
		if cfg.Width < minImageSide || cfg.Height < minImageSide {
			return nil
		}
		ct := imageContentType(format)
		if ct == "" {
			return nil
		}
		sum := sha256.Sum256(raw)
		hash := hex.EncodeToString(sum[:])
		if seen[hash] {
			return nil
		}
		seen[hash] = true

		out = append(out, ExtractedImage{
			Page:        img.PageNr,
			Width:       cfg.Width,
			Height:      cfg.Height,
			ContentType: ct,
			SHA256:      hash,
			Data:        raw,
		})
		return nil
	}

	conf := model.NewDefaultConfiguration()
	// Reading someone else's file: never let a malformed one abort the whole
	// import, and never let it write anything.
	conf.ValidationMode = model.ValidationRelaxed
	if err := api.ExtractImages(bytes.NewReader(data), nil, digest, conf); err != nil {
		return nil, fmt.Errorf("pdf images: %w", err)
	}
	return out, nil
}

// imageContentType maps the decoded format to a MIME type. Unknown formats are
// refused rather than guessed: a wrong Content-Type on a signed URL is a file
// the browser will not render and nobody can explain.
func imageContentType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return ""
	}
}
