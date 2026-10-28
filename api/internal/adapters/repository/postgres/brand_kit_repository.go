package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type BrandKitRepository struct{ db DBTX }

func NewBrandKitRepository(db DBTX) *BrandKitRepository {
	return &BrandKitRepository{db: db}
}

// Get returns nil when the account has not filled one in. Nil rather than a
// zero-valued kit: the composer must be able to tell "no brand configured"
// from "a brand with an empty name", because the first is a setup nudge and
// the second is a mistake.
func (r *BrandKitRepository) Get(ctx context.Context, accountID string) (*domain.BrandKit, error) {
	k := &domain.BrandKit{AccountID: accountID}
	err := r.db.QueryRow(ctx,
		`SELECT company_name, about, tone_of_voice, logo_url, primary_color,
		        signature, allowed_claims, forbidden_claims, updated_at
		   FROM brand_kits WHERE account_id = $1`, accountID).
		Scan(&k.CompanyName, &k.About, &k.ToneOfVoice, &k.LogoURL, &k.PrimaryColor,
			&k.Signature, &k.AllowedClaims, &k.ForbiddenClaims, &k.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return k, nil
}

func (r *BrandKitRepository) Save(ctx context.Context, k *domain.BrandKit) error {
	// Never nil: an empty array and NULL mean the same thing to a reader and
	// different things to a NOT NULL column.
	allowed, forbidden := k.AllowedClaims, k.ForbiddenClaims
	if allowed == nil {
		allowed = []string{}
	}
	if forbidden == nil {
		forbidden = []string{}
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO brand_kits
		   (account_id, company_name, about, tone_of_voice, logo_url,
		    primary_color, signature, allowed_claims, forbidden_claims, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
		 ON CONFLICT (account_id) DO UPDATE
		    SET company_name = EXCLUDED.company_name,
		        about = EXCLUDED.about,
		        tone_of_voice = EXCLUDED.tone_of_voice,
		        logo_url = EXCLUDED.logo_url,
		        primary_color = EXCLUDED.primary_color,
		        signature = EXCLUDED.signature,
		        allowed_claims = EXCLUDED.allowed_claims,
		        forbidden_claims = EXCLUDED.forbidden_claims,
		        updated_at = NOW()`,
		k.AccountID, k.CompanyName, k.About, k.ToneOfVoice, k.LogoURL,
		k.PrimaryColor, k.Signature, allowed, forbidden)
	return err
}

func (r *BrandKitRepository) GetOverlay(ctx context.Context, accountID, userID string) (*domain.SellerOverlay, error) {
	o := &domain.SellerOverlay{AccountID: accountID, UserID: userID}
	err := r.db.QueryRow(ctx,
		`SELECT tone_note, signature, updated_at
		   FROM seller_overlays WHERE account_id = $1 AND user_id = $2`,
		accountID, userID).Scan(&o.ToneNote, &o.Signature, &o.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return o, nil
}

func (r *BrandKitRepository) SaveOverlay(ctx context.Context, o *domain.SellerOverlay) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO seller_overlays (account_id, user_id, tone_note, signature, updated_at)
		 VALUES ($1,$2,$3,$4,NOW())
		 ON CONFLICT (account_id, user_id) DO UPDATE
		    SET tone_note = EXCLUDED.tone_note,
		        signature = EXCLUDED.signature,
		        updated_at = NOW()`,
		o.AccountID, o.UserID, o.ToneNote, o.Signature)
	return err
}
