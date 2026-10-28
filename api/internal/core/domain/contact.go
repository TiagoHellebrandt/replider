package domain

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// e164Pattern matches E.164 phone numbers: a leading '+', a non-zero first
// digit, then up to 14 more digits (15 digits total max per ITU-T E.164).
// Garbage phones (letters, spaces, missing '+') break contact dedup/search,
// so save paths reject them before persisting.
var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// ErrInvalidPhone is returned by Contact save paths when Phone is set but is
// not a valid E.164 number.
var ErrInvalidPhone = &AppError{Code: "INVALID_PHONE", Message: "phone must be E.164 format (e.g. +5511999999999)"}

type Contact struct {
	ID          string  `json:"id"`
	AccountID   string  `json:"account_id"`
	ExternalID  string  `json:"external_id"`
	ChannelType *string `json:"channel_type,omitempty"`
	Name        *string `json:"name,omitempty"`
	// CompanyName is the organization the person is at. Kept on the contact
	// rather than in `companies` (which nothing reads): a CSV row "Clínica
	// Aurora;Marina" used to lose the clinic the moment it had a person.
	CompanyName *string         `json:"company_name,omitempty"`
	Email       *string         `json:"email,omitempty"`
	Phone       *string         `json:"phone,omitempty"`
	AvatarURL   *string         `json:"avatar_url,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	// OptedOutAt, when non-nil, means the contact explicitly asked to stop
	// receiving broadcast messages (LGPD Art. 18 / CAN-SPAM). The timestamp
	// is kept for audit; campaigns must filter these out at creation time.
	OptedOutAt  *time.Time `json:"opted_out_at,omitempty"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

var ErrContactNotFound = &AppError{Code: "CONTACT_NOT_FOUND", Message: "contact not found"}

// ErrContactOptedOut is returned by outbound send paths (template or text)
// when the recipient has previously opted out (contacts.opted_out_at set).
// Messaging an opted-out contact in cold outbound is the single fastest way
// to get a WhatsApp number banned, so the send is aborted BEFORE any call to
// Meta and this error (code CONTACT_OPTED_OUT) is logged (US-025).
var ErrContactOptedOut = &AppError{Code: "CONTACT_OPTED_OUT", Message: "contact has opted out of messaging"}

// NormalizeE164 strips human-formatting (spaces, dashes, parentheses, dots)
// from a phone number and returns the canonical E.164 form plus true when the
// result is a valid E.164 number. A number without a leading '+' (e.g. a
// directory's national-format fallback) cannot be reliably promoted to E.164,
// so it returns ("", false) — callers then treat the candidate as phone-less.
// NormalizeTypedPhone accepts a phone the way a person actually types one
// and returns it in E.164.
//
// NormalizeE164 is deliberately strict: it is fed by channel webhooks, where
// the number already arrives fully qualified, and loosening it there would
// silently mis-country foreign contacts. But a human filling in a form types
// "51 98888-7766" or "(51) 98888-7766", and this product is BR-first — a
// salesperson at a hospital door rejected for typing their own country's
// format has met a wall, not a validation.
//
// So: already-qualified input goes through untouched; a bare national number
// (10 digits landline, 11 mobile) gets +55; digits already starting with the
// country code get the plus. Anything else is still refused — guessing a
// country for an ambiguous string would put messages on a stranger's phone.
func NormalizeTypedPhone(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	// An explicit + means the caller already said which country.
	if strings.HasPrefix(trimmed, "+") {
		return NormalizeE164(trimmed)
	}
	var digits strings.Builder
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case len(d) == 10 || len(d) == 11:
		// DDD + number. 11 digits is a mobile (the leading 9), 10 a landline.
		return NormalizeE164("+55" + d)
	case (len(d) == 12 || len(d) == 13) && strings.HasPrefix(d, "55"):
		return NormalizeE164("+" + d)
	default:
		return "", false
	}
}

// ContactKey is THE external_id for a person, whichever door they came in by.
//
// Every door used to key contacts.external_id its own way — the CSV import
// wrote bare digits, a manual lead or a Places hit wrote E.164 with the '+',
// the inbound webhook wrote whatever Meta sent — so the same hospital was
// two, three, four rows, and the sales history lived on whichever one the
// door had happened to touch. One rule, everywhere: the phone as digits only
// (the way the WhatsApp channel keys a sender, which is what every
// conversation lookup arrives on), else the lower-cased e-mail.
//
// The phone must already be E.164 (NormalizeTypedPhone for what a human
// typed, NormalizeE164 for a qualified number); a bare national number is
// refused here because this function cannot know its country. Returns
// ("", false) when neither address is usable — such a contact is keyed by
// the door's own id (cnes:<id>, pncp:<id>, a place_id) and must not pretend
// to be reachable.
func ContactKey(phone, email string) (string, bool) {
	if e164, ok := NormalizeE164(phone); ok {
		return strings.TrimPrefix(e164, "+"), true
	}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" && IsEmailAddress(e) {
		return e, true
	}
	return "", false
}

func NormalizeE164(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	var b strings.Builder
	for i, r := range raw {
		switch {
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if !e164Pattern.MatchString(out) {
		return "", false
	}
	return out, true
}

// Validate checks the contact's persistable fields. Currently it guards the
// phone number: an empty/nil phone is allowed (it's optional), but a present
// one must be E.164. Call this from every save path (Upsert/Update).
// DisplayLabel is the shortest string that lets a human recognise this
// contact. Name first, then whatever channel identifier we do have —
// falling all the way back to ExternalID, which is at least the address the
// message would actually go to.
// DisplayAddress is where a message to this contact would actually go —
// the phone, the email, or the raw channel id. Never the name.
//
// A confirmation that says only "Clínica Aurora" cannot catch the mistake
// it exists to catch: two clinics with similar names, or a number typed
// wrong. The person approving a cold outbound message has to see the
// address, not a label someone chose.
func (c *Contact) DisplayAddress() string {
	for _, v := range []*string{c.Phone, c.Email} {
		if v != nil && strings.TrimSpace(*v) != "" {
			return strings.TrimSpace(*v)
		}
	}
	return c.ExternalID
}

func (c *Contact) DisplayLabel() string {
	for _, v := range []*string{c.Name, c.Phone, c.Email} {
		if v != nil && strings.TrimSpace(*v) != "" {
			return strings.TrimSpace(*v)
		}
	}
	return c.ExternalID
}

func (c *Contact) Validate() error {
	if c.Phone != nil && strings.TrimSpace(*c.Phone) != "" {
		if !e164Pattern.MatchString(*c.Phone) {
			return ErrInvalidPhone
		}
	}
	return nil
}
