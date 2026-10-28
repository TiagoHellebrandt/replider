package email

import (
	"strings"
	"testing"
)

// US-018 review fixes: the customer-facing transactional composers
// (verification, reset, welcome, invite, usage warning/limit, payment
// failed) take a locale and resolve copy through tr() instead of
// hardcoding pt-BR. These tests pin (a) that every key set exists in
// BOTH locale maps with distinct copy, and (b) that the composers
// accept + resolve the locale without erroring in stub mode.
func TestLocalizedTransactionalCopy_PtEnParity(t *testing.T) {
	keys := []string{
		"verify_subject", "verify_title", "verify_intro", "verify_cta",
		"reset_subject", "reset_title", "reset_intro", "reset_cta",
		"welcome_subject", "welcome_title", "welcome_intro", "welcome_cta",
		"invite_subject", "invite_title", "invite_intro_template", "invite_cta",
		"usage_warning_subject_template", "usage_warning_title", "usage_warning_intro_template", "usage_warning_cta",
		"usage_limit_subject", "usage_limit_title", "usage_limit_intro_template", "usage_limit_cta",
		"payment_failed_subject", "payment_failed_title", "payment_failed_intro_template", "payment_failed_cta",
	}
	for _, key := range keys {
		pt, en := stringsPtBR[key], stringsEn[key]
		if pt == "" || en == "" {
			t.Errorf("key %q must exist in both locales: pt=%q en=%q", key, pt, en)
			continue
		}
		if pt == en {
			t.Errorf("key %q: pt and en copy should differ, both %q", key, pt)
		}
	}
}

func TestLocalizedComposers_AcceptEnLocale(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	base := "https://app.replider.com"

	sends := map[string]func() error{
		"reset":          func() error { return adapter.SendPasswordResetEmail("u@x.com", "tok", base, "en") },
		"verify":         func() error { return adapter.SendVerificationEmail("u@x.com", "tok", base, "en") },
		"welcome":        func() error { return adapter.SendWelcomeEmail("u@x.com", "Ann", base, "en") },
		"usage_warning":  func() error { return adapter.SendUsageWarningEmail("u@x.com", "Ann", 80, "Pro", base, "en") },
		"usage_limit":    func() error { return adapter.SendUsageLimitEmail("u@x.com", "Ann", "Pro", base, "en") },
		"payment_failed": func() error { return adapter.SendPaymentFailedEmail("u@x.com", "Ann", "Pro", base, "en") },
		"invite":         func() error { return adapter.SendTeamInviteEmail("u@x.com", "Boss", "tok", base, "en") },
	}
	for name, send := range sends {
		if err := send(); err != nil {
			t.Errorf("%s composer errored with en locale: %v", name, err)
		}
	}
}

// resolveLocale is the single fallback point: en variants map to en,
// everything else (including empty) stays pt-BR.
func TestResolveLocale_EnVariants(t *testing.T) {
	for _, raw := range []string{"en", "EN", "en-US", "en-us", "en-GB"} {
		if got := resolveLocale(raw); got != LocaleEn {
			t.Errorf("resolveLocale(%q) = %q, want en", raw, got)
		}
	}
	for _, raw := range []string{"", "pt-BR", "pt", "es", "de-DE"} {
		if got := resolveLocale(raw); got != LocalePtBR {
			t.Errorf("resolveLocale(%q) = %q, want pt-BR", raw, got)
		}
	}
}

// The en subject actually reads as English — guards against someone
// wiring the en map to pt copy.
func TestEnSubjectsReadEnglish(t *testing.T) {
	if s := tr(LocaleEn, "verify_subject"); !strings.Contains(s, "Confirm") {
		t.Errorf("en verify subject should be English, got %q", s)
	}
	if s := tr(LocaleEn, "payment_failed_subject"); !strings.Contains(s, "Payment") {
		t.Errorf("en payment-failed subject should be English, got %q", s)
	}
}
