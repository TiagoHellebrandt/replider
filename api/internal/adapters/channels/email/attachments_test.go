package email

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replider/api/internal/adapters/httpsafe"
)

// domain.Message already carried a MediaURL, and WhatsApp, Telegram and
// Instagram all sent it. Email read Content and nothing else, so "manda uma
// foto do equipamento" on an email thread had no answer: the picture existed,
// was one click from the product screen, and could not travel.

func TestBuildMIME_PlainWhenNothingIsAttached(t *testing.T) {
	raw := string(buildMIME(smtpConfig{FromAddress: "eu@x.com"}, "a@b.com",
		"Assunto", "corpo", mimeThreading{messageID: "<1@x>"}))

	if strings.Contains(raw, "multipart/mixed") {
		t.Fatal("wrapped a plain message in a multipart envelope for nothing")
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("body part lost its type:\n%s", raw)
	}
}

func TestBuildMIME_AttachesTheFile(t *testing.T) {
	raw := string(buildMIME(smtpConfig{FromAddress: "eu@x.com"}, "a@b.com",
		"Assunto", "segue a foto", mimeThreading{messageID: "<1@x>"},
		attachment{Filename: "p004-9f2c1a.jpg", ContentType: "image/jpeg", Data: []byte("bytes")}))

	for _, want := range []string{
		"multipart/mixed; boundary=",
		"Content-Type: image/jpeg",
		"Content-Transfer-Encoding: base64",
		`filename="p004-9f2c1a.jpg"`,
		"segue a foto",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("missing %q in:\n%s", want, raw)
		}
	}
	// The envelope has to close, or the whole message is malformed.
	if strings.Count(raw, "--replider-") < 3 {
		t.Fatalf("multipart envelope not closed:\n%s", raw)
	}
}

// RFC 2045 caps a line at 78 and servers enforce it: an unwrapped base64 blob
// comes back as a bounce.
func TestBuildMIME_WrapsBase64(t *testing.T) {
	raw := string(buildMIME(smtpConfig{FromAddress: "eu@x.com"}, "a@b.com", "s", "b",
		mimeThreading{messageID: "<1@x>"},
		attachment{Filename: "big.jpg", ContentType: "image/jpeg",
			Data: []byte(strings.Repeat("x", 5000))}))

	for _, line := range strings.Split(raw, "\r\n") {
		if len(line) > 78 {
			t.Fatalf("line of %d chars: %.90s", len(line), line)
		}
	}
}

func TestFetchAttachment_ReadsTheFileAndItsName(t *testing.T) {
	// The fetch goes through the guarded client, which refuses internal
	// addresses — and httptest serves on loopback. Opening the door for the
	// length of the test is the point: in production the URL is a GCS signed
	// link or our own media proxy, both public, and the guard staying shut for
	// everything else is exactly what we want.
	allowLoopback(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpegbytes"))
	}))
	defer srv.Close()

	got := fetchAttachment(context.Background(), srv.URL+"/p004-9f2c1a.jpg?sig=abc")
	if got == nil {
		t.Fatal("nil")
	}
	if got.ContentType != "image/jpeg" || string(got.Data) != "jpegbytes" {
		t.Fatalf("got %+v", got)
	}
	// The recipient sees this in their mail client — never the signed URL's
	// query string.
	if got.Filename != "p004-9f2c1a.jpg" {
		t.Fatalf("filename = %q", got.Filename)
	}
}

// Every failure returns nil and the message still goes: a reply that never
// arrives is worse than one missing its attachment.
func TestFetchAttachment_FailuresAreNil(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFound.Close()

	allowLoopback(t)
	cases := map[string]string{
		"empty":     "",
		"not a url": "://nonsense",
		"404":       notFound.URL + "/x.jpg",
	}
	for name, url := range cases {
		t.Run(name, func(t *testing.T) {
			if got := fetchAttachment(context.Background(), url); got != nil {
				t.Fatalf("got %+v, want nil", got)
			}
		})
	}
}

func TestAttachmentName_FallsBackToATypeNotAQueryString(t *testing.T) {
	if got := attachmentName("https://x/objeto?X-Goog-Signature=abc", "image/png"); !strings.HasPrefix(got, "anexo") {
		t.Fatalf("got %q", got)
	}
	if got := attachmentName("https://x/foto.png?sig=1", "image/png"); got != "foto.png" {
		t.Fatalf("got %q", got)
	}
}

// allowLoopback lets the SSRF-guarded client reach httptest for one test.
func allowLoopback(t *testing.T) {
	t.Helper()
	httpsafe.SetAllowPrivateTargets(true)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(false) })
}
