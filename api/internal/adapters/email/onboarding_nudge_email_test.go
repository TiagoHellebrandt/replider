package email

import "testing"

// The six sales milestones the nudges ask for — mirrors
// usecases.onboardingMilestoneOrder (the adapter must not import usecases).
var nudgeMilestones = []string{"channel", "products", "customers", "proposal", "first_inbound", "team_invited"}

func TestSendOnboardingNudgeEmail_StubModeAllMilestones(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	for _, m := range nudgeMilestones {
		if err := adapter.SendOnboardingNudgeEmail("admin@example.com", "Maria Silva", m,
			"https://app.replider.com/settings/channels?utm_campaign=onboarding_nudge", ""); err != nil {
			t.Fatalf("milestone %s: expected no error in stub mode, got: %v", m, err)
		}
	}
}

func TestSendOnboardingNudgeEmail_RendersCopyBothLocales(t *testing.T) {
	// Every milestone must resolve non-empty copy in both locales so the
	// email never renders blank fields.
	for _, loc := range []Locale{LocalePtBR, LocaleEn} {
		for _, m := range nudgeMilestones {
			prefix := onboardingNudgeKeyPrefix(m)
			if prefix != "onboarding_nudge_"+m {
				t.Fatalf("milestone %s maps to %s", m, prefix)
			}
			for _, suffix := range []string{"_subject", "_title", "_intro", "_body", "_cta"} {
				if got := tr(loc, prefix+suffix); got == "" {
					t.Fatalf("%v %s%s must not be empty", loc, prefix, suffix)
				}
			}
		}
		if got := tr(loc, "onboarding_nudge_footnote"); got == "" {
			t.Fatalf("%v onboarding_nudge_footnote must not be empty", loc)
		}
	}
	// The retired v1 copy (knowledge base, "share your agent") is gone: those
	// screens no longer exist and the links pointed at them.
	for _, dead := range []string{"onboarding_nudge_knowledge_subject", "onboarding_nudge_share_subject"} {
		if got := tr(LocalePtBR, dead); got != "" && got != dead {
			t.Fatalf("%s should no longer have copy, got %q", dead, got)
		}
	}
}

func TestSendOnboardingNudgeEmail_UnknownMilestoneFallsBackToTheFirstStep(t *testing.T) {
	if got := onboardingNudgeKeyPrefix("knowledge"); got != "onboarding_nudge_channel" {
		t.Fatalf("unknown milestone should render the channel copy, got %s", got)
	}
}

func TestSendOnboardingNudgeEmail_EnglishLocale(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	if err := adapter.SendOnboardingNudgeEmail("admin@example.com", "John",
		"products", "https://app.replider.com/sales/products/import", "en"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
