package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// GetReferralInfoUseCase returns the signed-in account's referral code,
// a ready-to-share link, and the running referral stats (US-174). The
// code is minted lazily on first view via the repository.
type GetReferralInfoUseCase struct {
	referralRepo domain.ReferralRepository
	// appBaseURL is the dashboard origin; the share link points new
	// signups at it with ?ref=CODE so the registration flow can forward
	// the code to CreateAccount.
	appBaseURL string
}

func NewGetReferralInfoUseCase(referralRepo domain.ReferralRepository, appBaseURL string) *GetReferralInfoUseCase {
	return &GetReferralInfoUseCase{referralRepo: referralRepo, appBaseURL: appBaseURL}
}

// ReferralInfo is the GET /me/referral payload.
type ReferralInfo struct {
	Code     string                `json:"code"`
	ShareURL string                `json:"share_url"`
	Stats    *domain.ReferralStats `json:"stats"`
}

func (uc *GetReferralInfoUseCase) Execute(ctx context.Context, accountID string) (*ReferralInfo, error) {
	code, err := uc.referralRepo.GetOrCreateCode(ctx, accountID)
	if err != nil {
		return nil, err
	}
	stats, err := uc.referralRepo.StatsByReferrer(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &ReferralInfo{
		Code:     code,
		ShareURL: uc.appBaseURL + "/register?ref=" + code,
		Stats:    stats,
	}, nil
}
