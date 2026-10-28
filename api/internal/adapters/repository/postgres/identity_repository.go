package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Identity repositories (US-F004). Raw SQL rather than sqlc, matching the
// sibling ContactRepository — these tables are written by hand-rolled upserts
// whose ON CONFLICT semantics are the whole point, and sqlc adds nothing over
// a handful of statements.

// ── Companies ───────────────────────────────────────────────────────────────

type CompanyRepository struct {
	db DBTX
}

func NewCompanyRepository(db DBTX) *CompanyRepository {
	return &CompanyRepository{db: db}
}

var _ domain.CompanyRepository = (*CompanyRepository)(nil)

const companyColumns = `id, account_id, name, cnpj, website, metadata, created_at, updated_at`

func (r *CompanyRepository) Upsert(ctx context.Context, c *domain.Company) (*domain.Company, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.ID == "" {
		c.ID = ulid.Make().String()
	}
	now := time.Now()

	// A CNPJ-less company has no dedupe key: two "Clínica São Paulo" rows may
	// be two genuinely different clinics, so we insert rather than guess.
	if c.CNPJ == nil {
		err := r.db.QueryRow(ctx,
			`INSERT INTO companies (id, account_id, name, cnpj, website, metadata, created_at, updated_at)
			 VALUES ($1, $2, $3, NULL, $4, $5, $6, $6)
			 RETURNING `+companyColumns,
			c.ID, c.AccountID, c.Name, c.Website, c.Metadata, now,
		).Scan(&c.ID, &c.AccountID, &c.Name, &c.CNPJ, &c.Website, &c.Metadata, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		return c, nil
	}

	// With a CNPJ the row is idempotent per account. COALESCE keeps a known
	// website/name when the caller only has the CNPJ (a prospecting import
	// must not blank out data an enrichment run already filled in).
	err := r.db.QueryRow(ctx,
		`INSERT INTO companies (id, account_id, name, cnpj, website, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		 ON CONFLICT (account_id, cnpj) WHERE cnpj IS NOT NULL DO UPDATE SET
		   name       = EXCLUDED.name,
		   website    = COALESCE(EXCLUDED.website, companies.website),
		   metadata   = companies.metadata || EXCLUDED.metadata,
		   updated_at = $7
		 RETURNING `+companyColumns,
		c.ID, c.AccountID, c.Name, c.CNPJ, c.Website, c.Metadata, now,
	).Scan(&c.ID, &c.AccountID, &c.Name, &c.CNPJ, &c.Website, &c.Metadata, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CompanyRepository) GetByID(ctx context.Context, accountID, id string) (*domain.Company, error) {
	var c domain.Company
	err := r.db.QueryRow(ctx,
		`SELECT `+companyColumns+` FROM companies WHERE id = $1 AND account_id = $2`,
		id, accountID,
	).Scan(&c.ID, &c.AccountID, &c.Name, &c.CNPJ, &c.Website, &c.Metadata, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Same no-leak convention as ContactRepository.GetByID: a foreign
		// account's id is indistinguishable from a missing one.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CompanyRepository) GetByCNPJ(ctx context.Context, accountID, cnpj string) (*domain.Company, error) {
	digits := domain.NormalizeCNPJ(cnpj)
	if digits == "" {
		return nil, nil
	}
	var c domain.Company
	err := r.db.QueryRow(ctx,
		`SELECT `+companyColumns+` FROM companies WHERE account_id = $1 AND cnpj = $2`,
		accountID, digits,
	).Scan(&c.ID, &c.AccountID, &c.Name, &c.CNPJ, &c.Website, &c.Metadata, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CompanyRepository) ListByAccount(ctx context.Context, accountID string, limit, offset int) ([]*domain.Company, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+companyColumns+` FROM companies
		 WHERE account_id = $1 ORDER BY name ASC LIMIT $2 OFFSET $3`,
		accountID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.Company{}
	for rows.Next() {
		var c domain.Company
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &c.CNPJ, &c.Website,
			&c.Metadata, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// ── Contact identities ──────────────────────────────────────────────────────

type ContactIdentityRepository struct {
	db DBTX
}

func NewContactIdentityRepository(db DBTX) *ContactIdentityRepository {
	return &ContactIdentityRepository{db: db}
}

var _ domain.ContactIdentityRepository = (*ContactIdentityRepository)(nil)

const identityColumns = `id, account_id, contact_id, kind, value, verified_at, created_at, updated_at`

func (r *ContactIdentityRepository) Upsert(ctx context.Context, i *domain.ContactIdentity) (*domain.ContactIdentity, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	if i.ID == "" {
		i.ID = ulid.Make().String()
	}
	now := time.Now()
	err := r.db.QueryRow(ctx,
		`INSERT INTO contact_identities (id, account_id, contact_id, kind, value, verified_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		 ON CONFLICT (account_id, kind, value) DO UPDATE SET
		   -- The existing owner wins. A later door (a CSV re-import, a Places
		   -- hit, a manual lead) reaching an address that already belongs to
		   -- somebody used to STEAL it, re-pointing the identity away from the
		   -- contact that has the sales and the conversations. The no-op
		   -- assignment (not DO NOTHING) is what makes RETURNING hand the
		   -- caller the row that stands, so it can reuse that contact.
		   contact_id  = contact_identities.contact_id,
		   -- Never un-verify: proof once obtained stays proof.
		   verified_at = COALESCE(contact_identities.verified_at, EXCLUDED.verified_at),
		   updated_at  = $7
		 RETURNING `+identityColumns,
		i.ID, i.AccountID, i.ContactID, string(i.Kind), i.Value, i.VerifiedAt, now,
	).Scan(&i.ID, &i.AccountID, &i.ContactID, &i.Kind, &i.Value, &i.VerifiedAt, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *ContactIdentityRepository) ListByContact(ctx context.Context, accountID, contactID string) ([]*domain.ContactIdentity, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+identityColumns+` FROM contact_identities
		 WHERE account_id = $1 AND contact_id = $2
		 ORDER BY created_at ASC`,
		accountID, contactID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.ContactIdentity{}
	for rows.Next() {
		var i domain.ContactIdentity
		if err := rows.Scan(&i.ID, &i.AccountID, &i.ContactID, &i.Kind, &i.Value,
			&i.VerifiedAt, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &i)
	}
	return out, rows.Err()
}

func (r *ContactIdentityRepository) ContactIDByValue(ctx context.Context, accountID string, kind domain.IdentityKind, value string) (string, error) {
	normalized := domain.NormalizeIdentityValue(kind, value)
	if normalized == "" {
		return "", nil
	}
	var contactID string
	err := r.db.QueryRow(ctx,
		`SELECT contact_id FROM contact_identities
		 WHERE account_id = $1 AND kind = $2 AND value = $3`,
		accountID, string(kind), normalized,
	).Scan(&contactID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Unknown identifier is the normal case for a first inbound — not
		// an error, the caller creates the contact.
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return contactID, nil
}

func (r *ContactIdentityRepository) MarkVerified(ctx context.Context, accountID, id string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE contact_identities SET verified_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND account_id = $2 AND verified_at IS NULL`,
		id, accountID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *ContactIdentityRepository) Delete(ctx context.Context, accountID, id string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM contact_identities WHERE id = $1 AND account_id = $2`,
		id, accountID,
	)
	return err
}
