package domain

import "strings"

// Provider/model pairing.
//
// The two fields are stored side by side and were never checked against each
// other. On 2026-09-09 four of the six agents in production carried
// `provider = google` with `model = gpt-4o` — an OpenAI model name on a
// Google provider, a combination that cannot answer anything. Nothing
// validated it on save, nothing surfaced it on the screen, and the owner of
// the account found out when the agent went quiet.

// AIProviderGoogle and AIProviderOpenAI are the two provider families. The
// factory accepts "gemini" as a synonym for google, so both are normalized
// here rather than at each call site.
const (
	AIProviderGoogle = "google"
	AIProviderOpenAI = "openai"
)

// ErrModelProviderMismatch is a model name that belongs to another provider.
var ErrModelProviderMismatch = &AppError{
	Code:    "MODEL_PROVIDER_MISMATCH",
	Message: "this model belongs to another provider",
}

// NormalizeAIProvider folds the accepted spellings to one name. An unknown
// value comes back as given, so the caller can refuse it on its own terms.
func NormalizeAIProvider(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "google", "gemini":
		return AIProviderGoogle
	case "openai":
		return AIProviderOpenAI
	}
	return strings.ToLower(strings.TrimSpace(p))
}

// ProviderForModel names the provider a model belongs to, or "" when the
// name says nothing recognisable.
//
// Prefix matching, deliberately: model names carry versions and dates
// ("gemini-3.7-flash", "gpt-4o-2024-08-06") and an exact list would go stale
// on the vendor's next release, which is precisely when a wrong pairing gets
// introduced.
func ProviderForModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(m, "gemini"), strings.HasPrefix(m, "text-embedding-00"):
		return AIProviderGoogle
	case strings.HasPrefix(m, "gpt"), strings.HasPrefix(m, "o1"),
		strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "text-embedding-3"):
		return AIProviderOpenAI
	}
	return ""
}

// ValidateModelForProvider refuses a pairing that cannot work.
//
// An unrecognised model name PASSES. A new release, a fine-tune or a
// self-hosted alias must not be blocked by a list this file cannot keep up
// with — the check exists to catch the pairing that is provably wrong, not
// to be an allow-list. Empty values pass too: they mean "use the default",
// which is resolved elsewhere.
func ValidateModelForProvider(provider, model string) error {
	p := NormalizeAIProvider(provider)
	if p == "" || strings.TrimSpace(model) == "" {
		return nil
	}
	belongs := ProviderForModel(model)
	if belongs == "" || belongs == p {
		return nil
	}
	return ErrModelProviderMismatch
}

// Default models per provider.
//
// They live next to the pairing rule because that is the invariant they have
// to satisfy: a default model must belong to the provider it defaults for.
// Production had them as two independent constants — provider defaulted to
// openai, model defaulted to gpt-4o — so an agent created as google with no
// model named inherited an OpenAI model and could never answer.
const (
	DefaultGoogleModel = "gemini-3.7-flash"
	DefaultOpenAIModel = "gpt-4o"
)

// DefaultModelFor names the model to use when the caller named none.
// An unknown provider gets "", which leaves the choice to the layer that
// knows what that provider is.
func DefaultModelFor(provider string) string {
	switch NormalizeAIProvider(provider) {
	case AIProviderGoogle:
		return DefaultGoogleModel
	case AIProviderOpenAI:
		return DefaultOpenAIModel
	}
	return ""
}
