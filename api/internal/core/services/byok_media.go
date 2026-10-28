package services

import (
	"context"
	"io"
	"log/slog"

	"github.com/replider/api/internal/adapters/ai"
	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
)

// BYOK media adapters (transcription + vision).
//
// The transcriber and image describer were built once at boot with the
// PLATFORM keys, so every voice note and product photo a customer sent was
// transcribed on our bill regardless of AI_REQUIRE_BYOK. These wrappers
// resolve the account from the context (WithAIAccount — the handlers tag it
// where they hold the channel) and build the adapter on the ACCOUNT's key.
//
// Rules, in order:
//   - platform account → the platform adapter (its own CS agent);
//   - account with a key for the vendor → an adapter on that key;
//   - keyless account with AI_REQUIRE_BYOK on → domain.ErrBYOKKeyMissing
//     (the caller already degrades: "[áudio não transcrito]", no caption);
//   - keyless account with the flag OFF → the platform adapter (pre-flag
//     behaviour, self-serve plans still ride the env key);
//   - no account on the context → platform adapter only when the flag is
//     off; otherwise refused, because "unknown tenant" must not mean "free".
//
// Neither vendor reports token usage for these calls, so nothing lands in
// the ledger here — the same gap the platform adapters always had.

// BYOKTranscriber implements domain.Transcriber per account.
type BYOKTranscriber struct {
	cfg      *config.Config
	accounts domain.AccountRepository
	platform domain.Transcriber // may be nil when no platform key is configured
}

// NewBYOKTranscriber wraps the platform transcriber (nil allowed).
func NewBYOKTranscriber(cfg *config.Config, accounts domain.AccountRepository, platform domain.Transcriber) *BYOKTranscriber {
	return &BYOKTranscriber{cfg: cfg, accounts: accounts, platform: platform}
}

func (t *BYOKTranscriber) Transcribe(ctx context.Context, r io.Reader, languageHint string) (string, error) {
	account, usePlatform, err := resolveMediaAccount(ctx, t.cfg, t.accounts)
	if err != nil {
		return "", err
	}
	if usePlatform {
		if t.platform == nil {
			return "", domain.ErrAIProviderUnavailable
		}
		return t.platform.Transcribe(ctx, r, languageHint)
	}
	// Whisper first (best PT-BR accuracy on noisy audio), Gemini otherwise —
	// the same preference order main.go applies to the platform adapters.
	if key := derefStringPtr(account.OpenAIAPIKey); key != "" {
		return ai.NewWhisperTranscriber(key).Transcribe(ctx, r, languageHint)
	}
	if key := derefStringPtr(account.GeminiAPIKey); key != "" {
		gt, err := ai.NewGeminiTranscriber(ctx, key, "")
		if err != nil {
			return "", err
		}
		return gt.Transcribe(ctx, r, languageHint)
	}
	if account.PlatformKeyAllowed && t.platform != nil {
		return t.platform.Transcribe(ctx, r, languageHint)
	}
	return "", domain.ErrBYOKKeyMissing
}

// BYOKImageDescriber implements domain.ImageDescriber per account.
type BYOKImageDescriber struct {
	cfg      *config.Config
	accounts domain.AccountRepository
	platform domain.ImageDescriber // may be nil
}

// NewBYOKImageDescriber wraps the platform describer (nil allowed).
func NewBYOKImageDescriber(cfg *config.Config, accounts domain.AccountRepository, platform domain.ImageDescriber) *BYOKImageDescriber {
	return &BYOKImageDescriber{cfg: cfg, accounts: accounts, platform: platform}
}

func (d *BYOKImageDescriber) Describe(ctx context.Context, imageURL, language string) (string, error) {
	account, usePlatform, err := resolveMediaAccount(ctx, d.cfg, d.accounts)
	if err != nil {
		return "", err
	}
	if usePlatform {
		if d.platform == nil {
			return "", domain.ErrAIProviderUnavailable
		}
		return d.platform.Describe(ctx, imageURL, language)
	}
	// Vision runs on OpenAI only (gpt-4o-mini class); a Gemini-only BYOK
	// account gets no caption rather than a caption on our key.
	if key := derefStringPtr(account.OpenAIAPIKey); key != "" {
		return ai.NewOpenAIImageDescriber(key).Describe(ctx, imageURL, language)
	}
	if account.PlatformKeyAllowed && d.platform != nil {
		return d.platform.Describe(ctx, imageURL, language)
	}
	return "", domain.ErrBYOKKeyMissing
}

// resolveMediaAccount applies the rules in the file comment. usePlatform is
// true when the platform adapter must serve the call.
func resolveMediaAccount(ctx context.Context, cfg *config.Config, accounts domain.AccountRepository) (account *domain.Account, usePlatform bool, err error) {
	requireBYOK := cfg != nil && cfg.AIRequireBYOK
	accountID := AIAccountFrom(ctx)
	if accountID == "" || accounts == nil {
		if requireBYOK {
			slog.WarnContext(ctx, "byok media: no account on context; refusing the platform key")
			return nil, false, domain.ErrBYOKKeyMissing
		}
		return nil, true, nil
	}
	if cfg != nil && cfg.IsPlatformAccount(accountID) {
		return nil, true, nil
	}
	account, lookupErr := accounts.GetByID(ctx, accountID)
	if lookupErr != nil || account == nil {
		if requireBYOK {
			return nil, false, domain.ErrBYOKKeyMissing
		}
		return nil, true, nil
	}
	if !requireBYOK && derefStringPtr(account.OpenAIAPIKey) == "" && derefStringPtr(account.GeminiAPIKey) == "" {
		// Flag off and no key of their own: the pre-flag behaviour.
		return account, true, nil
	}
	return account, false, nil
}
