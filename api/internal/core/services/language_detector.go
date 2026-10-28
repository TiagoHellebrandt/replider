package services

import (
	"strings"
	"sync"

	lingua "github.com/pemistahl/lingua-go"
)

// DetectReplyLanguage returns the BCP-47-ish tag the AI should reply in,
// based on the customer's last message. Restricted to the languages the
// product supports — Portuguese, English, Spanish — so lingua's
// confidence ranking doesn't get diluted across dozens of languages we
// don't actually target.
//
// Falls back to "pt" when the input is too short to be reliable or when
// lingua returns no confident match. The fallback matches the Brazilian
// market focus; flip it when the account-default-language setting ships.
func DetectReplyLanguage(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return "pt"
	}
	if countWords(trimmed) < 4 {
		return "pt"
	}
	lang, ok := detector().DetectLanguageOf(trimmed)
	if !ok {
		return "pt"
	}
	switch lang {
	case lingua.English:
		return "en"
	case lingua.Spanish:
		return "es"
	case lingua.Portuguese:
		return "pt"
	default:
		return "pt"
	}
}

// detector builds the lingua detector once per process — building it
// allocates ~50MB of language models and takes ~100ms, way too much
// for a per-request cost.
var (
	detectorOnce     sync.Once
	detectorInstance lingua.LanguageDetector
)

func detector() lingua.LanguageDetector {
	detectorOnce.Do(func() {
		detectorInstance = lingua.NewLanguageDetectorBuilder().
			FromLanguages(lingua.English, lingua.Portuguese, lingua.Spanish).
			Build()
	})
	return detectorInstance
}

func countWords(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}
