-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: CreateUser :exec
INSERT INTO users (
    id, account_id, name, email, password_hash, role, is_active, is_email_verified, verification_token, verification_token_expires_at, picture, terms_accepted_at, terms_version, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
);

-- name: GetUserByGoogleID :one
SELECT * FROM users
WHERE google_id = $1 LIMIT 1;

-- name: GetUserByMetaID :one
SELECT * FROM users
WHERE meta_id = $1 LIMIT 1;

-- name: UpdateUserPasswordHash :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetResetToken :exec
UPDATE users
SET reset_token = $2, reset_token_expires_at = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetUserByResetToken :one
SELECT * FROM users
WHERE reset_token = $1 AND reset_token_expires_at > NOW() LIMIT 1;

-- name: ClearResetToken :exec
UPDATE users
SET reset_token = NULL, reset_token_expires_at = NULL, updated_at = NOW()
WHERE id = $1;

-- name: SetUserGoogleID :exec
UPDATE users
SET google_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetUserPicture :exec
UPDATE users
SET picture = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetUserMetaID :exec
UPDATE users
SET meta_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserProfile :exec
UPDATE users
SET name = $2, email = $3, updated_at = NOW()
WHERE id = $1;

-- Signature is set separately so a nil (absent) value in PUT /me can
-- preserve the current one. Empty string clears to NULL.
-- name: SetUserSignature :exec
UPDATE users
SET signature = NULLIF(sqlc.arg(signature)::text, ''), updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: SetVerificationToken :exec
UPDATE users
SET verification_token = $2, verification_token_expires_at = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetUserByVerificationToken :one
SELECT * FROM users
WHERE verification_token = $1 AND verification_token_expires_at > NOW() LIMIT 1;

-- name: VerifyUserEmail :exec
UPDATE users
SET is_email_verified = true, verification_token = NULL, verification_token_expires_at = NULL, updated_at = NOW()
WHERE id = $1;

-- name: ListUsersByAccountID :many
SELECT id, account_id, name, email, role, is_active, is_email_verified, picture, invite_sent_at, last_seen_at, created_at, updated_at
FROM users
WHERE account_id = $1
ORDER BY created_at;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1;

-- name: DeactivateUser :exec
-- session_invalidated_at ends every JWT the member still holds: the auth
-- middleware refuses tokens issued at or before this stamp.
UPDATE users
SET is_active = false, session_invalidated_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: GetUserSessionInvalidatedAt :one
SELECT session_invalidated_at FROM users WHERE id = $1;

-- name: GetAdminByAccountID :one
-- The account's decision-maker for owner-directed mail (SDR digest, nudges,
-- billing). Migration 000149 promoted every creator to role='owner', so this
-- MUST accept both — filtering role='admin' alone returned NULL for every
-- existing account and silently dropped all of these emails. Owner wins over a
-- later-invited admin.
SELECT * FROM users
WHERE account_id = $1 AND role IN ('owner', 'admin') AND is_active = true
ORDER BY (role = 'owner') DESC, created_at ASC
LIMIT 1;

-- name: MarkInviteSent :exec
-- Bumped when an admin invites a new member or re-sends the invite,
-- so the Team screen can surface a "Pending" badge + resend button
-- until the invitee clicks the email link.
UPDATE users
SET invite_sent_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: UpdateLastSeen :exec
-- Idempotent per 5-minute window — the auth middleware calls this on
-- every JWT-validated request, and writing on every request would
-- hammer the WAL for no real UX gain. The grace window is small
-- enough that "Active now" is still accurate to a few minutes.
UPDATE users
SET last_seen_at = NOW()
WHERE id = $1
  AND (last_seen_at IS NULL OR last_seen_at < NOW() - INTERVAL '5 minutes');

-- name: SetUserTOTPSecret :exec
-- Stage a TOTP secret on the user without flipping totp_enabled — that
-- only happens after the user proves possession by entering a valid
-- 6-digit code. Wipes recovery codes too so a second enrollment cannot
-- inherit codes from a previous attempt.
UPDATE users
SET totp_secret_encrypted = $2,
    totp_enabled = false,
    totp_recovery_codes = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: EnableUserTOTP :exec
-- Promotes a staged secret to "enabled" and stores the bcrypt'd
-- recovery codes. Called from the Verify endpoint after the first
-- valid code, so the secret is never live without proof of possession.
UPDATE users
SET totp_enabled = true,
    totp_recovery_codes = $2,
    totp_verified_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: DisableUserTOTP :exec
-- Tears down 2FA wholesale. Must come from a request that already
-- proved possession (current TOTP code or a valid recovery code), so
-- the handler — not this query — owns the gate.
UPDATE users
SET totp_secret_encrypted = NULL,
    totp_enabled = false,
    totp_recovery_codes = NULL,
    totp_verified_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: ConsumeTOTPRecoveryCode :one
-- Atomically removes one recovery-code hash from the array and reports
-- whether anything was removed. Compared as plain strings — the handler
-- bcrypt-compares the user-supplied code against each hash and passes
-- the matching hash here.
UPDATE users
SET totp_recovery_codes = array_remove(totp_recovery_codes, $2),
    updated_at = NOW()
WHERE id = $1
  AND $2 = ANY(totp_recovery_codes)
RETURNING id;

-- name: StampTOTPVerified :exec
-- Refreshes totp_verified_at after a successful 2FA proof. Phase 2 will
-- read this to decide whether the owner middleware should re-prompt.
UPDATE users
SET totp_verified_at = NOW()
WHERE id = $1;
