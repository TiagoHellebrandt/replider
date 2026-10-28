package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

type GetMeUseCase struct {
	userRepo                domain.UserRepository
	accountRepo             domain.AccountRepository
	planRepo                domain.PlanRepository
	getOnboardingProgressUC *GetOnboardingProgressUseCase
	// superAdminEmails is the SUPER_ADMIN_EMAILS allowlist. A staff
	// account on it is never billing-locked — /me reports it as
	// trial-bypassed so the dashboard suppresses every trial/expired
	// banner. Wired via SetSuperAdminEmails from main.go (optional).
	superAdminEmails []string
	// platformAccountID is the Replider platform account id
	// (PLATFORM_ACCOUNT_ID). /me reports is_platform_account so the app
	// knows whether to surface the seller panel. Empty = feature off.
	// Wired via SetPlatformAccountID from main.go (optional).
	platformAccountID string
	// sellerFlags resolves the explicit seller flag (US-014) so /me can
	// report is_seller and the app only shows the seller panel to actual
	// sellers. Optional — nil reports false. Wired via
	// SetSellerFlagChecker from main.go.
	sellerFlags sellerFlagChecker
}

// SetSellerFlagChecker wires the seller_settings.is_seller lookup so /me
// reports is_seller for platform-account users (US-014). Nil = always
// false.
func (uc *GetMeUseCase) SetSellerFlagChecker(checker sellerFlagChecker) {
	uc.sellerFlags = checker
}

// SetSuperAdminEmails wires the super-admin allowlist so /me auto-bypasses
// the trial gate for staff accounts. Nil/empty = no auto-bypass.
func (uc *GetMeUseCase) SetSuperAdminEmails(emails []string) {
	uc.superAdminEmails = emails
}

// SetPlatformAccountID wires the PLATFORM_ACCOUNT_ID so /me can flag the
// Replider platform account (seller features). Empty = never flagged.
func (uc *GetMeUseCase) SetPlatformAccountID(id string) {
	uc.platformAccountID = id
}

func NewGetMeUseCase(userRepo domain.UserRepository, accountRepo domain.AccountRepository, planRepo domain.PlanRepository, getOnboardingProgressUC *GetOnboardingProgressUseCase) *GetMeUseCase {
	return &GetMeUseCase{
		userRepo:                userRepo,
		accountRepo:             accountRepo,
		planRepo:                planRepo,
		getOnboardingProgressUC: getOnboardingProgressUC,
	}
}

type GetMeDTO struct {
	Email string // Since UserRepository doesn't have GetByID yet, we'll fetch by Email assuming it's also in the token or we can just add GetByID.
	// We'll update UserRepository to include GetByID in domain, but for now we can either use Email or implement GetByID.
	// Actually, let's just pass UserID and AccountID from token, but wait, UserRepository doesn't have GetByID.
	// Let's add GetByID to UserRepository interface first. No wait, JWT has UserID. Let's assume we add it.
}

// UserDetails encapsulates user info
type UserDetails struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Email           string  `json:"email"`
	Role            string  `json:"role"`
	Picture         *string `json:"picture,omitempty"`
	IsEmailVerified bool    `json:"is_email_verified"`
	// Signature is the attendant's optional manual-reply sign-off (US-018).
	// The composer offers it on manual sends; never applied to AI replies.
	Signature *string `json:"signature,omitempty"`
	// TOTPEnabled lets the dashboard render the 2FA card state instantly
	// from the cached /me user instead of waiting on a dedicated
	// GET /me/2fa/status round-trip.
	TOTPEnabled bool `json:"totp_enabled"`
}

// AccountDetails encapsulates account info
type AccountDetails struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Currency is the account's immutable billing currency, set at
	// signup from the user's locale (US-001). The dashboard uses it to
	// pick the plan catalogue + price formatting (R$ vs $).
	Currency           string  `json:"currency" enums:"brl,usd"`
	PlanID             *string `json:"plan_id,omitempty"`
	SubscriptionStatus string  `json:"subscription_status"`
	IsTrialing         bool    `json:"is_trialing"`
	// TrialDaysRemaining is computed from trial_expires_at so the Flutter
	// app can render a countdown banner without re-doing the timezone math.
	// Zero when the trial is over (or missing a deadline). Null when the
	// account has already converted to a paid plan.
	TrialDaysRemaining *int   `json:"trial_days_remaining,omitempty"`
	PlanName           string `json:"plan_name"`
	// IsContractPlan flags Partner / Enterprise accounts where the plan
	// is sold via off-platform contract (no Stripe price). The dashboard
	// uses this to hide self-serve checkout buttons and surface the BYOK
	// API-keys settings card.
	IsContractPlan bool `json:"is_contract_plan"`
	// TrialBypass exempts the account from the trial/subscription gate
	// (owner-provisioned contract deals, Meta reviewers). The dashboard
	// uses it to suppress every "trial expired / upgrade" banner — such an
	// account is never billing-locked even if its trial window lapsed.
	TrialBypass bool `json:"trial_bypass"`
	// OpenAIKeyMasked / GeminiKeyMasked are the safe-to-display previews
	// of the BYOK keys (first 4 + last 4 chars). Empty string means
	// "not configured". Always empty for non-contract accounts so the
	// platform's env keys never leak into a /me response.
	OpenAIKeyMasked     string `json:"openai_key_masked"`
	GeminiKeyMasked     string `json:"gemini_key_masked"`
	OnboardingCompleted bool   `json:"onboarding_completed"`
	// BusinessLines / Territory: what the company does and where. The
	// onboarding asks them first and reads them back from here on resume.
	BusinessLines []domain.BusinessLine `json:"business_lines"`
	Territory     *domain.Territory     `json:"territory,omitempty"`
}

type GetMeResponse struct {
	User               UserDetails                `json:"user"`
	Account            AccountDetails             `json:"account"`
	OnboardingProgress *domain.OnboardingProgress `json:"onboarding_progress,omitempty"`
	// IsPlatformAccount is true only for the Replider platform account
	// (PLATFORM_ACCOUNT_ID) — the app uses it to decide whether to show
	// the seller (vendedores) panel. Always false when unconfigured.
	IsPlatformAccount bool `json:"is_platform_account"`
	// IsSeller is the explicit owner-granted seller flag (US-014). The
	// seller panel entry needs BOTH is_platform_account and is_seller —
	// a regular platform attendant is not a payable seller.
	IsSeller bool `json:"is_seller"`
	// ImpersonatingActor — non-empty when the JWT was minted by the
	// owner panel's /impersonate endpoint. The dashboard reads this
	// (server-truth, not local storage) to gate the red banner.
	// Populated by the handler from gin context, not the use case.
	ImpersonatingActor string `json:"impersonating_actor,omitempty"`
}

// EffectivePlanLimits carries the resolved per-account caps the dashboard
// uses to render quotas + plan-card details. Computed from
// Account.EffectiveMax* helpers so contract plans (Partner / Enterprise)
// expose their per-account overrides instead of the zeroed plan row.
//
// Encoding contract: a value of -1 signals "unlimited". The Account
// helpers normally return domain.UnlimitedLimit (1<<31), which surfaces
// in JSON as a huge number that looks like a real cap to clients. We
// fold both sentinels (UnlimitedLimit and 0-on-contract) onto -1 here so
// the Flutter side can branch on a stable marker.
type EffectivePlanLimits struct {
	PlanID                       *string `json:"plan_id,omitempty"`
	PlanName                     string  `json:"plan_name"`
	IsContractPlan               bool    `json:"is_contract_plan"`
	MaxAgents                    int     `json:"max_agents"`
	MaxChannels                  int     `json:"max_channels"`
	MaxSeats                     int     `json:"max_seats"`
	MaxConversationsPerMonth     int     `json:"max_conversations_per_month"`
	MaxAITokensPerMonth          int64   `json:"max_ai_tokens_per_month"`
	MaxStorageMB                 int     `json:"max_storage_mb"`
	MaxBroadcastMessagesPerMonth int     `json:"max_broadcast_messages_per_month"`
	MaxRecipientsPerCampaign     int     `json:"max_recipients_per_campaign"`
	OverageConversationCents     int     `json:"overage_conversation_cents"`
	OverageCapMultiplier         float64 `json:"overage_cap_multiplier"`

	// What the customer actually buys today (US-M013). These caps already existed
	// on the plan and were already ENFORCED — the enrichment one blocks company
	// research mid-task — but never left the server. Whoever hit one found out by
	// clicking.
	MaxLeadsPerMonth               int `json:"max_leads_per_month"`
	MaxEnrichmentsPerMonth         int `json:"max_enrichments_per_month"`
	MaxAIDraftsPerMonth            int `json:"max_ai_drafts_per_month"`
	MaxDocumentExtractionsPerMonth int `json:"max_document_extractions_per_month"`
	MaxEmailMailboxes              int `json:"max_email_mailboxes"`
	MaxFunnels                     int `json:"max_funnels"`
}

func capForJSON(v int) int {
	if v >= domain.UnlimitedLimit {
		return -1
	}
	return v
}

func capForJSON64(v int64) int64 {
	if v >= int64(domain.UnlimitedLimit) {
		return -1
	}
	return v
}

// ResolvePlanLimits returns the EffectivePlanLimits for the calling
// account. Reuses the same fetch path as Execute() so a contract account
// whose plan row is all zeros still gets the override-resolved values.
// Trial accounts (PlanID nil) fall back to TrialPlan().
func (uc *GetMeUseCase) ResolvePlanLimits(ctx context.Context, accountID string) (*EffectivePlanLimits, error) {
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}

	var plan *domain.Plan
	if account.PlanID != nil {
		p, err := uc.planRepo.GetByID(ctx, *account.PlanID)
		if err == nil && p != nil {
			plan = p
		}
	}
	if plan == nil {
		plan = domain.TrialPlan()
	}

	return &EffectivePlanLimits{
		PlanID:                       account.PlanID,
		PlanName:                     plan.Name,
		IsContractPlan:               account.IsContractPlan(plan),
		MaxAgents:                    capForJSON(account.EffectiveMaxAgents(plan)),
		MaxChannels:                  capForJSON(account.EffectiveMaxChannels(plan)),
		MaxSeats:                     capForJSON(account.EffectiveMaxSeats(plan)),
		MaxConversationsPerMonth:     capForJSON(account.EffectiveMaxConversationsPerMonth(plan)),
		MaxAITokensPerMonth:          capForJSON64(account.EffectiveMaxAITokensPerMonth(plan)),
		MaxStorageMB:                 capForJSON(account.EffectiveMaxStorageMB(plan)),
		MaxBroadcastMessagesPerMonth: capForJSON(account.EffectiveMaxBroadcastMessagesPerMonth(plan)),
		MaxRecipientsPerCampaign:     capForJSON(account.EffectiveMaxRecipientsPerCampaign(plan)),

		MaxLeadsPerMonth:       capForJSON(account.EffectiveMaxLeadsPerMonth(plan)),
		MaxEnrichmentsPerMonth: capForJSON(account.EffectiveMaxEnrichmentsPerMonth(plan)),
		MaxAIDraftsPerMonth:    capForJSON(account.EffectiveMaxAIDraftsPerMonth(plan)),
		MaxDocumentExtractionsPerMonth: capForJSON(
			account.EffectiveMaxDocumentExtractionsPerMonth(plan)),
		MaxEmailMailboxes:        capForJSON(account.EffectiveMaxEmailMailboxes(plan)),
		MaxFunnels:               capForJSON(account.EffectiveMaxFunnels(plan)),
		OverageConversationCents: plan.OverageConversationCents,
		OverageCapMultiplier:     plan.OverageCapMultiplier,
	}, nil
}

func (uc *GetMeUseCase) Execute(ctx context.Context, userID, accountID string) (*GetMeResponse, error) {
	// 1. Fetch Account
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, domain.ErrAccountNotFound
	}

	if account == nil {
		return nil, domain.ErrAccountNotFound
	}

	// 2. Fetch User
	// Note: We need GetByID on UserRepository. I'll need to modify the repository.go and postgres implementation.
	// Let's assume uc.userRepo.GetByID exists for the execution plan, I will implement it next.
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUnauthorized // Or ErrUserNotFound
	}
	if user == nil {
		return nil, domain.ErrUnauthorized
	}

	role := "attendant"
	if user.Role != nil {
		role = *user.Role
	}

	// Staff (super-admin allowlist) accounts are never billing-locked — a
	// support operator's own account shouldn't show trial banners while
	// they work the panel. Treat it exactly like an explicit trial_bypass.
	isSuperAdmin := false
	for _, e := range uc.superAdminEmails {
		if strings.EqualFold(strings.TrimSpace(e), user.Email) {
			isSuperAdmin = true
			break
		}
	}
	effectiveBypass := account.TrialBypass || isSuperAdmin

	// trial_bypass exempts an account from the trial gate (owner-provisioned
	// contract/Enterprise/Partner deals), so it must NOT read as "trialing"
	// — otherwise the dashboard shows a trial-expired banner on a paid
	// contract account. Contract plans are likewise never on trial.
	isTrialing := account.SubscriptionStatus == domain.SubscriptionStatusTrialing &&
		!effectiveBypass
	planName := "Trial"
	isContractPlan := false
	if account.PlanID != nil {
		plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
		if err == nil && plan != nil {
			planName = plan.Name
			isContractPlan = account.IsContractPlan(plan)
		}
	}
	if isContractPlan {
		isTrialing = false
	}

	// BYOK keys are surfaced masked so the admin can confirm the right
	// key is on file without the platform shipping the secret to every
	// /me caller (the response goes to every dashboard tab on every
	// page-load). Only echo masks for contract accounts; self-serve
	// accounts use the platform's env keys and have no per-account
	// override to reveal.
	var openAIMask, geminiMask string
	if isContractPlan {
		if account.OpenAIAPIKey != nil && *account.OpenAIAPIKey != "" {
			openAIMask = secrets.MaskKey(*account.OpenAIAPIKey)
		}
		if account.GeminiAPIKey != nil && *account.GeminiAPIKey != "" {
			geminiMask = secrets.MaskKey(*account.GeminiAPIKey)
		}
	}

	// Days left in the trial. Nil when the account is already on a paid
	// plan; 0 when the deadline has passed (trial ended). We round UP so
	// a 12h slice still reports "1 day remaining" — underselling here is
	// worse for conversion than overselling by a few hours.
	var trialDays *int
	if isTrialing && account.TrialExpiresAt != nil {
		hoursLeft := time.Until(*account.TrialExpiresAt).Hours()
		days := int(hoursLeft / 24)
		if hoursLeft > 0 && hoursLeft-float64(days*24) > 0 {
			days++
		}
		if days < 0 {
			days = 0
		}
		trialDays = &days
	}

	// Inline the onboarding checklist data so the dashboard can render
	// the persistent card from a single /me roundtrip. Failure here is
	// non-fatal — the dedicated /me/onboarding-progress endpoint exists
	// as a fallback and a missing block just hides the card.
	var progress *domain.OnboardingProgress
	if uc.getOnboardingProgressUC != nil {
		if p, err := uc.getOnboardingProgressUC.Execute(ctx, accountID); err == nil {
			progress = p
		}
	}

	// is_seller only matters (and only costs a lookup) on the platform
	// account. Best-effort: a lookup failure reports false — the API
	// routes stay the enforcement point (RequireSeller), this is UI hint.
	isPlatform := uc.platformAccountID != "" && account.ID == uc.platformAccountID
	isSeller := false
	if isPlatform && uc.sellerFlags != nil {
		if ok, err := uc.sellerFlags.IsSeller(ctx, userID); err == nil {
			isSeller = ok
		}
	}

	return &GetMeResponse{
		User: UserDetails{
			ID:              user.ID,
			Name:            user.Name,
			Email:           user.Email,
			Role:            role,
			Picture:         user.Picture,
			IsEmailVerified: user.IsEmailVerified,
			Signature:       user.Signature,
			TOTPEnabled:     user.TOTPEnabled,
		},
		Account: AccountDetails{
			ID:                  account.ID,
			Name:                account.Name,
			Currency:            account.Currency,
			BusinessLines:       nonNilLines(account.BusinessLines),
			Territory:           account.Territory,
			PlanID:              account.PlanID,
			SubscriptionStatus:  account.SubscriptionStatus,
			IsTrialing:          isTrialing,
			TrialDaysRemaining:  trialDays,
			PlanName:            planName,
			IsContractPlan:      isContractPlan,
			TrialBypass:         effectiveBypass,
			OpenAIKeyMasked:     openAIMask,
			GeminiKeyMasked:     geminiMask,
			OnboardingCompleted: account.OnboardingCompleted,
		},
		OnboardingProgress: progress,
		IsPlatformAccount:  isPlatform,
		IsSeller:           isSeller,
	}, nil
}

// nonNilLines keeps the JSON an array ("[]") rather than null, so the client
// can iterate without a nil check.
func nonNilLines(l []domain.BusinessLine) []domain.BusinessLine {
	if l == nil {
		return []domain.BusinessLine{}
	}
	return l
}
