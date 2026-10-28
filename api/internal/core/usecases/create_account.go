package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

type CreateAccountUseCase struct {
	accountRepo  domain.AccountRepository
	userRepo     domain.UserRepository
	usageRepo    domain.UsageRepository
	txManager    domain.TransactionManager
	emailService email.EmailService
	appBaseURL   string
	// referralRepo links a ?ref=CODE signup to its referrer (US-174).
	// Optional — when unwired, signups proceed but no referral is tracked.
	referralRepo domain.ReferralRepository
	// sellerRefRepo attributes a ?ref=SELLERCODE signup to its
	// prospecting seller (US-003). Optional — when unwired (or
	// PLATFORM_ACCOUNT_ID unset), no seller attribution happens.
	sellerRefRepo     domain.SellerRefRepository
	platformAccountID string
	// registrationPaused closes public signup (US-F001). Zero value =
	// open, so an unwired use case (tests, self-hosted) behaves as before.
	registrationPaused bool
	// funnelRepo gives the new account its default funnel. Optional — see
	// ensureDefaultFunnel.
	funnelRepo domain.SalesFunnelRepository
	// inviteRepo resolves and burns one-shot signup invites (sales-led flow).
	// Optional seam — unwired, any invite_token is refused, never ignored:
	// a permission we cannot check is a permission we do not have.
	inviteRepo domain.AccountInviteRepository
}

// SetFunnelRepo wires the default-funnel creation into signup.
func (uc *CreateAccountUseCase) SetFunnelRepo(r domain.SalesFunnelRepository) { uc.funnelRepo = r }

// SetInviteRepo wires the one-shot signup invites (optional setter).
func (uc *CreateAccountUseCase) SetInviteRepo(r domain.AccountInviteRepository) {
	uc.inviteRepo = r
}

// SetRegistrationPaused closes/opens the public signup door (US-F001).
// Setter (not constructor param) so the existing call sites stay untouched;
// the zero value keeps registration OPEN.
func (uc *CreateAccountUseCase) SetRegistrationPaused(paused bool) {
	uc.registrationPaused = paused
}

// SetReferralLinker wires referral capture (US-174). Kept as a setter so
// the constructor signature — and its existing test call sites — stay
// untouched. Referral linking is best-effort and never fails a signup.
func (uc *CreateAccountUseCase) SetReferralLinker(referralRepo domain.ReferralRepository) {
	uc.referralRepo = referralRepo
}

// SetSellerAttribution wires seller ?ref= capture (US-003). Setter for the
// same reason as SetReferralLinker. Attribution is best-effort and never
// fails a signup.
func (uc *CreateAccountUseCase) SetSellerAttribution(sellerRefRepo domain.SellerRefRepository, platformAccountID string) {
	uc.sellerRefRepo = sellerRefRepo
	uc.platformAccountID = platformAccountID
}

func NewCreateAccountUseCase(
	accountRepo domain.AccountRepository,
	userRepo domain.UserRepository,
	usageRepo domain.UsageRepository,
	txManager domain.TransactionManager,
	emailService email.EmailService,
	appBaseURL string,
) *CreateAccountUseCase {
	return &CreateAccountUseCase{
		accountRepo:  accountRepo,
		userRepo:     userRepo,
		usageRepo:    usageRepo,
		txManager:    txManager,
		emailService: emailService,
		appBaseURL:   appBaseURL,
	}
}

// CreateAccountDTO defines the input for the use case
type CreateAccountDTO struct {
	AccountName string `json:"account_name" binding:"required"`
	AdminName   string `json:"admin_name" binding:"required"`
	AdminEmail  string `json:"admin_email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	// LGPD: must be true. The frontend renders the Terms + Privacy Policy
	// links next to the checkbox; submitting without ticking it is a 400.
	TermsAccepted bool `json:"terms_accepted"`
	// First-touch acquisition attribution forwarded from the marketing
	// site's register CTA (utm_*). Optional; absent for social-auth and
	// direct signups.
	UTMSource   string `json:"utm_source"`
	UTMMedium   string `json:"utm_medium"`
	UTMCampaign string `json:"utm_campaign"`
	// ReferralCode is the referrer's code forwarded from a ?ref=CODE
	// signup link (US-174). Optional; an unknown code is silently ignored.
	ReferralCode string `json:"referral_code"`
	// InviteToken is the one-shot permission from /register?invite=… (sales-led
	// flow, docs/fluxo-criacao-conta.md). With registration paused it is the
	// ONLY way in; a valid token also makes the account be born on the
	// invite's contracted plan instead of trial.
	InviteToken string `json:"invite_token"`
	// Locale is the client's DEVICE locale tag at signup (e.g. "en-US",
	// "pt-BR", "de-DE"). Optional — it only decides the account's
	// immutable billing currency (only a real en-US → USD, everything
	// else → BRL — US-017 review fixes).
	Locale string `json:"locale"`
	// Signup network origin (US-015 review fixes): stamped by the
	// HANDLER from the request (ClientIP + CDN geo header), never
	// bound from the client body — hence json:"-". Persisted on the
	// account row so locale-driven currency arbitrage is auditable.
	SignupIP      string `json:"-"`
	SignupCountry string `json:"-"`
}

// utmCap bounds each persisted UTM value so a crafted-link cannot bloat the
// row. UTM values are conventionally short campaign slugs.
const utmCap = 255

// utmPtr trims, length-caps and nil-ifies an inbound UTM value so empty
// strings land as SQL NULL rather than "".
func utmPtr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if len(v) > utmCap {
		v = v[:utmCap]
	}
	return &v
}

func (uc *CreateAccountUseCase) Execute(ctx context.Context, dto CreateAccountDTO) error {
	// 0. Registration gate (US-F001) — refuse before touching the DB or
	// sending any mail. Checked here (not in a route middleware) so every
	// caller of the use case inherits it. A one-shot invite is the ONE
	// permission that passes a closed door (validated just below).
	inviteToken := strings.TrimSpace(dto.InviteToken)
	if uc.registrationPaused && inviteToken == "" {
		return domain.ErrRegistrationPaused
	}

	// 0b. Invite resolution — fail CLOSED on everything. A token that does
	// not resolve to a redeemable invite bound to THIS e-mail is refused
	// with one uniform error, whatever the actual reason.
	var invite *domain.AccountInvite
	if inviteToken != "" {
		if uc.inviteRepo == nil {
			return domain.ErrInviteInvalid
		}
		inv, err := uc.inviteRepo.FindValid(ctx, domain.HashInviteToken(inviteToken))
		if err != nil {
			slog.ErrorContext(ctx, "invite lookup failed — refusing signup", "error", err)
			return domain.ErrInviteInvalid
		}
		if inv == nil || !strings.EqualFold(strings.TrimSpace(dto.AdminEmail), inv.Email) {
			return domain.ErrInviteInvalid
		}
		invite = inv
	}

	// 0. Validate required fields (defense-in-depth)
	if strings.TrimSpace(dto.AdminName) == "" || strings.TrimSpace(dto.AdminEmail) == "" || strings.TrimSpace(dto.Password) == "" {
		return &domain.AppError{Code: "VALIDATION_ERROR", Message: "name, email and password are required"}
	}
	// LGPD: the contract is only valid when the user explicitly accepted the
	// Terms + Privacy Policy. We refuse here rather than silently auto-
	// accepting on the frontend side, so the audit trail (DB column) reflects
	// real user intent.
	if !dto.TermsAccepted {
		return &domain.AppError{
			Code:    "TERMS_NOT_ACCEPTED",
			Message: "you must accept the Terms of Service and Privacy Policy to create an account",
		}
	}
	if strings.TrimSpace(dto.AccountName) == "" {
		dto.AccountName = dto.AdminName
	}

	// 1. Verify if email already exists
	existingUser, err := uc.userRepo.GetByEmail(ctx, dto.AdminEmail)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return domain.ErrEmailAlreadyExists
	}

	// Hoisted so the post-commit referral linker (US-174) and the seller
	// self-referral guard (US-013) can reference them.
	accountID := ulid.Make().String()
	userID := ulid.Make().String()

	// Burn the invite BEFORE creating anything: consume-first is what makes
	// two racing signups on one link resolve to ONE account (the loser's
	// Consume returns false before it touched the database). The cost is the
	// narrow crash window between here and the commit, where a clean failure
	// un-burns below and a hard crash means the owner reissues — a burned
	// invite is recoverable, a duplicated account is not.
	if invite != nil {
		ok, err := uc.inviteRepo.Consume(ctx, invite.ID, accountID)
		if err != nil || !ok {
			return domain.ErrInviteInvalid
		}
	}

	if err := uc.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		// 1. Create Account
		// 7-day trial window. After this, RequireActiveSubscription middleware
		// returns 402 on paid routes until the user upgrades via Stripe (which
		// clears trial_expires_at and flips subscription_status to "active").
		trialDeadline := time.Now().Add(7 * 24 * time.Hour)
		account := &domain.Account{
			ID:                 accountID,
			Name:               dto.AccountName,
			Currency:           domain.CurrencyFromLocale(dto.Locale),
			SubscriptionStatus: "trialing",
			TrialExpiresAt:     &trialDeadline,
			UTMSource:          utmPtr(dto.UTMSource),
			UTMMedium:          utmPtr(dto.UTMMedium),
			UTMCampaign:        utmPtr(dto.UTMCampaign),
			SignupIP:           utmPtr(dto.SignupIP),
			SignupCountry:      utmPtr(dto.SignupCountry),
			SignupLocale:       utmPtr(dto.Locale),
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		if err := uc.accountRepo.Create(ctx, account); err != nil {
			return err
		}

		// 2. Create Admin User
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(dto.Password), 12)
		if err != nil {
			return err
		}

		hashStr := string(hashedPassword)
		// The account creator is the OWNER (migration 000149 model). Writing
		// "admin" here left owner-directed queries (GetAdminByAccountID) unable
		// to find the decision-maker once 000149 promoted creators to owner.
		role := "owner"
		now := time.Now()
		termsVer := domain.CurrentTermsVersion
		user := &domain.User{
			ID:              userID,
			AccountID:       accountID,
			Name:            dto.AdminName,
			Email:           dto.AdminEmail,
			PasswordHash:    &hashStr,
			Role:            &role,
			IsActive:        true,
			TermsAcceptedAt: &now,
			TermsVersion:    &termsVer,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := uc.userRepo.Create(ctx, user); err != nil {
			return err
		}

		// 2b. Generate verification token and send verification email
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return err
		}
		verificationToken := hex.EncodeToString(tokenBytes)
		verificationExpiresAt := time.Now().Add(24 * time.Hour)
		if err := uc.userRepo.SetVerificationToken(ctx, userID, verificationToken, verificationExpiresAt); err != nil {
			return err
		}

		emailLocale := domain.EmailLocaleForCurrency(account.Currency)
		go func() {
			if err := uc.emailService.SendVerificationEmail(user.Email, verificationToken, uc.appBaseURL, emailLocale); err != nil {
				slog.Error("failed to send verification email", "email", user.Email, "error", err)
			}
		}()

		// 3. Initialize Usage for the first billing cycle
		usageID := ulid.Make().String()
		billingStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		zeroConversations := int32(0)
		zeroTokens := int64(0)
		zeroStorage := int32(0)

		usage := &domain.AccountUsage{
			ID:                 usageID,
			AccountID:          accountID,
			BillingCycleStart:  billingStart,
			ConversationsCount: &zeroConversations,
			AITokensUsed:       &zeroTokens,
			StorageUsedMB:      &zeroStorage,
			UpdatedAt:          now,
		}

		if err := uc.usageRepo.Create(ctx, usage); err != nil {
			return err
		}

		// 3b. An invited account is born CONTRACTED — same shape the owner
		// panel's apply-contract writes, inside the same transaction as the
		// account itself, so there is no window where the customer exists on
		// trial limits.
		if invite != nil {
			if err := uc.accountRepo.SetPlan(ctx, accountID, invite.PlanID); err != nil {
				return err
			}
			if err := uc.accountRepo.SetLimitOverrides(ctx, accountID, invite.Overrides); err != nil {
				return err
			}
			if err := uc.accountRepo.SetTrialBypass(ctx, accountID, true, "account_invite"); err != nil {
				return err
			}
			if err := uc.accountRepo.SetSubscriptionStatus(ctx, accountID, "active"); err != nil {
				return err
			}
		}

		// 4. The default funnel — the Clientes tab must never open empty of
		// a place to put customers.
		return ensureDefaultFunnel(ctx, uc.funnelRepo, accountID)
	}); err != nil {
		// A cleanly failed creation must not cost the invite. Best-effort:
		// the WARN is the trace when the revert itself fails, and the owner
		// reissues.
		if invite != nil {
			if unErr := uc.inviteRepo.Unconsume(ctx, invite.ID); unErr != nil {
				slog.WarnContext(ctx, "invite unconsume failed — invite burned, reissue it",
					"invite_id", invite.ID, "error", unErr)
			}
		}
		return err
	}
	if invite != nil {
		slog.InfoContext(ctx, "account created via invite",
			"invite_id", invite.ID, "account_id", accountID, "plan_id", invite.PlanID)
	}

	// 4. Best-effort ?ref= attribution. Runs after the account commits —
	// a bad/unknown code must never fail registration. The single ref
	// param serves two separate namespaces: seller prospecting codes
	// (US-003, users.seller_ref_code) are tried first; only when the code
	// is not a seller code does it fall through to the customer referral
	// program (US-174, accounts.referral_code).
	if !stampSellerRefAttribution(ctx, uc.sellerRefRepo, uc.platformAccountID, accountID, dto.ReferralCode, userID, dto.AdminEmail) {
		uc.linkReferral(ctx, accountID, dto.ReferralCode)
	}
	return nil
}

// linkReferral resolves a ?ref=CODE signup to its referrer and records
// the pending referral. Every failure is swallowed (logged) so signup
// never breaks on a referral problem.
func (uc *CreateAccountUseCase) linkReferral(ctx context.Context, refereeAccountID, code string) {
	if uc.referralRepo == nil || strings.TrimSpace(code) == "" {
		return
	}
	referrerID, err := uc.referralRepo.ResolveReferrer(ctx, strings.TrimSpace(code))
	if err != nil {
		slog.Error("referral: failed to resolve referrer code", "code", code, "error", err)
		return
	}
	if referrerID == "" {
		slog.Info("referral: unknown referral code on signup, ignoring", "code", code)
		return
	}
	if err := uc.referralRepo.LinkReferral(ctx, refereeAccountID, referrerID); err != nil {
		slog.Error("referral: failed to link referral", "referee", refereeAccountID, "referrer", referrerID, "error", err)
	}
}
