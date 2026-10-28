package usecases

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// helper: agent owned by "acc1"
func agentOwnedByAcc1() *domain.Agent {
	return &domain.Agent{ID: "agent1", AccountID: "acc1"}
}

func TestCreateChannelUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should create channel with agent", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		agentRepo.On("GetByID", ctx, "agent1").Return(agentOwnedByAcc1(), nil)
		channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{}, nil)
		channelRepo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil)

		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		ch, err := uc.Execute(ctx, CreateChannelDTO{
			AccountID: "acc1",
			AgentID:   strPtr("agent1"),
			Type:      "whatsapp",
			Name:      "WhatsApp Principal",
		})

		assert.NoError(t, err)
		assert.NotNil(t, ch)
		assert.Equal(t, "acc1", ch.AccountID)
		assert.Equal(t, "agent1", *ch.AgentID)
		assert.NotEmpty(t, ch.WebhookToken)
		channelRepo.AssertExpectations(t)
		agentRepo.AssertExpectations(t)
	})

	t.Run("should create channel without agent (human-only)", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{}, nil)
		channelRepo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil)

		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		ch, err := uc.Execute(ctx, CreateChannelDTO{
			AccountID: "acc1",
			AgentID:   nil, // no agent
			Type:      "telegram",
			Name:      "Human Only",
		})

		assert.NoError(t, err)
		assert.NotNil(t, ch)
		assert.Equal(t, "acc1", ch.AccountID)
		assert.Nil(t, ch.AgentID)
		// Agent repo should NOT have been called
		agentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		channelRepo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent belongs to other account", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		agentRepo.On("GetByID", ctx, "agent1").Return(agentOwnedByAcc1(), nil)
		channelRepo.On("ListByAccountID", ctx, "other_acc").Return([]*domain.Channel{}, nil)

		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		_, err := uc.Execute(ctx, CreateChannelDTO{
			AccountID: "other_acc",
			AgentID:   strPtr("agent1"),
			Type:      "whatsapp",
			Name:      "Test",
		})
		assert.Equal(t, domain.ErrAgentNotFound, err)
	})

	t.Run("should return error when type is empty", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		_, err := uc.Execute(ctx, CreateChannelDTO{AccountID: "acc1", Name: "X"})
		assert.Error(t, err)
	})

	t.Run("should return error when name is empty", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		_, err := uc.Execute(ctx, CreateChannelDTO{AccountID: "acc1", Type: "telegram"})
		assert.Error(t, err)
	})
}

func TestListChannelsUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should list all channels for account", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		channels := []*domain.Channel{
			{ID: "ch1", AccountID: "acc1", AgentID: strPtr("agent1"), Type: "whatsapp"},
			{ID: "ch2", AccountID: "acc1", AgentID: nil, Type: "telegram"},
		}
		channelRepo.On("ListByAccountID", ctx, "acc1").Return(channels, nil)

		uc := NewListChannelsUseCase(channelRepo)
		result, err := uc.Execute(ctx, "acc1")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		channelRepo.AssertExpectations(t)
	})
}

func TestUpdateChannelUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should update channel owned by account", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1", AgentID: strPtr("agent1"), Name: "Old Name"}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		channelRepo.On("Update", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil)

		name := "New Name"
		uc := NewUpdateChannelUseCase(channelRepo, agentRepo)
		_, err := uc.Execute(ctx, UpdateChannelDTO{AccountID: "acc1", ChannelID: "ch1", Name: &name})
		assert.NoError(t, err)
		channelRepo.AssertExpectations(t)
	})

	t.Run("should return ErrChannelNotFound when channel belongs to other account", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		existing := &domain.Channel{ID: "ch1", AccountID: "other_acc", Name: "Old Name"}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)

		name := "X"
		uc := NewUpdateChannelUseCase(channelRepo, agentRepo)
		_, err := uc.Execute(ctx, UpdateChannelDTO{AccountID: "acc1", ChannelID: "ch1", Name: &name})
		assert.Equal(t, domain.ErrChannelNotFound, err)
	})

	t.Run("should assign agent to channel", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1", AgentID: nil}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		agentRepo.On("GetByID", ctx, "agent1").Return(agentOwnedByAcc1(), nil)
		channelRepo.On("Update", ctx, mock.MatchedBy(func(ch *domain.Channel) bool {
			return ch.AgentID != nil && *ch.AgentID == "agent1"
		})).Return(nil)

		uc := NewUpdateChannelUseCase(channelRepo, agentRepo)
		_, err := uc.Execute(ctx, UpdateChannelDTO{AccountID: "acc1", ChannelID: "ch1", AgentID: strPtr("agent1")})
		assert.NoError(t, err)
		channelRepo.AssertExpectations(t)
	})
}

// US-003: a client that round-trips the masked config from GET /channels
// (Twilio auth_token comes back "real******") must never overwrite the
// real credential. These fail on the old code, which assigned dto.Config
// to the channel wholesale.
func TestUpdateChannelUseCase_PreservesSensitiveConfig(t *testing.T) {
	ctx := context.Background()
	storedCfg := json.RawMessage(`{"account_sid":"AC00000000000000000000000000000000","auth_token":"realsecret123","from_number":"+15551234567"}`)

	// runUpdate performs an Update with the given incoming config and
	// returns the config map the repo was asked to persist.
	runUpdateTyped := func(t *testing.T, chType string, stored interface{}, incoming string) map[string]interface{} {
		t.Helper()
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1", Type: chType, Name: "SMS", Config: stored}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)

		var persisted map[string]interface{}
		channelRepo.On("Update", ctx, mock.MatchedBy(func(ch *domain.Channel) bool {
			raw, ok := ch.Config.(json.RawMessage)
			if !ok {
				return false
			}
			return json.Unmarshal(raw, &persisted) == nil
		})).Return(nil)

		uc := NewUpdateChannelUseCase(channelRepo, agentRepo)
		_, err := uc.Execute(ctx, UpdateChannelDTO{
			AccountID: "acc1",
			ChannelID: "ch1",
			Config:    json.RawMessage(incoming),
		})
		assert.NoError(t, err)
		channelRepo.AssertExpectations(t)
		return persisted
	}
	runUpdate := func(t *testing.T, stored interface{}, incoming string) map[string]interface{} {
		t.Helper()
		return runUpdateTyped(t, "sms", stored, incoming)
	}

	t.Run("masked auth_token keeps stored value", func(t *testing.T) {
		got := runUpdate(t, storedCfg,
			`{"account_sid":"AC00000000000000000000000000000000","auth_token":"real*********","from_number":"+15551234567"}`)
		assert.Equal(t, "realsecret123", got["auth_token"])
	})

	t.Run("missing auth_token keeps stored value", func(t *testing.T) {
		got := runUpdate(t, storedCfg,
			`{"account_sid":"AC00000000000000000000000000000000","from_number":"+15559999999"}`)
		assert.Equal(t, "realsecret123", got["auth_token"])
		assert.Equal(t, "+15559999999", got["from_number"], "non-sensitive fields still update")
	})

	t.Run("empty auth_token keeps stored value", func(t *testing.T) {
		got := runUpdate(t, storedCfg,
			`{"account_sid":"AC00000000000000000000000000000000","auth_token":"","from_number":"+15551234567"}`)
		assert.Equal(t, "realsecret123", got["auth_token"])
	})

	t.Run("new auth_token rotates credential", func(t *testing.T) {
		got := runUpdate(t, storedCfg,
			`{"account_sid":"AC00000000000000000000000000000000","auth_token":"newsecret456","from_number":"+15551234567"}`)
		assert.Equal(t, "newsecret456", got["auth_token"])
	})

	t.Run("masked value with nothing stored is dropped", func(t *testing.T) {
		got := runUpdateTyped(t, "telegram", json.RawMessage(`{}`),
			`{"auth_token":"abcd****","from_number":"+15551234567"}`)
		_, hasToken := got["auth_token"]
		assert.False(t, hasToken, "a mask must never be persisted as a credential")
		assert.Equal(t, "+15551234567", got["from_number"])
	})

	t.Run("masked telegram bot_token keeps stored value", func(t *testing.T) {
		got := runUpdateTyped(t, "telegram", json.RawMessage(`{"bot_token":"1234567890:AArealBotToken"}`),
			`{"bot_token":"1234**************"}`)
		assert.Equal(t, "1234567890:AArealBotToken", got["bot_token"])
	})
}

// US-011: Instagram long-lived tokens die after ~60 days with no
// warning from Meta — the use cases must stamp token_expires_at into
// the config whenever the access_token is set or rotated so the
// dashboard can flag "reconnect soon" before DMs silently stop.
func TestInstagramTokenExpiryStamp(t *testing.T) {
	ctx := context.Background()

	expiryOf := func(t *testing.T, cfg interface{}) time.Time {
		t.Helper()
		var m map[string]interface{}
		raw, ok := cfg.(json.RawMessage)
		require.True(t, ok)
		require.NoError(t, json.Unmarshal(raw, &m))
		s, _ := m["token_expires_at"].(string)
		require.NotEmpty(t, s, "token_expires_at must be stamped")
		ts, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return ts
	}

	t.Run("create stamps ~60d expiry", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{}, nil)
		channelRepo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil)

		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		ch, err := uc.Execute(ctx, CreateChannelDTO{
			AccountID: "acc1",
			Type:      "instagram",
			Name:      "IG",
			Config:    json.RawMessage(`{"page_id":"178414","access_token":"IGQVJlong"}`),
		})
		require.NoError(t, err)

		exp := expiryOf(t, ch.Config)
		want := time.Now().Add(domain.InstagramTokenTTL)
		assert.WithinDuration(t, want, exp, time.Minute)
	})

	t.Run("create without token is rejected (US-012)", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)

		uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
		_, err := uc.Execute(ctx, CreateChannelDTO{
			AccountID: "acc1",
			Type:      "instagram",
			Name:      "IG stub",
			Config:    json.RawMessage(`{"page_id":"178414"}`),
		})
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
		channelRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	runIGUpdate := func(t *testing.T, stored, incoming string) map[string]interface{} {
		t.Helper()
		channelRepo := &MockChannelRepo{}
		agentRepo := new(MockAgentRepo)
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1", Type: "instagram", Name: "IG", Config: json.RawMessage(stored)}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		var persisted map[string]interface{}
		channelRepo.On("Update", ctx, mock.MatchedBy(func(ch *domain.Channel) bool {
			raw, ok := ch.Config.(json.RawMessage)
			return ok && json.Unmarshal(raw, &persisted) == nil
		})).Return(nil)

		uc := NewUpdateChannelUseCase(channelRepo, agentRepo)
		_, err := uc.Execute(ctx, UpdateChannelDTO{
			AccountID: "acc1",
			ChannelID: "ch1",
			Config:    json.RawMessage(incoming),
		})
		require.NoError(t, err)
		return persisted
	}

	t.Run("token rotation re-stamps expiry", func(t *testing.T) {
		got := runIGUpdate(t,
			`{"page_id":"178414","access_token":"IGQVJold","token_expires_at":"2026-01-01T00:00:00Z"}`,
			`{"page_id":"178414","access_token":"IGQVJnewtoken"}`)
		s, _ := got["token_expires_at"].(string)
		require.NotEmpty(t, s)
		ts, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(domain.InstagramTokenTTL), ts, time.Minute)
	})

	t.Run("masked token round-trip preserves original stamp", func(t *testing.T) {
		// The dashboard round-trips the masked config on rename — the
		// unchanged (preserved) token must keep its ORIGINAL issuance
		// stamp, not get a fresh 60 days.
		got := runIGUpdate(t,
			`{"page_id":"178414","access_token":"IGQVJold","token_expires_at":"2026-08-01T00:00:00Z"}`,
			`{"page_id":"178414","access_token":"IGQV*****"}`)
		assert.Equal(t, "IGQVJold", got["access_token"])
		assert.Equal(t, "2026-08-01T00:00:00Z", got["token_expires_at"])
	})
}

func TestDeleteChannelUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should delete channel owned by account", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1"}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		channelRepo.On("Delete", ctx, "ch1").Return(nil)

		uc := NewDeleteChannelUseCase(channelRepo)
		err := uc.Execute(ctx, "acc1", "ch1")
		assert.NoError(t, err)
		channelRepo.AssertExpectations(t)
	})

	t.Run("should return ErrChannelNotFound when channel belongs to other account", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{ID: "ch1", AccountID: "other_acc"}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)

		uc := NewDeleteChannelUseCase(channelRepo)
		err := uc.Execute(ctx, "acc1", "ch1")
		assert.Equal(t, domain.ErrChannelNotFound, err)
	})

	t.Run("should return ErrChannelNotFound when channel does not exist", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		channelRepo.On("GetByID", ctx, "ch99").Return(nil, nil)

		uc := NewDeleteChannelUseCase(channelRepo)
		err := uc.Execute(ctx, "acc1", "ch99")
		assert.Equal(t, domain.ErrChannelNotFound, err)
	})
}

func TestToggleChannelUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should toggle channel active state", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1", IsActive: true}
		toggled := &domain.Channel{ID: "ch1", AccountID: "acc1", IsActive: false}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		channelRepo.On("Toggle", ctx, "ch1", false).Return(toggled, nil)

		uc := NewToggleChannelUseCase(channelRepo)
		result, err := uc.Execute(ctx, "acc1", "ch1")
		assert.NoError(t, err)
		assert.False(t, result.IsActive)
		channelRepo.AssertExpectations(t)
	})

	t.Run("should return ErrChannelNotFound when account doesn't own channel", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{ID: "ch1", AccountID: "other_acc"}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)

		uc := NewToggleChannelUseCase(channelRepo)
		_, err := uc.Execute(ctx, "acc1", "ch1")
		assert.Equal(t, domain.ErrChannelNotFound, err)
	})
}

// US-014: the channel cap is type-agnostic — instagram and sms channels
// count toward EffectiveMaxChannels exactly like whatsapp/telegram, and
// creating one of the new types is itself subject to the cap.
func TestCreateChannelUseCase_LimitCountsNewChannelTypes(t *testing.T) {
	ctx := context.Background()

	planID := "plan-solo"
	priceID := "price_solo"
	account := &domain.Account{ID: "acc1", PlanID: &planID}
	// Self-serve plan (non-nil StripePriceID) so MaxChannels resolves
	// verbatim instead of the contract-plan unlimited path.
	plan := &domain.Plan{ID: planID, MaxChannels: 3, StripePriceID: &priceID}

	newUC := func(existing []*domain.Channel) (*CreateChannelUseCase, *MockChannelRepo) {
		channelRepo := &MockChannelRepo{}
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)
		channelRepo.On("ListByAccountID", ctx, "acc1").Return(existing, nil)
		channelRepo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil).Maybe()
		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		planRepo.On("GetByID", ctx, planID).Return(plan, nil)
		return NewCreateChannelUseCase(channelRepo, new(MockAgentRepo), accountRepo, planRepo), channelRepo
	}

	t.Run("existing instagram and sms channels fill the cap", func(t *testing.T) {
		uc, channelRepo := newUC([]*domain.Channel{
			{ID: "ch1", AccountID: "acc1", Type: "whatsapp"},
			{ID: "ch2", AccountID: "acc1", Type: "instagram"},
			{ID: "ch3", AccountID: "acc1", Type: "sms"},
		})
		_, err := uc.Execute(ctx, CreateChannelDTO{AccountID: "acc1", Type: "telegram", Name: "Fourth"})

		var appErr *domain.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "PLAN_LIMIT_EXCEEDED", appErr.Code)
		assert.Equal(t, "channels", appErr.Metadata["resource"])
		assert.Equal(t, 3, appErr.Metadata["limit"])
		channelRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	// Valid per-type configs — US-012 validates config shape BEFORE the
	// plan-limit check, so these subtests must send real-looking creds.
	validCfg := map[string]json.RawMessage{
		"instagram": json.RawMessage(`{"page_id":"178414","access_token":"IGQVJtok"}`),
		"sms":       json.RawMessage(`{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"tok","from_number":"+15551234567"}`),
	}
	for _, newType := range []string{"instagram", "sms"} {
		t.Run("creating "+newType+" at the cap is blocked", func(t *testing.T) {
			uc, channelRepo := newUC([]*domain.Channel{
				{ID: "ch1", AccountID: "acc1", Type: "whatsapp"},
				{ID: "ch2", AccountID: "acc1", Type: "telegram"},
				{ID: "ch3", AccountID: "acc1", Type: "whatsapp"},
			})
			_, err := uc.Execute(ctx, CreateChannelDTO{AccountID: "acc1", Type: newType, Name: "Over", Config: validCfg[newType]})

			var appErr *domain.AppError
			assert.ErrorAs(t, err, &appErr)
			assert.Equal(t, "PLAN_LIMIT_EXCEEDED", appErr.Code)
			channelRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})

		t.Run("creating "+newType+" under the cap succeeds", func(t *testing.T) {
			uc, channelRepo := newUC([]*domain.Channel{
				{ID: "ch1", AccountID: "acc1", Type: "whatsapp"},
			})
			ch, err := uc.Execute(ctx, CreateChannelDTO{AccountID: "acc1", Type: newType, Name: "New", Config: validCfg[newType]})

			assert.NoError(t, err)
			assert.Equal(t, newType, ch.Type)
			channelRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*domain.Channel"))
		})
	}
}

func TestCreateChannelUseCase_TelegramSetsConfig(t *testing.T) {
	ctx := context.Background()
	channelRepo := &MockChannelRepo{}
	agentRepo := new(MockAgentRepo)
	agentRepo.On("GetByID", ctx, "agent1").Return(agentOwnedByAcc1(), nil)
	channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{}, nil)
	channelRepo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil)

	uc := NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
	cfg := json.RawMessage(`{"bot_token":"test"}`)
	ch, err := uc.Execute(ctx, CreateChannelDTO{
		AccountID: "acc1",
		AgentID:   strPtr("agent1"),
		Type:      "telegram",
		Name:      "Telegram Bot",
		Config:    cfg,
	})

	assert.NoError(t, err)
	assert.NotNil(t, ch)
	assert.Equal(t, "telegram", ch.Type)
	assert.NotEmpty(t, ch.WebhookToken)

	raw, err := json.Marshal(ch.Config)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"bot_token":"test"}`, string(raw))

	channelRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}

// US-012: server-side per-type config validation. All of these fail on
// the old code, which persisted whatever blob the client sent (an SMS
// channel with a pasted API-Key SID or a national-format number only
// failed on the first send, opaquely).
func TestChannelConfigValidation_US012(t *testing.T) {
	ctx := context.Background()

	newCreateUC := func() (*CreateChannelUseCase, *MockChannelRepo) {
		channelRepo := &MockChannelRepo{}
		channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{}, nil).Maybe()
		channelRepo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil).Maybe()
		return NewCreateChannelUseCase(channelRepo, new(MockAgentRepo), nil, nil), channelRepo
	}

	create := func(t *testing.T, chType, config string) error {
		t.Helper()
		uc, _ := newCreateUC()
		var cfg json.RawMessage
		if config != "" {
			cfg = json.RawMessage(config)
		}
		_, err := uc.Execute(ctx, CreateChannelDTO{
			AccountID: "acc1", Type: chType, Name: "N", Config: cfg,
		})
		return err
	}

	assertValidation := func(t *testing.T, err error) {
		t.Helper()
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	}

	validSMS := `{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"tok_secret","from_number":"+15551234567"}`

	t.Run("type outside the allow-list is rejected", func(t *testing.T) {
		assertValidation(t, create(t, "carrier-pigeon", ""))
	})

	t.Run("webchat type is allowed", func(t *testing.T) {
		assert.NoError(t, create(t, "webchat", ""))
	})

	t.Run("valid sms config passes", func(t *testing.T) {
		assert.NoError(t, create(t, "sms", validSMS))
	})

	t.Run("sms with API-Key SID shape is rejected", func(t *testing.T) {
		// "SK..." is a Twilio API Key SID — the classic wrong paste.
		assertValidation(t, create(t, "sms",
			`{"account_sid":"SK0123456789abcdef0123456789abcdef","auth_token":"tok","from_number":"+15551234567"}`))
	})

	t.Run("sms with short SID is rejected", func(t *testing.T) {
		assertValidation(t, create(t, "sms",
			`{"account_sid":"AC1234","auth_token":"tok","from_number":"+15551234567"}`))
	})

	t.Run("sms with empty auth_token is rejected", func(t *testing.T) {
		assertValidation(t, create(t, "sms",
			`{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"  ","from_number":"+15551234567"}`))
	})

	t.Run("sms with national-format from_number is rejected", func(t *testing.T) {
		assertValidation(t, create(t, "sms",
			`{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"tok","from_number":"5551234567"}`))
	})

	t.Run("sms with nil config is rejected", func(t *testing.T) {
		assertValidation(t, create(t, "sms", ""))
	})

	t.Run("instagram without page_id is rejected", func(t *testing.T) {
		assertValidation(t, create(t, "instagram", `{"access_token":"IGQVJtok"}`))
	})

	t.Run("whatsapp stub with nil config still allowed", func(t *testing.T) {
		assert.NoError(t, create(t, "whatsapp", ""))
	})

	t.Run("whatsapp config without phone identifiers rejected", func(t *testing.T) {
		assertValidation(t, create(t, "whatsapp", `{"access_token":"x"}`))
	})

	// Update path: rules run on the MERGED config so a masked
	// credential round-trip passes, while a real corruption is caught.
	runUpdateErr := func(t *testing.T, chType, stored, incoming string) error {
		t.Helper()
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{ID: "ch1", AccountID: "acc1", Type: chType, Name: "N",
			Config: json.RawMessage(stored)}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		channelRepo.On("Update", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil).Maybe()
		uc := NewUpdateChannelUseCase(channelRepo, new(MockAgentRepo))
		_, err := uc.Execute(ctx, UpdateChannelDTO{
			AccountID: "acc1", ChannelID: "ch1", Config: json.RawMessage(incoming),
		})
		return err
	}

	t.Run("update: masked sms round-trip passes validation", func(t *testing.T) {
		assert.NoError(t, runUpdateErr(t, "sms", validSMS,
			`{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"tok_*******","from_number":"+15551234567"}`))
	})

	t.Run("update: invalid from_number is rejected", func(t *testing.T) {
		assertValidation(t, runUpdateErr(t, "sms", validSMS,
			`{"account_sid":"AC0123456789abcdef0123456789abcdef","auth_token":"","from_number":"not-a-phone"}`))
	})
}

// The send proof means "this provider accepted an authenticated send from us".
// Repoint the channel at a different host, user or address and that sentence is
// about a transport we no longer use — so the proof goes, and the customer runs
// the one-click test again.
func TestUpdateChannelUseCase_RevokesTheSendProofWhenTheTransportChanges(t *testing.T) {
	ctx := context.Background()
	stored := json.RawMessage(`{"from_address":"contato@clinica.com.br","smtp_host":"email-ssl.com.br","smtp_port":"465","smtp_username":"contato@clinica.com.br","smtp_password":"segredo"}`)

	run := func(t *testing.T, incoming string) int {
		t.Helper()
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{
			ID: "ch1", AccountID: "acc1", Type: "email", Name: "E-mail", Config: stored,
		}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		channelRepo.On("Update", ctx, mock.Anything).Return(nil)

		mailboxes := &fakeMailboxRepo{}
		uc := NewUpdateChannelUseCase(channelRepo, new(MockAgentRepo))
		uc.SetEmailMailboxRepo(mailboxes)

		_, err := uc.Execute(ctx, UpdateChannelDTO{
			AccountID: "acc1", ChannelID: "ch1", Config: json.RawMessage(incoming),
		})
		require.NoError(t, err)
		return mailboxes.cleared
	}

	t.Run("a new host revokes it", func(t *testing.T) {
		assert.Equal(t, 1, run(t, `{"from_address":"contato@clinica.com.br","smtp_host":"smtp.outro.com.br","smtp_port":"587","smtp_username":"contato@clinica.com.br","smtp_password":"segredo"}`))
	})

	t.Run("a new from_address revokes it", func(t *testing.T) {
		assert.Equal(t, 1, run(t, `{"from_address":"vendas@clinica.com.br","smtp_host":"email-ssl.com.br","smtp_port":"465","smtp_username":"vendas@clinica.com.br","smtp_password":"segredo"}`))
	})

	// The edit dialog resubmits the whole form on every save, with the password
	// masked. Re-saving the same transport must NOT revoke the proof — that
	// would send the customer back to the blocked tier for changing nothing.
	t.Run("re-saving the same transport leaves it alone", func(t *testing.T) {
		assert.Zero(t, run(t, `{"from_address":"contato@clinica.com.br","smtp_host":"email-ssl.com.br","smtp_port":"465","smtp_username":"contato@clinica.com.br","smtp_password":"segr****"}`))
	})

	// A new port is a new transport: 465 and 587 are different TLS paths, and
	// the one we proved is the one we proved.
	t.Run("a new port revokes it", func(t *testing.T) {
		assert.Equal(t, 1, run(t, `{"from_address":"contato@clinica.com.br","smtp_host":"email-ssl.com.br","smtp_port":"587","smtp_username":"contato@clinica.com.br","smtp_password":"segr****"}`))
	})

	// Nothing about the transport was touched.
	t.Run("renaming the channel leaves it alone", func(t *testing.T) {
		channelRepo := &MockChannelRepo{}
		existing := &domain.Channel{
			ID: "ch1", AccountID: "acc1", Type: "email", Name: "E-mail", Config: stored,
		}
		channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)
		channelRepo.On("Update", ctx, mock.Anything).Return(nil)

		mailboxes := &fakeMailboxRepo{}
		uc := NewUpdateChannelUseCase(channelRepo, new(MockAgentRepo))
		uc.SetEmailMailboxRepo(mailboxes)

		name := "Comercial"
		_, err := uc.Execute(ctx, UpdateChannelDTO{
			AccountID: "acc1", ChannelID: "ch1", Name: &name,
		})
		require.NoError(t, err)
		assert.Zero(t, mailboxes.cleared)
	})
}

// A Gmail mailbox connected by consent is edited from a form that never
// renders auth_method. Re-submitting the visible fields used to drop the
// key, and the merged config was then validated as a hand-typed SMTP setup
// — which refuses @gmail.com — so the channel could not be renamed.
func TestUpdateChannelUseCase_PreservesEmailAuthMethod(t *testing.T) {
	ctx := context.Background()
	stored := json.RawMessage(`{"auth_method":"google_oauth","oauth_refresh_token":"refresh-abc","from_address":"dona@gmail.com"}`)
	channelRepo := &MockChannelRepo{}
	agentRepo := new(MockAgentRepo)
	existing := &domain.Channel{ID: "ch1", AccountID: "acc1", Type: "email", Name: "Gmail", Config: stored}
	channelRepo.On("GetByID", ctx, "ch1").Return(existing, nil)

	var persisted map[string]interface{}
	channelRepo.On("Update", ctx, mock.MatchedBy(func(ch *domain.Channel) bool {
		raw, ok := ch.Config.(json.RawMessage)
		return ok && json.Unmarshal(raw, &persisted) == nil
	})).Return(nil)

	uc := NewUpdateChannelUseCase(channelRepo, agentRepo)
	_, err := uc.Execute(ctx, UpdateChannelDTO{
		AccountID: "acc1",
		ChannelID: "ch1",
		Config:    json.RawMessage(`{"from_address":"dona@gmail.com"}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, "google_oauth", persisted["auth_method"])
	assert.Equal(t, "refresh-abc", persisted["oauth_refresh_token"])
	channelRepo.AssertExpectations(t)
}
