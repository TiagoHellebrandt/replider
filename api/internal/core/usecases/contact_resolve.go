package usecases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// One customer, one key.
//
// The same hospital used to become two, three, four contact rows because
// every door created its own: the CSV import keyed by bare digits, a manual
// lead or a Places hit by "+55…", the CNES radar by "cnes:<id>", and only the
// import ever asked the identity index before writing. This is the single
// place that answers "do we already know this person?" and, when we do not,
// creates them — every door goes through it BEFORE Upsert.
//
// Resolution order: the identity index (phone, then e-mail, then whatever
// channel-specific id the caller has), then contacts.external_id under the
// canonical key, then the legacy spellings of the same address ("+"-prefixed
// phone, bare digits, the e-mail) that older doors wrote. The legacy step is
// what lets a contact created before this file existed keep its history
// instead of gaining a twin.

// identityProbe is one (kind, value) a door knows about the person beyond
// phone and e-mail — a Telegram chat id, an Instagram handle.
type identityProbe struct {
	Kind  domain.IdentityKind
	Value string
}

type contactResolveOpts struct {
	// Extra identities to look up and register, beyond Draft.Phone/Email.
	Extra []identityProbe
	// VerifiedAt is stamped on the identities registered by this call. An
	// inbound message is proof the address reaches the person; a spreadsheet
	// row or a directory hit is a claim, and passes nil.
	VerifiedAt *time.Time
	// LegacyKeys are external ids an older version of the SAME door wrote
	// for this person ("cnes:<id>" before the radar keyed by phone), or the
	// key ANOTHER door uses for the same organization (the buyer's CNPJ,
	// which is how a tender files it). Tried after the addresses, so the
	// contact keeps its history instead of gaining a twin.
	LegacyKeys []string
	// EmailNotAKey says the draft's e-mail reaches an ORGANIZATION, not this
	// contact alone — so it may be stored on the row but must never decide
	// who this is.
	//
	// The CNES registry is the case that needs it. Measured over its 3.208
	// published addresses in Mato Grosso, 538 are shared: thirty-three
	// different establishments publish saude@rondonopolis.mt.gov.br. Keying
	// on that would merge thirty-three hospitals into one contact and hand
	// the seller one card where there are thirty-three buyers. The phone
	// (3.955 distinct of 4.449) is the weaker version of the same problem
	// and is already the key; the e-mail is an order of magnitude worse.
	EmailNotAKey bool
}

type contactResolver struct {
	contacts   domain.ContactRepository
	identities domain.ContactIdentityRepository // optional
}

func newContactResolver(contacts domain.ContactRepository, identities domain.ContactIdentityRepository) *contactResolver {
	return &contactResolver{contacts: contacts, identities: identities}
}

// lookup finds the contact behind any of the given addresses, or nil.
// Lookup failures are treated as misses — every caller is on a write path
// where "create a possible duplicate" beats "lose the row".
func (r *contactResolver) lookup(ctx context.Context, accountID, phone, email, externalID string, extra ...identityProbe) *domain.Contact {
	if r == nil || r.contacts == nil {
		return nil
	}
	probes := make([]identityProbe, 0, 2+len(extra))
	if e164, ok := domain.NormalizeE164(phone); ok {
		probes = append(probes, identityProbe{domain.IdentityKindPhone, e164})
	}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		probes = append(probes, identityProbe{domain.IdentityKindEmail, e})
	}
	probes = append(probes, extra...)

	if r.identities != nil {
		for _, p := range probes {
			if p.Value == "" {
				continue
			}
			id, err := r.identities.ContactIDByValue(ctx, accountID, p.Kind, p.Value)
			if err != nil || id == "" {
				continue
			}
			if c, err := r.contacts.GetByID(ctx, id, accountID); err == nil && c != nil {
				return c
			}
		}
	}

	// The keys older doors wrote, canonical first. Deduplicated so a caller
	// whose externalID already IS the canonical key costs one query.
	seen := map[string]bool{}
	keys := make([]string, 0, 5)
	add := func(k string) {
		if k = strings.TrimSpace(k); k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	add(externalID)
	if key, ok := domain.ContactKey(phone, email); ok {
		add(key)
	}
	if e164, ok := domain.NormalizeE164(phone); ok {
		add(e164)
		add(strings.TrimPrefix(e164, "+"))
	}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		add(e)
	}
	for _, k := range keys {
		if c, err := r.contacts.GetByExternalID(ctx, accountID, k); err == nil && c != nil {
			return c
		}
	}
	return nil
}

// find is the whole question "does this account already know this person?",
// under one caller's rules: the addresses first, then the keys other doors
// use for the same organization.
//
// Exported to the package (rather than living inside resolve) because the
// SWEEP has to ask exactly what the WRITER answers. It did not: the nightly
// hunt asked "is there a contact under cnes:<id>?" while EnsureLead had long
// since started keying reachable establishments by phone. Measured in
// production on 2026-09-17, 51 of 60 hunted hospitals carried a phone key, so
// the sweep called 52 of them new ground, re-walked the same hospitals every
// night and created nothing: `auto hunt: tick complete found=52 created=0`.
func (r *contactResolver) find(
	ctx context.Context, accountID, phone, email, externalID string, opts contactResolveOpts,
) *domain.Contact {
	if r == nil || r.contacts == nil {
		return nil
	}
	if opts.EmailNotAKey {
		email = ""
	}
	if c := r.lookup(ctx, accountID, phone, email, externalID, opts.Extra...); c != nil {
		return c
	}
	for _, k := range opts.LegacyKeys {
		if k = strings.TrimSpace(k); k == "" {
			continue
		}
		if c, err := r.contacts.GetByExternalID(ctx, accountID, k); err == nil && c != nil {
			return c
		}
	}
	return nil
}

// resolve returns the existing contact for the draft's addresses, or creates
// the draft. The second result reports which. Either way the draft's phone
// and e-mail end up registered as identities of the contact returned.
//
// A found contact is completed, never overwritten: a name, phone, e-mail or
// company the draft carries fills a blank column and leaves a filled one
// alone (what the channel learned outranks what a spreadsheet says).
func (r *contactResolver) resolve(ctx context.Context, draft *domain.Contact, opts contactResolveOpts) (*domain.Contact, bool, error) {
	if r == nil || r.contacts == nil || draft == nil {
		return nil, false, domain.ErrContactNotFound
	}
	phone, email := derefStr(draft.Phone), derefStr(draft.Email)

	found := r.find(ctx, draft.AccountID, phone, email, draft.ExternalID, opts)
	if found != nil {
		if r.backfill(found, draft) {
			if err := r.contacts.Update(ctx, found); err != nil {
				slog.WarnContext(ctx, "contact resolve: backfill update failed",
					"contact_id", found.ID, "error", err)
			}
		}
		r.registerIdentities(ctx, found, phone, email, opts)
		return found, false, nil
	}

	saved, err := r.contacts.Upsert(ctx, draft)
	if err != nil {
		return nil, false, err
	}
	r.registerIdentities(ctx, saved, phone, email, opts)
	return saved, true, nil
}

// backfill copies the draft's fields into the blanks of an existing contact.
// Reports whether anything changed.
func (r *contactResolver) backfill(existing, draft *domain.Contact) bool {
	changed := false
	fill := func(dst **string, src *string) {
		if src == nil || strings.TrimSpace(*src) == "" {
			return
		}
		if *dst == nil || strings.TrimSpace(**dst) == "" {
			v := *src
			*dst = &v
			changed = true
		}
	}
	fill(&existing.Name, draft.Name)
	fill(&existing.CompanyName, draft.CompanyName)
	fill(&existing.Phone, draft.Phone)
	fill(&existing.Email, draft.Email)
	return changed
}

// registerIdentities indexes every address the caller knows for the contact.
// Best-effort: the index is how the NEXT door finds this person, and a
// failure to write it degrades to a possible duplicate later — never to a
// lost write now. An address that already belongs to another contact stays
// there (the repository never re-points); it is logged, not fought over.
func (r *contactResolver) registerIdentities(ctx context.Context, contact *domain.Contact, phone, email string, opts contactResolveOpts) {
	if r.identities == nil || contact == nil {
		return
	}
	probes := make([]identityProbe, 0, 2+len(opts.Extra))
	if e164, ok := domain.NormalizeE164(phone); ok {
		probes = append(probes, identityProbe{domain.IdentityKindPhone, e164})
	}
	// A shared organizational address is not an identity of this contact:
	// indexing it would hand the NEXT door a merge key we just refused.
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" && !opts.EmailNotAKey {
		probes = append(probes, identityProbe{domain.IdentityKindEmail, e})
	}
	probes = append(probes, opts.Extra...)
	for _, p := range probes {
		if p.Value == "" {
			continue
		}
		row, err := r.identities.Upsert(ctx, &domain.ContactIdentity{
			AccountID:  contact.AccountID,
			ContactID:  contact.ID,
			Kind:       p.Kind,
			Value:      p.Value,
			VerifiedAt: opts.VerifiedAt,
		})
		if err != nil {
			slog.WarnContext(ctx, "contact resolve: failed to register identity",
				"contact_id", contact.ID, "kind", string(p.Kind), "error", err)
			continue
		}
		if row != nil && row.ContactID != "" && row.ContactID != contact.ID {
			slog.InfoContext(ctx, "contact resolve: identity already belongs to another contact; left in place",
				"contact_id", contact.ID, "owner_contact_id", row.ContactID, "kind", string(p.Kind))
		}
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
