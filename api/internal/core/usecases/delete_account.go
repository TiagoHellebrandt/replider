package usecases

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// SubscriptionCanceller is the slice of the Stripe adapter this use
// case needs — defined here so the usecases layer doesn't import the
// adapters package (arch direction adapters→usecases→domain).
type SubscriptionCanceller interface {
	CancelSubscription(subscriptionID string) error
}

// MediaCleaner wipes every object stored under a prefix. The
// DeleteAccountUseCase calls it with `accounts/{id}/` so GCS-hosted
// re-hosted media, profile pictures, ingestion uploads, and signed
// URL targets are removed alongside the database cascade.
type MediaCleaner interface {
	DeletePrefix(ctx context.Context, prefix string) (int, error)
}

// DeleteAccountUseCase handles GDPR-compliant account deletion.
// Since all tables use ON DELETE CASCADE foreign keys, deleting the account
// will cascade to users, agents, channels, conversations, messages,
// knowledge, calendars, usage, and audit logs.
type DeleteAccountUseCase struct {
	accountRepo domain.AccountRepository
	userRepo    domain.UserRepository
	stripe      SubscriptionCanceller // optional; when nil we skip cancellation (e.g. tests)
	media       MediaCleaner          // optional; when nil GCS bytes survive — log only
}

func NewDeleteAccountUseCase(accountRepo domain.AccountRepository, userRepo domain.UserRepository) *DeleteAccountUseCase {
	return &DeleteAccountUseCase{accountRepo: accountRepo, userRepo: userRepo}
}

// SetSubscriptionCanceller wires Stripe so account-delete also cancels
// the active subscription. Without it the row is deleted while Stripe
// keeps charging the saved card — the orphaned subscription has no
// account to look up on invoice.paid, so the next invoice fails
// silently.
func (uc *DeleteAccountUseCase) SetSubscriptionCanceller(s SubscriptionCanceller) {
	uc.stripe = s
}

// SetMediaCleaner wires the GCS-backed media cleaner. Without it the
// database row is deleted but inbound-media, profile pictures, and
// ingestion uploads stored under accounts/{id}/ survive in the bucket
// — a GDPR right-to-be-forgotten violation.
func (uc *DeleteAccountUseCase) SetMediaCleaner(m MediaCleaner) {
	uc.media = m
}

func (uc *DeleteAccountUseCase) Execute(ctx context.Context, accountID, userID string) error {
	// Verify the requesting user is an admin of this account
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.AccountID != accountID {
		return domain.ErrForbidden
	}
	// RoleSatisfies, not string equality: an OWNER is not literally "admin", and
	// was left out of what is theirs by definition. And the refusal is 403 — with
	// a 401 the app concludes the credential died and clears the session.
	if !domain.RoleSatisfies(roleOf(user), domain.RoleAdminRole) {
		return domain.ErrForbidden
	}

	// GDPR forensic trail. Emitted at WARN so external log retention
	// (Stackdriver / Loki) keeps it long after the DB row is gone — an
	// audit_logs row written here would itself be CASCADE-deleted along
	// with the account, leaving no record of who tore down what or when.
	slog.Warn("account delete requested",
		"account_id", accountID,
		"actor_user_id", userID,
		"actor_email", user.Email,
	)

	// Cancel the Stripe subscription BEFORE the DB delete so a Stripe
	// failure aborts the whole flow and the customer can retry. The
	// reverse order would leave a Stripe-billable subscription pointing
	// at a non-existent account if the DB delete succeeded and the
	// Stripe call later failed.
	if uc.stripe != nil {
		account, lookupErr := uc.accountRepo.GetByID(ctx, accountID)
		if lookupErr != nil {
			return lookupErr
		}
		if account != nil && account.StripeSubscriptionID != nil && *account.StripeSubscriptionID != "" {
			if cancelErr := uc.stripe.CancelSubscription(*account.StripeSubscriptionID); cancelErr != nil {
				slog.Error("account delete: failed to cancel Stripe subscription — aborting",
					"account_id", accountID, "subscription_id", *account.StripeSubscriptionID, "error", cancelErr)
				return cancelErr
			}
		}
	}

	// Wipe GCS objects under accounts/{id}/ BEFORE the DB delete so a
	// transient bucket error aborts the flow and the user can retry
	// (CancelSubscription is idempotent on retry, DeletePrefix is
	// trivially idempotent — only the surviving objects get deleted).
	if uc.media != nil {
		prefix := "accounts/" + accountID + "/"
		if n, err := uc.media.DeletePrefix(ctx, prefix); err != nil {
			slog.Error("account delete: failed to wipe GCS prefix — aborting",
				"account_id", accountID, "prefix", prefix, "deleted_so_far", n, "error", err)
			return err
		} else {
			slog.Info("account delete: wiped GCS prefix",
				"account_id", accountID, "prefix", prefix, "objects_deleted", n)
		}
	}

	return uc.accountRepo.Delete(ctx, accountID)
}
