package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// fakeAbacate is a hand-rolled domain.AbacatePayAdapter for routing tests.
type fakeAbacate struct {
	enabled     bool
	pixCalled   bool
	cardCalled  bool
	lastPixIn   domain.PixChargeInput
	lastCardIn  domain.CardCheckoutInput
	pixErr      error
	cardURL     string
	statusValue string
}

func (f *fakeAbacate) Enabled() bool { return f.enabled }
func (f *fakeAbacate) CreatePixCharge(_ context.Context, in domain.PixChargeInput) (*domain.PixCharge, error) {
	f.pixCalled = true
	f.lastPixIn = in
	if f.pixErr != nil {
		return nil, f.pixErr
	}
	return &domain.PixCharge{ID: "pix_1", BRCode: "000201...", BRCodeBase64: "data:image/png;base64,AAA", AmountCents: in.AmountCents, Status: "PENDING"}, nil
}
func (f *fakeAbacate) CreateCardCheckout(_ context.Context, in domain.CardCheckoutInput) (string, error) {
	f.cardCalled = true
	f.lastCardIn = in
	if f.cardURL == "" {
		return "https://app.abacatepay.com/pay/bill_1", nil
	}
	return f.cardURL, nil
}
func (f *fakeAbacate) GetChargeStatus(_ context.Context, _ string) (string, error) {
	return f.statusValue, nil
}
func (f *fakeAbacate) VerifySignature(_ []byte, _ string) bool { return true }

func (f *fakeAbacate) VerifyWebhookSecret(_ string) bool { return true }

func (f *fakeAbacate) ParseWebhook(_ []byte) (*domain.AbacateWebhookEvent, error) {
	return &domain.AbacateWebhookEvent{}, nil
}

// fakeStripeDelegate records that the non-BRL path delegated to Stripe.
type fakeStripeDelegate struct {
	called bool
	dto    CreateCheckoutSessionDTO
	url    string
	err    error
}

func (f *fakeStripeDelegate) Execute(_ context.Context, dto CreateCheckoutSessionDTO) (string, error) {
	f.called = true
	f.dto = dto
	return f.url, f.err
}

func brlPlan() *domain.Plan {
	price := "price_x"
	return &domain.Plan{ID: "plan_1", Name: "Pro", Currency: domain.CurrencyBRL, Price: 9900, StripePriceID: &price}
}

func setupTransparent(acct *domain.Account, plan *domain.Plan, ab *fakeAbacate, st *fakeStripeDelegate) *CreateTransparentCheckoutUseCase {
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	accountRepo.On("GetByID", context.Background(), acct.ID).Return(acct, nil)
	planRepo.On("GetByID", context.Background(), plan.ID).Return(plan, nil)
	return NewCreateTransparentCheckoutUseCase(accountRepo, planRepo, ab, st)
}

func TestTransparentCheckout_BRLDefaultsToPix(t *testing.T) {
	ab := &fakeAbacate{enabled: true}
	st := &fakeStripeDelegate{}
	acct := &domain.Account{ID: "acc_1", Currency: domain.CurrencyBRL}
	uc := setupTransparent(acct, brlPlan(), ab, st)

	res, err := uc.Execute(context.Background(), CreateTransparentCheckoutDTO{
		AccountID: "acc_1", PlanID: "plan_1",
		SuccessURL: "https://app.replider.com/ok", CancelURL: "https://app.replider.com/no",
	})
	assert.NoError(t, err)
	assert.Equal(t, "abacatepay", res.Provider)
	assert.Equal(t, CheckoutMethodPix, res.Method)
	assert.NotNil(t, res.Pix)
	assert.Equal(t, int64(9900), res.Pix.AmountCents)
	assert.True(t, ab.pixCalled)
	assert.False(t, st.called)
	// Metadata must carry account + plan so the webhook can activate.
	assert.Equal(t, "acc_1", ab.lastPixIn.Metadata["account_id"])
	assert.Equal(t, "plan_1", ab.lastPixIn.Metadata["plan_id"])
}

func TestTransparentCheckout_BRLCardUsesAbacateHosted(t *testing.T) {
	ab := &fakeAbacate{enabled: true}
	st := &fakeStripeDelegate{}
	acct := &domain.Account{ID: "acc_1", Currency: domain.CurrencyBRL}
	uc := setupTransparent(acct, brlPlan(), ab, st)

	res, err := uc.Execute(context.Background(), CreateTransparentCheckoutDTO{
		AccountID: "acc_1", PlanID: "plan_1", Method: CheckoutMethodCard,
		SuccessURL: "https://app.replider.com/ok", CancelURL: "https://app.replider.com/no",
	})
	assert.NoError(t, err)
	assert.Equal(t, "abacatepay", res.Provider)
	assert.Equal(t, CheckoutMethodCard, res.Method)
	assert.NotEmpty(t, res.URL)
	assert.True(t, ab.cardCalled)
	assert.False(t, st.called)
	assert.Equal(t, int64(9900), ab.lastCardIn.AmountCents)
}

func TestTransparentCheckout_USDDelegatesToStripe(t *testing.T) {
	ab := &fakeAbacate{enabled: true}
	st := &fakeStripeDelegate{url: "https://checkout.stripe.com/s_1"}
	acct := &domain.Account{ID: "acc_1", Currency: domain.CurrencyUSD}
	usdPrice := "price_usd"
	plan := &domain.Plan{ID: "plan_1", Name: "Pro", Currency: domain.CurrencyUSD, Price: 19900, StripePriceID: &usdPrice}
	uc := setupTransparent(acct, plan, ab, st)

	res, err := uc.Execute(context.Background(), CreateTransparentCheckoutDTO{
		AccountID: "acc_1", PlanID: "plan_1", Method: CheckoutMethodCard,
		SuccessURL: "https://app.replider.com/ok", CancelURL: "https://app.replider.com/no",
	})
	assert.NoError(t, err)
	assert.Equal(t, "stripe", res.Provider)
	assert.Equal(t, CheckoutMethodCard, res.Method)
	assert.Equal(t, "https://checkout.stripe.com/s_1", res.URL)
	assert.True(t, st.called)
	assert.False(t, ab.pixCalled)
	assert.False(t, ab.cardCalled)
}

func TestTransparentCheckout_AbacateDisabledFallsBackToStripe(t *testing.T) {
	ab := &fakeAbacate{enabled: false} // no key configured
	st := &fakeStripeDelegate{url: "https://checkout.stripe.com/s_2"}
	acct := &domain.Account{ID: "acc_1", Currency: domain.CurrencyBRL}
	uc := setupTransparent(acct, brlPlan(), ab, st)

	res, err := uc.Execute(context.Background(), CreateTransparentCheckoutDTO{
		AccountID: "acc_1", PlanID: "plan_1",
		SuccessURL: "https://app.replider.com/ok", CancelURL: "https://app.replider.com/no",
	})
	assert.NoError(t, err)
	assert.Equal(t, "stripe", res.Provider)
	assert.True(t, st.called)
	assert.False(t, ab.pixCalled)
}

func TestTransparentCheckout_ContractPlanNotPurchasable(t *testing.T) {
	ab := &fakeAbacate{enabled: true}
	st := &fakeStripeDelegate{}
	acct := &domain.Account{ID: "acc_1", Currency: domain.CurrencyBRL}
	// No StripePriceID → contract plan → not purchasable online.
	plan := &domain.Plan{ID: "plan_1", Name: "Enterprise", Currency: domain.CurrencyBRL, Price: 0}
	uc := setupTransparent(acct, plan, ab, st)

	_, err := uc.Execute(context.Background(), CreateTransparentCheckoutDTO{
		AccountID: "acc_1", PlanID: "plan_1",
		SuccessURL: "https://app.replider.com/ok", CancelURL: "https://app.replider.com/no",
	})
	assert.True(t, errors.Is(err, domain.ErrPlanNotPurchasable) || err == domain.ErrPlanNotPurchasable)
	assert.False(t, ab.pixCalled)
}
