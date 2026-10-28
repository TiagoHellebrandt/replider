package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Importing 19 brochures, the third exhausted the account's free-tier Gemini
// quota. Every failure — quota, timeout, anything — was flattened into
// PRODUCT_PDF_EXTRACT_FAILED ("tente de novo ou use o import por CSV"), a 400
// that says nothing about which of those happened. So the batch kept going,
// spending minutes uploading sixteen more files to reach advice that could not
// work. These pin the three answers apart.

func TestClassifyExtractionError(t *testing.T) {
	geminiQuota := errors.New("googleapi: Error 429: You exceeded your current quota, " +
		"please check your plan and billing details.\n" +
		"* Quota exceeded for metric: generativelanguage.googleapis.com/" +
		"generate_content_free_tier_requests, limit: 20, model: gemini-3.7-flash\n" +
		"Please retry in 57.403730605s.")

	cases := []struct {
		name     string
		err      error
		wantCode string
		wantIn   string // substring the message must carry, "" for none
	}{
		{"gemini quota", geminiQuota, "PRODUCT_PDF_QUOTA_EXCEEDED", "57s"},
		{"resource exhausted", errors.New("rpc error: RESOURCE_EXHAUSTED"), "PRODUCT_PDF_QUOTA_EXCEEDED", ""},
		{"deadline sentinel", context.DeadlineExceeded, "PRODUCT_PDF_TIMEOUT", ""},
		{"deadline in text", errors.New(`Post "https://generativelanguage.googleapis.com/v1beta": context deadline exceeded`),
			"PRODUCT_PDF_TIMEOUT", ""},
		{"anything else", errors.New("malformed response"), "PRODUCT_PDF_EXTRACT_FAILED", ""},
		{"nil", nil, "PRODUCT_PDF_EXTRACT_FAILED", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyExtractionError(tc.err)
			if got.Code != tc.wantCode {
				t.Fatalf("code = %s, want %s", got.Code, tc.wantCode)
			}
			if tc.wantIn != "" && !strings.Contains(got.Message, tc.wantIn) {
				t.Fatalf("message %q does not carry the provider's wait (%s)", got.Message, tc.wantIn)
			}
		})
	}
}

// The wait is the difference between "try again" and "try again at 14:05".
func TestProviderRetryAfter(t *testing.T) {
	if got := providerRetryAfter("Please retry in 57.403730605s."); got != "57s" {
		t.Fatalf("got %q, want 57s", got)
	}
	if got := providerRetryAfter("retry in 45s"); got != "45s" {
		t.Fatalf("got %q, want 45s", got)
	}
	if got := providerRetryAfter("no number here"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
