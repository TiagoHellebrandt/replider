package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// --- List Contacts ---

type ListContactsUseCase struct {
	contactRepo domain.ContactRepository
}

func NewListContactsUseCase(contactRepo domain.ContactRepository) *ListContactsUseCase {
	return &ListContactsUseCase{contactRepo: contactRepo}
}

func (uc *ListContactsUseCase) Execute(ctx context.Context, accountID, query string, limit, offset int) ([]*domain.Contact, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return uc.contactRepo.ListByAccountID(ctx, accountID, query, limit, offset)
}

// --- Get Contact ---

type GetContactUseCase struct {
	contactRepo domain.ContactRepository
}

func NewGetContactUseCase(contactRepo domain.ContactRepository) *GetContactUseCase {
	return &GetContactUseCase{contactRepo: contactRepo}
}

func (uc *GetContactUseCase) Execute(ctx context.Context, id, accountID string) (*domain.Contact, error) {
	// Account scope is enforced in the repository's WHERE clause; a
	// foreign-account ID comes back as (nil, nil), indistinguishable
	// from a missing one.
	contact, err := uc.contactRepo.GetByID(ctx, id, accountID)
	if err != nil {
		return nil, err
	}
	if contact == nil {
		return nil, domain.ErrContactNotFound
	}
	return contact, nil
}

// --- Update Contact ---

type UpdateContactDTO struct {
	Name        *string `json:"name"`
	CompanyName *string `json:"company_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	Metadata    []byte  `json:"metadata,omitempty"`
}

type UpdateContactUseCase struct {
	contactRepo domain.ContactRepository
	// identityRepo is optional (SetIdentityRepo). Without it an edited
	// address is saved on the row but not indexed — the next inbound from
	// the new number would create a stranger.
	identityRepo domain.ContactIdentityRepository
}

func NewUpdateContactUseCase(contactRepo domain.ContactRepository) *UpdateContactUseCase {
	return &UpdateContactUseCase{contactRepo: contactRepo}
}

// SetIdentityRepo wires the identity index so a changed phone/e-mail becomes
// a way to find this contact.
func (uc *UpdateContactUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identityRepo = r
}

func (uc *UpdateContactUseCase) Execute(ctx context.Context, id, accountID string, dto UpdateContactDTO) (*domain.Contact, error) {
	// Repository scopes the lookup to accountID; foreign IDs return
	// (nil, nil) so we can't mutate another tenant's contact.
	contact, err := uc.contactRepo.GetByID(ctx, id, accountID)
	if err != nil {
		return nil, err
	}
	if contact == nil {
		return nil, domain.ErrContactNotFound
	}

	if dto.Name != nil {
		contact.Name = dto.Name
	}
	if dto.CompanyName != nil {
		contact.CompanyName = dto.CompanyName
	}
	var newPhone, newEmail string
	if dto.Email != nil {
		if e := strings.ToLower(strings.TrimSpace(*dto.Email)); e != "" && !sameAddress(contact.Email, e) {
			newEmail = e
		}
		contact.Email = dto.Email
	}
	if dto.Phone != nil {
		if p := strings.TrimSpace(*dto.Phone); p != "" && !sameAddress(contact.Phone, p) {
			newPhone = p
		}
		contact.Phone = dto.Phone
	}
	if dto.Metadata != nil {
		contact.Metadata = dto.Metadata
	}

	// Reject garbage phone before hitting the DB; the repository revalidates
	// as the durable backstop (US-049).
	if err := contact.Validate(); err != nil {
		return nil, err
	}

	if err := uc.contactRepo.Update(ctx, contact); err != nil {
		return nil, err
	}

	// A changed address is a NEW way to reach the same person, so it joins
	// the index. The old identity is never deleted: it is how a legacy
	// thread — a reply to something sent to the old address — still finds
	// them. An address already owned by somebody else stays theirs (the
	// index never re-points), which the resolver logs rather than fights.
	if newPhone != "" || newEmail != "" {
		newContactResolver(uc.contactRepo, uc.identityRepo).
			registerIdentities(ctx, contact, newPhone, newEmail, contactResolveOpts{})
	}
	return contact, nil
}

// sameAddress reports whether the stored address already equals the new
// one, ignoring case and surrounding space.
func sameAddress(stored *string, candidate string) bool {
	return stored != nil && strings.EqualFold(strings.TrimSpace(*stored), strings.TrimSpace(candidate))
}
