package usecases

import (
	"context"
	"encoding/json"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// Shared Mocks for Use Case Tests

type MockAccountRepo struct{ mock.Mock }

func (m *MockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	return m.Called(ctx, account).Error(0)
}
func (m *MockAccountRepo) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Account), args.Error(1)
}

func (m *MockAccountRepo) GetAccountByStripeSubscription(ctx context.Context, stripeSubID string) (*domain.Account, error) {
	args := m.Called(ctx, stripeSubID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Account), args.Error(1)
}

func (m *MockAccountRepo) UpdateAccountSubscription(ctx context.Context, accountID string, stripeCustomerID, stripeSubID *string, status string) error {
	args := m.Called(ctx, accountID, stripeCustomerID, stripeSubID, status)
	return args.Error(0)
}
func (m *MockAccountRepo) SetStripeCustomerIfAbsent(ctx context.Context, accountID, customerID string) (string, error) {
	args := m.Called(ctx, accountID, customerID)
	return args.String(0), args.Error(1)
}
func (m *MockAccountRepo) UpdateName(ctx context.Context, accountID string, name string) error {
	return m.Called(ctx, accountID, name).Error(0)
}
func (m *MockAccountRepo) UpdateBranding(ctx context.Context, accountID string, logoURL *string, timezone string) error {
	return m.Called(ctx, accountID, logoURL, timezone).Error(0)
}

func (m *MockAccountRepo) UpdateBusinessProfile(ctx context.Context, accountID string, lines []domain.BusinessLine, territory *domain.Territory) error {
	return m.Called(ctx, accountID, lines, territory).Error(0)
}
func (m *MockAccountRepo) SetTrialBypass(ctx context.Context, accountID string, bypass bool, reason string) error {
	args := m.Called(ctx, accountID, bypass, reason)
	return args.Error(0)
}
func (m *MockAccountRepo) UpdateCSATEnabled(ctx context.Context, accountID string, enabled bool) error {
	return m.Called(ctx, accountID, enabled).Error(0)
}
func (m *MockAccountRepo) CompleteOnboarding(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}
func (m *MockAccountRepo) GetOnboardingProgress(ctx context.Context, accountID string) (*domain.OnboardingProgress, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OnboardingProgress), args.Error(1)
}
func (m *MockAccountRepo) MarkPreviewChatTested(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}
func (m *MockAccountRepo) MarkFirstInboundReceived(ctx context.Context, accountID string, at time.Time) error {
	return m.Called(ctx, accountID, at).Error(0)
}
func (m *MockAccountRepo) DismissOnboardingChecklist(ctx context.Context, accountID string, at time.Time) error {
	return m.Called(ctx, accountID, at).Error(0)
}
func (m *MockAccountRepo) UpdateOnboardingDraft(ctx context.Context, accountID string, currentStep int, draft json.RawMessage) error {
	return m.Called(ctx, accountID, currentStep, draft).Error(0)
}
func (m *MockAccountRepo) SetAPIKeys(ctx context.Context, accountID string, openAIKey, geminiKey string) error {
	return m.Called(ctx, accountID, openAIKey, geminiKey).Error(0)
}
func (m *MockAccountRepo) SetLimitOverrides(ctx context.Context, accountID string, overrides domain.AccountLimitOverrides) error {
	return m.Called(ctx, accountID, overrides).Error(0)
}
func (m *MockAccountRepo) Suspend(ctx context.Context, accountID, reason string) error {
	return m.Called(ctx, accountID, reason).Error(0)
}
func (m *MockAccountRepo) Reactivate(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}
func (m *MockAccountRepo) SetSubscriptionStatus(ctx context.Context, accountID, status string) error {
	return m.Called(ctx, accountID, status).Error(0)
}
func (m *MockAccountRepo) SetPlan(ctx context.Context, accountID, planID string) error {
	return m.Called(ctx, accountID, planID).Error(0)
}
func (m *MockAccountRepo) ClearTrialExpiresAt(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}
func (m *MockAccountRepo) SetPlatformKeyAllowed(ctx context.Context, accountID string, allowed bool) error {
	return m.Called(ctx, accountID, allowed).Error(0)
}
func (m *MockAccountRepo) UpdateSubscriptionPeriodEnd(ctx context.Context, accountID string, periodEnd time.Time) error {
	return m.Called(ctx, accountID, periodEnd).Error(0)
}
func (m *MockAccountRepo) ListAccountsDueForPrebilling(ctx context.Context, cutoff time.Time) ([]*domain.PrebillingAccount, error) {
	args := m.Called(ctx, cutoff)
	if v := args.Get(0); v != nil {
		return v.([]*domain.PrebillingAccount), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAccountRepo) MarkPrebillingNotified(ctx context.Context, accountID string, periodEnd time.Time) error {
	return m.Called(ctx, accountID, periodEnd).Error(0)
}
func (m *MockAccountRepo) SetReplyReminderSettings(ctx context.Context, accountID string, enabled bool, thresholdMinutes int) error {
	return m.Called(ctx, accountID, enabled, thresholdMinutes).Error(0)
}
func (m *MockAccountRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type MockUserRepo struct{ mock.Mock }

func (m *MockUserRepo) Create(ctx context.Context, user *domain.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByGoogleID(ctx context.Context, googleID string) (*domain.User, error) {
	args := m.Called(ctx, googleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByMetaID(ctx context.Context, metaID string) (*domain.User, error) {
	args := m.Called(ctx, metaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) UpdatePasswordHash(ctx context.Context, userID string, hash string) error {
	return m.Called(ctx, userID, hash).Error(0)
}
func (m *MockUserRepo) UpdateProfile(ctx context.Context, userID string, name string, email string, signature *string) error {
	return m.Called(ctx, userID, name, email, signature).Error(0)
}
func (m *MockUserRepo) SetResetToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
	return m.Called(ctx, userID, token, expiresAt).Error(0)
}
func (m *MockUserRepo) GetByResetToken(ctx context.Context, token string) (*domain.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) ClearResetToken(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *MockUserRepo) SetGoogleID(ctx context.Context, userID string, googleID string) error {
	return m.Called(ctx, userID, googleID).Error(0)
}
func (m *MockUserRepo) SetMetaID(ctx context.Context, userID string, metaID string) error {
	return m.Called(ctx, userID, metaID).Error(0)
}
func (m *MockUserRepo) SetPicture(ctx context.Context, userID string, picture string) error {
	return m.Called(ctx, userID, picture).Error(0)
}
func (m *MockUserRepo) SetVerificationToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
	return m.Called(ctx, userID, token, expiresAt).Error(0)
}
func (m *MockUserRepo) GetByVerificationToken(ctx context.Context, token string) (*domain.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) VerifyEmail(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *MockUserRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.User, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}
func (m *MockUserRepo) UpdateRole(ctx context.Context, userID string, role string) error {
	return m.Called(ctx, userID, role).Error(0)
}
func (m *MockUserRepo) Deactivate(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *MockUserRepo) GetAdminByAccountID(ctx context.Context, accountID string) (*domain.User, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) MarkInviteSent(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *MockUserRepo) UpdateLastSeen(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *MockUserRepo) SetTOTPSecret(ctx context.Context, userID, secret string) error {
	return m.Called(ctx, userID, secret).Error(0)
}
func (m *MockUserRepo) EnableTOTP(ctx context.Context, userID string, hashed []string) error {
	return m.Called(ctx, userID, hashed).Error(0)
}
func (m *MockUserRepo) DisableTOTP(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *MockUserRepo) ConsumeTOTPRecoveryCode(ctx context.Context, userID, hashed string) (bool, error) {
	a := m.Called(ctx, userID, hashed)
	return a.Bool(0), a.Error(1)
}
func (m *MockUserRepo) StampTOTPVerified(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

type MockChannelRepo struct{ mock.Mock }

func (m *MockChannelRepo) Create(ctx context.Context, ch *domain.Channel) error {
	return m.Called(ctx, ch).Error(0)
}
func (m *MockChannelRepo) GetByID(ctx context.Context, id string) (*domain.Channel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) GetByWebhookToken(ctx context.Context, token string) (*domain.Channel, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) GetByPhoneNumberID(ctx context.Context, phoneNumberID string) (*domain.Channel, error) {
	args := m.Called(ctx, phoneNumberID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) GetByWABAID(ctx context.Context, wabaID string) (*domain.Channel, error) {
	args := m.Called(ctx, wabaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) ListByWABAID(ctx context.Context, wabaID string) ([]*domain.Channel, error) {
	args := m.Called(ctx, wabaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) UpdateCredentialField(ctx context.Context, channelID, key, value string) error {
	args := m.Called(ctx, channelID, key, value)
	return args.Error(0)
}
func (m *MockChannelRepo) MergeCredentials(ctx context.Context, channelID string, fields map[string]string) error {
	args := m.Called(ctx, channelID, fields)
	return args.Error(0)
}
func (m *MockChannelRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Channel, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) ListByAgentID(ctx context.Context, agentID string) ([]*domain.Channel, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) ListAllByProvider(ctx context.Context, provider string) ([]*domain.Channel, error) {
	args := m.Called(ctx, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Channel), args.Error(1)
}
func (m *MockChannelRepo) Update(ctx context.Context, ch *domain.Channel) error {
	return m.Called(ctx, ch).Error(0)
}
func (m *MockChannelRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockChannelRepo) Toggle(ctx context.Context, id string, isActive bool) (*domain.Channel, error) {
	args := m.Called(ctx, id, isActive)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Channel), args.Error(1)
}

type MockMessageRepo struct{ mock.Mock }

func (m *MockMessageRepo) CreateMessageAttempt(ctx context.Context, a *domain.MessageAttempt) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}
func (m *MockMessageRepo) ApplyMessageAttemptStatus(ctx context.Context, s domain.WhatsAppStatusUpdate) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}
func (m *MockMessageRepo) GetMessageAttemptByExternalID(ctx context.Context, channelID, externalID string) (*domain.MessageAttempt, error) {
	args := m.Called(ctx, channelID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MessageAttempt), args.Error(1)
}
func (m *MockMessageRepo) ApplyWhatsAppStatus(ctx context.Context, s domain.WhatsAppStatusUpdate) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockMessageRepo) Create(ctx context.Context, msg *domain.Message) (bool, error) {
	args := m.Called(ctx, msg)
	// Default: treat any success path as inserted=true unless the test
	// explicitly returned a bool (legacy mocks only specify the error).
	if len(args) > 1 {
		if v, ok := args.Get(0).(bool); ok {
			return v, args.Error(1)
		}
	}
	return true, args.Error(0)
}
func (m *MockMessageRepo) GetByExternalID(ctx context.Context, channelID, externalID string) (*domain.Message, error) {
	args := m.Called(ctx, channelID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) GetByID(ctx context.Context, messageID, accountID string) (*domain.Message, error) {
	args := m.Called(ctx, messageID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) SetTranscript(ctx context.Context, messageID, transcript string) error {
	return m.Called(ctx, messageID, transcript).Error(0)
}
func (m *MockMessageRepo) MarkSendFailed(ctx context.Context, messageID, reason string) error {
	return m.Called(ctx, messageID, reason).Error(0)
}
func (m *MockMessageRepo) LastInboundInConversation(ctx context.Context, conversationID string) (*domain.Message, error) {
	args := m.Called(ctx, conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) ListByConversationID(ctx context.Context, conversationID string, limit, offset int32) ([]*domain.Message, error) {
	args := m.Called(ctx, conversationID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) ListConversationMedia(ctx context.Context, conversationID string, limit, offset int32) ([]*domain.Message, error) {
	args := m.Called(ctx, conversationID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) ListByConversationSince(ctx context.Context, conversationID string, since time.Time) ([]*domain.Message, error) {
	args := m.Called(ctx, conversationID, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) MarkConversationRead(ctx context.Context, conversationID string, readAt time.Time) error {
	return m.Called(ctx, conversationID, readAt).Error(0)
}
func (m *MockMessageRepo) ListUnreadInbound(ctx context.Context, conversationID string) ([]*domain.Message, error) {
	args := m.Called(ctx, conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}
func (m *MockMessageRepo) SearchMessages(ctx context.Context, accountID, query string, limit, offset int) ([]*domain.MessageSearchResult, error) {
	args := m.Called(ctx, accountID, query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MessageSearchResult), args.Error(1)
}

type MockUsageRepo struct{ mock.Mock }

func (m *MockUsageRepo) Create(ctx context.Context, usage *domain.AccountUsage) error {
	return m.Called(ctx, usage).Error(0)
}
func (m *MockUsageRepo) GetByAccountID(ctx context.Context, accountID string) (*domain.AccountUsage, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AccountUsage), args.Error(1)
}
func (m *MockUsageRepo) GetSummaryByAccountID(ctx context.Context, accountID string) (*domain.AccountUsage, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AccountUsage), args.Error(1)
}
func (m *MockUsageRepo) Update(ctx context.Context, usage *domain.AccountUsage) error {
	return m.Called(ctx, usage).Error(0)
}
func (m *MockUsageRepo) AddUsage(ctx context.Context, accountID string, tokensToAdd int64, inputTokens int64, outputTokens int64, conversationsToAdd int32) error {
	args := m.Called(ctx, accountID, tokensToAdd, inputTokens, outputTokens, conversationsToAdd)
	return args.Error(0)
}

func (m *MockUsageRepo) AddAddonTokens(ctx context.Context, accountID string, tokens int64) error {
	args := m.Called(ctx, accountID, tokens)
	return args.Error(0)
}

func (m *MockUsageRepo) AddBroadcastMessages(ctx context.Context, accountID string, count int32) error {
	args := m.Called(ctx, accountID, count)
	return args.Error(0)
}

func (m *MockUsageRepo) ResetUsage(ctx context.Context, accountID string) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}
func (m *MockUsageRepo) SetUsageWarned80(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}
func (m *MockUsageRepo) SetUsageWarned100(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}
func (m *MockUsageRepo) ListOverageCandidates(ctx context.Context) ([]domain.OverageCandidate, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.OverageCandidate), args.Error(1)
}
func (m *MockUsageRepo) RecordOverageCharge(ctx context.Context, accountID string, overageConvs, totalCentsCharged int32, capReached bool) error {
	return m.Called(ctx, accountID, overageConvs, totalCentsCharged, capReached).Error(0)
}
func (m *MockUsageRepo) GetOverageCandidate(ctx context.Context, accountID string) (*domain.OverageCandidate, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OverageCandidate), args.Error(1)
}

type MockAgentRepo struct{ mock.Mock }

func (m *MockAgentRepo) Create(ctx context.Context, agent *domain.Agent) error {
	return m.Called(ctx, agent).Error(0)
}
func (m *MockAgentRepo) GetByID(ctx context.Context, id string) (*domain.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}
func (m *MockAgentRepo) Update(ctx context.Context, agent *domain.Agent) error {
	return m.Called(ctx, agent).Error(0)
}
func (m *MockAgentRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Agent, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).([]*domain.Agent), args.Error(1)
}
func (m *MockAgentRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockAgentRepo) ListAllTimezones(ctx context.Context) ([]*domain.AgentTimezone, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentTimezone), args.Error(1)
}

type MockPlanRepo struct{ mock.Mock }

func (m *MockPlanRepo) GetByID(ctx context.Context, id string) (*domain.Plan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Plan), args.Error(1)
}
func (m *MockPlanRepo) GetByStripePriceID(ctx context.Context, priceID string) (*domain.Plan, error) {
	args := m.Called(ctx, priceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Plan), args.Error(1)
}
func (m *MockPlanRepo) ListAll(ctx context.Context) ([]*domain.Plan, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Plan), args.Error(1)
}

type MockConversationRepo struct{ mock.Mock }

func (m *MockConversationRepo) Create(ctx context.Context, conv *domain.Conversation) error {
	return m.Called(ctx, conv).Error(0)
}
func (m *MockConversationRepo) GetByID(ctx context.Context, id string, accountID string) (*domain.Conversation, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Conversation), args.Error(1)
}
func (m *MockConversationRepo) Update(ctx context.Context, conv *domain.Conversation) error {
	return m.Called(ctx, conv).Error(0)
}
func (m *MockConversationRepo) ListByAccountID(ctx context.Context, accountID string, status string) ([]*domain.Conversation, error) {
	args := m.Called(ctx, accountID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Conversation), args.Error(1)
}
func (m *MockConversationRepo) AutoResumeStuckConversations(ctx context.Context, idleMinutes int) (int64, error) {
	args := m.Called(ctx, idleMinutes)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockConversationRepo) SetBotPaused(ctx context.Context, conversationID string, accountID string, paused bool) error {
	return m.Called(ctx, conversationID, accountID, paused).Error(0)
}
func (m *MockConversationRepo) RefreshContactAvatar(ctx context.Context, conversationID, accountID, url, bio string) error {
	args := m.Called(ctx, conversationID, accountID, url, bio)
	return args.Error(0)
}
func (m *MockConversationRepo) UpdateLastCustomerMessageAt(ctx context.Context, conversationID string, accountID string) error {
	return m.Called(ctx, conversationID, accountID).Error(0)
}
func (m *MockConversationRepo) UpdateSummary(ctx context.Context, conversationID, accountID, summary string) error {
	return m.Called(ctx, conversationID, accountID, summary).Error(0)
}
func (m *MockConversationRepo) ClearWaitingSince(ctx context.Context, conversationID string, accountID string) error {
	return m.Called(ctx, conversationID, accountID).Error(0)
}
func (m *MockConversationRepo) FindByContact(ctx context.Context, channelID string, contactID string) (*domain.Conversation, error) {
	args := m.Called(ctx, channelID, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Conversation), args.Error(1)
}
func (m *MockConversationRepo) GetContactStats(ctx context.Context, accountID, contactID string) (*domain.ContactStats, error) {
	args := m.Called(ctx, accountID, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ContactStats), args.Error(1)
}
func (m *MockConversationRepo) ListEnriched(ctx context.Context, accountID string, status string, limit, offset int32, sort string, assignedTo string, channelType string) ([]*domain.ConversationSummary, error) {
	args := m.Called(ctx, accountID, status, limit, offset, sort, assignedTo, channelType)
	return args.Get(0).([]*domain.ConversationSummary), args.Error(1)
}
func (m *MockConversationRepo) AssignConversation(ctx context.Context, conversationID, accountID string, assignedTo *string) error {
	return m.Called(ctx, conversationID, accountID, assignedTo).Error(0)
}
func (m *MockConversationRepo) GetActiveAttendants(ctx context.Context, accountID string) ([]string, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockConversationRepo) AssumeConversation(ctx context.Context, conversationID, accountID, userID string) error {
	return m.Called(ctx, conversationID, accountID, userID).Error(0)
}
func (m *MockConversationRepo) SetFirstInboundAt(ctx context.Context, conversationID, accountID string) error {
	return m.Called(ctx, conversationID, accountID).Error(0)
}
func (m *MockConversationRepo) SetLanguage(ctx context.Context, conversationID, lang string) error {
	return m.Called(ctx, conversationID, lang).Error(0)
}
func (m *MockConversationRepo) SetSentiment(ctx context.Context, conversationID, accountID, sentiment string, score float32) error {
	return m.Called(ctx, conversationID, accountID, sentiment, score).Error(0)
}
func (m *MockConversationRepo) SetFirstResponse(ctx context.Context, conversationID, accountID, by string) error {
	return m.Called(ctx, conversationID, accountID, by).Error(0)
}
func (m *MockConversationRepo) ReleaseConversation(ctx context.Context, conversationID, accountID string) error {
	return m.Called(ctx, conversationID, accountID).Error(0)
}
func (m *MockConversationRepo) GetAssignment(ctx context.Context, conversationID, accountID string) (*domain.ConversationAssignment, error) {
	args := m.Called(ctx, conversationID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ConversationAssignment), args.Error(1)
}
func (m *MockConversationRepo) ListPendingReminders(ctx context.Context, limit int) ([]*domain.ReminderCandidate, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ReminderCandidate), args.Error(1)
}
func (m *MockConversationRepo) MarkReminderSent(ctx context.Context, conversationID string) error {
	return m.Called(ctx, conversationID).Error(0)
}

// IncrementCost is a side-effect method most ProcessAI tests don't care
// about; return nil unless a test explicitly registers an expectation so
// existing setups don't all need a new .On(...) line.
func (m *MockConversationRepo) IncrementCost(ctx context.Context, conversationID, accountID string, costUSD float64) error {
	for _, c := range m.ExpectedCalls {
		if c.Method == "IncrementCost" {
			return m.Called(ctx, conversationID, accountID, costUSD).Error(0)
		}
	}
	return nil
}

type MockTxManager struct{ mock.Mock }

func (m *MockTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type MockAuditLogRepo struct{ mock.Mock }

func (m *MockAuditLogRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	return m.Called(ctx, log).Error(0)
}
func (m *MockAuditLogRepo) ListByAccount(ctx context.Context, accountID string, limit, offset int32) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, accountID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}
func (m *MockAuditLogRepo) CountByAccount(ctx context.Context, accountID string) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockAuditLogRepo) ListFiltered(ctx context.Context, f domain.AuditLogFilter) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}
func (m *MockAuditLogRepo) CountFiltered(ctx context.Context, f domain.AuditLogFilter) (int64, error) {
	args := m.Called(ctx, f)
	return args.Get(0).(int64), args.Error(1)
}

type MockKnowledgeRepo struct{ mock.Mock }

func (m *MockKnowledgeRepo) CreateSource(ctx context.Context, source *domain.KnowledgeSource) error {
	return m.Called(ctx, source).Error(0)
}
func (m *MockKnowledgeRepo) GetSource(ctx context.Context, id string, agentID string) (*domain.KnowledgeSource, error) {
	args := m.Called(ctx, id, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KnowledgeSource), args.Error(1)
}
func (m *MockKnowledgeRepo) GetSourceByID(ctx context.Context, id string) (*domain.KnowledgeSource, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KnowledgeSource), args.Error(1)
}
func (m *MockKnowledgeRepo) GetChunkByID(ctx context.Context, id string) (*domain.KnowledgeChunk, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KnowledgeChunk), args.Error(1)
}
func (m *MockKnowledgeRepo) ListSources(ctx context.Context, agentID string) ([]*domain.KnowledgeSource, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.KnowledgeSource), args.Error(1)
}
func (m *MockKnowledgeRepo) UpdateSource(ctx context.Context, source *domain.KnowledgeSource) error {
	return m.Called(ctx, source).Error(0)
}
func (m *MockKnowledgeRepo) DeleteSource(ctx context.Context, id string, agentID string) error {
	return m.Called(ctx, id, agentID).Error(0)
}
func (m *MockKnowledgeRepo) CreateChunk(ctx context.Context, chunk *domain.KnowledgeChunk) error {
	return m.Called(ctx, chunk).Error(0)
}
func (m *MockKnowledgeRepo) Search(ctx context.Context, sourceID string, embedding []float32, limit int32) ([]*domain.KnowledgeChunk, error) {
	args := m.Called(ctx, sourceID, embedding, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.KnowledgeChunk), args.Error(1)
}
func (m *MockKnowledgeRepo) ListChunksBySource(ctx context.Context, sourceID string) ([]*domain.KnowledgeChunk, error) {
	args := m.Called(ctx, sourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.KnowledgeChunk), args.Error(1)
}
func (m *MockKnowledgeRepo) UpdateEmbedding(ctx context.Context, chunkID string, embedding []float32, provider string) error {
	return m.Called(ctx, chunkID, embedding, provider).Error(0)
}
func (m *MockKnowledgeRepo) DeleteChunk(ctx context.Context, chunkID string) error {
	return m.Called(ctx, chunkID).Error(0)
}

type MockEmailService struct{ mock.Mock }

func (m *MockEmailService) SendPasswordResetEmail(to string, resetToken string, appBaseURL, locale string) error {
	return m.Called(to, resetToken, appBaseURL, locale).Error(0)
}
func (m *MockEmailService) SendVerificationEmail(to string, verificationToken string, appBaseURL, locale string) error {
	return m.Called(to, verificationToken, appBaseURL, locale).Error(0)
}
func (m *MockEmailService) SendWelcomeEmail(to, userName, appURL, locale string) error {
	return m.Called(to, userName, appURL, locale).Error(0)
}
func (m *MockEmailService) SendUsageWarningEmail(to, userName string, usagePercent int, planName string, upgradeURL, locale string) error {
	return m.Called(to, userName, usagePercent, planName, upgradeURL, locale).Error(0)
}
func (m *MockEmailService) SendUsageLimitEmail(to, userName string, planName string, upgradeURL, locale string) error {
	return m.Called(to, userName, planName, upgradeURL, locale).Error(0)
}
func (m *MockEmailService) SendPaymentFailedEmail(to, userName, planName, retryURL, locale string) error {
	return m.Called(to, userName, planName, retryURL, locale).Error(0)
}
func (m *MockEmailService) SendPrebillingEmail(to, userName, planName string, daysUntil int, manageURL, locale string) error {
	return m.Called(to, userName, planName, daysUntil, manageURL, locale).Error(0)
}
func (m *MockEmailService) SendTeamInviteEmail(to string, inviterName string, resetToken string, appBaseURL, locale string) error {
	return m.Called(to, inviterName, resetToken, appBaseURL, locale).Error(0)
}
func (m *MockEmailService) SendTestEmail(to, userName string) error {
	return m.Called(to, userName).Error(0)
}
func (m *MockEmailService) SendReplyReminderEmail(to, attendantName, contactName, conversationURL string, minutesWaiting int) error {
	return m.Called(to, attendantName, contactName, conversationURL, minutesWaiting).Error(0)
}

type MockCalendarAdapter struct{ mock.Mock }

func (m *MockCalendarAdapter) ListFreeSlots(ctx context.Context, accountID, calendarID string, date time.Time) ([]time.Time, error) {
	args := m.Called(ctx, accountID, calendarID, date)
	return args.Get(0).([]time.Time), args.Error(1)
}
func (m *MockCalendarAdapter) CreateAppointment(ctx context.Context, accountID, calendarID string, start, end time.Time, attendeeEmail string) error {
	return m.Called(ctx, accountID, calendarID, start, end, attendeeEmail).Error(0)
}

type MockCalendarRepo struct{ mock.Mock }

func (m *MockCalendarRepo) Create(ctx context.Context, cal *domain.Calendar) error {
	return m.Called(ctx, cal).Error(0)
}
func (m *MockCalendarRepo) GetByID(ctx context.Context, id string) (*domain.Calendar, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Calendar), args.Error(1)
}
func (m *MockCalendarRepo) ListByAgentID(ctx context.Context, agentID string) ([]*domain.Calendar, error) {
	args := m.Called(ctx, agentID)
	return args.Get(0).([]*domain.Calendar), args.Error(1)
}
func (m *MockCalendarRepo) Update(ctx context.Context, cal *domain.Calendar) error {
	return m.Called(ctx, cal).Error(0)
}
func (m *MockCalendarRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type MockAIFactory struct{ mock.Mock }

func (m *MockAIFactory) GetProvider(ctx context.Context, providerName string) (domain.AIProvider, error) {
	args := m.Called(ctx, providerName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(domain.AIProvider), args.Error(1)
}
func (m *MockAIFactory) GetProviderForAccount(ctx context.Context, providerName string, account *domain.Account, plan *domain.Plan) (domain.AIProvider, error) {
	return m.GetProvider(ctx, providerName)
}
func (m *MockAIFactory) GetCostConfig(provider, model string) (float64, float64) {
	args := m.Called(provider, model)
	return args.Get(0).(float64), args.Get(1).(float64)
}

type MockJWTService struct{ mock.Mock }

func (m *MockJWTService) GenerateToken(user *domain.User) (string, error) {
	args := m.Called(user)
	return args.String(0), args.Error(1)
}
func (m *MockJWTService) ValidateToken(token string) (*domain.UserClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserClaims), args.Error(1)
}
func (m *MockJWTService) GenerateImpersonationToken(claims domain.ImpersonationClaims) (string, error) {
	args := m.Called(claims)
	return args.String(0), args.Error(1)
}
func (m *MockJWTService) GenerateMfaLoginToken(userID string) (string, error) {
	args := m.Called(userID)
	return args.String(0), args.Error(1)
}
func (m *MockJWTService) ValidateMfaLoginToken(token string) (*domain.MfaLoginClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MfaLoginClaims), args.Error(1)
}

type MockPaymentAdapter struct{ mock.Mock }

func (m *MockPaymentAdapter) CreateCheckoutSession(priceID, accountID, customerID, successURL, cancelURL, promoID string) (string, error) {
	args := m.Called(priceID, accountID, customerID, successURL, cancelURL, promoID)
	return args.String(0), args.Error(1)
}
func (m *MockPaymentAdapter) EnsureCustomer(accountID string) (string, error) {
	args := m.Called(accountID)
	return args.String(0), args.Error(1)
}
func (m *MockPaymentAdapter) CreateAddonCheckoutSession(packID, packName string, priceInCents int64, accountID, currency, successURL, cancelURL string) (string, error) {
	args := m.Called(packID, packName, priceInCents, accountID, currency, successURL, cancelURL)
	return args.String(0), args.Error(1)
}
func (m *MockPaymentAdapter) CreateBillingPortalSession(customerID, returnURL string) (string, error) {
	args := m.Called(customerID, returnURL)
	return args.String(0), args.Error(1)
}
func (m *MockPaymentAdapter) GetPromotionCodeRedeemed(promoID string) (int, error) {
	args := m.Called(promoID)
	return args.Int(0), args.Error(1)
}
func (m *MockPaymentAdapter) ReportMeteredUsage(subscriptionID, meteredPriceID string, quantity int64, idempotencyKey string) error {
	args := m.Called(subscriptionID, meteredPriceID, quantity, idempotencyKey)
	return args.Error(0)
}
func (m *MockPaymentAdapter) GrantReferralCredit(customerID string, amountCents int64, currency, description, idempotencyKey string) error {
	args := m.Called(customerID, amountCents, currency, description, idempotencyKey)
	return args.Error(0)
}
func (m *MockPaymentAdapter) GetSubscriptionPriceID(subscriptionID string) (string, error) {
	args := m.Called(subscriptionID)
	return args.String(0), args.Error(1)
}

// MockReferralRepository implements domain.ReferralRepository (US-174).
type MockReferralRepository struct{ mock.Mock }

func (m *MockReferralRepository) GetOrCreateCode(ctx context.Context, accountID string) (string, error) {
	args := m.Called(ctx, accountID)
	return args.String(0), args.Error(1)
}
func (m *MockReferralRepository) ResolveReferrer(ctx context.Context, code string) (string, error) {
	args := m.Called(ctx, code)
	return args.String(0), args.Error(1)
}
func (m *MockReferralRepository) LinkReferral(ctx context.Context, refereeAccountID, referrerAccountID string) error {
	args := m.Called(ctx, refereeAccountID, referrerAccountID)
	return args.Error(0)
}
func (m *MockReferralRepository) GetPendingByReferee(ctx context.Context, refereeAccountID string) (*domain.Referral, error) {
	args := m.Called(ctx, refereeAccountID)
	r, _ := args.Get(0).(*domain.Referral)
	return r, args.Error(1)
}
func (m *MockReferralRepository) MarkRewarded(ctx context.Context, id string, amountCents int64, currency string, at time.Time) error {
	args := m.Called(ctx, id, amountCents, currency, at)
	return args.Error(0)
}
func (m *MockReferralRepository) StatsByReferrer(ctx context.Context, referrerAccountID string) (*domain.ReferralStats, error) {
	args := m.Called(ctx, referrerAccountID)
	s, _ := args.Get(0).(*domain.ReferralStats)
	return s, args.Error(1)
}

// MockSellerRefRepository implements domain.SellerRefRepository (US-003).
type MockSellerRefRepository struct{ mock.Mock }

func (m *MockSellerRefRepository) GetOrCreateSellerRefCode(ctx context.Context, userID string) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}
func (m *MockSellerRefRepository) ResolveSellerRef(ctx context.Context, code, platformAccountID string) (string, string, error) {
	args := m.Called(ctx, code, platformAccountID)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *MockSellerRefRepository) StampSellerAttribution(ctx context.Context, accountID, sellerUserID string) error {
	args := m.Called(ctx, accountID, sellerUserID)
	return args.Error(0)
}

type MockCouponRepo struct{ mock.Mock }

func (m *MockCouponRepo) Insert(ctx context.Context, c *domain.Coupon) error {
	return m.Called(ctx, c).Error(0)
}
func (m *MockCouponRepo) GetByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	a := m.Called(ctx, code)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Coupon), a.Error(1)
}
func (m *MockCouponRepo) GetByID(ctx context.Context, id string) (*domain.Coupon, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Coupon), a.Error(1)
}
func (m *MockCouponRepo) List(ctx context.Context, limit, offset int) ([]*domain.Coupon, error) {
	a := m.Called(ctx, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.Coupon), a.Error(1)
}
func (m *MockCouponRepo) SetActive(ctx context.Context, id string, active bool) error {
	return m.Called(ctx, id, active).Error(0)
}
func (m *MockCouponRepo) BumpRedeemed(ctx context.Context, id string, count int) error {
	return m.Called(ctx, id, count).Error(0)
}

type MockAnalyticsRepo struct{ mock.Mock }

func (m *MockAnalyticsRepo) GetConversationStats(ctx context.Context, accountID string) (*domain.ConversationStats, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ConversationStats), args.Error(1)
}
func (m *MockAnalyticsRepo) GetMessageStats(ctx context.Context, accountID string) (*domain.MessageStats, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MessageStats), args.Error(1)
}
func (m *MockAnalyticsRepo) GetDailyConversationCounts(ctx context.Context, accountID string) ([]domain.DailyCount, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.DailyCount), args.Error(1)
}
func (m *MockAnalyticsRepo) GetSLAMetrics(ctx context.Context, accountID string) (*domain.SLAMetrics, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SLAMetrics), args.Error(1)
}
func (m *MockAnalyticsRepo) GetAgentPerformance(ctx context.Context, accountID string) ([]domain.AgentPerformance, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AgentPerformance), args.Error(1)
}
func (m *MockAnalyticsRepo) GetAIPerformance(ctx context.Context, accountID string) (*domain.AIPerformance, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AIPerformance), args.Error(1)
}
func (m *MockAnalyticsRepo) GetAttendantPerformance(ctx context.Context, f domain.AttendantPerformanceFilter) ([]domain.AttendantPerformance, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AttendantPerformance), args.Error(1)
}
func (m *MockAnalyticsRepo) GetAttendantLiveSnapshot(ctx context.Context, accountID string) ([]domain.AttendantLiveRow, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AttendantLiveRow), args.Error(1)
}
func (m *MockAnalyticsRepo) GetLiveQueueSnapshot(ctx context.Context, accountID string) (*domain.LiveQueue, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LiveQueue), args.Error(1)
}
func (m *MockAnalyticsRepo) GetOverview(ctx context.Context, accountID string, days int) (*domain.AnalyticsOverview, error) {
	args := m.Called(ctx, accountID, days)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AnalyticsOverview), args.Error(1)
}
func (m *MockAnalyticsRepo) ListIdleForCSAT(ctx context.Context, idleMinutes, limit int) ([]*domain.IdleConversationForCSAT, error) {
	args := m.Called(ctx, idleMinutes, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.IdleConversationForCSAT), args.Error(1)
}
func (m *MockAnalyticsRepo) GetTopExpensiveConversations(ctx context.Context, accountID string, limit int) ([]*domain.ExpensiveConversation, error) {
	args := m.Called(ctx, accountID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ExpensiveConversation), args.Error(1)
}

type MockAIProvider struct{ mock.Mock }

func (m *MockAIProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	args := m.Called(ctx, systemPrompt, history, tools)
	return args.Get(0).(domain.StreamResult), args.Error(1)
}
func (m *MockAIProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	args := m.Called(ctx, systemPrompt, history, tools)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AICallbackResponse), args.Error(1)
}
func (m *MockAIProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

type MockStripeEventRepo struct{ mock.Mock }

func (m *MockStripeEventRepo) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	args := m.Called(ctx, eventID)
	return args.Bool(0), args.Error(1)
}
func (m *MockStripeEventRepo) MarkProcessed(ctx context.Context, eventID string, eventType string) error {
	return m.Called(ctx, eventID, eventType).Error(0)
}
func (m *MockStripeEventRepo) ClaimEvent(ctx context.Context, eventID string, eventType string, payload []byte) (bool, error) {
	args := m.Called(ctx, eventID, eventType, payload)
	return args.Bool(0), args.Error(1)
}
func (m *MockStripeEventRepo) ListStuckEvents(ctx context.Context) ([]domain.StripeEventRecord, error) {
	args := m.Called(ctx)
	if r := args.Get(0); r != nil {
		return r.([]domain.StripeEventRecord), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStripeEventRepo) CleanupOldEvents(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

type MockIngestionJobRepo struct{ mock.Mock }

func (m *MockIngestionJobRepo) Create(ctx context.Context, job *domain.IngestionJob) error {
	return m.Called(ctx, job).Error(0)
}
func (m *MockIngestionJobRepo) GetByID(ctx context.Context, id, accountID string) (*domain.IngestionJob, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IngestionJob), args.Error(1)
}
func (m *MockIngestionJobRepo) ListBySource(ctx context.Context, sourceID, accountID string, limit int32) ([]*domain.IngestionJob, error) {
	args := m.Called(ctx, sourceID, accountID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.IngestionJob), args.Error(1)
}
func (m *MockIngestionJobRepo) ClaimNext(ctx context.Context) (*domain.IngestionJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IngestionJob), args.Error(1)
}
func (m *MockIngestionJobRepo) Complete(ctx context.Context, id string, chunksCreated int32) error {
	return m.Called(ctx, id, chunksCreated).Error(0)
}
func (m *MockIngestionJobRepo) Fail(ctx context.Context, id, errorMessage string) error {
	return m.Called(ctx, id, errorMessage).Error(0)
}

type MockUsageChecker struct{ mock.Mock }

func (m *MockUsageChecker) Check(account *domain.Account, plan *domain.Plan, usage *domain.AccountUsage, kind string) error {
	return m.Called(plan, usage, kind).Error(0)
}
func (m *MockUsageChecker) CheckAllowance(ctx context.Context, accountID string, kind string) error {
	return m.Called(ctx, accountID, kind).Error(0)
}
func (m *MockUsageChecker) CheckAccountActive(account *domain.Account) error {
	return m.Called(account).Error(0)
}
