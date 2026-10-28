package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestGetMeUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should return user and account details", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		role := "admin"
		planID := "plan1"
		user := &domain.User{
			ID:        "user1",
			AccountID: "acc1",
			Name:      "John Doe",
			Email:     "john@example.com",
			Role:      &role,
		}
		account := &domain.Account{
			ID:                 "acc1",
			Name:               "Acme Corp",
			PlanID:             &planID,
			SubscriptionStatus: "active",
		}

		userRepo.On("GetByID", ctx, "user1").Return(user, nil)
		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		planRepo.On("GetByID", ctx, "plan1").Return(&domain.Plan{ID: "plan1", Name: "Bronze"}, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		resp, err := uc.Execute(ctx, "user1", "acc1")

		assert.NoError(t, err)
		assert.Equal(t, "user1", resp.User.ID)
		assert.Equal(t, "John Doe", resp.User.Name)
		assert.Equal(t, "john@example.com", resp.User.Email)
		assert.Equal(t, "admin", resp.User.Role)
		assert.Equal(t, "acc1", resp.Account.ID)
		assert.Equal(t, "Acme Corp", resp.Account.Name)
		assert.Equal(t, "active", resp.Account.SubscriptionStatus)
		assert.False(t, resp.Account.IsTrialing)
		assert.Equal(t, "Bronze", resp.Account.PlanName)
		userRepo.AssertExpectations(t)
		accountRepo.AssertExpectations(t)
		planRepo.AssertExpectations(t)
	})

	t.Run("is_platform_account true only for the configured platform account", func(t *testing.T) {
		role := "attendant"
		newUC := func(accountID string) *GetMeUseCase {
			userRepo := new(MockUserRepo)
			accountRepo := new(MockAccountRepo)
			planRepo := new(MockPlanRepo)
			userRepo.On("GetByID", ctx, "u").Return(&domain.User{ID: "u", AccountID: accountID, Role: &role}, nil)
			accountRepo.On("GetByID", ctx, accountID).Return(&domain.Account{
				ID: accountID, SubscriptionStatus: "active",
			}, nil)
			return NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		}

		uc := newUC("acc-platform")
		uc.SetPlatformAccountID("acc-platform")
		resp, err := uc.Execute(ctx, "u", "acc-platform")
		assert.NoError(t, err)
		assert.True(t, resp.IsPlatformAccount, "platform account must be flagged")

		uc = newUC("acc-customer")
		uc.SetPlatformAccountID("acc-platform")
		resp, err = uc.Execute(ctx, "u", "acc-customer")
		assert.NoError(t, err)
		assert.False(t, resp.IsPlatformAccount, "regular accounts must not be flagged")

		// PLATFORM_ACCOUNT_ID unset ⇒ never flagged.
		uc = newUC("acc-platform")
		resp, err = uc.Execute(ctx, "u", "acc-platform")
		assert.NoError(t, err)
		assert.False(t, resp.IsPlatformAccount, "unset platform id must disable the flag")
	})

	t.Run("is_seller requires the explicit flag (US-014)", func(t *testing.T) {
		role := "attendant"
		newUC := func() *GetMeUseCase {
			userRepo := new(MockUserRepo)
			accountRepo := new(MockAccountRepo)
			userRepo.On("GetByID", ctx, "u").Return(&domain.User{ID: "u", AccountID: "acc-platform", Role: &role}, nil)
			accountRepo.On("GetByID", ctx, "acc-platform").Return(&domain.Account{
				ID: "acc-platform", SubscriptionStatus: "active",
			}, nil)
			uc := NewGetMeUseCase(userRepo, accountRepo, new(MockPlanRepo), nil)
			uc.SetPlatformAccountID("acc-platform")
			return uc
		}

		// No checker wired ⇒ false.
		resp, err := newUC().Execute(ctx, "u", "acc-platform")
		assert.NoError(t, err)
		assert.False(t, resp.IsSeller)

		// Flagged seller ⇒ true.
		uc := newUC()
		uc.SetSellerFlagChecker(&mockSellerFlags{sellers: map[string]bool{"u": true}})
		resp, err = uc.Execute(ctx, "u", "acc-platform")
		assert.NoError(t, err)
		assert.True(t, resp.IsSeller)

		// Platform user WITHOUT the flag ⇒ false (regular attendant is
		// not a payable seller).
		uc = newUC()
		uc.SetSellerFlagChecker(&mockSellerFlags{sellers: map[string]bool{}})
		resp, err = uc.Execute(ctx, "u", "acc-platform")
		assert.NoError(t, err)
		assert.False(t, resp.IsSeller)
	})

	t.Run("trial_bypass account is not reported as trialing", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)
		role := "admin"
		planID := "plan-ent"
		userRepo.On("GetByID", ctx, "u").Return(&domain.User{ID: "u", AccountID: "a", Role: &role}, nil)
		accountRepo.On("GetByID", ctx, "a").Return(&domain.Account{
			ID: "a", PlanID: &planID,
			SubscriptionStatus: domain.SubscriptionStatusTrialing,
			TrialBypass:        true,
		}, nil)
		// Contract plan (no Stripe price) — also exempt.
		planRepo.On("GetByID", ctx, "plan-ent").Return(&domain.Plan{ID: "plan-ent", Name: "Enterprise"}, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		resp, err := uc.Execute(ctx, "u", "a")
		assert.NoError(t, err)
		assert.False(t, resp.Account.IsTrialing, "trial_bypass/contract must not read as trialing")
		assert.True(t, resp.Account.TrialBypass, "trial_bypass must be exposed on /me")
	})

	t.Run("super-admin account auto-bypasses the trial gate", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)
		role := "admin"
		userRepo.On("GetByID", ctx, "u").Return(
			&domain.User{ID: "u", AccountID: "a", Email: "ops@replider.com", Role: &role}, nil)
		// Expired trial, no plan, no explicit bypass — but the email is on
		// the super-admin allowlist, so /me must not read as trialing.
		accountRepo.On("GetByID", ctx, "a").Return(&domain.Account{
			ID: "a", SubscriptionStatus: domain.SubscriptionStatusTrialing,
		}, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		uc.SetSuperAdminEmails([]string{"OPS@replider.com"}) // case-insensitive
		resp, err := uc.Execute(ctx, "u", "a")
		assert.NoError(t, err)
		assert.False(t, resp.Account.IsTrialing, "super-admin must not read as trialing")
		assert.True(t, resp.Account.TrialBypass, "super-admin must read as bypassed")
	})

	t.Run("should return ErrAccountNotFound when account does not exist", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		accountRepo.On("GetByID", ctx, "bad-acc").Return(nil, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		resp, err := uc.Execute(ctx, "user1", "bad-acc")

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrAccountNotFound, err)
	})

	t.Run("should return ErrUnauthorized when user not found", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		account := &domain.Account{ID: "acc1", Name: "Acme"}
		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		userRepo.On("GetByID", ctx, "bad-user").Return(nil, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		resp, err := uc.Execute(ctx, "bad-user", "acc1")

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrUnauthorized, err)
	})

	t.Run("ResolvePlanLimits should return plan caps for self-serve account", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		planID := "plan-pro"
		stripeID := "price_pro"
		account := &domain.Account{
			ID:     "acc-pro",
			PlanID: &planID,
		}
		plan := &domain.Plan{
			ID:                           planID,
			Name:                         "Pro",
			StripePriceID:                &stripeID,
			MaxAgents:                    5,
			MaxChannels:                  8,
			MaxSeats:                     5,
			MaxConversationsPerMonth:     2000,
			MaxAITokensPerMonth:          800000,
			MaxStorageMB:                 102400,
			MaxBroadcastMessagesPerMonth: 0,
			MaxRecipientsPerCampaign:     0,
			OverageConversationCents:     50,
			OverageCapMultiplier:         2.0,
		}

		accountRepo.On("GetByID", ctx, "acc-pro").Return(account, nil)
		planRepo.On("GetByID", ctx, planID).Return(plan, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		limits, err := uc.ResolvePlanLimits(ctx, "acc-pro")

		assert.NoError(t, err)
		assert.NotNil(t, limits)
		assert.Equal(t, "Pro", limits.PlanName)
		assert.False(t, limits.IsContractPlan)
		assert.Equal(t, 5, limits.MaxAgents)
		assert.Equal(t, 8, limits.MaxChannels)
		assert.Equal(t, 2000, limits.MaxConversationsPerMonth)
		assert.Equal(t, int64(800000), limits.MaxAITokensPerMonth)
		// Self-serve plan with broadcast = 0: feature disabled, NOT unlimited.
		assert.Equal(t, 0, limits.MaxBroadcastMessagesPerMonth)
		assert.Equal(t, 50, limits.OverageConversationCents)
		assert.Equal(t, 2.0, limits.OverageCapMultiplier)
	})

	t.Run("ResolvePlanLimits should resolve overrides on contract plan", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		planID := "plan-enterprise"
		agentsOverride := 25
		broadcastOverride := 100000
		account := &domain.Account{
			ID:                                   "acc-ent",
			PlanID:                               &planID,
			MaxAgentsOverride:                    &agentsOverride,
			MaxBroadcastMessagesPerMonthOverride: &broadcastOverride,
		}
		plan := &domain.Plan{
			ID:            planID,
			Name:          "Enterprise",
			StripePriceID: nil, // contract plan
			// All zeros — contract row, overrides resolve.
		}

		accountRepo.On("GetByID", ctx, "acc-ent").Return(account, nil)
		planRepo.On("GetByID", ctx, planID).Return(plan, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		limits, err := uc.ResolvePlanLimits(ctx, "acc-ent")

		assert.NoError(t, err)
		assert.True(t, limits.IsContractPlan)
		assert.Equal(t, 25, limits.MaxAgents)
		assert.Equal(t, 100000, limits.MaxBroadcastMessagesPerMonth)
		// No override on channels/seats: contract plan with 0 → -1 (unlimited sentinel).
		assert.Equal(t, domain.CeilingChannels, limits.MaxChannels)
		assert.Equal(t, domain.CeilingSeats, limits.MaxSeats)
		assert.Equal(t, domain.CeilingConversationsPerMonth, limits.MaxConversationsPerMonth)
	})

	t.Run("ResolvePlanLimits should fall back to TrialPlan when account has no plan", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		account := &domain.Account{ID: "acc-trial", PlanID: nil}
		accountRepo.On("GetByID", ctx, "acc-trial").Return(account, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		limits, err := uc.ResolvePlanLimits(ctx, "acc-trial")

		assert.NoError(t, err)
		assert.Equal(t, "Trial", limits.PlanName)
		assert.False(t, limits.IsContractPlan)
		assert.Equal(t, 1, limits.MaxAgents)
		assert.Equal(t, 50, limits.MaxConversationsPerMonth)
		assert.Equal(t, int64(10000), limits.MaxAITokensPerMonth)
	})

	t.Run("should default role to attendant when role is nil", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		user := &domain.User{
			ID:    "user1",
			Name:  "Jane",
			Email: "jane@example.com",
			Role:  nil,
		}
		account := &domain.Account{ID: "acc1", Name: "Acme", SubscriptionStatus: "trialing"}

		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)

		uc := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
		resp, err := uc.Execute(ctx, "user1", "acc1")

		assert.NoError(t, err)
		assert.Equal(t, "attendant", resp.User.Role)
		assert.True(t, resp.Account.IsTrialing)
		assert.Equal(t, "Trial", resp.Account.PlanName)
	})
}
