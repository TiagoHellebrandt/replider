package domain

import "testing"

// A batch must stop on the FIRST account-level refusal rather than counting
// to forty. Production spent nine nights logging 63-70 identical failures and
// recording the count as if it were seventy separate problems.
func TestIsAccountLevelDraftRefusal(t *testing.T) {
	stops := []string{
		ErrApproachNeedsProducts.Code,
		ErrApproachNeedsChannel.Code,
		"SPEND_AI_DRAFTS_BUDGET_EXCEEDED",
		"SPEND_ANYTHING",
	}
	for _, c := range stops {
		if !IsAccountLevelDraftRefusal(c) {
			t.Errorf("%s must stop the batch — no other lead gets past it either", c)
		}
	}

	// A per-lead failure must NOT stop the batch: the next lead may well
	// work, and giving up on forty because of one is the opposite bug.
	keepsGoing := []string{
		"", "LEAD_NOT_FOUND", "AI_PROVIDER_ERROR",
		ErrApproachHasPlaceholder.Code, "SPENDING", "NOT_SPEND_ANYTHING",
	}
	for _, c := range keepsGoing {
		if IsAccountLevelDraftRefusal(c) {
			t.Errorf("%q must not stop the whole batch", c)
		}
	}
}
