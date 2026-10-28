package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// US-012: the onboarding draft is a resume convenience, not a credential
// store — channel secrets must never be persisted in plaintext on
// accounts.onboarding_draft. These fail on the old code, which
// round-tripped the blob verbatim.
func TestUpdateOnboardingDraft_StripsChannelCredentials(t *testing.T) {
	ctx := context.Background()

	persistedDraft := func(t *testing.T, incoming string) map[string]interface{} {
		t.Helper()
		accountRepo := new(MockAccountRepo)
		var persisted map[string]interface{}
		accountRepo.On("UpdateOnboardingDraft", ctx, "acc_1", mock.Anything,
			mock.MatchedBy(func(d json.RawMessage) bool {
				return json.Unmarshal(d, &persisted) == nil
			})).Return(nil)

		uc := NewUpdateOnboardingDraftUseCase(accountRepo)
		err := uc.Execute(ctx, "acc_1", UpdateOnboardingDraftDTO{
			CurrentStep: 2,
			Draft:       json.RawMessage(incoming),
		})
		require.NoError(t, err)
		accountRepo.AssertExpectations(t)
		return persisted
	}

	t.Run("sms auth_token is stripped, siblings survive", func(t *testing.T) {
		got := persistedDraft(t, `{
			"agent_name": "Bot",
			"channel_type": "sms",
			"channel_config": {
				"account_sid": "AC0123456789abcdef0123456789abcdef",
				"auth_token": "supersecret",
				"from_number": "+15551234567"
			}
		}`)
		cfg, ok := got["channel_config"].(map[string]interface{})
		require.True(t, ok)
		_, hasToken := cfg["auth_token"]
		assert.False(t, hasToken, "auth_token must not be persisted in the draft")
		assert.Equal(t, "AC0123456789abcdef0123456789abcdef", cfg["account_sid"])
		assert.Equal(t, "+15551234567", cfg["from_number"])
		assert.Equal(t, "Bot", got["agent_name"])
	})

	t.Run("instagram and telegram tokens are stripped", func(t *testing.T) {
		got := persistedDraft(t,
			`{"channel_config":{"page_id":"178414","access_token":"IGQVJtok","bot_token":"123:AA"}}`)
		cfg, ok := got["channel_config"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, map[string]interface{}{"page_id": "178414"}, cfg)
	})

	t.Run("draft without channel_config passes through", func(t *testing.T) {
		got := persistedDraft(t, `{"agent_name":"Bot"}`)
		assert.Equal(t, map[string]interface{}{"agent_name": "Bot"}, got)
	})
}

// Legacy drafts written before US-012 may still hold plaintext tokens —
// the read path must not hand them back to the client.
func TestGetOnboardingProgress_SanitizesLegacyDraft(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	accountRepo.On("GetOnboardingProgress", ctx, "acc_1").Return(&domain.OnboardingProgress{
		Draft: json.RawMessage(`{"channel_config":{"auth_token":"supersecret","from_number":"+15551234567"}}`),
	}, nil)
	accountRepo.On("GetByID", ctx, "acc_1").Return(&domain.Account{ID: "acc_1"}, nil)

	uc := NewGetOnboardingProgressUseCase(accountRepo)
	got, err := uc.Execute(ctx, "acc_1")
	require.NoError(t, err)

	assert.NotContains(t, string(got.Draft), "supersecret")
	assert.Contains(t, string(got.Draft), "+15551234567")
}
