package services

import "testing"

// plans.ai_model was decorative: buildProvider hardcoded the model string, so
// every account ran gemini-2.5-flash no matter what its plan said — and the
// cost meter then labelled that spend with the plan's model, one that never
// ran. These pin that the requested model is the one used.
func TestResolveModel_UsesWhatWasAsked(t *testing.T) {
	for _, tc := range []struct{ provider, requested, want string }{
		{"gemini", "gemini-3.1-pro", "gemini-3.1-pro"},
		{"google", "gemini-3.1-flash-lite", "gemini-3.1-flash-lite"},
		{"openai", "gpt-5", "gpt-5"},
		{"gemini", "  gemini-3.1-pro  ", "gemini-3.1-pro"},
	} {
		if got := resolveModel(tc.provider, tc.requested); got != tc.want {
			t.Errorf("resolveModel(%q,%q) = %q; quero %q", tc.provider, tc.requested, got, tc.want)
		}
	}
}

// Nothing asked for: fall back per provider, so an unconfigured install and
// every existing test keep working unchanged.
func TestResolveModel_FallsBackPerProvider(t *testing.T) {
	if got := resolveModel("openai", ""); got != defaultOpenAIModel {
		t.Errorf("openai sem modelo = %q; quero %q", got, defaultOpenAIModel)
	}
	for _, p := range []string{"gemini", "google", "", "desconhecido"} {
		if got := resolveModel(p, "   "); got != defaultGeminiModel {
			t.Errorf("%q sem modelo = %q; quero %q", p, got, defaultGeminiModel)
		}
	}
}
