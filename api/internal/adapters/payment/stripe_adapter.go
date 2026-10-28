package payment

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/stripe/stripe-go/v76"
	billingportalsession "github.com/stripe/stripe-go/v76/billingportal/session"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/coupon"
	"github.com/stripe/stripe-go/v76/customer"
	customerbalancetransaction "github.com/stripe/stripe-go/v76/customerbalancetransaction"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/invoiceitem"
	"github.com/stripe/stripe-go/v76/promotioncode"
	stripesub "github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/usagerecord"
	"github.com/stripe/stripe-go/v76/webhook"
)

// stripePinnedAPIVersion is the Stripe API version this code is written
// and tested against (US-019). In stripe-go the API version is fixed by
// the SDK module version: v76.25.0 sends "2023-10-16" on the
// Stripe-Version header of every request, and the const stripe.APIVersion
// is read-only. So the real pin lives in go.mod — this constant is the
// guard: NewStripeAdapter logs loudly if a future `go get -u` bumps the
// SDK to a different API version, forcing a deliberate review of the
// webhook + checkout request/response shapes instead of silent breakage.
// To intentionally upgrade: bump stripe-go in go.mod, update this
// constant + api/CLAUDE.md, and re-test the billing paths.
const stripePinnedAPIVersion = "2023-10-16"

type StripeAdapter struct {
	secretKey     string
	webhookSecret string
}

func NewStripeAdapter(secretKey, webhookSecret string) *StripeAdapter {
	if secretKey == "" {
		slog.Warn("STRIPE_SECRET_KEY is empty — Stripe checkout will fail")
	}
	if webhookSecret == "" {
		slog.Warn("STRIPE_WEBHOOK_SECRET is empty — webhook signature verification will reject all events")
	}
	if stripe.APIVersion != stripePinnedAPIVersion {
		// The SDK was bumped to a different API version than we pinned.
		// Request/response shapes may have changed — review before trusting.
		slog.Warn("Stripe API version drift: SDK differs from pinned version",
			"sdk_version", stripe.APIVersion,
			"pinned_version", stripePinnedAPIVersion)
	}
	stripe.Key = secretKey
	return &StripeAdapter{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
	}
}

// EnsureCustomer creates (or idempotently re-fetches) a Stripe customer
// for the account. The idempotency key is derived from accountID, so two
// concurrent first-checkouts for the same account return the SAME Stripe
// customer instead of minting duplicates (US-017). The account_id is also
// stamped in metadata for back-reference in the Stripe dashboard.
func (s *StripeAdapter) EnsureCustomer(accountID string) (string, error) {
	params := &stripe.CustomerParams{}
	params.AddMetadata("account_id", accountID)
	// Same key → Stripe replays the original response for 24h, collapsing
	// a checkout double-click into one customer.
	params.SetIdempotencyKey("customer_create_" + accountID)
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create stripe customer: %w", err)
	}
	return cust.ID, nil
}

// CreateCheckoutSession creates a Stripe Checkout Session for a
// specific Plan and Account. [customerID] binds the session to an
// existing Stripe customer so a re-checkout reuses it instead of
// spawning a new one; pass "" to let Stripe create one. [promoID] is
// the Stripe Promotion Code id (empty = no coupon).
func (s *StripeAdapter) CreateCheckoutSession(priceID, accountID, customerID, successURL, cancelURL, promoID string) (string, error) {
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		ClientReferenceID: stripe.String(accountID), // We use this to identify the account in the webhook
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
	}
	if customerID != "" {
		params.Customer = stripe.String(customerID)
	}
	if promoID != "" {
		params.Discounts = []*stripe.CheckoutSessionDiscountParams{
			{PromotionCode: stripe.String(promoID)},
		}
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session: %w", err)
	}

	return sess.URL, nil
}

// CreateAddonCheckoutSession creates a one-time payment checkout session
// for an add-on credit pack. [currency] is the ISO code the line item is
// priced in (the account's billing currency, US-003) — never hardcode it.
func (s *StripeAdapter) CreateAddonCheckoutSession(packID, packName string, priceInCents int64, accountID, currency, successURL, cancelURL string) (string, error) {
	if currency == "" {
		return "", fmt.Errorf("addon checkout: empty currency")
	}
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(currency),
					UnitAmount: stripe.Int64(priceInCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(packName),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		ClientReferenceID: stripe.String(accountID),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
	}
	params.AddMetadata("type", "addon")
	params.AddMetadata("pack_id", packID)

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create addon checkout session: %w", err)
	}

	return sess.URL, nil
}

// ConstructEvent validates the signature and constructs a Stripe Event
// CreateBillingPortalSession returns a short-lived URL for the
// Stripe-hosted Customer Portal. Caller passes the persisted
// stripe_customer_id (from accounts table) and a return URL the user
// lands on after they close the portal.
func (s *StripeAdapter) CreateBillingPortalSession(customerID, returnURL string) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("customer id is empty — account has never transacted with Stripe")
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	sess, err := billingportalsession.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create billing portal session: %w", err)
	}
	return sess.URL, nil
}

func (s *StripeAdapter) ConstructEvent(payload []byte, signature string) (stripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, signature, s.webhookSecret)
	if err != nil {
		return stripe.Event{}, fmt.Errorf("invalid webhook signature: %w", err)
	}
	return event, nil
}

// ListInvoicesForCustomer pulls the most recent N invoices Stripe has
// on file for the supplied customer. Read-only — the owner panel uses
// this to spot past-due / failed payments without leaving the app.
// Returns empty (not an error) for accounts without a Stripe customer.
// The shape lives in the domain package so the use case doesn't have
// to import this adapter.
func (s *StripeAdapter) ListInvoicesForCustomer(customerID string, limit int64) ([]domain.StripeInvoiceSummary, error) {
	if customerID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	params.Limit = stripe.Int64(limit)

	iter := invoice.List(params)
	out := make([]domain.StripeInvoiceSummary, 0, limit)
	for iter.Next() {
		inv := iter.Invoice()
		summary := domain.StripeInvoiceSummary{
			ID:         inv.ID,
			Number:     inv.Number,
			Status:     string(inv.Status),
			AmountDue:  inv.AmountDue,
			AmountPaid: inv.AmountPaid,
			Currency:   string(inv.Currency),
			Created:    inv.Created,
		}
		if inv.HostedInvoiceURL != "" {
			h := inv.HostedInvoiceURL
			summary.HostedURL = &h
		}
		if inv.InvoicePDF != "" {
			p := inv.InvoicePDF
			summary.InvoicePDFURL = &p
		}
		out = append(out, summary)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe list invoices: %w", err)
	}
	return out, nil
}

// ListPaidInvoicesInRange returns the customer's invoices whose PAID
// timestamp falls inside [from, to), each carrying the REAL processing
// fee from the charge's balance_transaction (expanded on the list call
// — no per-invoice roundtrip) plus the charge's amount_refunded so the
// profit computation can net refunds out of revenue. Feeds the monthly
// profit computation (US-006): revenue must be what Stripe actually
// settled, never an estimated fee percentage.
func (s *StripeAdapter) ListPaidInvoicesInRange(customerID string, from, to time.Time) ([]domain.StripePaidInvoice, error) {
	if customerID == "" {
		return nil, nil
	}
	iter := invoice.List(paidInvoiceListParams(customerID, to))
	out := []domain.StripePaidInvoice{}
	for iter.Next() {
		inv := iter.Invoice()
		if inv.StatusTransitions == nil {
			continue
		}
		paidAt := inv.StatusTransitions.PaidAt
		if paidAt < from.Unix() || paidAt >= to.Unix() || inv.AmountPaid <= 0 {
			continue
		}
		p := domain.StripePaidInvoice{
			ID:              inv.ID,
			AmountPaidCents: inv.AmountPaid,
			Currency:        string(inv.Currency),
			PaidAt:          paidAt,
		}
		if inv.Charge != nil {
			p.RefundedCents = inv.Charge.AmountRefunded
			if inv.Charge.BalanceTransaction != nil {
				p.FeeCents = inv.Charge.BalanceTransaction.Fee
				// The fee is in the balance_transaction's own
				// settlement currency — surface it so the profit
				// computation never assumes BRL (US-008 review fix).
				p.FeeCurrency = string(inv.Charge.BalanceTransaction.Currency)
			}
		}
		out = append(out, p)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe list paid invoices: %w", err)
	}
	return out, nil
}

// paidInvoiceListParams builds the Stripe list filter for
// ListPaidInvoicesInRange. Stripe's list filter only supports
// created-at, and a subscription invoice can be paid ANY time after
// creation (dunning can drag past months), so the only created bound
// is "before the window ends" — an invoice cannot be paid before it
// exists. The [from, to) window is enforced client-side on
// status_transitions.paid_at. Bounding created below (the old
// 3-months-back over-fetch) silently dropped old invoices recovered
// late (US-007 review fix).
func paidInvoiceListParams(customerID string, to time.Time) *stripe.InvoiceListParams {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
		Status:   stripe.String(string(stripe.InvoiceStatusPaid)),
		CreatedRange: &stripe.RangeQueryParams{
			LesserThan: to.Unix(),
		},
	}
	params.Limit = stripe.Int64(100)
	params.AddExpand("data.charge.balance_transaction")
	return params
}

// CreateOverageInvoiceItem adds a one-off pending invoice item to the
// customer. Stripe sweeps pending items into the customer's next
// subscription invoice automatically, so conversation overage rides
// along on the regular monthly charge — no separate payment flow.
//
// idempotencyKey makes a retry after a partial failure safe: Stripe
// remembers the key for 24h and returns the original item instead of
// creating a duplicate. Bill the running cycle total in the key so a
// retry with no new conversations dedupes, but a later tick with more
// conversations gets a fresh item.
func (s *StripeAdapter) CreateOverageInvoiceItem(customerID string, amountCents int64, currency, description, idempotencyKey string) error {
	if customerID == "" {
		return fmt.Errorf("customer id is empty — account has never transacted with Stripe")
	}
	if amountCents <= 0 {
		return nil
	}
	if currency == "" {
		return fmt.Errorf("overage invoice item: empty currency")
	}
	params := &stripe.InvoiceItemParams{
		Customer:    stripe.String(customerID),
		Amount:      stripe.Int64(amountCents),
		Currency:    stripe.String(currency),
		Description: stripe.String(description),
	}
	if idempotencyKey != "" {
		params.SetIdempotencyKey(idempotencyKey)
	}
	if _, err := invoiceitem.New(params); err != nil {
		return fmt.Errorf("stripe invoice item new: %w", err)
	}
	return nil
}

// CreateCoupon mints a Stripe Coupon + a Promotion Code that exposes
// the human-readable [code] string. We use this two-step shape (vs.
// just a raw coupon) because Promotion Codes are the recommended
// channel for customer-facing redemption — the Coupon is the rate,
// the Promotion Code is the slug.
//
// [percentOff] is a whole number 1..100. [maxRedemptions] is nil for
// unlimited. [expiresAt] is a Unix timestamp; pass 0 to disable the
// hard stop.
func (s *StripeAdapter) CreateCoupon(code string, percentOff int, maxRedemptions *int, expiresAt int64) (couponID string, promoID string, err error) {
	cp, err := coupon.New(&stripe.CouponParams{
		PercentOff: stripe.Float64(float64(percentOff)),
		Duration:   stripe.String(string(stripe.CouponDurationOnce)),
		Name:       stripe.String(code),
	})
	if err != nil {
		return "", "", fmt.Errorf("stripe coupon new: %w", err)
	}
	params := &stripe.PromotionCodeParams{
		Coupon: stripe.String(cp.ID),
		Code:   stripe.String(code),
	}
	if maxRedemptions != nil {
		params.MaxRedemptions = stripe.Int64(int64(*maxRedemptions))
	}
	if expiresAt > 0 {
		params.ExpiresAt = stripe.Int64(expiresAt)
	}
	pc, err := promotioncode.New(params)
	if err != nil {
		return cp.ID, "", fmt.Errorf("stripe promo code new: %w", err)
	}
	return cp.ID, pc.ID, nil
}

// CreateWinBackCoupon mints the automatic recovery coupon offered to a
// just-churned customer (US-179): WinBackDiscountPercent off for
// WinBackDurationMonths months. It creates a *repeating* Coupon plus a
// Promotion Code restricted to [customerID] and limited to a single
// redemption, so the discount is personal to that churned account and
// can't leak or be reused. The code expires shortly after the 30-day
// win-back follow-up window. Returns the redeemable, customer-facing code.
func (s *StripeAdapter) CreateWinBackCoupon(customerID string) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("winback coupon: empty customer id")
	}
	cp, err := coupon.New(&stripe.CouponParams{
		PercentOff:       stripe.Float64(float64(domain.WinBackDiscountPercent)),
		Duration:         stripe.String(string(stripe.CouponDurationRepeating)),
		DurationInMonths: stripe.Int64(int64(domain.WinBackDurationMonths)),
		Name:             stripe.String("Win-back"),
	})
	if err != nil {
		return "", fmt.Errorf("stripe winback coupon new: %w", err)
	}
	code := "WINBACK" + ulid.Make().String()
	pc, err := promotioncode.New(&stripe.PromotionCodeParams{
		Coupon:         stripe.String(cp.ID),
		Code:           stripe.String(code),
		Customer:       stripe.String(customerID),
		MaxRedemptions: stripe.Int64(1),
		// Give a few days of slack past the last (30-day) follow-up so a
		// late click still redeems.
		ExpiresAt: stripe.Int64(time.Now().Add(35 * 24 * time.Hour).Unix()),
	})
	if err != nil {
		return "", fmt.Errorf("stripe winback promo code new: %w", err)
	}
	return pc.Code, nil
}

// ReportMeteredUsage reports usage-based overage to Stripe's metered
// billing (US-168). It resolves the subscription item on
// [subscriptionID] whose price is [meteredPriceID], then records the
// running cycle total via POST /subscription_items/{id}/usage_records
// with action=`set`. The metered prices MUST be created with
// `aggregate_usage=last_during_period` so the last `set` of the cycle is
// the billed quantity (see docs/billing.md).
//
// No-op (returns nil) when either id is empty, or when the subscription
// has no line item for that metered price — i.e. the plan doesn't bill
// that meter — so the caller can blindly report every meter without
// branching on plan shape. [idempotencyKey] collapses retried reports of
// the same (account, meter, cycle) into a single Stripe call.
func (s *StripeAdapter) ReportMeteredUsage(subscriptionID, meteredPriceID string, quantity int64, idempotencyKey string) error {
	if subscriptionID == "" || meteredPriceID == "" {
		return nil
	}
	itemID, err := s.resolveMeteredItem(subscriptionID, meteredPriceID)
	if err != nil {
		return err
	}
	if itemID == "" {
		// Subscription doesn't carry this metered line item — the plan
		// doesn't bill this meter. Nothing to report.
		return nil
	}
	params := &stripe.UsageRecordParams{
		SubscriptionItem: stripe.String(itemID),
		Quantity:         stripe.Int64(quantity),
		Action:           stripe.String(stripe.UsageRecordActionSet),
		// Timestamp left unset → Stripe records against "now", which is
		// always inside the current billing period.
	}
	if idempotencyKey != "" {
		params.SetIdempotencyKey(idempotencyKey)
	}
	if _, err := usagerecord.New(params); err != nil {
		return fmt.Errorf("stripe usage record (sub=%s price=%s): %w", subscriptionID, meteredPriceID, err)
	}
	return nil
}

// GetSubscriptionPriceID fetches the subscription from Stripe and returns
// the price id of its first line item — the checkout webhook uses it to
// resolve which plan was purchased (checkout.session.completed carries
// only the subscription id). Returns "" when the subscription has no
// line items.
func (s *StripeAdapter) GetSubscriptionPriceID(subscriptionID string) (string, error) {
	if subscriptionID == "" {
		return "", nil
	}
	sub, err := stripesub.Get(subscriptionID, nil)
	if err != nil {
		return "", fmt.Errorf("stripe subscription get (%s): %w", subscriptionID, err)
	}
	if sub.Items == nil {
		return "", nil
	}
	for _, item := range sub.Items.Data {
		if item != nil && item.Price != nil && item.Price.ID != "" {
			return item.Price.ID, nil
		}
	}
	return "", nil
}

// resolveMeteredItem returns the subscription_item id on [subscriptionID]
// whose price matches [meteredPriceID], or "" when the subscription has
// no such line item. We resolve at report time (rather than persisting
// item ids) so a plan change that swaps the metered price is picked up
// automatically on the next cycle.
func (s *StripeAdapter) resolveMeteredItem(subscriptionID, meteredPriceID string) (string, error) {
	sub, err := stripesub.Get(subscriptionID, nil)
	if err != nil {
		return "", fmt.Errorf("stripe subscription get (%s): %w", subscriptionID, err)
	}
	if sub.Items == nil {
		return "", nil
	}
	for _, item := range sub.Items.Data {
		if item != nil && item.Price != nil && item.Price.ID == meteredPriceID {
			return item.ID, nil
		}
	}
	return "", nil
}

// DeactivatePromotionCode flips Stripe's `active=false` on the
// promotion code, preventing future redemptions without touching the
// underlying Coupon (so existing subscriptions keep their discount).
func (s *StripeAdapter) DeactivatePromotionCode(promoID string) error {
	if promoID == "" {
		return nil
	}
	_, err := promotioncode.Update(promoID, &stripe.PromotionCodeParams{
		Active: stripe.Bool(false),
	})
	if err != nil {
		return fmt.Errorf("stripe promo code deactivate: %w", err)
	}
	return nil
}

// CancelSubscription terminates the supplied Stripe subscription
// immediately (no proration refund). Called from account-delete flows
// so a customer who exercises right-to-be-forgotten stops being billed
// the moment their row is removed; without this, Stripe keeps charging
// the saved payment method even though the account is gone and the
// invoice.paid webhook can no longer find the matching account row.
//
// Returns nil when subscriptionID is empty (account never subscribed).
// Treats Stripe's "resource_missing" / "subscription already canceled"
// outcomes as success so a re-run after a partial failure doesn't trip
// on already-cancelled subscriptions.
func (s *StripeAdapter) CancelSubscription(subscriptionID string) error {
	if subscriptionID == "" {
		return nil
	}
	if _, err := stripesub.Cancel(subscriptionID, nil); err != nil {
		if stripeErr, ok := err.(*stripe.Error); ok {
			if stripeErr.Code == stripe.ErrorCodeResourceMissing {
				return nil
			}
		}
		return fmt.Errorf("stripe subscription cancel: %w", err)
	}
	return nil
}

// GetPromotionCodeRedeemed returns the current `times_redeemed` value
// for the given promotion code. The owner panel calls this in a
// background refresh so the listed redeemed_count stays current
// without each row triggering a roundtrip.
func (s *StripeAdapter) GetPromotionCodeRedeemed(promoID string) (int, error) {
	if promoID == "" {
		return 0, nil
	}
	pc, err := promotioncode.Get(promoID, nil)
	if err != nil {
		return 0, fmt.Errorf("stripe promo code get: %w", err)
	}
	return int(pc.TimesRedeemed), nil
}

// GrantReferralCredit applies an account credit to the customer by
// posting a NEGATIVE customer balance transaction — Stripe deducts a
// negative balance from the next finalized invoice, which is exactly the
// "1 month free" referral reward (US-174). No-op when the customer id or
// amount is empty/zero. [idempotencyKey] makes a webhook replay safe.
func (s *StripeAdapter) GrantReferralCredit(customerID string, amountCents int64, currency, description, idempotencyKey string) error {
	if customerID == "" || amountCents <= 0 {
		return nil
	}
	if currency == "" {
		// The caller resolves the account's currency (US-003) — refusing an
		// empty value beats silently crediting in the wrong currency.
		return fmt.Errorf("referral credit: empty currency")
	}
	params := &stripe.CustomerBalanceTransactionParams{
		Customer:    stripe.String(customerID),
		Amount:      stripe.Int64(-amountCents), // negative = credit
		Currency:    stripe.String(currency),
		Description: stripe.String(description),
	}
	if idempotencyKey != "" {
		params.SetIdempotencyKey(idempotencyKey)
	}
	if _, err := customerbalancetransaction.New(params); err != nil {
		return fmt.Errorf("stripe referral credit: %w", err)
	}
	return nil
}
