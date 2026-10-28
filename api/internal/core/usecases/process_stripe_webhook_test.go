package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stripe/stripe-go/v76"
)

// newStripeWebhookUC creates a ProcessStripeWebhookUseCase with a pass-through idempotency mock.
// ClaimEvent returns (true, nil) so every test event is treated as a
// fresh claim; tests that need to assert dedup override this return.
func newStripeWebhookUC(accountRepo *MockAccountRepo, usageRepo *MockUsageRepo) *ProcessStripeWebhookUseCase {
	stripeEventRepo := new(MockStripeEventRepo)
	stripeEventRepo.On("ClaimEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	// Success path stamps processed_at after the side-effects commit (US-011).
	stripeEventRepo.On("MarkProcessed", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
}

func TestProcessStripeWebhookUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("checkout.session.completed should activate subscription", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		sessionData := map[string]interface{}{
			"id":                  "cs_123",
			"client_reference_id": "acc1",
			"customer":            map[string]interface{}{"id": "cus_123"},
			"subscription":        map[string]interface{}{"id": "sub_123"},
		}
		raw, _ := json.Marshal(sessionData)

		event := stripe.Event{
			ID:   "evt_checkout_1",
			Type: "checkout.session.completed",
			Data: &stripe.EventData{Raw: raw},
		}

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_123"), ptrStr("sub_123"),
			string(domain.SubscriptionStatusActive),
		).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("checkout.session.completed should fail when client_reference_id is missing", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		sessionData := map[string]interface{}{
			"id":       "cs_123",
			"customer": map[string]interface{}{"id": "cus_123"},
		}
		raw, _ := json.Marshal(sessionData)

		event := stripe.Event{
			ID:   "evt_checkout_2",
			Type: "checkout.session.completed",
			Data: &stripe.EventData{Raw: raw},
		}

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing client reference ID")
	})

	t.Run("invoice.paid should reset usage for known subscription", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id":           "inv_123",
			"subscription": map[string]interface{}{"id": "sub_123"},
		}
		raw, _ := json.Marshal(invoiceData)

		event := stripe.Event{
			ID:   "evt_invoice_1",
			Type: "invoice.paid",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc1", SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		usageRepo.On("ResetUsage", ctx, "acc1").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
	})

	t.Run("invoice.paid should silently ignore unknown subscription", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id":           "inv_123",
			"subscription": map[string]interface{}{"id": "sub_unknown"},
		}
		raw, _ := json.Marshal(invoiceData)

		event := stripe.Event{
			ID:   "evt_invoice_2",
			Type: "invoice.paid",
			Data: &stripe.EventData{Raw: raw},
		}

		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_unknown").Return(nil, nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
	})

	t.Run("customer.subscription.deleted should cancel subscription", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{
			"id": "sub_123",
		}
		raw, _ := json.Marshal(subData)

		event := stripe.Event{
			ID:   "evt_sub_del_1",
			Type: "customer.subscription.deleted",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc1"}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			(*string)(nil), ptrStr("sub_123"), "canceled",
		).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("customer.subscription.deleted should silently ignore unknown subscription", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{
			"id": "sub_unknown",
		}
		raw, _ := json.Marshal(subData)

		event := stripe.Event{
			ID:   "evt_sub_del_2",
			Type: "customer.subscription.deleted",
			Data: &stripe.EventData{Raw: raw},
		}

		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_unknown").Return(nil, nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
	})

	t.Run("customer.subscription.deleted writes a churned billing event (US-071)", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{"id": "sub_123"}
		raw, _ := json.Marshal(subData)
		event := stripe.Event{
			ID:      "evt_sub_del_ledger",
			Type:    "customer.subscription.deleted",
			Created: 1717660800,
			Data:    &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc1"}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			(*string)(nil), ptrStr("sub_123"), "canceled",
		).Return(nil)

		billing := new(mockBillingEventRepo)
		var captured *domain.BillingEvent
		billing.On("Create", mock.Anything, mock.MatchedBy(func(e *domain.BillingEvent) bool {
			captured = e
			return true
		})).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetBillingEventRecorder(billing)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		billing.AssertExpectations(t)
		if assert.NotNil(t, captured) {
			assert.Equal(t, "acc1", captured.AccountID)
			assert.Equal(t, domain.BillingEventChurned, captured.EventType)
			assert.Equal(t, int64(0), captured.AmountCents)
		}
	})

	t.Run("invoice.paid writes a renewal billing event (US-071)", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id":           "inv_paid_ledger",
			"subscription": map[string]interface{}{"id": "sub_123"},
			"amount_paid":  9900,
			"currency":     "brl",
		}
		raw, _ := json.Marshal(invoiceData)
		event := stripe.Event{
			ID:      "evt_invoice_ledger",
			Type:    "invoice.paid",
			Created: 1717660800,
			Data:    &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc1", SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		usageRepo.On("ResetUsage", ctx, "acc1").Return(nil)

		billing := new(mockBillingEventRepo)
		var captured *domain.BillingEvent
		billing.On("Create", mock.Anything, mock.MatchedBy(func(e *domain.BillingEvent) bool {
			captured = e
			return true
		})).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetBillingEventRecorder(billing)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		billing.AssertExpectations(t)
		if assert.NotNil(t, captured) {
			assert.Equal(t, "acc1", captured.AccountID)
			assert.Equal(t, domain.BillingEventSubscriptionRenewed, captured.EventType)
			assert.Equal(t, int64(9900), captured.AmountCents)
			assert.Equal(t, "brl", captured.Currency)
		}
	})

	t.Run("invoice.payment_failed should mark account as past_due", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id":           "inv_fail",
			"subscription": map[string]interface{}{"id": "sub_123"},
		}
		raw, _ := json.Marshal(invoiceData)

		event := stripe.Event{
			ID:   "evt_fail_1",
			Type: "invoice.payment_failed",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc1", SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			account.StripeCustomerID, ptrStr("sub_123"), domain.SubscriptionStatusPastDue,
		).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("customer.subscription.updated should update status", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{
			"id":     "sub_123",
			"status": "past_due",
		}
		raw, _ := json.Marshal(subData)

		event := stripe.Event{
			ID:   "evt_sub_upd_1",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc1", SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			account.StripeCustomerID, ptrStr("sub_123"), domain.SubscriptionStatusPastDue,
		).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("unhandled event type should return nil", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		event := stripe.Event{
			ID:   "evt_unknown",
			Type: "payment_intent.succeeded",
			Data: &stripe.EventData{Raw: json.RawMessage(`{}`)},
		}

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
	})

	t.Run("duplicate event should be skipped", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		stripeEventRepo := new(MockStripeEventRepo)

		event := stripe.Event{
			ID:   "evt_duplicate",
			Type: "checkout.session.completed",
			Data: &stripe.EventData{Raw: json.RawMessage(`{}`)},
		}

		// ClaimEvent returns false → another concurrent retry already
		// claimed the event id. processEvent must not run.
		stripeEventRepo.On("ClaimEvent", ctx, "evt_duplicate", "checkout.session.completed", mock.Anything).Return(false, nil)

		uc := NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		// accountRepo should NOT have been called
		accountRepo.AssertNotCalled(t, "UpdateAccountSubscription")
		// US-012: the side effects must NOT run on a duplicate claim —
		// in particular no second addon credit for the same event id.
		usageRepo.AssertNotCalled(t, "AddAddonTokens", mock.Anything, mock.Anything, mock.Anything)
	})

	// ── Addon checkout flow ──────────────────────────────────────────────

	t.Run("checkout.session.completed with addon metadata should add tokens", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		sessionData := map[string]interface{}{
			"id":                  "cs_addon_1",
			"client_reference_id": "acc1",
			"metadata": map[string]interface{}{
				"type":    "addon",
				"pack_id": "addon_m",
			},
		}
		raw, _ := json.Marshal(sessionData)

		event := stripe.Event{
			ID:   "evt_addon_1",
			Type: "checkout.session.completed",
			Data: &stripe.EventData{Raw: raw},
		}

		usageRepo.On("AddAddonTokens", ctx, "acc1", int64(75000)).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		usageRepo.AssertExpectations(t)
		// Subscription-related repos should NOT be called for addon
		accountRepo.AssertNotCalled(t, "UpdateAccountSubscription")
	})

	t.Run("checkout.session.completed with invalid addon pack_id should return error", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		sessionData := map[string]interface{}{
			"id":                  "cs_addon_bad",
			"client_reference_id": "acc1",
			"metadata": map[string]interface{}{
				"type":    "addon",
				"pack_id": "addon_nonexistent",
			},
		}
		raw, _ := json.Marshal(sessionData)

		event := stripe.Event{
			ID:   "evt_addon_bad",
			Type: "checkout.session.completed",
			Data: &stripe.EventData{Raw: raw},
		}

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown addon pack ID")
	})

	// ── Invoice flows (additional edge cases) ──────────────────────────

	t.Run("invoice.paid should reactivate past_due subscription", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id":           "inv_reactivate",
			"subscription": map[string]interface{}{"id": "sub_pastdue"},
		}
		raw, _ := json.Marshal(invoiceData)

		event := stripe.Event{
			ID:   "evt_reactivate",
			Type: "invoice.paid",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{
			ID:                   "acc2",
			SubscriptionStatus:   domain.SubscriptionStatusPastDue,
			StripeCustomerID:     ptrStr("cus_456"),
			StripeSubscriptionID: ptrStr("sub_pastdue"),
		}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_pastdue").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc2",
			ptrStr("cus_456"), ptrStr("sub_pastdue"), domain.SubscriptionStatusActive,
		).Return(nil)
		usageRepo.On("ResetUsage", ctx, "acc2").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
	})

	t.Run("invoice.paid without subscription should return nil silently", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id": "inv_no_sub",
			// no subscription field
		}
		raw, _ := json.Marshal(invoiceData)

		event := stripe.Event{
			ID:   "evt_no_sub",
			Type: "invoice.paid",
			Data: &stripe.EventData{Raw: raw},
		}

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		// Nothing should be called
		accountRepo.AssertNotCalled(t, "GetAccountByStripeSubscription")
		usageRepo.AssertNotCalled(t, "ResetUsage")
	})

	// ── Subscription updated edge cases ────────────────────────────────

	t.Run("subscription.updated with unmapped Stripe status is skipped, not persisted", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{
			"id":     "sub_weird",
			"status": "future_unknown_status_xyz", // genuinely unmapped
		}
		raw, _ := json.Marshal(subData)

		event := stripe.Event{
			ID:   "evt_weird_status",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc3", SubscriptionStatus: "active"}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_weird").Return(account, nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		// Unmapped Stripe status must NOT be written to the column —
		// the usage guard wouldn't recognise it and the account could
		// silently bypass enforcement. Log + skip is the safe path.
		accountRepo.AssertNotCalled(t, "UpdateAccountSubscription")
	})

	t.Run("subscription.updated unpaid maps to canceled", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{"id": "sub_unpaid", "status": "unpaid"}
		raw, _ := json.Marshal(subData)
		event := stripe.Event{
			ID:   "evt_unpaid",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		subID := "sub_unpaid"
		customerID := "cus_xyz"
		account := &domain.Account{
			ID:                 "acc_unpaid",
			SubscriptionStatus: domain.SubscriptionStatusActive,
			StripeCustomerID:   &customerID,
		}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_unpaid").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc_unpaid", &customerID, &subID, domain.SubscriptionStatusCanceled).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("subscription.updated incomplete maps to past_due", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{"id": "sub_inc", "status": "incomplete"}
		raw, _ := json.Marshal(subData)
		event := stripe.Event{
			ID:   "evt_inc",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		subID := "sub_inc"
		customerID := "cus_inc"
		account := &domain.Account{
			ID:                 "acc_inc",
			SubscriptionStatus: domain.SubscriptionStatusActive,
			StripeCustomerID:   &customerID,
		}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_inc").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc_inc", &customerID, &subID, domain.SubscriptionStatusPastDue).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("subscription.updated incomplete_expired maps to canceled", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{"id": "sub_exp", "status": "incomplete_expired"}
		raw, _ := json.Marshal(subData)
		event := stripe.Event{
			ID:   "evt_exp",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		subID := "sub_exp"
		customerID := "cus_exp"
		account := &domain.Account{
			ID:                 "acc_exp",
			SubscriptionStatus: domain.SubscriptionStatusActive,
			StripeCustomerID:   &customerID,
		}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_exp").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc_exp", &customerID, &subID, domain.SubscriptionStatusCanceled).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("subscription.updated paused maps to domain paused", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{
			"id":     "sub_paused",
			"status": "paused",
		}
		raw, _ := json.Marshal(subData)

		event := stripe.Event{
			ID:   "evt_paused",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc_p", SubscriptionStatus: "active"}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_paused").Return(account, nil)
		accountRepo.On("UpdateAccountSubscription", ctx, "acc_p",
			account.StripeCustomerID, ptrStr("sub_paused"), domain.SubscriptionStatusPaused,
		).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("subscription.updated with same status should not call update", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		subData := map[string]interface{}{
			"id":     "sub_same",
			"status": "active",
		}
		raw, _ := json.Marshal(subData)

		event := stripe.Event{
			ID:   "evt_same_status",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{ID: "acc4", SubscriptionStatus: "active"}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_same").Return(account, nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		// Should NOT call update when status hasn't changed
		accountRepo.AssertNotCalled(t, "UpdateAccountSubscription")
	})

	// ── Idempotency edge case ──────────────────────────────────────────

	t.Run("idempotency DB error should fail so Stripe retries", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		stripeEventRepo := new(MockStripeEventRepo)

		// DB error on ClaimEvent — return error so the webhook
		// handler responds 5xx and Stripe retries with at-least-once
		// delivery. Continuing on transient DB error would risk
		// double-processing (no idempotency claim took effect).
		stripeEventRepo.On("ClaimEvent", ctx, "evt_db_err", "payment_intent.succeeded", mock.Anything).Return(false, assert.AnError)

		event := stripe.Event{
			ID:   "evt_db_err",
			Type: "payment_intent.succeeded",
			Data: &stripe.EventData{Raw: json.RawMessage(`{}`)},
		}

		uc := NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
		err := uc.Execute(ctx, event)

		assert.Error(t, err)
		accountRepo.AssertNotCalled(t, "UpdateAccountSubscription")
	})

	t.Run("invoice.paid should clear past_due and reactivate", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)

		invoiceData := map[string]interface{}{
			"id":           "inv_recovery",
			"subscription": map[string]interface{}{"id": "sub_pastdue2"},
		}
		raw, _ := json.Marshal(invoiceData)

		event := stripe.Event{
			ID:   "evt_recovery",
			Type: "invoice.paid",
			Data: &stripe.EventData{Raw: raw},
		}

		account := &domain.Account{
			ID:                   "acc5",
			SubscriptionStatus:   domain.SubscriptionStatusPastDue,
			StripeCustomerID:     ptrStr("cus_789"),
			StripeSubscriptionID: ptrStr("sub_pastdue2"),
		}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_pastdue2").Return(account, nil)
		// SQL clears past_due_at when status != past_due
		accountRepo.On("UpdateAccountSubscription", ctx, "acc5",
			ptrStr("cus_789"), ptrStr("sub_pastdue2"), domain.SubscriptionStatusActive,
		).Return(nil)
		usageRepo.On("ResetUsage", ctx, "acc5").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
	})
}

func ptrStr(s string) *string { return &s }

// TestProcessStripeWebhook_CrashMidProcessLeavesStuckClaim locks in US-011:
// when a handler fails AFTER claiming the event, Execute must return an
// error AND must NOT stamp processed_at — leaving the row as a stuck claim
// the reconciler can replay. The old code marked the event done on claim,
// so a crash silently dropped it forever.
func TestProcessStripeWebhook_CrashMidProcessLeavesStuckClaim(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	usageRepo := new(MockUsageRepo)
	stripeEventRepo := new(MockStripeEventRepo)

	// Checkout event missing client_reference_id → handler returns an error,
	// simulating a mid-process failure.
	event := stripe.Event{
		ID:   "evt_crash",
		Type: "checkout.session.completed",
		Data: &stripe.EventData{Raw: json.RawMessage(`{"id":"cs_x"}`)},
	}

	stripeEventRepo.On("ClaimEvent", ctx, "evt_crash", "checkout.session.completed", mock.Anything).Return(true, nil)

	uc := NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
	err := uc.Execute(ctx, event)

	assert.Error(t, err)
	// The decisive assertion: the event was NOT marked processed, so the
	// stuck claim survives for the reconciler / Stripe retry.
	stripeEventRepo.AssertNotCalled(t, "MarkProcessed", mock.Anything, mock.Anything, mock.Anything)
}

// TestProcessStripeWebhook_SuccessMarksProcessed confirms the happy path
// stamps processed_at after the side-effects commit.
func TestProcessStripeWebhook_SuccessMarksProcessed(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	usageRepo := new(MockUsageRepo)
	stripeEventRepo := new(MockStripeEventRepo)

	raw, _ := json.Marshal(map[string]interface{}{
		"id":                  "cs_ok",
		"client_reference_id": "acc_ok",
		"customer":            map[string]interface{}{"id": "cus_ok"},
		"subscription":        map[string]interface{}{"id": "sub_ok"},
	})
	event := stripe.Event{ID: "evt_ok", Type: "checkout.session.completed", Data: &stripe.EventData{Raw: raw}}

	stripeEventRepo.On("ClaimEvent", ctx, "evt_ok", "checkout.session.completed", mock.Anything).Return(true, nil)
	accountRepo.On("UpdateAccountSubscription", ctx, "acc_ok", ptrStr("cus_ok"), ptrStr("sub_ok"), domain.SubscriptionStatusActive).Return(nil)
	stripeEventRepo.On("MarkProcessed", ctx, "evt_ok", "checkout.session.completed").Return(nil)

	uc := NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
	err := uc.Execute(ctx, event)

	assert.NoError(t, err)
	stripeEventRepo.AssertExpectations(t)
}

// TestProcessStripeWebhook_ReconcileStuckReplaysEvent locks in the
// reconciler half of US-011: a stuck event is re-claimed, its stored
// payload replayed, and the row marked processed on success.
func TestProcessStripeWebhook_ReconcileStuckReplaysEvent(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	usageRepo := new(MockUsageRepo)
	stripeEventRepo := new(MockStripeEventRepo)

	raw, _ := json.Marshal(map[string]interface{}{
		"id":           "inv_stuck",
		"subscription": map[string]interface{}{"id": "sub_stuck"},
	})
	stored, _ := json.Marshal(stripe.Event{
		ID: "evt_stuck", Type: "invoice.paid", Data: &stripe.EventData{Raw: raw},
	})

	stripeEventRepo.On("ListStuckEvents", ctx).Return([]domain.StripeEventRecord{
		{EventID: "evt_stuck", EventType: "invoice.paid", Payload: stored},
	}, nil)
	stripeEventRepo.On("ClaimEvent", ctx, "evt_stuck", "invoice.paid", mock.Anything).Return(true, nil)

	account := &domain.Account{
		ID:                   "acc_stuck",
		SubscriptionStatus:   domain.SubscriptionStatusActive,
		StripeCustomerID:     ptrStr("cus_stuck"),
		StripeSubscriptionID: ptrStr("sub_stuck"),
	}
	accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_stuck").Return(account, nil)
	usageRepo.On("ResetUsage", ctx, "acc_stuck").Return(nil)
	stripeEventRepo.On("MarkProcessed", ctx, "evt_stuck", "invoice.paid").Return(nil)

	uc := NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
	recovered, err := uc.ReconcileStuck(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 1, recovered)
	stripeEventRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
}

// TestProcessStripeWebhook_ReconcileSkipsWhenReclaimLost confirms a stuck
// row grabbed by another worker (ClaimEvent=false) is not re-processed.
func TestProcessStripeWebhook_ReconcileSkipsWhenReclaimLost(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	usageRepo := new(MockUsageRepo)
	stripeEventRepo := new(MockStripeEventRepo)

	stored, _ := json.Marshal(stripe.Event{ID: "evt_taken", Type: "invoice.paid", Data: &stripe.EventData{Raw: json.RawMessage(`{}`)}})
	stripeEventRepo.On("ListStuckEvents", ctx).Return([]domain.StripeEventRecord{
		{EventID: "evt_taken", EventType: "invoice.paid", Payload: stored},
	}, nil)
	stripeEventRepo.On("ClaimEvent", ctx, "evt_taken", "invoice.paid", mock.Anything).Return(false, nil)

	uc := NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
	recovered, err := uc.ReconcileStuck(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 0, recovered)
	accountRepo.AssertNotCalled(t, "GetAccountByStripeSubscription")
	stripeEventRepo.AssertNotCalled(t, "MarkProcessed", mock.Anything, mock.Anything, mock.Anything)
}

// TestProcessStripeWebhook_PlanActivation (US-001, review-fixes) covers the
// launch blocker: a paid checkout must write accounts.plan_id (price → plan)
// and clear the trial deadline — before this fix the webhook only flipped
// subscription_status and the paying customer kept trial limits forever.
func TestProcessStripeWebhook_PlanActivation(t *testing.T) {
	ctx := context.Background()

	checkoutEvent := func(id string) stripe.Event {
		sessionData := map[string]interface{}{
			"id":                  "cs_" + id,
			"client_reference_id": "acc1",
			"customer":            map[string]interface{}{"id": "cus_123"},
			"subscription":        map[string]interface{}{"id": "sub_123"},
			"amount_total":        9900,
			"currency":            "usd",
		}
		raw, _ := json.Marshal(sessionData)
		return stripe.Event{
			ID:      "evt_" + id,
			Type:    "checkout.session.completed",
			Created: 1717660800,
			Data:    &stripe.EventData{Raw: raw},
		}
	}

	for _, tc := range []struct {
		name    string
		priceID string
		plan    *domain.Plan
	}{
		{
			name:    "BRL checkout activates the purchased plan",
			priceID: "price_core_brl",
			plan: &domain.Plan{ID: "plan_core_brl", Name: "Core", Currency: "brl",
				MaxAgents: 3, MaxConversationsPerMonth: 1000, MaxAITokensPerMonth: 500_000},
		},
		{
			name:    "USD checkout activates the purchased plan",
			priceID: "price_pro_usd",
			plan: &domain.Plan{ID: "plan_pro_usd", Name: "Pro", Currency: "usd",
				MaxAgents: 10, MaxConversationsPerMonth: 5000, MaxAITokensPerMonth: 2_000_000},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accountRepo := new(MockAccountRepo)
			usageRepo := new(MockUsageRepo)
			payment := new(MockPaymentAdapter)
			planRepo := new(MockPlanRepo)

			accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
				ptrStr("cus_123"), ptrStr("sub_123"), domain.SubscriptionStatusActive,
			).Return(nil)
			payment.On("GetSubscriptionPriceID", "sub_123").Return(tc.priceID, nil)
			planRepo.On("GetByStripePriceID", ctx, tc.priceID).Return(tc.plan, nil)
			accountRepo.On("SetPlan", ctx, "acc1", tc.plan.ID).Return(nil)
			accountRepo.On("ClearTrialExpiresAt", ctx, "acc1").Return(nil)

			uc := newStripeWebhookUC(accountRepo, usageRepo)
			uc.SetPlanActivator(planRepo, payment)
			err := uc.Execute(ctx, checkoutEvent("checkout_"+tc.plan.Currency))

			assert.NoError(t, err)
			accountRepo.AssertExpectations(t)
			planRepo.AssertExpectations(t)
			payment.AssertExpectations(t)

			// The activated plan's limits are the paid tier's, not the
			// trial floor the account was stuck on before the fix.
			acct := &domain.Account{ID: "acc1", PlanID: &tc.plan.ID}
			assert.Greater(t, acct.EffectiveMaxAgents(tc.plan), domain.TrialPlan().MaxAgents)
			assert.Greater(t, acct.EffectiveMaxConversationsPerMonth(tc.plan), domain.TrialPlan().MaxConversationsPerMonth)
		})
	}

	t.Run("unmapped price id ledgers an alert instead of silently keeping trial", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		payment := new(MockPaymentAdapter)
		planRepo := new(MockPlanRepo)
		billing := new(mockBillingEventRepo)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_123"), ptrStr("sub_123"), domain.SubscriptionStatusActive,
		).Return(nil)
		payment.On("GetSubscriptionPriceID", "sub_123").Return("price_orphan", nil)
		planRepo.On("GetByStripePriceID", ctx, "price_orphan").Return(nil, nil)
		// The generic subscription_created ledger row also runs (it looks
		// the account up for the plan-name snapshot).
		accountRepo.On("GetByID", ctx, "acc1").Return(&domain.Account{ID: "acc1"}, nil)

		var events []*domain.BillingEvent
		billing.On("Create", mock.Anything, mock.MatchedBy(func(e *domain.BillingEvent) bool {
			events = append(events, e)
			return true
		})).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, payment)
		uc.SetBillingEventRecorder(billing)
		err := uc.Execute(ctx, checkoutEvent("orphan"))

		// Webhook still succeeds — a retry can't fix bad price wiring.
		assert.NoError(t, err)
		accountRepo.AssertNotCalled(t, "SetPlan", mock.Anything, mock.Anything, mock.Anything)
		accountRepo.AssertNotCalled(t, "ClearTrialExpiresAt", mock.Anything, mock.Anything)

		var alert *domain.BillingEvent
		for _, e := range events {
			if e.EventType == domain.BillingEventPlanActivationFailed {
				alert = e
			}
		}
		if assert.NotNil(t, alert, "expected a plan_activation_failed alert row") {
			assert.Equal(t, "acc1", alert.AccountID)
			assert.Equal(t, int64(9900), alert.AmountCents)
			assert.Equal(t, "usd", alert.Currency)
		}
	})

	t.Run("transient stripe fetch failure returns error so the event retries", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		payment := new(MockPaymentAdapter)
		planRepo := new(MockPlanRepo)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_123"), ptrStr("sub_123"), domain.SubscriptionStatusActive,
		).Return(nil)
		payment.On("GetSubscriptionPriceID", "sub_123").Return("", assert.AnError)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, payment)
		err := uc.Execute(ctx, checkoutEvent("transient"))

		assert.Error(t, err)
		accountRepo.AssertNotCalled(t, "SetPlan", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("subscription.updated syncs plan from line-item price (reinforcement)", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)

		subData := map[string]interface{}{
			"id":     "sub_123",
			"status": "active",
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "si_1", "price": map[string]interface{}{"id": "price_pro_usd"}},
				},
			},
		}
		raw, _ := json.Marshal(subData)
		event := stripe.Event{
			ID:   "evt_sub_sync",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		trialPlanID := "__trial__"
		account := &domain.Account{ID: "acc1", PlanID: &trialPlanID,
			SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		plan := &domain.Plan{ID: "plan_pro_usd", Name: "Pro", Currency: "usd"}
		planRepo.On("GetByStripePriceID", ctx, "price_pro_usd").Return(plan, nil)
		accountRepo.On("SetPlan", ctx, "acc1", "plan_pro_usd").Return(nil)
		accountRepo.On("ClearTrialExpiresAt", ctx, "acc1").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, new(MockPaymentAdapter))
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		planRepo.AssertExpectations(t)
	})

	t.Run("subscription.updated is a no-op when plan already matches", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)

		subData := map[string]interface{}{
			"id":     "sub_123",
			"status": "active",
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "si_1", "price": map[string]interface{}{"id": "price_pro_usd"}},
				},
			},
		}
		raw, _ := json.Marshal(subData)
		event := stripe.Event{
			ID:   "evt_sub_sync_noop",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{Raw: raw},
		}

		planID := "plan_pro_usd"
		account := &domain.Account{ID: "acc1", PlanID: &planID,
			SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		planRepo.On("GetByStripePriceID", ctx, "price_pro_usd").
			Return(&domain.Plan{ID: "plan_pro_usd", Name: "Pro"}, nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, new(MockPaymentAdapter))
		err := uc.Execute(ctx, event)

		assert.NoError(t, err)
		accountRepo.AssertNotCalled(t, "SetPlan", mock.Anything, mock.Anything, mock.Anything)
	})

	// Self-serve upgrade signal (US-007 review fix): a Stripe-driven
	// paid→pricier-paid plan change must ledger 'plan_upgraded' — the
	// event the commission upgrade bonus (US-008) reads. Before the fix
	// only the owner-panel apply-contract path wrote it, so self-serve
	// upgrades never paid the bonus.
	subUpdatedEvent := func(id, priceID string) stripe.Event {
		subData := map[string]interface{}{
			"id":     "sub_123",
			"status": "active",
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "si_1", "price": map[string]interface{}{"id": priceID}},
				},
			},
		}
		raw, _ := json.Marshal(subData)
		return stripe.Event{
			ID:      "evt_" + id,
			Type:    "customer.subscription.updated",
			Created: 1717660800,
			Data:    &stripe.EventData{Raw: raw},
		}
	}
	upgradeEvents := func(billing *mockBillingEventRepo) *[]*domain.BillingEvent {
		var events []*domain.BillingEvent
		billing.On("Create", mock.Anything, mock.MatchedBy(func(e *domain.BillingEvent) bool {
			events = append(events, e)
			return true
		})).Return(nil)
		return &events
	}
	findUpgrade := func(events []*domain.BillingEvent) *domain.BillingEvent {
		for _, e := range events {
			if e.EventType == domain.BillingEventPlanUpgraded {
				return e
			}
		}
		return nil
	}

	t.Run("subscription.updated paid plan change ledgers plan_upgraded", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		billing := new(mockBillingEventRepo)

		corePlanID := "plan_core_usd"
		account := &domain.Account{ID: "acc1", PlanID: &corePlanID,
			SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		planRepo.On("GetByStripePriceID", ctx, "price_pro_usd").
			Return(&domain.Plan{ID: "plan_pro_usd", Name: "Pro", Currency: "usd", Price: 19900}, nil)
		planRepo.On("GetByID", ctx, "plan_core_usd").
			Return(&domain.Plan{ID: "plan_core_usd", Name: "Core", Currency: "usd", Price: 9900}, nil)
		accountRepo.On("SetPlan", ctx, "acc1", "plan_pro_usd").Return(nil)
		accountRepo.On("ClearTrialExpiresAt", ctx, "acc1").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, new(MockPaymentAdapter))
		uc.SetBillingEventRecorder(billing)
		events := upgradeEvents(billing)

		assert.NoError(t, uc.Execute(ctx, subUpdatedEvent("sub_upgrade", "price_pro_usd")))

		up := findUpgrade(*events)
		if assert.NotNil(t, up, "expected a plan_upgraded billing event") {
			assert.Equal(t, "acc1", up.AccountID)
			assert.Equal(t, "Pro", up.Plan)
			assert.Equal(t, int64(19900), up.AmountCents)
			assert.Equal(t, "usd", up.Currency)
		}
	})

	t.Run("subscription.updated trial-to-paid is a conversion, not an upgrade", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		billing := new(mockBillingEventRepo)

		trialPlanID := "__trial__"
		account := &domain.Account{ID: "acc1", PlanID: &trialPlanID,
			SubscriptionStatus: domain.SubscriptionStatusActive}
		accountRepo.On("GetAccountByStripeSubscription", ctx, "sub_123").Return(account, nil)
		planRepo.On("GetByStripePriceID", ctx, "price_pro_usd").
			Return(&domain.Plan{ID: "plan_pro_usd", Name: "Pro", Currency: "usd", Price: 19900}, nil)
		// Trial pseudo-plan: price 0 → conversion, not an upgrade.
		planRepo.On("GetByID", ctx, "__trial__").
			Return(&domain.Plan{ID: "__trial__", Name: "Trial", Currency: "usd", Price: 0}, nil)
		accountRepo.On("SetPlan", ctx, "acc1", "plan_pro_usd").Return(nil)
		accountRepo.On("ClearTrialExpiresAt", ctx, "acc1").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, new(MockPaymentAdapter))
		uc.SetBillingEventRecorder(billing)
		events := upgradeEvents(billing)

		assert.NoError(t, uc.Execute(ctx, subUpdatedEvent("sub_trial_conv", "price_pro_usd")))
		assert.Nil(t, findUpgrade(*events), "trial→paid must not ledger plan_upgraded")
	})

	t.Run("checkout upgrade from a cheaper paid plan ledgers plan_upgraded", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		payment := new(MockPaymentAdapter)
		planRepo := new(MockPlanRepo)
		billing := new(mockBillingEventRepo)

		corePlanID := "plan_core_usd"
		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_123"), ptrStr("sub_123"), domain.SubscriptionStatusActive,
		).Return(nil)
		payment.On("GetSubscriptionPriceID", "sub_123").Return("price_pro_usd", nil)
		planRepo.On("GetByStripePriceID", ctx, "price_pro_usd").
			Return(&domain.Plan{ID: "plan_pro_usd", Name: "Pro", Currency: "usd", Price: 19900}, nil)
		accountRepo.On("GetByID", ctx, "acc1").
			Return(&domain.Account{ID: "acc1", PlanID: &corePlanID}, nil)
		planRepo.On("GetByID", ctx, "plan_core_usd").
			Return(&domain.Plan{ID: "plan_core_usd", Name: "Core", Currency: "usd", Price: 9900}, nil)
		accountRepo.On("SetPlan", ctx, "acc1", "plan_pro_usd").Return(nil)
		accountRepo.On("ClearTrialExpiresAt", ctx, "acc1").Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetPlanActivator(planRepo, payment)
		uc.SetBillingEventRecorder(billing)
		events := upgradeEvents(billing)

		assert.NoError(t, uc.Execute(ctx, checkoutEvent("checkout_upgrade")))

		up := findUpgrade(*events)
		if assert.NotNil(t, up, "expected a plan_upgraded billing event") {
			assert.Equal(t, int64(19900), up.AmountCents)
			assert.Equal(t, "usd", up.Currency)
		}
	})
}
