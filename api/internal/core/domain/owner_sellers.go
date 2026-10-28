package domain

import (
	"context"
	"time"
)

// OwnerSellerRepository backs the owner panel's Vendedores section
// (owner-sellers US-001). It re-exposes the seller_settings, ref-code
// and auto-assign writes the feature repos already own, plus InTx so
// the onboard action (flag + ref code + rotation) commits or rolls
// back as one unit — a seller flagged but silently missing from the
// rotation is exactly the fragmentation this section removes.
type OwnerSellerRepository interface {
	// InTx runs fn with a transaction-bound repository; any error rolls
	// the whole onboard back.
	InTx(ctx context.Context, fn func(OwnerSellerRepository) error) error
	// ListPlatformSellerRows returns every user of the platform account
	// with seller state and carteira count joined. InRotation is left
	// false — the use case fills it in.
	ListPlatformSellerRows(ctx context.Context, platformAccountID string) ([]OwnerSellerRow, error)
	// SetSellerFlag grants/revokes the explicit seller flag, keeping
	// every other seller_settings column.
	SetSellerFlag(ctx context.Context, userID string, isSeller bool) error
	// UpsertSellerActivity writes the activity-clause state, keeping
	// existing rates/pix_key.
	UpsertSellerActivity(ctx context.Context, userID string, isActive bool, deactivatedAt, residualUntil *time.Time) error
	// GetOrCreateSellerRefCode returns the user's prospecting code,
	// minting one lazily (idempotent).
	GetOrCreateSellerRefCode(ctx context.Context, userID string) (string, error)
	GetAutoAssignSettings(ctx context.Context, accountID string) (*AutoAssignSettings, error)
	UpsertAutoAssignSettings(ctx context.Context, settings *AutoAssignSettings) error
}
