package whatsapp

// SetBaseURLForTesting overrides the Meta Graph API base URL used by all
// HTTP wrappers in this package. Intended for tests only; call with the
// original value to restore.
func SetBaseURLForTesting(u string) {
	metaGraphBaseURL = u
}

// GetBaseURLForTesting returns the current Meta Graph API base URL so
// tests can save and restore it.
func GetBaseURLForTesting() string {
	return metaGraphBaseURL
}
