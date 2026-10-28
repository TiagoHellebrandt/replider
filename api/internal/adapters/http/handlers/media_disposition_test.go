package handlers

import "testing"

// The attack this exists to stop: a file arrives from a stranger declaring a
// type the browser will EXECUTE, and the proxy echoes it back on our own
// domain.
func TestMediaDisposition_ExecutableTypesNeverRenderInline(t *testing.T) {
	dangerous := []string{
		"text/html",
		"text/html; charset=utf-8",
		"TEXT/HTML",
		"application/xhtml+xml",
		// An image to every mime table, a script host to every browser. This
		// is why the image allowlist is explicit and not a "image/" prefix.
		"image/svg+xml",
		"application/xml",
		"text/xml",
		"application/javascript",
		"text/javascript",
		"application/x-shockwave-flash",
		"text/vbscript",
	}
	for _, ct := range dangerous {
		served, inline := mediaDisposition(ct)
		if inline {
			t.Errorf("%q was allowed to render inline", ct)
		}
		if served != "application/octet-stream" {
			t.Errorf("%q was echoed back as %q instead of being neutralised", ct, served)
		}
	}
}

func TestMediaDisposition_TheEverydayTypesStillOpen(t *testing.T) {
	fine := []string{
		"image/jpeg", "image/png", "image/gif", "image/webp",
		"application/pdf", "text/plain",
		"audio/mpeg", "audio/ogg", "video/mp4",
	}
	for _, ct := range fine {
		served, inline := mediaDisposition(ct)
		if !inline {
			t.Errorf("%q should render inline — the seller opens these all day", ct)
		}
		if served != ct {
			t.Errorf("%q came back as %q", ct, served)
		}
	}
}

// Case and parameters are attacker-supplied too. The decision is made on the
// normalised essence, and the parameters are dropped rather than echoed.
func TestMediaDisposition_NormalisesBeforeDeciding(t *testing.T) {
	for _, ct := range []string{"IMAGE/JPEG", "image/jpeg; charset=utf-8", "  image/jpeg  "} {
		served, inline := mediaDisposition(ct)
		if !inline || served != "image/jpeg" {
			t.Errorf("%q → (%q, %v); wanted (image/jpeg, true)", ct, served, inline)
		}
	}
}

// An absent or nonsense type is a file, not a document.
func TestMediaDisposition_UnknownAndEmptyDownload(t *testing.T) {
	for _, ct := range []string{"", "   ", "application/octet-stream", "application/zip", "garbage", ";"} {
		served, inline := mediaDisposition(ct)
		if inline {
			t.Errorf("%q was allowed inline", ct)
		}
		if served != "application/octet-stream" {
			t.Errorf("%q came back as %q", ct, served)
		}
	}
}
