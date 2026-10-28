package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// onboardingMilestoneCount is the number of items rendered in the
// dashboard checklist. We compute completion_percentage on the server so
// the UI doesn't have to know the weighting (today they're equal).
const onboardingMilestoneCount = 6

type GetOnboardingProgressUseCase struct {
	accountRepo domain.AccountRepository
}

func NewGetOnboardingProgressUseCase(accountRepo domain.AccountRepository) *GetOnboardingProgressUseCase {
	return &GetOnboardingProgressUseCase{accountRepo: accountRepo}
}

func (uc *GetOnboardingProgressUseCase) Execute(ctx context.Context, accountID string) (*domain.OnboardingProgress, error) {
	progress, err := uc.accountRepo.GetOnboardingProgress(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		return nil, domain.ErrAccountNotFound
	}
	progress.CompletionPercentage = computeOnboardingPercentage(progress)

	// Drafts written before US-012 may still carry plaintext channel
	// credentials — never hand those back to the client.
	progress.Draft = sanitizeOnboardingDraft(progress.Draft)

	// Attach the current account name so the onboarding wizard's
	// company-name field can pre-fill without a second roundtrip.
	// Best-effort — if the account fetch fails we still return the
	// progress flags, the UI will just show an empty field.
	if account, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && account != nil {
		progress.AccountName = account.Name
	}

	return progress, nil
}

// computeOnboardingPercentage counts the five SALES milestones.
//
// It used to count agent, preview chat and knowledge base — the chatbot product.
// An account that connected a mailbox, imported a catalogue and sent a proposal
// read as 20% done, and the dashboard nagged it to build a bot it will never use.
//
// The three bot flags are still on the payload (they are real facts, and the
// `full` onboarding cohort renders them); they are simply not what getting
// started means here. This number also feeds the seller "onboarding_stuck"
// suggestion, which is the other reason it has to measure the right five: being
// stuck should mean stuck on SELLING.
func computeOnboardingPercentage(p *domain.OnboardingProgress) int {
	done := 0
	// A way to send. Without it a proposal has nowhere to go.
	if p.ChannelConnected {
		done++
	}
	if p.ProductsAdded {
		done++
	}
	if p.CustomersAdded {
		done++
	}
	if p.ProposalMade {
		done++
	}
	// Somebody answered. The only milestone that is not our doing.
	if p.FirstInboundReceived {
		done++
	}
	// The seller has her seat (2026-08-18). Asked after onboarding, in the
	// checklist, on purpose: the owner finishes the wizard alone in three
	// minutes and invites her once the first offer has made the case.
	if p.TeamInvited {
		done++
	}
	return (done * 100) / onboardingMilestoneCount
}
