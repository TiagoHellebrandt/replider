package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// OwnerSellerRepository backs the owner panel's Vendedores section
// (owner-sellers US-001). It composes the existing raw-SQL feature
// repos (seller_settings, users.seller_ref_code, auto_assign_settings)
// over one shared DBTX, so InTx rebinds ALL of them to the same
// transaction — the onboard's flag + ref-code + rotation writes commit
// or roll back together.
type OwnerSellerRepository struct {
	db         DBTX
	commission *CommissionRepository
	sellerRef  *SellerRefRepository
	autoAssign *AutoAssignSettingsRepository
}

func NewOwnerSellerRepository(db DBTX) *OwnerSellerRepository {
	return &OwnerSellerRepository{
		db:         db,
		commission: NewCommissionRepository(db),
		sellerRef:  NewSellerRefRepository(db),
		autoAssign: NewAutoAssignSettingsRepository(db),
	}
}

// InTx runs fn with every delegate bound to one transaction. Nested
// calls (db already a pgx.Tx) just run — the outer tx owns the commit.
func (r *OwnerSellerRepository) InTx(ctx context.Context, fn func(domain.OwnerSellerRepository) error) error {
	b, ok := r.db.(txBeginner)
	if !ok {
		return fn(r)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := fn(NewOwnerSellerRepository(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListPlatformSellerRows joins users with their seller_settings, ref
// code and carteira size in one query. is_active_am defaults true with
// no row (the callers-apply-active-default convention); is_seller
// defaults false (US-014 — never payable by side-effect). InRotation
// is left false — the use case owns the rotation semantics.
func (r *OwnerSellerRepository) ListPlatformSellerRows(ctx context.Context, platformAccountID string) ([]domain.OwnerSellerRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id, u.name, u.email, u.role, u.is_active,
		        COALESCE(ss.is_seller, false),
		        COALESCE(ss.is_active_am, true),
		        u.seller_ref_code, ss.pix_key,
		        (SELECT COUNT(*) FROM accounts a WHERE a.seller_user_id = u.id)
		 FROM users u
		 LEFT JOIN seller_settings ss ON ss.user_id = u.id
		 WHERE u.account_id = $1
		 ORDER BY u.created_at ASC, u.id ASC`,
		platformAccountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.OwnerSellerRow{}
	for rows.Next() {
		var row domain.OwnerSellerRow
		if err := rows.Scan(&row.UserID, &row.Name, &row.Email, &row.Role, &row.IsActive,
			&row.IsSeller, &row.IsActiveAM, &row.SellerRefCode, &row.PixKey,
			&row.CarteiraCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *OwnerSellerRepository) SetSellerFlag(ctx context.Context, userID string, isSeller bool) error {
	return r.commission.SetSellerFlag(ctx, userID, isSeller)
}

func (r *OwnerSellerRepository) UpsertSellerActivity(ctx context.Context, userID string, isActive bool, deactivatedAt, residualUntil *time.Time) error {
	return r.commission.UpsertSellerActivity(ctx, userID, isActive, deactivatedAt, residualUntil)
}

func (r *OwnerSellerRepository) GetOrCreateSellerRefCode(ctx context.Context, userID string) (string, error) {
	return r.sellerRef.GetOrCreateSellerRefCode(ctx, userID)
}

func (r *OwnerSellerRepository) GetAutoAssignSettings(ctx context.Context, accountID string) (*domain.AutoAssignSettings, error) {
	return r.autoAssign.GetAutoAssignSettings(ctx, accountID)
}

func (r *OwnerSellerRepository) UpsertAutoAssignSettings(ctx context.Context, settings *domain.AutoAssignSettings) error {
	return r.autoAssign.UpsertAutoAssignSettings(ctx, settings)
}
