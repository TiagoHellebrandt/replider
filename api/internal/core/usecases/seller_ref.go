package usecases

import (
	"context"
	"log/slog"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// EnsureSellerRefCodeUseCase returns the calling seller's personal
// prospecting code (US-003), minting it lazily on first call. Idempotent —
// the same user always gets the same code. Routes using it are gated by
// middleware.RequirePlatformAccount, so only users of the Replider
// platform account ever reach it.
type EnsureSellerRefCodeUseCase struct {
	sellerRefRepo domain.SellerRefRepository
	// webBaseURL is the marketing-site origin — the canonical seller link
	// is replider.com/register?ref=CODE (the landing page forwards the
	// query to the app register flow).
	webBaseURL string
}

func NewEnsureSellerRefCodeUseCase(sellerRefRepo domain.SellerRefRepository, webBaseURL string) *EnsureSellerRefCodeUseCase {
	return &EnsureSellerRefCodeUseCase{sellerRefRepo: sellerRefRepo, webBaseURL: webBaseURL}
}

// SellerRefInfo is the POST /seller/ref-code payload.
type SellerRefInfo struct {
	Code     string `json:"code"`
	ShareURL string `json:"share_url"`
}

func (uc *EnsureSellerRefCodeUseCase) Execute(ctx context.Context, userID string) (*SellerRefInfo, error) {
	code, err := uc.sellerRefRepo.GetOrCreateSellerRefCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &SellerRefInfo{
		Code:     code,
		ShareURL: uc.webBaseURL + "/register?ref=" + code,
	}, nil
}

// stampSellerRefAttribution resolves a signup ?ref= code against seller
// ref codes (US-003) and, on a match, stamps the new account with
// seller_user_id + lead_source='seller_ref'. Reports whether the code was
// consumed as a seller ref so callers skip the customer-referral path
// (US-174) — the two namespaces are separate and one code belongs to at
// most one of them. Best-effort: every failure is swallowed (logged) so a
// seller-attribution problem never breaks a signup.
//
// Self-referral guard (US-013): a seller cannot net 50% of their own
// subscription, so when the new account's admin IS the seller behind the
// code (same user id or same email, case-insensitive), the ref is ignored
// — the account is created WITHOUT attribution and the code is still
// consumed (it never falls through to the customer-referral path).
func stampSellerRefAttribution(ctx context.Context, repo domain.SellerRefRepository, platformAccountID, accountID, code, newAdminUserID, newAdminEmail string) bool {
	code = strings.TrimSpace(code)
	if repo == nil || platformAccountID == "" || code == "" {
		return false
	}
	sellerUserID, sellerEmail, err := repo.ResolveSellerRef(ctx, code, platformAccountID)
	if err != nil {
		slog.Error("seller ref: failed to resolve code on signup", "code", code, "error", err)
		return false
	}
	if sellerUserID == "" {
		return false
	}
	if isSellerSelfReferral(sellerUserID, sellerEmail, newAdminUserID, newAdminEmail) {
		slog.Warn("seller ref: self-referral ignored, account created without attribution",
			"code", code, "seller", sellerUserID, "account", accountID)
		return true // code WAS a seller code — never fall through to referral
	}
	if err := repo.StampSellerAttribution(ctx, accountID, sellerUserID); err != nil {
		slog.Error("seller ref: failed to stamp attribution", "account", accountID, "seller", sellerUserID, "error", err)
		return true // code WAS a seller code — never fall through to referral
	}
	slog.Info("seller ref: signup attributed to seller",
		"account", accountID, "seller", sellerUserID, "lead_source", domain.LeadSourceSellerRef)
	return true
}

// isSellerSelfReferral reports whether the signing-up admin and the code's
// seller are the same person: same user id, or same email (trimmed,
// case-insensitive). Empty values never match — a missing email must not
// pass the guard vacuously.
func isSellerSelfReferral(sellerUserID, sellerEmail, newAdminUserID, newAdminEmail string) bool {
	if sellerUserID != "" && newAdminUserID != "" && sellerUserID == newAdminUserID {
		return true
	}
	se := strings.TrimSpace(sellerEmail)
	ae := strings.TrimSpace(newAdminEmail)
	return se != "" && ae != "" && strings.EqualFold(se, ae)
}
