package email

import (
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

func sampleDigest() domain.WeeklyDigest {
	return domain.WeeklyDigest{
		Stats: domain.WeeklyDigestStats{
			MessagesHandled:  128,
			TimeSavedMinutes: 256,
			ConversionPct:    73,
			TopFAQs: []domain.WeeklyDigestFAQ{
				{Question: "qual o horário de funcionamento?", Count: 14},
				{Question: "vocês fazem entrega?", Count: 9},
			},
		},
		PeriodStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
	}
}

// TestSendWeeklyDigestEmail_StubMode is the US-176 "test renders for a
// sample account" acceptance check: rendering a populated digest for a
// sample account must not error in stub mode.
func TestSendWeeklyDigestEmail_StubMode(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	err := adapter.SendWeeklyDigestEmail("admin@example.com", "Maria Silva", sampleDigest(),
		"https://app.replider.com/home", "https://app.replider.com/settings", "")
	if err != nil {
		t.Fatalf("expected no error in stub mode, got: %v", err)
	}
}

func TestDigestBodyHTML_RendersMetricsAndFAQs(t *testing.T) {
	html := digestBodyHTML(LocalePtBR, sampleDigest().Stats)

	for _, want := range []string{"128", "256 min", "73%", "qual o horário de funcionamento?", "(14)"} {
		if !strings.Contains(html, want) {
			t.Errorf("digest HTML should contain %q\n--- got ---\n%s", want, html)
		}
	}
}

func TestDigestBodyHTML_NoActivityFallback(t *testing.T) {
	empty := domain.WeeklyDigestStats{}
	html := digestBodyHTML(LocalePtBR, empty)
	if !strings.Contains(html, tr(LocalePtBR, "digest_no_activity")) {
		t.Errorf("empty-week digest should render the no-activity note, got:\n%s", html)
	}
}

func TestDigestPlainText_MirrorsStats(t *testing.T) {
	text := digestPlainText(LocaleEn, sampleDigest().Stats, "https://app.replider.com/home")
	for _, want := range []string{"128", "256 min", "73%", "vocês fazem entrega?", "https://app.replider.com/home"} {
		if !strings.Contains(text, want) {
			t.Errorf("digest plain text should contain %q\n--- got ---\n%s", want, text)
		}
	}
}

func TestSendWeeklyDigestEmail_EnglishLocale(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	// English locale path must resolve without falling back to empty copy.
	if got := tr(LocaleEn, "digest_subject"); got == "" {
		t.Fatal("en digest_subject must not be empty")
	}
	if err := adapter.SendWeeklyDigestEmail("admin@example.com", "John", sampleDigest(),
		"https://app.replider.com/home", "", "en"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
