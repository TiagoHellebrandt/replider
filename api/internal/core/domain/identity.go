package domain

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// Multi-channel identity model (US-F004).
//
// The pre-pivot model keyed a person by their phone number: contacts.id ==
// external_id == E.164. Outbound B2B breaks that assumption — a prospect is
// usually found by company + work email, and the phone (if any) arrives
// later. So identity splits in two:
//
//	Company           the organization we sell TO
//	Contact           the person (opaque id, from US-F005 on)
//	ContactIdentity   one addressable identifier of that person
//
// A contact with a phone AND a work email is ONE contact with TWO
// identities, which is what makes "they replied by email, follow up on
// WhatsApp" a single thread instead of two strangers.

// IdentityKind enumerates the channels an identifier can address. The DB
// mirrors this list in a CHECK constraint — extend both together.
type IdentityKind string

const (
	IdentityKindPhone     IdentityKind = "phone"
	IdentityKindEmail     IdentityKind = "email"
	IdentityKindLinkedIn  IdentityKind = "linkedin"
	IdentityKindInstagram IdentityKind = "instagram"
	IdentityKindTelegram  IdentityKind = "telegram"
)

var validIdentityKinds = map[IdentityKind]struct{}{
	IdentityKindPhone:     {},
	IdentityKindEmail:     {},
	IdentityKindLinkedIn:  {},
	IdentityKindInstagram: {},
	IdentityKindTelegram:  {},
}

func (k IdentityKind) Valid() bool {
	_, ok := validIdentityKinds[k]
	return ok
}

var (
	ErrInvalidIdentityKind  = &AppError{Code: "IDENTITY_INVALID_KIND", Message: "identity kind must be one of phone, email, linkedin, instagram, telegram"}
	ErrInvalidIdentityValue = &AppError{Code: "IDENTITY_INVALID_VALUE", Message: "identity value is empty or malformed for its kind"}
	ErrIdentityNotFound     = &AppError{Code: "IDENTITY_NOT_FOUND", Message: "contact identity not found"}
	ErrCompanyNotFound      = &AppError{Code: "COMPANY_NOT_FOUND", Message: "company not found"}
	ErrInvalidCNPJ          = &AppError{Code: "COMPANY_INVALID_CNPJ", Message: "cnpj must be 14 digits"}
)

// emailPattern is deliberately permissive — it rejects the shapes that
// would poison the uniqueness index (no @, spaces, empty parts), not every
// RFC-invalid address. Real validation is "did it bounce".
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$`)

// cnpjDigits strips a Brazilian CNPJ down to its 14 digits so "12.345.678/
// 0001-99" and "12345678000199" are the same company.
var nonDigits = regexp.MustCompile(`\D`)

// NormalizeCNPJ returns the digits-only form, or "" when there is nothing
// left. Callers persist "" as NULL.
func NormalizeCNPJ(raw string) string {
	return nonDigits.ReplaceAllString(strings.TrimSpace(raw), "")
}

// Company is an organization in an account's world: a prospect, a customer,
// or a supplier. Scoped per account — two Replider customers prospecting the
// same CNPJ each own their own row (their notes, their pipeline).
type Company struct {
	ID        string          `json:"id"`
	AccountID string          `json:"account_id"`
	Name      string          `json:"name"`
	CNPJ      *string         `json:"cnpj,omitempty"`
	Website   *string         `json:"website,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Validate normalizes the CNPJ in place and rejects rows that would be
// unusable (no name, malformed CNPJ).
func (c *Company) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return &AppError{Code: "VALIDATION_ERROR", Message: "company name is required"}
	}
	if c.CNPJ != nil {
		digits := NormalizeCNPJ(*c.CNPJ)
		if digits == "" {
			c.CNPJ = nil
		} else {
			if len(digits) != 14 {
				return ErrInvalidCNPJ
			}
			c.CNPJ = &digits
		}
	}
	if len(c.Metadata) == 0 {
		c.Metadata = json.RawMessage("{}")
	}
	return nil
}

// ContactIdentity is one way to reach a contact. (account, kind, value) is
// unique: inside an account an identifier belongs to exactly one person,
// which is what lets the inbound resolver answer "who is this?" with a
// single lookup.
type ContactIdentity struct {
	ID         string       `json:"id"`
	AccountID  string       `json:"account_id"`
	ContactID  string       `json:"contact_id"`
	Kind       IdentityKind `json:"kind"`
	Value      string       `json:"value"`
	VerifiedAt *time.Time   `json:"verified_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// Validate normalizes the value for its kind and rejects malformed ones.
// Normalization is part of validation on purpose: an un-normalized value
// silently defeats the uniqueness index (`Joe@ACME.com` vs `joe@acme.com`
// would be two identities for one mailbox).
func (i *ContactIdentity) Validate() error {
	if !i.Kind.Valid() {
		return ErrInvalidIdentityKind
	}
	i.Value = NormalizeIdentityValue(i.Kind, i.Value)
	if i.Value == "" {
		return ErrInvalidIdentityValue
	}
	switch i.Kind {
	case IdentityKindPhone:
		if !e164Pattern.MatchString(i.Value) {
			return ErrInvalidPhone
		}
	case IdentityKindEmail:
		if !emailPattern.MatchString(i.Value) {
			return ErrInvalidIdentityValue
		}
	}
	if strings.TrimSpace(i.ContactID) == "" || strings.TrimSpace(i.AccountID) == "" {
		return ErrInvalidIdentityValue
	}
	return nil
}

// NormalizeIdentityValue is the single normalization rule per kind. Every
// write AND every lookup must go through it, or a lookup misses a row that
// is really there.
func NormalizeIdentityValue(kind IdentityKind, value string) string {
	v := strings.TrimSpace(value)
	switch kind {
	case IdentityKindEmail:
		return strings.ToLower(v)
	case IdentityKindPhone:
		// Values reaching here must ALREADY be E.164. This function cannot
		// promote a bare number, because the promotion depends on where the
		// number came from and it does not know: "11999990000" typed into a
		// form is a Brazilian national number, and the same digits with a
		// '+' in front address Canada. Callers convert at the boundary that
		// knows the origin — NormalizeTypedPhone for what a human typed,
		// IdentityValueFromChannelSender for what a channel sent.
		return strings.ReplaceAll(v, " ", "")
	case IdentityKindLinkedIn, IdentityKindInstagram:
		return strings.ToLower(strings.TrimPrefix(v, "@"))
	default:
		return v
	}
}

// IdentityValueFromChannelSender turns a sender as a CHANNEL wrote it into an
// identity value.
//
// This exists because the identity index was empty for the product's main
// channel. Meta writes a WhatsApp sender as bare digits ("5511999999999"); that
// value failed the E.164 check in Validate, the write was rejected, and every
// caller — all of which treat identity registration as best-effort — logged a
// warning and carried on. The index whose entire job is "this email and this
// WhatsApp are the same person" therefore had no WhatsApp rows at all.
//
// A sender from a channel is already fully qualified: the platform routed the
// message to us, so the number carries its country code. That is what makes a
// bare '+' correct HERE and wrong in NormalizeIdentityValue, which sees values
// from every origin. Deliberately not NormalizeTypedPhone either: that promotes
// eleven digits to +55, right for a Brazilian filling in a form and wrong for a
// sender whose digits are already country-coded.
//
// Returns "" when the raw value cannot be an identity of that kind, which the
// caller treats as "do not index this one" rather than as an error.
func IdentityValueFromChannelSender(kind IdentityKind, raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	if kind == IdentityKindPhone && !strings.HasPrefix(v, "+") {
		digits := nonDigits.ReplaceAllString(v, "")
		// A channel id that is not a phone at all (a webchat session key, say)
		// resolves to the phone kind by default. Leaving it unqualified means
		// Validate refuses it, which is the behaviour that was already right.
		if digits != v || digits == "" {
			return v
		}
		v = "+" + digits
	}
	return NormalizeIdentityValue(kind, v)
}

// CompanyRepository persists organizations. Every method is account-scoped;
// there is no cross-account read (the shared company knowledge base is a
// separate, deliberately-shared cache — US-M001).
type CompanyRepository interface {
	// Upsert creates the company, or updates the existing row when the
	// account already has one with the same CNPJ. A company with no CNPJ
	// is always an insert — we have no reliable key to dedupe it on, and
	// silently merging two same-named companies loses data.
	Upsert(ctx context.Context, company *Company) (*Company, error)
	GetByID(ctx context.Context, accountID, id string) (*Company, error)
	// GetByCNPJ returns (nil, nil) when the account has no company with
	// that CNPJ. The argument is normalized internally, so callers may
	// pass a masked CNPJ.
	GetByCNPJ(ctx context.Context, accountID, cnpj string) (*Company, error)
	ListByAccount(ctx context.Context, accountID string, limit, offset int) ([]*Company, error)
}

// ContactIdentityRepository persists the identifiers a contact can be
// reached on, and answers the inbound resolver's question: given an
// identifier, who is this?
type ContactIdentityRepository interface {
	// Upsert is idempotent on (account_id, kind, value). Re-registering an
	// identity that already belongs to ANOTHER contact does NOT re-point
	// it: the row that stands is returned, with ITS contact_id, and the
	// caller reuses that contact. The older contact is the one with the
	// sales and the conversations; a later door must never take its
	// address away.
	Upsert(ctx context.Context, identity *ContactIdentity) (*ContactIdentity, error)
	ListByContact(ctx context.Context, accountID, contactID string) ([]*ContactIdentity, error)
	// ContactIDByValue returns the contact owning this identifier, or ""
	// when nobody does. The value is normalized internally so callers
	// can't miss a row by passing "Joe@ACME.com".
	ContactIDByValue(ctx context.Context, accountID string, kind IdentityKind, value string) (string, error)
	// MarkVerified stamps verified_at (first proof the identifier really
	// reaches the person). Idempotent: an already-verified identity keeps
	// its original timestamp.
	MarkVerified(ctx context.Context, accountID, id string) (bool, error)
	Delete(ctx context.Context, accountID, id string) error
}
