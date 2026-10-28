package email

import "testing"

func TestSendReactivationEmail_StubMode(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	err := adapter.SendReactivationEmail("admin@example.com", "Maria Silva", 7,
		"https://app.replider.com/login?utm_campaign=reactivation", "")
	if err != nil {
		t.Fatalf("expected no error in stub mode, got: %v", err)
	}
}

func TestSendReactivationEmail_EnglishLocale(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	// English locale path must resolve without falling back to empty copy.
	for _, key := range []string{"reactivation_subject", "reactivation_intro", "reactivation_cta"} {
		if got := tr(LocaleEn, key); got == "" {
			t.Fatalf("en %s must not be empty", key)
		}
	}
	if err := adapter.SendReactivationEmail("admin@example.com", "John", 14,
		"https://app.replider.com/login", "en"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
