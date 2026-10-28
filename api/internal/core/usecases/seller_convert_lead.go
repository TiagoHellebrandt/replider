package usecases

import (
	"context"
	"log/slog"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// sellerConversationGetter is the one slice of ConversationRepository the
// convert-lead flow needs. Narrow local interface so tests don't have to
// mock the full ~30-method repository.
type sellerConversationGetter interface {
	GetByID(ctx context.Context, id string, accountID string) (*domain.Conversation, error)
}

// sellerContactGetter resolves the lead conversation's contact so the
// conversion can check whether the contact belongs to the target account
// (US-014). Satisfied by ContactRepository.
type sellerContactGetter interface {
	GetByExternalID(ctx context.Context, accountID, externalID string) (*domain.Contact, error)
}

// sellerFlagChecker reports whether a user carries the explicit
// owner-granted seller flag (US-014). Satisfied by
// postgres.SellerRefRepository.
type sellerFlagChecker interface {
	IsSeller(ctx context.Context, userID string) (bool, error)
}

// ConvertLeadUseCase implements the explicit inbound lead conversion
// (US-004): a seller — an attendant on the Replider platform account —
// marks their lead conversation as converted into an existing customer
// account. The stamped seller is the conversation's ASSIGNED attendant
// (the person who actually worked the lead), at the 20% inbound track.
// Routes reaching this are gated by middleware.RequirePlatformAccount +
// middleware.RequireSeller.
type ConvertLeadUseCase struct {
	convRepo          sellerConversationGetter
	leadRepo          domain.SellerLeadRepository
	contacts          sellerContactGetter
	sellerFlags       sellerFlagChecker
	platformAccountID string
}

func NewConvertLeadUseCase(convRepo sellerConversationGetter, leadRepo domain.SellerLeadRepository, contacts sellerContactGetter, sellerFlags sellerFlagChecker, platformAccountID string) *ConvertLeadUseCase {
	return &ConvertLeadUseCase{convRepo: convRepo, leadRepo: leadRepo, contacts: contacts, sellerFlags: sellerFlags, platformAccountID: platformAccountID}
}

// Search finds candidate customer accounts for the convert dialog. Blank
// or too-short queries return nothing (avoids listing the whole customer
// base to every seller).
func (uc *ConvertLeadUseCase) Search(ctx context.Context, search string) ([]domain.SellerLeadAccount, error) {
	search = strings.TrimSpace(search)
	if len(search) < 2 {
		return []domain.SellerLeadAccount{}, nil
	}
	out, err := uc.leadRepo.SearchCustomerAccounts(ctx, search, uc.platformAccountID, 10)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.SellerLeadAccount{}
	}
	return out, nil
}

// Execute converts the lead: stamps targetAccountID with the
// conversation's assigned attendant as seller + lead_source='inbound'.
// Rules (all enforced here, not in the handler):
//   - conversation must belong to the platform account and have an
//     assigned attendant — that attendant is who gets the commission;
//   - only that assigned attendant or a platform-account admin may
//     trigger the conversion;
//   - the stamped attendant must carry the explicit seller flag
//     (US-014) — converting must never mint a payable seller;
//   - a target with payment history (organically paying customer) is
//     only convertible when the conversation's contact provably belongs
//     to it (contact email matches one of its users) — an attendant
//     cannot claim attribution of an arbitrary existing customer;
//   - an account already linked to any seller is never overwritten
//     (409 — reallocation is an owner-panel action, US-002).
func (uc *ConvertLeadUseCase) Execute(ctx context.Context, actorUserID, actorRole, conversationID, targetAccountID string) (*domain.SellerLeadAccount, error) {
	conv, err := uc.convRepo.GetByID(ctx, conversationID, uc.platformAccountID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, domain.ErrConversationNotFound
	}
	if conv.AssignedTo == nil || *conv.AssignedTo == "" {
		return nil, domain.ErrSellerConvUnassigned
	}
	sellerUserID := *conv.AssignedTo
	if actorRole != "admin" && actorUserID != sellerUserID {
		return nil, domain.ErrSellerNotConvAssignee
	}

	// The person getting the commission must be an explicitly flagged
	// seller — conversion is an attribution write, never a grant.
	isSeller, err := uc.sellerFlags.IsSeller(ctx, sellerUserID)
	if err != nil {
		return nil, err
	}
	if !isSeller {
		return nil, domain.ErrSellerNotFlagged
	}

	target, err := uc.leadRepo.GetLeadAccount(ctx, targetAccountID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, domain.ErrAccountNotFound
	}
	if target.ID == uc.platformAccountID {
		return nil, domain.ErrSellerCannotConvertPlatform
	}
	if target.SellerUserID != nil {
		return nil, domain.ErrSellerAccountAlreadyLinked
	}
	if target.HasPaymentHistory {
		// Organic paying customer: only convertible when the lead
		// conversation's contact provably belongs to it. Users carry no
		// phone, so email is the one verifiable link — no contact email
		// or no match means the claim is unverifiable and is rejected.
		related, err := uc.conversationContactBelongsTo(ctx, conv, target.ID)
		if err != nil {
			return nil, err
		}
		if !related {
			slog.Warn("seller: conversion of paying account blocked — contact does not match",
				"actor_user", actorUserID, "conversation", conversationID, "account", target.ID)
			return nil, domain.ErrSellerConvTargetUnrelated
		}
	}

	stamped, err := uc.leadRepo.StampInboundConversion(ctx, target.ID, sellerUserID, conversationID)
	if err != nil {
		return nil, err
	}
	if !stamped {
		// Lost a race against a concurrent convert (or an owner link).
		return nil, domain.ErrSellerAccountAlreadyLinked
	}

	// Audit trail: structured log with everything needed to reconstruct
	// who converted what, when, and on whose behalf.
	slog.Info("seller: inbound lead converted",
		"actor_user", actorUserID,
		"actor_role", actorRole,
		"seller_user", sellerUserID,
		"conversation", conversationID,
		"account", target.ID,
		"lead_source", domain.LeadSourceInbound,
	)

	return uc.leadRepo.GetLeadAccount(ctx, target.ID)
}

// conversationContactBelongsTo reports whether the conversation's contact
// email matches a user of the target account. Conversation.ContactID is
// the contact's external id (phone/IGSID) on the platform account.
func (uc *ConvertLeadUseCase) conversationContactBelongsTo(ctx context.Context, conv *domain.Conversation, targetAccountID string) (bool, error) {
	if conv.ContactID == "" {
		return false, nil
	}
	contact, err := uc.contacts.GetByExternalID(ctx, uc.platformAccountID, conv.ContactID)
	if err != nil {
		return false, err
	}
	if contact == nil || contact.Email == nil || strings.TrimSpace(*contact.Email) == "" {
		return false, nil
	}
	return uc.leadRepo.AccountHasUserEmail(ctx, targetAccountID, strings.TrimSpace(*contact.Email))
}
