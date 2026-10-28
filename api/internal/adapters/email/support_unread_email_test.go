package email

import "testing"

// Canal-interno US-004: the unread-fallback email must render in stub mode
// for both locales (en copy parity is additionally pinned by
// localized_email_test.go's map-parity check).
func TestSendSupportUnreadEmail_StubMode(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	if err := adapter.SendSupportUnreadEmail("ana@cliente.com", "Ana",
		"https://app.replider.com/support", ""); err != nil {
		t.Fatalf("expected no error in stub mode, got: %v", err)
	}
}

func TestSendSupportUnreadEmail_EnglishLocale(t *testing.T) {
	adapter := NewSMTPAdapter("", "", "", "", "noreply@replider.com")
	if got := tr(LocaleEn, "support_unread_subject"); got == "" {
		t.Fatal("en support_unread_subject must not be empty")
	}
	if err := adapter.SendSupportUnreadEmail("john@customer.com", "John",
		"https://app.replider.com/support", "en"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
