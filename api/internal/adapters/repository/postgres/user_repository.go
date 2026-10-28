package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

// --- UserRepository ---

type UserRepository struct {
	queries *Queries
	// totpCipher seals/opens the TOTP shared secret at rest (US-076).
	// nil disables 2FA persistence entirely — Setup is rejected and any
	// stored ciphertext is treated as absent (never returned raw). Wired
	// from TOTP_KEK in main.go; nil only in tests / when TOTP_KEK is unset.
	totpCipher *secrets.AESGCMCipher
}

// NewUserRepository wires the repo with the TOTP key-encryption cipher.
// Pass nil to disable TOTP secret persistence (dev without TOTP_KEK or
// unit tests that don't exercise 2FA).
func NewUserRepository(db DBTX, totpCipher *secrets.AESGCMCipher) *UserRepository {
	return &UserRepository{queries: New(db), totpCipher: totpCipher}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	q := getQueries(ctx, r.queries)
	var verificationTokenExpiresAt pgtype.Timestamptz
	if user.VerificationTokenExpiresAt != nil {
		verificationTokenExpiresAt = pgtype.Timestamptz{Time: *user.VerificationTokenExpiresAt, Valid: true}
	}
	var termsAcceptedAt pgtype.Timestamptz
	if user.TermsAcceptedAt != nil {
		termsAcceptedAt = pgtype.Timestamptz{Time: *user.TermsAcceptedAt, Valid: true}
	}
	err := q.CreateUser(ctx, CreateUserParams{
		ID:                         user.ID,
		AccountID:                  user.AccountID,
		Name:                       user.Name,
		Email:                      user.Email,
		PasswordHash:               user.PasswordHash,
		Role:                       user.Role,
		IsActive:                   &user.IsActive,
		IsEmailVerified:            user.IsEmailVerified,
		VerificationToken:          hashedVerificationToken(user.VerificationToken),
		VerificationTokenExpiresAt: verificationTokenExpiresAt,
		Picture:                    user.Picture,
		TermsAcceptedAt:            termsAcceptedAt,
		TermsVersion:               user.TermsVersion,
		CreatedAt:                  pgtype.Timestamptz{Time: user.CreatedAt, Valid: true},
		UpdatedAt:                  pgtype.Timestamptz{Time: user.UpdatedAt, Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	return nil
}

func (r *UserRepository) mapUserToDomain(row User) *domain.User {
	u := &domain.User{
		ID:                row.ID,
		AccountID:         row.AccountID,
		Name:              row.Name,
		Email:             row.Email,
		PasswordHash:      row.PasswordHash,
		Role:              row.Role,
		GoogleID:          row.GoogleID,
		MetaID:            row.MetaID,
		Picture:           row.Picture,
		Signature:         row.Signature,
		ResetToken:        row.ResetToken,
		IsEmailVerified:   row.IsEmailVerified,
		VerificationToken: row.VerificationToken,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
	if row.IsActive != nil {
		u.IsActive = *row.IsActive
	}
	if row.ResetTokenExpiresAt.Valid {
		t := row.ResetTokenExpiresAt.Time
		u.ResetTokenExpiresAt = &t
	}
	if row.VerificationTokenExpiresAt.Valid {
		t := row.VerificationTokenExpiresAt.Time
		u.VerificationTokenExpiresAt = &t
	}
	u.TOTPSecret = r.decryptTOTPSecret(row.ID, row.TotpSecretEncrypted)
	u.TOTPEnabled = row.TotpEnabled
	u.TOTPRecoveryCodes = row.TotpRecoveryCodes
	if row.TotpVerifiedAt.Valid {
		t := row.TotpVerifiedAt.Time
		u.TOTPVerifiedAt = &t
	}
	return u
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetUserByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) GetByGoogleID(ctx context.Context, googleID string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetUserByGoogleID(ctx, &googleID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) GetByMetaID(ctx context.Context, metaID string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetUserByMetaID(ctx, &metaID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) UpdatePasswordHash(ctx context.Context, userID string, hash string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateUserPasswordHash(ctx, UpdateUserPasswordHashParams{ID: userID, PasswordHash: &hash})
}

// Reset and verification tokens are stored HASHED (domain.HashToken) and
// looked up by hash — 2026-09-01 hardening, migration 000222. The use cases
// keep handing the raw token to the e-mail; the repository is the single
// place that decides how it rests, so every writer (including Create) and
// every reader agree without each of them remembering to hash.

func (r *UserRepository) SetResetToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
	q := getQueries(ctx, r.queries)
	hashed := domain.HashToken(token)
	return q.SetResetToken(ctx, SetResetTokenParams{
		ID:                  userID,
		ResetToken:          &hashed,
		ResetTokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
}

func (r *UserRepository) GetByResetToken(ctx context.Context, token string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	hashed := domain.HashToken(token)
	row, err := q.GetUserByResetToken(ctx, &hashed)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) ClearResetToken(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.ClearResetToken(ctx, userID)
}

func (r *UserRepository) SetGoogleID(ctx context.Context, userID string, googleID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetUserGoogleID(ctx, SetUserGoogleIDParams{ID: userID, GoogleID: &googleID})
}

func (r *UserRepository) SetMetaID(ctx context.Context, userID string, metaID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetUserMetaID(ctx, SetUserMetaIDParams{ID: userID, MetaID: &metaID})
}

func (r *UserRepository) SetPicture(ctx context.Context, userID string, picture string) error {
	q := getQueries(ctx, r.queries)
	return q.SetUserPicture(ctx, SetUserPictureParams{ID: userID, Picture: &picture})
}

func (r *UserRepository) UpdateProfile(ctx context.Context, userID string, name string, email string, signature *string) error {
	q := getQueries(ctx, r.queries)
	if err := q.UpdateUserProfile(ctx, UpdateUserProfileParams{ID: userID, Name: name, Email: email}); err != nil {
		return err
	}
	// nil = client didn't send the field (older app builds) — preserve.
	// "" clears to NULL via the query's NULLIF.
	if signature != nil {
		return q.SetUserSignature(ctx, SetUserSignatureParams{ID: userID, Signature: *signature})
	}
	return nil
}

func (r *UserRepository) SetVerificationToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
	q := getQueries(ctx, r.queries)
	hashed := domain.HashToken(token)
	return q.SetVerificationToken(ctx, SetVerificationTokenParams{
		ID:                         userID,
		VerificationToken:          &hashed,
		VerificationTokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
}

func (r *UserRepository) GetByVerificationToken(ctx context.Context, token string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	hashed := domain.HashToken(token)
	row, err := q.GetUserByVerificationToken(ctx, &hashed)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) VerifyEmail(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.VerifyUserEmail(ctx, userID)
}

func (r *UserRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.User, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListUsersByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		u := &domain.User{
			ID:           row.ID,
			AccountID:    row.AccountID,
			Name:         row.Name,
			Email:        row.Email,
			Role:         row.Role,
			Picture:      row.Picture,
			InviteSentAt: timestamptzPtr(row.InviteSentAt),
			LastSeenAt:   timestamptzPtr(row.LastSeenAt),
			CreatedAt:    row.CreatedAt.Time,
			UpdatedAt:    row.UpdatedAt.Time,
		}
		if row.IsActive != nil {
			u.IsActive = *row.IsActive
		}
		u.IsEmailVerified = row.IsEmailVerified
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) UpdateRole(ctx context.Context, userID string, role string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateUserRole(ctx, UpdateUserRoleParams{ID: userID, Role: &role})
}

func (r *UserRepository) Deactivate(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.DeactivateUser(ctx, userID)
}

func (r *UserRepository) MarkInviteSent(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkInviteSent(ctx, userID)
}

func (r *UserRepository) UpdateLastSeen(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateLastSeen(ctx, userID)
}

func (r *UserRepository) GetAdminByAccountID(ctx context.Context, accountID string) (*domain.User, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAdminByAccountID(ctx, accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.mapUserToDomain(row), nil
}

func (r *UserRepository) SetTOTPSecret(ctx context.Context, userID string, secret string) error {
	if r.totpCipher == nil {
		// Refuse to persist a TOTP seed when no KEK is configured rather
		// than silently storing plaintext — the whole point of US-076.
		return &domain.AppError{Code: "TOTP_CIPHER_UNCONFIGURED", Message: "2FA is not available: encryption key not configured on this server"}
	}
	sealed, err := r.totpCipher.SealBytes(secret)
	if err != nil {
		return err
	}
	q := getQueries(ctx, r.queries)
	return q.SetUserTOTPSecret(ctx, SetUserTOTPSecretParams{ID: userID, TotpSecretEncrypted: sealed})
}

// decryptTOTPSecret opens a sealed TOTP seed read from the DB. Returns
// nil for an absent secret, a nil cipher (treat as absent — never surface
// raw ciphertext), or a decrypt failure (logged; treated as absent so a
// key rotation can't hard-fail every user fetch).
func (r *UserRepository) decryptTOTPSecret(userID string, sealed []byte) *string {
	if len(sealed) == 0 {
		return nil
	}
	if r.totpCipher == nil {
		slog.Warn("user: TOTP cipher not configured, treating secret as absent", "user_id", userID)
		return nil
	}
	plain, err := r.totpCipher.OpenBytes(sealed)
	if err != nil {
		slog.Error("user: failed to decrypt TOTP secret", "user_id", userID, "error", err)
		return nil
	}
	if plain == "" {
		return nil
	}
	return &plain
}

func (r *UserRepository) EnableTOTP(ctx context.Context, userID string, hashedRecoveryCodes []string) error {
	q := getQueries(ctx, r.queries)
	return q.EnableUserTOTP(ctx, EnableUserTOTPParams{ID: userID, TotpRecoveryCodes: hashedRecoveryCodes})
}

func (r *UserRepository) DisableTOTP(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.DisableUserTOTP(ctx, userID)
}

func (r *UserRepository) ConsumeTOTPRecoveryCode(ctx context.Context, userID, hashedCode string) (bool, error) {
	q := getQueries(ctx, r.queries)
	_, err := q.ConsumeTOTPRecoveryCode(ctx, ConsumeTOTPRecoveryCodeParams{ID: userID, ArrayRemove: hashedCode})
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *UserRepository) StampTOTPVerified(ctx context.Context, userID string) error {
	q := getQueries(ctx, r.queries)
	return q.StampTOTPVerified(ctx, userID)
}

// hashedVerificationToken hashes a raw token set on the domain struct by a
// writer that goes through Create (the owner panel's account creation),
// so Create stores the same shape SetVerificationToken does.
func hashedVerificationToken(raw *string) *string {
	if raw == nil || *raw == "" {
		return raw
	}
	h := domain.HashToken(*raw)
	return &h
}

// SessionInvalidatedAt implements domain.SessionInvalidationReader.
func (r *UserRepository) SessionInvalidatedAt(ctx context.Context, userID string) (*time.Time, error) {
	q := getQueries(ctx, r.queries)
	stamp, err := q.GetUserSessionInvalidatedAt(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if !stamp.Valid {
		return nil, nil
	}
	t := stamp.Time
	return &t, nil
}
