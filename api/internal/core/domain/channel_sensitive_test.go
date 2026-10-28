package domain

import "testing"

// A credential that does not read as sensitive is masked nowhere, encrypted
// nowhere, and overwritten by any client that echoes back what it fetched.
// The email channel's smtp_password was exactly that until the rule matched
// the `_password` suffix (US-M003 follow-up).
func TestSensitiveConfigKey(t *testing.T) {
	sensitive := []string{
		"password",
		"smtp_password",
		"bot_token",
		"access_token",
		"auth_token",
		"app_secret",
		"api_key",
	}
	for _, key := range sensitive {
		if !SensitiveConfigKey(key) {
			t.Errorf("%q must be treated as a credential", key)
		}
	}

	// Lookup keys are deliberately NOT sensitive: raw SQL filters on them,
	// so masking or encrypting them would break channel routing.
	plain := []string{
		"phone_number_id",
		"waba_id",
		"from_address",
		"smtp_host",
		"smtp_port",
		"smtp_username",
		"from_number",
		"account_sid",
		"page_id",
	}
	for _, key := range plain {
		if SensitiveConfigKey(key) {
			t.Errorf("%q must stay readable — it is a lookup key, not a credential", key)
		}
	}
}
