package usecases

import "github.com/replider/api/internal/core/domain"

// onboardingPendingFor answers the login response's `is_new`: should THIS
// user be sent to the setup wizard?
//
// It is a fact about the account (onboarding_completed) gated by the user's
// role, because the wizard's first step is PATCH /accounts — admin only. A
// vendedora invited before her admin finished the wizard used to be sent to
// it anyway, hit 403 on step one, had no Skip, and the app's local
// "completed" flag was recomputed from /me on the next cold start: a loop
// with no exit (analysis 2026-09-01). Below admin the wizard is simply not
// hers to run; she lands on the product.
func onboardingPendingFor(account *domain.Account, user *domain.User) bool {
	if account == nil || account.OnboardingCompleted || user == nil {
		return false
	}
	role := ""
	if user.Role != nil {
		role = *user.Role
	}
	return domain.RoleSatisfies(domain.NormalizeRole(role), domain.RoleAdminRole)
}

// maxOnboardingStepIndex is the highest page index a wizard variant can
// resume at. The reduced flow's canonical pages are [0, 5, 6, 4, 7]; the old
// clamp at 3 sent everyone who closed the tab on Customers/Channel/Offer
// back to Products.
const maxOnboardingStepIndex = 7
