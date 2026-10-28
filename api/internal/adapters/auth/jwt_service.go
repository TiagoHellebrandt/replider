package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/replider/api/internal/core/domain"
)

type jwtService struct {
	// secretKey is the CURRENT signing key — every freshly minted token is
	// signed with it.
	secretKey []byte
	// previousKeys are accepted for VERIFICATION only (never for signing).
	// During a secret rotation, tokens minted under the prior key keep
	// validating until they naturally expire, so a rotation never logs
	// every active session out at once. Empty when no rotation is in flight.
	previousKeys  [][]byte
	tokenLifespan time.Duration
}

// NewJWTService creates a new instance of JWTService.
//
// `secret` is the CURRENT signing key. Any non-empty `previousSecrets` are
// additionally accepted when verifying tokens (but never used to sign), which
// is what makes a zero-downtime JWT secret rotation possible: set
// JWT_SECRET_CURRENT to the new key and JWT_SECRET_PREVIOUS to the old one,
// deploy, then drop JWT_SECRET_PREVIOUS once all tokens minted under the old
// key have expired (<=24h, the token lifespan). See docs/secret-rotation.md.
func NewJWTService(secret string, lifespanHours int, previousSecrets ...string) domain.JWTService {
	prev := make([][]byte, 0, len(previousSecrets))
	for _, s := range previousSecrets {
		if s == "" || s == secret {
			continue
		}
		prev = append(prev, []byte(s))
	}
	return &jwtService{
		secretKey:     []byte(secret),
		previousKeys:  prev,
		tokenLifespan: time.Duration(lifespanHours) * time.Hour,
	}
}

// verificationKeys returns the ordered set of keys to try when validating a
// token: the current signing key first, then any rotation-grace keys.
func (s *jwtService) verificationKeys() [][]byte {
	keys := make([][]byte, 0, 1+len(s.previousKeys))
	keys = append(keys, s.secretKey)
	keys = append(keys, s.previousKeys...)
	return keys
}

// parseWithRotation parses tokenStr into claims, trying the current signing
// key first and then each rotation-grace key. It returns ErrUnauthorized if no
// key validates. The HMAC method check is enforced for every attempt.
func (s *jwtService) parseWithRotation(tokenStr string, claims jwt.Claims) error {
	var lastErr error
	for _, key := range s.verificationKeys() {
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return key, nil
		})
		if err == nil && token.Valid {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = domain.ErrUnauthorized
	}
	return lastErr
}

func (s *jwtService) GenerateToken(user *domain.User) (string, error) {
	now := time.Now()
	role := "attendant"
	if user.Role != nil {
		role = *user.Role
	}

	claims := &domain.UserClaims{
		UserID:    user.ID,
		UserName:  user.Name,
		AccountID: user.AccountID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenLifespan)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// GenerateImpersonationToken mints a token whose claims include the
// `impersonating_actor` field — the auth middleware copies it into the
// gin context, the audit middleware stamps it on every write, and the
// dashboard renders a red "Acting as" banner whenever it's present.
// Lifespan is taken from claims.ExpiresAt (capped upstream by the use
// case to <=60min), bypassing the regular tokenLifespan setting.
func (s *jwtService) GenerateImpersonationToken(in domain.ImpersonationClaims) (string, error) {
	now := time.Now()
	claims := &domain.UserClaims{
		UserID:             in.UserID,
		UserName:           in.UserName,
		AccountID:          in.AccountID,
		Role:               in.Role,
		ImpersonatingActor: in.ImpersonatingActor,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(in.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *jwtService) ValidateToken(tokenStr string) (*domain.UserClaims, error) {
	claims := &domain.UserClaims{}
	if err := s.parseWithRotation(tokenStr, claims); err != nil {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

const mfaLoginAudience = "mfa_login"

// GenerateMfaLoginToken mints a 5-minute token bound to a single
// /auth/2fa/verify-login round-trip. It carries only the user id and
// is namespaced via the "mfa_login" audience claim — ValidateToken
// will reject it because it would deserialize into UserClaims with
// blank role/account, and the auth middleware refuses those.
func (s *jwtService) GenerateMfaLoginToken(userID string) (string, error) {
	now := time.Now()
	claims := &domain.MfaLoginClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{mfaLoginAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *jwtService) ValidateMfaLoginToken(tokenStr string) (*domain.MfaLoginClaims, error) {
	claims := &domain.MfaLoginClaims{}
	if err := s.parseWithRotation(tokenStr, claims); err != nil {
		return nil, domain.ErrUnauthorized
	}
	hasAudience := false
	for _, aud := range claims.Audience {
		if aud == mfaLoginAudience {
			hasAudience = true
			break
		}
	}
	if !hasAudience {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}
