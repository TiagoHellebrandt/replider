package usecases

import (
	"context"
	"encoding/json"

	"github.com/replider/api/internal/core/domain"
)

// UpdateOnboardingDraftUseCase persists the in-progress onboarding state
// so the client can resume the wizard after a cold start. The backend is
// deliberately agnostic about the draft's shape — the Flutter controller
// owns it and we just round-trip the JSON blob.
type UpdateOnboardingDraftUseCase struct {
	accountRepo domain.AccountRepository
}

func NewUpdateOnboardingDraftUseCase(accountRepo domain.AccountRepository) *UpdateOnboardingDraftUseCase {
	return &UpdateOnboardingDraftUseCase{accountRepo: accountRepo}
}

type UpdateOnboardingDraftDTO struct {
	CurrentStep int
	Draft       json.RawMessage
}

func (uc *UpdateOnboardingDraftUseCase) Execute(ctx context.Context, accountID string, dto UpdateOnboardingDraftDTO) error {
	// Clamp the step into the page range the UI understands; a bad client
	// write must not land the next login on a non-existent page. The
	// ceiling follows the wizard's canonical page indices (see
	// maxOnboardingStepIndex) — it was 3 for years after the flow grew to
	// 8 pages, which sent every resume back to Products.
	step := dto.CurrentStep
	if step < 0 {
		step = 0
	}
	if step > maxOnboardingStepIndex {
		step = maxOnboardingStepIndex
	}
	draft := dto.Draft
	if len(draft) == 0 {
		draft = json.RawMessage("{}")
	}
	return uc.accountRepo.UpdateOnboardingDraft(ctx, accountID, step, sanitizeOnboardingDraft(draft))
}

// sanitizeOnboardingDraft strips credential values from the draft's
// channel_config so secrets (Twilio auth_token, IG access_token, bot
// tokens) never sit in plaintext on accounts.onboarding_draft — the
// draft is a resume convenience, not a credential store; the real
// config is encrypted at rest by the channel repository when the
// channel is created. Applied on write AND on read (see
// GetOnboardingProgressUseCase) so drafts persisted by older clients
// stop leaking too. Non-object drafts pass through untouched — they
// can't carry a channel_config map.
func sanitizeOnboardingDraft(draft json.RawMessage) json.RawMessage {
	if len(draft) == 0 {
		return draft
	}
	var m map[string]interface{}
	if err := json.Unmarshal(draft, &m); err != nil || m == nil {
		return draft
	}
	cfg, ok := m["channel_config"].(map[string]interface{})
	if !ok {
		return draft
	}
	changed := false
	for key := range cfg {
		if domain.SensitiveConfigKey(key) {
			delete(cfg, key)
			changed = true
		}
	}
	if !changed {
		return draft
	}
	out, err := json.Marshal(m)
	if err != nil {
		return draft
	}
	return out
}
