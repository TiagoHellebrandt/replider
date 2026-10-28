package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// --- Google OAuth2 ---

type GoogleAuthUseCase struct {
	userRepo       domain.UserRepository
	accountRepo    domain.AccountRepository
	usageRepo      domain.UsageRepository
	txManager      domain.TransactionManager
	jwtService     domain.JWTService
	googleClientID string
	// sellerRefRepo attributes a ?ref=SELLERCODE signup to its
	// prospecting seller (US-003). Optional — unwired means no
	// attribution, never a failed login.
	sellerRefRepo     domain.SellerRefRepository
	platformAccountID string
	// registrationPaused refuses the NEW-account branch (US-F001).
	// Existing users (by google id or by email) still log in.
	registrationPaused bool
	// funnelRepo — see ensureDefaultFunnel. Optional.
	funnelRepo domain.SalesFunnelRepository
}

// SetFunnelRepo wires default-funnel creation into Google signups.
func (uc *GoogleAuthUseCase) SetFunnelRepo(r domain.SalesFunnelRepository) { uc.funnelRepo = r }

// SetSellerAttribution wires seller ?ref= capture (US-003) for signups
// that arrive via Google. Setter keeps the constructor and its test call
// sites untouched.
func (uc *GoogleAuthUseCase) SetSellerAttribution(sellerRefRepo domain.SellerRefRepository, platformAccountID string) {
	uc.sellerRefRepo = sellerRefRepo
	uc.platformAccountID = platformAccountID
}

// SetRegistrationPaused closes the signup-via-Google door (US-F001).
func (uc *GoogleAuthUseCase) SetRegistrationPaused(paused bool) {
	uc.registrationPaused = paused
}

func NewGoogleAuthUseCase(
	userRepo domain.UserRepository,
	accountRepo domain.AccountRepository,
	usageRepo domain.UsageRepository,
	txManager domain.TransactionManager,
	jwtService domain.JWTService,
	googleClientID string,
) *GoogleAuthUseCase {
	return &GoogleAuthUseCase{
		userRepo:       userRepo,
		accountRepo:    accountRepo,
		usageRepo:      usageRepo,
		txManager:      txManager,
		jwtService:     jwtService,
		googleClientID: googleClientID,
	}
}

type SocialAuthDTO struct {
	IDToken string `json:"id_token" binding:"required"`
	// Locale is the client's DEVICE locale tag (e.g. "en-US", "pt-BR"),
	// used only when this social login creates a brand-new account —
	// it decides the immutable billing currency (only a real en-US →
	// USD, US-017 review fixes). Ignored on login to an existing account.
	Locale string `json:"locale"`
	// ReferralCode is the ?ref=CODE captured from the signup link,
	// forwarded so social signups get the same seller attribution
	// (US-003) as email/password ones. Only consulted when this login
	// creates a brand-new account; unknown codes silently ignored.
	ReferralCode string `json:"referral_code"`
	// Signup network origin (US-015 review fixes): stamped by the
	// HANDLER from the request, never bound from the client body —
	// hence json:"-". Only persisted when this login creates a new
	// account; audit trail for locale-based currency arbitrage.
	SignupIP      string `json:"-"`
	SignupCountry string `json:"-"`
}

type SocialAuthResponse struct {
	Token string       `json:"token"`
	User  *domain.User `json:"user"`
	IsNew bool         `json:"is_new"` // true if this is a new user created via social login
}

// GoogleTokenInfo represents the response from Google's tokeninfo endpoint
type GoogleTokenInfo struct {
	Sub           string `json:"sub"` // unique Google user ID
	Aud           string `json:"aud"` // audience — must match our client ID
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// verifyGoogleTokenFunc is the function used to verify Google tokens.
// It is a variable so tests can replace it.
var verifyGoogleTokenFunc = verifyGoogleToken

// verifyMetaTokenFunc is the function used to verify Meta tokens.
// It is a variable so tests can replace it.
var verifyMetaTokenFunc = verifyMetaToken

// verifyMetaAppBindingFunc introspects a token via /debug_token to prove which
// app issued it. A variable so tests can replace it.
var verifyMetaAppBindingFunc = verifyMetaAppBinding

// MetaTokenDebugInfo is the subset of /debug_token we need to bind a token to
// our app.
type MetaTokenDebugInfo struct {
	AppID   string
	IsValid bool
}

// verifyMetaAppBinding calls Meta's /debug_token with an app access token
// (`{app_id}|{app_secret}`) and reports the app the input token was issued for.
// This is what stops a valid Facebook token from any OTHER app being replayed
// against our login (confused-deputy account takeover).
func verifyMetaAppBinding(appID, appSecret, inputToken string) (*MetaTokenDebugInfo, error) {
	resp, err := http.Get(fmt.Sprintf(
		"https://graph.facebook.com/debug_token?input_token=%s&access_token=%s|%s",
		url.QueryEscape(inputToken), url.QueryEscape(appID), url.QueryEscape(appSecret),
	))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Meta debug_token failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			AppID   string `json:"app_id"`
			IsValid bool   `json:"is_valid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	return &MetaTokenDebugInfo{AppID: env.Data.AppID, IsValid: env.Data.IsValid}, nil
}

func (uc *GoogleAuthUseCase) Execute(ctx context.Context, dto SocialAuthDTO) (*SocialAuthResponse, error) {
	// 1. Verify the id_token with Google's API
	googleUser, err := verifyGoogleTokenFunc(dto.IDToken)
	if err != nil {
		slog.WarnContext(ctx, "google auth: token verification failed",
			"error", err.Error(),
			"reason", "verify_failed",
		)
		return nil, domain.ErrSocialAuthFailed
	}

	// 2. Validate audience — token must have been issued for this app. Fail
	// CLOSED when the client id is unset: an empty GOOGLE_CLIENT_ID would
	// otherwise accept an id_token minted for ANY other Google OAuth client
	// (confused deputy). Prod always sets it; an unset value is a
	// misconfiguration, never a reason to trust every token.
	if uc.googleClientID == "" {
		slog.ErrorContext(ctx, "google auth: GOOGLE_CLIENT_ID not configured; refusing login",
			"reason", "client_id_unconfigured")
		return nil, domain.ErrSocialAuthFailed
	}
	if googleUser.Aud != uc.googleClientID {
		slog.WarnContext(ctx, "google auth: audience mismatch",
			"reason", "aud_mismatch",
			"expected_aud", uc.googleClientID,
			"received_aud", googleUser.Aud,
			"email", googleUser.Email,
		)
		return nil, domain.ErrSocialAuthFailed
	}

	if googleUser.EmailVerified != "true" {
		slog.WarnContext(ctx, "google auth: email not verified by Google",
			"reason", "email_unverified",
			"email", googleUser.Email,
			"sub", googleUser.Sub,
		)
		return nil, domain.ErrSocialAuthFailed
	}

	// Capture Google profile picture if present (used by the UI for the
	// avatar). Refreshed on every login so users see their current Google
	// avatar, not the one from their first sign-in.
	var picturePtr *string
	if googleUser.Picture != "" {
		picturePtr = &googleUser.Picture
	}

	// 3. Find user by Google ID
	user, err := uc.userRepo.GetByGoogleID(ctx, googleUser.Sub)
	if err != nil {
		return nil, err
	}

	isNew := false
	if user == nil {
		// 3. Try to find by email (link social to existing account)
		user, err = uc.userRepo.GetByEmail(ctx, googleUser.Email)
		if err != nil {
			return nil, err
		}

		if user != nil {
			// Link Google ID to existing user
			if err := uc.userRepo.SetGoogleID(ctx, user.ID, googleUser.Sub); err != nil {
				return nil, err
			}
		} else {
			// Registration gate (US-F001): a Google login for an unknown
			// email would MINT a new account — refuse while paused.
			if uc.registrationPaused {
				return nil, domain.ErrRegistrationPaused
			}

			// 4. Create account + user + usage in a transaction. Creator = owner
			// (migration 000149 model) so owner-directed queries can find them.
			role := "owner"
			accountID := ulid.Make().String()
			now := time.Now()
			termsVer := domain.CurrentTermsVersion
			user = &domain.User{
				ID:              ulid.Make().String(),
				AccountID:       accountID,
				Name:            googleUser.Name,
				Email:           googleUser.Email,
				GoogleID:        &googleUser.Sub,
				Picture:         picturePtr,
				Role:            &role,
				IsActive:        true,
				IsEmailVerified: true, // Google already verified the email
				// LGPD: Google sign-in implicitly accepts terms (the social
				// button below the checkbox carries the same legal weight as
				// clicking "Accept and Continue"). Frontend renders the same
				// terms link next to the social buttons.
				TermsAcceptedAt: &now,
				TermsVersion:    &termsVer,
				CreatedAt:       now,
				UpdatedAt:       now,
			}

			if err := uc.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
				account := &domain.Account{
					ID:                 accountID,
					Name:               googleUser.Name,
					Currency:           domain.CurrencyFromLocale(dto.Locale),
					SubscriptionStatus: "trialing",
					SignupIP:           utmPtr(dto.SignupIP),
					SignupCountry:      utmPtr(dto.SignupCountry),
					SignupLocale:       utmPtr(dto.Locale),
					CreatedAt:          time.Now(),
					UpdatedAt:          time.Now(),
				}
				if err := uc.accountRepo.Create(ctx, account); err != nil {
					return err
				}
				if err := ensureDefaultFunnel(ctx, uc.funnelRepo, accountID); err != nil {
					return err
				}

				if err := uc.userRepo.Create(ctx, user); err != nil {
					return err
				}

				now := time.Now()
				billingStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
				zeroConversations := int32(0)
				zeroTokens := int64(0)
				zeroStorage := int32(0)
				usage := &domain.AccountUsage{
					ID:                 ulid.Make().String(),
					AccountID:          accountID,
					BillingCycleStart:  billingStart,
					ConversationsCount: &zeroConversations,
					AITokensUsed:       &zeroTokens,
					StorageUsedMB:      &zeroStorage,
					UpdatedAt:          now,
				}
				return uc.usageRepo.Create(ctx, usage)
			}); err != nil {
				return nil, err
			}
			isNew = true
			// Best-effort seller attribution (US-003), after commit so
			// the stamp references a persisted account.
			stampSellerRefAttribution(ctx, uc.sellerRefRepo, uc.platformAccountID, accountID, dto.ReferralCode, user.ID, user.Email)
		}
	}

	// Refresh avatar on every login (skip for brand-new users — Create
	// already persisted Picture above).
	if !isNew && picturePtr != nil && (user.Picture == nil || *user.Picture != *picturePtr) {
		if err := uc.userRepo.SetPicture(ctx, user.ID, *picturePtr); err != nil {
			return nil, err
		}
		user.Picture = picturePtr
	}

	// 5. Generate our JWT
	token, err := uc.jwtService.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	// For existing users, check whether they've completed onboarding.
	// New users already have isNew=true (account was just created with
	// onboarding_completed=false).
	if !isNew {
		account, err := uc.accountRepo.GetByID(ctx, user.AccountID)
		if err == nil && onboardingPendingFor(account, user) {
			isNew = true
		}
	}

	return &SocialAuthResponse{
		Token: token,
		User:  user,
		IsNew: isNew,
	}, nil
}

func verifyGoogleToken(idToken string) (*GoogleTokenInfo, error) {
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google token verification failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info GoogleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	if info.Sub == "" {
		return nil, fmt.Errorf("invalid Google token: no subject")
	}

	return &info, nil
}

// --- Meta (Facebook/Instagram) OAuth2 ---

type MetaAuthUseCase struct {
	userRepo    domain.UserRepository
	accountRepo domain.AccountRepository
	usageRepo   domain.UsageRepository
	txManager   domain.TransactionManager
	jwtService  domain.JWTService
	// Seller ?ref= attribution (US-003) — see GoogleAuthUseCase.
	sellerRefRepo     domain.SellerRefRepository
	platformAccountID string
	// registrationPaused — see GoogleAuthUseCase (US-F001).
	registrationPaused bool
	// funnelRepo — see ensureDefaultFunnel. Optional.
	funnelRepo domain.SalesFunnelRepository
	// appID / appSecret prove an inbound Meta access_token was minted for
	// OUR app. Without them, /me happily validates ANY valid Facebook token
	// from ANY app the user authorized — a confused-deputy account takeover.
	// See verifyMetaAppBinding. In production these are always set (they
	// fall back to the mandatory WhatsApp app credentials).
	appID     string
	appSecret string
}

// SetMetaAppCredentials wires the app-binding check (/debug_token app_id
// verification). When set, every Meta login must present a token issued for
// this exact app. Optional setter — same shape as the other wiring.
func (uc *MetaAuthUseCase) SetMetaAppCredentials(appID, appSecret string) {
	uc.appID, uc.appSecret = appID, appSecret
}

// SetFunnelRepo wires default-funnel creation into Meta signups.
func (uc *MetaAuthUseCase) SetFunnelRepo(r domain.SalesFunnelRepository) { uc.funnelRepo = r }

// SetSellerAttribution wires seller ?ref= capture (US-003) for signups
// that arrive via Meta.
func (uc *MetaAuthUseCase) SetSellerAttribution(sellerRefRepo domain.SellerRefRepository, platformAccountID string) {
	uc.sellerRefRepo = sellerRefRepo
	uc.platformAccountID = platformAccountID
}

// SetRegistrationPaused closes the signup-via-Meta door (US-F001).
func (uc *MetaAuthUseCase) SetRegistrationPaused(paused bool) {
	uc.registrationPaused = paused
}

func NewMetaAuthUseCase(
	userRepo domain.UserRepository,
	accountRepo domain.AccountRepository,
	usageRepo domain.UsageRepository,
	txManager domain.TransactionManager,
	jwtService domain.JWTService,
) *MetaAuthUseCase {
	return &MetaAuthUseCase{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		usageRepo:   usageRepo,
		txManager:   txManager,
		jwtService:  jwtService,
	}
}

// MetaTokenInfo from the Facebook Graph API
type MetaTokenInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (uc *MetaAuthUseCase) Execute(ctx context.Context, dto SocialAuthDTO) (*SocialAuthResponse, error) {
	// 1. Prove the token was minted for OUR app before trusting it. /me
	// validates any valid Facebook token; only /debug_token tells us which
	// app issued it. Skipped only when no app credentials are configured
	// (tests, or a deployment with no Meta app at all — where Meta login
	// cannot work anyway); production always has them via the WhatsApp
	// app-credential fallback.
	if uc.appID != "" && uc.appSecret != "" {
		info, err := verifyMetaAppBindingFunc(uc.appID, uc.appSecret, dto.IDToken)
		if err != nil {
			slog.Warn("meta auth: app-binding introspection failed", "error", err)
			return nil, domain.ErrSocialAuthFailed
		}
		if !info.IsValid || info.AppID != uc.appID {
			slog.Warn("meta auth: token not issued for this app; rejecting",
				"token_app_id", info.AppID, "is_valid", info.IsValid)
			return nil, domain.ErrSocialAuthFailed
		}
	}

	// 2. Verify the access token with Facebook Graph API
	metaUser, err := verifyMetaTokenFunc(dto.IDToken)
	if err != nil {
		return nil, domain.ErrSocialAuthFailed
	}

	// 2. Find user by Meta ID
	user, err := uc.userRepo.GetByMetaID(ctx, metaUser.ID)
	if err != nil {
		return nil, err
	}

	isNew := false
	if user == nil {
		// 3. Try to find by email
		if metaUser.Email != "" {
			user, err = uc.userRepo.GetByEmail(ctx, metaUser.Email)
			if err != nil {
				return nil, err
			}
		}

		if user != nil {
			// Link Meta ID to existing user
			if err := uc.userRepo.SetMetaID(ctx, user.ID, metaUser.ID); err != nil {
				return nil, err
			}
		} else {
			// Registration gate (US-F001) — same rule as Google.
			if uc.registrationPaused {
				return nil, domain.ErrRegistrationPaused
			}

			// 4. Create account + user + usage in a transaction. Creator = owner
			// (migration 000149 model) so owner-directed queries can find them.
			role := "owner"
			accountID := ulid.Make().String()
			now := time.Now()
			termsVer := domain.CurrentTermsVersion
			user = &domain.User{
				ID:              ulid.Make().String(),
				AccountID:       accountID,
				Name:            metaUser.Name,
				Email:           metaUser.Email,
				MetaID:          &metaUser.ID,
				Role:            &role,
				IsActive:        true,
				IsEmailVerified: true, // Meta already verified the email
				// LGPD: Meta sign-in implicitly accepts terms (same as Google).
				TermsAcceptedAt: &now,
				TermsVersion:    &termsVer,
				CreatedAt:       now,
				UpdatedAt:       now,
			}

			if err := uc.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
				account := &domain.Account{
					ID:                 accountID,
					Name:               metaUser.Name,
					Currency:           domain.CurrencyFromLocale(dto.Locale),
					SubscriptionStatus: "trialing",
					SignupIP:           utmPtr(dto.SignupIP),
					SignupCountry:      utmPtr(dto.SignupCountry),
					SignupLocale:       utmPtr(dto.Locale),
					CreatedAt:          time.Now(),
					UpdatedAt:          time.Now(),
				}
				if err := uc.accountRepo.Create(ctx, account); err != nil {
					return err
				}
				if err := ensureDefaultFunnel(ctx, uc.funnelRepo, accountID); err != nil {
					return err
				}

				if err := uc.userRepo.Create(ctx, user); err != nil {
					return err
				}

				now := time.Now()
				billingStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
				zeroConversations := int32(0)
				zeroTokens := int64(0)
				zeroStorage := int32(0)
				usage := &domain.AccountUsage{
					ID:                 ulid.Make().String(),
					AccountID:          accountID,
					BillingCycleStart:  billingStart,
					ConversationsCount: &zeroConversations,
					AITokensUsed:       &zeroTokens,
					StorageUsedMB:      &zeroStorage,
					UpdatedAt:          now,
				}
				return uc.usageRepo.Create(ctx, usage)
			}); err != nil {
				return nil, err
			}
			isNew = true
			// Best-effort seller attribution (US-003), same as Google.
			stampSellerRefAttribution(ctx, uc.sellerRefRepo, uc.platformAccountID, accountID, dto.ReferralCode, user.ID, user.Email)
		}
	}

	// 5. Generate JWT
	token, err := uc.jwtService.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	// Check onboarding for existing users (same logic as Google auth).
	if !isNew {
		account, err := uc.accountRepo.GetByID(ctx, user.AccountID)
		if err == nil && onboardingPendingFor(account, user) {
			isNew = true
		}
	}

	return &SocialAuthResponse{
		Token: token,
		User:  user,
		IsNew: isNew,
	}, nil
}

func verifyMetaToken(accessToken string) (*MetaTokenInfo, error) {
	resp, err := http.Get(fmt.Sprintf("https://graph.facebook.com/me?fields=id,name,email&access_token=%s", accessToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Meta token verification failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info MetaTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	if info.ID == "" {
		return nil, fmt.Errorf("invalid Meta token: no user ID")
	}

	return &info, nil
}
