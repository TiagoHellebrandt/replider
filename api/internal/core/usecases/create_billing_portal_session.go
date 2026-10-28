package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// CreateBillingPortalSessionUseCase returns a short-lived URL to the
// Stripe-hosted customer portal where the admin can manage their card,
// download invoices, or cancel the subscription. The Flutter billing
// screen opens this URL in a new tab.
type CreateBillingPortalSessionUseCase struct {
	accountRepo domain.AccountRepository
	paymentAdpt domain.PaymentAdapter
}

func NewCreateBillingPortalSessionUseCase(accountRepo domain.AccountRepository, paymentAdpt domain.PaymentAdapter) *CreateBillingPortalSessionUseCase {
	return &CreateBillingPortalSessionUseCase{
		accountRepo: accountRepo,
		paymentAdpt: paymentAdpt,
	}
}

type CreateBillingPortalSessionDTO struct {
	AccountID string
	ReturnURL string
}

func (uc *CreateBillingPortalSessionUseCase) Execute(ctx context.Context, dto CreateBillingPortalSessionDTO) (string, error) {
	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil {
		return "", err
	}
	if account == nil {
		return "", domain.ErrAccountNotFound
	}
	if account.StripeCustomerID == nil || *account.StripeCustomerID == "" {
		// Account has never transacted with Stripe (e.g. still on the
		// free trial without upgrading once). No customer object exists
		// to open the portal against.
		return "", domain.ErrNoCustomer
	}
	return uc.paymentAdpt.CreateBillingPortalSession(*account.StripeCustomerID, dto.ReturnURL)
}
