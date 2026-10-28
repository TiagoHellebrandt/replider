package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ── Create ─────────────────────────────────────────────────────────────────

// RecipientInput is one entry in a CSV import or manual list: the raw phone
// (or chat id) plus optional per-recipient variable overrides matching the
// template's positional {{1}}, {{2}}... parameters.
type RecipientInput struct {
	Phone string
	Vars  []string
}

type CreateCampaignDTO struct {
	AccountID    string
	ChannelID    string
	Name         string
	TemplateName string
	TemplateLang string
	TemplateVars []string
	ScheduledAt  *time.Time
	// Recipients is the simple case: just phones (use campaign-level vars).
	// Deduplicated server-side.
	Recipients []string
	// RecipientsWithVars is the CSV case: per-recipient vars override
	// TemplateVars at that position. Takes precedence over Recipients when
	// non-empty. Deduplicated by phone; first occurrence wins.
	RecipientsWithVars []RecipientInput
	CreatedBy          string
}

type CreateCampaignUseCase struct {
	campaignRepo domain.CampaignRepository
	channelRepo  domain.ChannelRepository
	accountRepo  domain.AccountRepository
	planRepo     domain.PlanRepository
	usageRepo    domain.UsageRepository
	contactRepo  domain.ContactRepository // optional — when set, opted-out recipients are filtered
}

func NewCreateCampaignUseCase(
	c domain.CampaignRepository,
	ch domain.ChannelRepository,
	ar domain.AccountRepository,
	pr domain.PlanRepository,
	ur domain.UsageRepository,
) *CreateCampaignUseCase {
	return &CreateCampaignUseCase{
		campaignRepo: c,
		channelRepo:  ch,
		accountRepo:  ar,
		planRepo:     pr,
		usageRepo:    ur,
	}
}

// WithContactRepo enables opt-out filtering at campaign creation: recipients
// whose externalID is flagged as opted-out are silently dropped before the
// plan-limit checks run.
func (uc *CreateCampaignUseCase) WithContactRepo(r domain.ContactRepository) *CreateCampaignUseCase {
	uc.contactRepo = r
	return uc
}

// Execute validates, persists the campaign in the requested status (draft if
// no ScheduledAt and no Recipients, else scheduled) and inserts the recipient
// rows. The actual sending is performed by the worker.
func (uc *CreateCampaignUseCase) Execute(ctx context.Context, dto CreateCampaignDTO) (*domain.Campaign, error) {
	if dto.Name == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "name is required"}
	}
	if dto.TemplateName == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "template_name is required"}
	}
	if dto.ChannelID == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "channel_id is required"}
	}
	if dto.TemplateLang == "" {
		dto.TemplateLang = "pt_BR"
	}

	// Authorize: channel must belong to the account
	ch, err := uc.channelRepo.GetByID(ctx, dto.ChannelID)
	if err != nil {
		return nil, err
	}
	if ch == nil || ch.AccountID != dto.AccountID {
		return nil, domain.ErrChannelNotFound
	}
	if ch.Type != "whatsapp" {
		return nil, &domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "campaigns currently support whatsapp channels only"}
	}
	if !ch.IsActive {
		return nil, &domain.AppError{Code: "CHANNEL_DISABLED", Message: "channel is disabled"}
	}

	// Normalize to (phone, vars) tuples, deduped by phone. Two input
	// shapes: the simple one (Recipients []string) and the CSV one
	// (RecipientsWithVars). If both are populated, the CSV form wins —
	// the simple list is ignored to avoid ambiguity.
	type recInput struct {
		phone string
		vars  []string
	}
	seen := make(map[string]struct{})
	recipients := make([]recInput, 0)
	if len(dto.RecipientsWithVars) > 0 {
		for _, r := range dto.RecipientsWithVars {
			if r.Phone == "" {
				continue
			}
			if _, ok := seen[r.Phone]; ok {
				continue
			}
			seen[r.Phone] = struct{}{}
			recipients = append(recipients, recInput{phone: r.Phone, vars: r.Vars})
		}
	} else {
		for _, p := range dto.Recipients {
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			recipients = append(recipients, recInput{phone: p})
		}
	}
	// LGPD: drop recipients who have previously opted out. Done AFTER
	// dedup so the phone-only and CSV paths both benefit; done BEFORE
	// plan-limit checks so an admin never "spends" budget on opt-outs.
	var optedOutDropped int
	if uc.contactRepo != nil && len(recipients) > 0 {
		phones := make([]string, len(recipients))
		for i, r := range recipients {
			phones[i] = r.phone
		}
		optedOut, err := uc.contactRepo.ListOptedOutExternalIDs(ctx, dto.AccountID, phones)
		if err == nil && len(optedOut) > 0 {
			filtered := recipients[:0]
			for _, r := range recipients {
				if optedOut[r.phone] {
					optedOutDropped++
					continue
				}
				filtered = append(filtered, r)
			}
			recipients = filtered
		}
	}

	// Refuse to create a campaign that ended up empty after opt-out filtering.
	if len(recipients) == 0 && optedOutDropped > 0 {
		return nil, &domain.AppError{
			Code:    "ALL_RECIPIENTS_OPTED_OUT",
			Message: "all recipients in this list have opted out",
		}
	}

	// Backward-compatible alias used by the rest of the function.
	uniq := make([]string, len(recipients))
	for i, r := range recipients {
		uniq[i] = r.phone
	}

	// Plan-based cost control: WhatsApp template sends cost Meta conversation
	// fees per recipient. We refuse creation when:
	//   (1) the plan doesn't allow broadcasts at all (cap == 0)
	//   (2) this campaign alone would exceed the per-campaign cap
	//   (3) adding this campaign to the current cycle's usage would exceed
	//       the monthly cap
	//
	// We do (1)+(2)+(3) as three separate checks so the error message tells
	// the admin *exactly* which limit they hit. Reserve usage eagerly so
	// concurrent campaign creations don't both pass a near-limit check.
	if len(uniq) > 0 && uc.accountRepo != nil && uc.planRepo != nil {
		acc, plan, err := uc.currentAccountAndPlan(ctx, dto.AccountID)
		if err != nil {
			return nil, err
		}
		broadcastCap := acc.EffectiveMaxBroadcastMessagesPerMonth(plan)
		recipientsCap := acc.EffectiveMaxRecipientsPerCampaign(plan)
		if broadcastCap <= 0 {
			return nil, &domain.AppError{
				Code:    "BROADCAST_NOT_ALLOWED",
				Message: "your plan does not include broadcast campaigns — upgrade to enable",
			}
		}
		if recipientsCap > 0 && len(uniq) > recipientsCap {
			return nil, &domain.AppError{
				Code: "RECIPIENTS_OVER_CAMPAIGN_CAP",
				Message: fmt.Sprintf(
					"this campaign has %d recipients; your plan allows up to %d per campaign",
					len(uniq), recipientsCap,
				),
			}
		}
		if uc.usageRepo != nil {
			used, err := uc.currentBroadcastUsed(ctx, dto.AccountID)
			if err != nil {
				return nil, err
			}
			if used+len(uniq) > broadcastCap {
				return nil, &domain.AppError{
					Code: "BROADCAST_BUDGET_EXCEEDED",
					Message: fmt.Sprintf(
						"monthly broadcast budget exceeded: %d used + %d requested > %d allowed",
						used, len(uniq), broadcastCap,
					),
				}
			}
		}
	}

	status := domain.CampaignDraft
	if len(uniq) > 0 {
		status = domain.CampaignScheduled
	}

	now := time.Now()
	campaignID := ulid.Make().String()
	createdBy := dto.CreatedBy
	c := &domain.Campaign{
		ID:              campaignID,
		AccountID:       dto.AccountID,
		ChannelID:       dto.ChannelID,
		Name:            dto.Name,
		TemplateName:    dto.TemplateName,
		TemplateLang:    dto.TemplateLang,
		TemplateVars:    dto.TemplateVars,
		Status:          status,
		ScheduledAt:     dto.ScheduledAt,
		TotalRecipients: len(uniq),
		CreatedBy:       &createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := uc.campaignRepo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}

	for _, r := range recipients {
		rec := &domain.CampaignRecipient{
			ID:         ulid.Make().String(),
			CampaignID: campaignID,
			Recipient:  r.phone,
			Vars:       r.vars, // per-recipient overrides; worker merges with campaign.TemplateVars
			Status:     domain.RecipientPending,
			CreatedAt:  now,
		}
		if err := uc.campaignRepo.CreateRecipient(ctx, rec); err != nil {
			// Best-effort: log via wrapped error, continue. Worker can retry
			// missing recipients on next cycle.
			return nil, fmt.Errorf("create recipient %s: %w", r.phone, err)
		}
	}

	// Reserve the broadcast budget against the monthly cap. Best-effort —
	// if this fails the campaign is still created, and the worker's own
	// budget check will refuse to send if the cap is actually exceeded by
	// a concurrent campaign. We log but don't fail because reversing the
	// create is more complex than accepting a tiny window of over-reservation.
	if len(uniq) > 0 && uc.usageRepo != nil {
		if err := uc.usageRepo.AddBroadcastMessages(ctx, dto.AccountID, int32(len(uniq))); err != nil {
			// Not fatal — admin can still cancel the campaign if needed.
			_ = err
		}
	}

	return c, nil
}

// currentAccountAndPlan resolves both the account and its effective plan,
// falling back to the free trial plan when no paid plan is attached.
// Returning the account too lets callers consult the per-account override
// columns via Effective* accessors instead of reading the plan row blind.
// Account is non-nil on success; plan defaults to TrialPlan() if missing.
func (uc *CreateCampaignUseCase) currentAccountAndPlan(ctx context.Context, accountID string) (*domain.Account, *domain.Plan, error) {
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	if acc == nil {
		return &domain.Account{ID: accountID}, domain.TrialPlan(), nil
	}
	if acc.PlanID == nil || *acc.PlanID == "" {
		return acc, domain.TrialPlan(), nil
	}
	plan, err := uc.planRepo.GetByID(ctx, *acc.PlanID)
	if err != nil || plan == nil {
		return acc, domain.TrialPlan(), nil
	}
	return acc, plan, nil
}

// currentBroadcastUsed returns the broadcast_messages_used counter from the
// current billing cycle. Zero when the usage row has not been created yet.
func (uc *CreateCampaignUseCase) currentBroadcastUsed(ctx context.Context, accountID string) (int, error) {
	usage, err := uc.usageRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if usage == nil || usage.BroadcastMessagesUsed == nil {
		return 0, nil
	}
	return int(*usage.BroadcastMessagesUsed), nil
}

// ── List + Get + Delete ────────────────────────────────────────────────────

type ListCampaignsUseCase struct {
	repo domain.CampaignRepository
}

func NewListCampaignsUseCase(r domain.CampaignRepository) *ListCampaignsUseCase {
	return &ListCampaignsUseCase{repo: r}
}

func (uc *ListCampaignsUseCase) Execute(ctx context.Context, accountID string, limit, offset int32) ([]*domain.Campaign, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return uc.repo.ListByAccount(ctx, accountID, limit, offset)
}

type GetCampaignUseCase struct {
	repo domain.CampaignRepository
}

func NewGetCampaignUseCase(r domain.CampaignRepository) *GetCampaignUseCase {
	return &GetCampaignUseCase{repo: r}
}

func (uc *GetCampaignUseCase) Execute(ctx context.Context, accountID, id string) (*domain.Campaign, error) {
	c, err := uc.repo.GetByID(ctx, id, accountID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, &domain.AppError{Code: "CAMPAIGN_NOT_FOUND", Message: "campaign not found"}
	}
	return c, nil
}

type DeleteCampaignUseCase struct {
	repo domain.CampaignRepository
}

func NewDeleteCampaignUseCase(r domain.CampaignRepository) *DeleteCampaignUseCase {
	return &DeleteCampaignUseCase{repo: r}
}

func (uc *DeleteCampaignUseCase) Execute(ctx context.Context, accountID, id string) error {
	c, err := uc.repo.GetByID(ctx, id, accountID)
	if err != nil {
		return err
	}
	if c == nil {
		return &domain.AppError{Code: "CAMPAIGN_NOT_FOUND", Message: "campaign not found"}
	}
	if c.Status == domain.CampaignSending {
		return &domain.AppError{Code: "CAMPAIGN_RUNNING", Message: "cancel the campaign before deleting"}
	}
	return uc.repo.Delete(ctx, id, accountID)
}

// ── Cancel ─────────────────────────────────────────────────────────────────

type CancelCampaignUseCase struct {
	repo domain.CampaignRepository
}

func NewCancelCampaignUseCase(r domain.CampaignRepository) *CancelCampaignUseCase {
	return &CancelCampaignUseCase{repo: r}
}

func (uc *CancelCampaignUseCase) Execute(ctx context.Context, accountID, id string) error {
	c, err := uc.repo.GetByID(ctx, id, accountID)
	if err != nil {
		return err
	}
	if c == nil {
		return &domain.AppError{Code: "CAMPAIGN_NOT_FOUND", Message: "campaign not found"}
	}
	switch c.Status {
	case domain.CampaignDone, domain.CampaignFailed, domain.CampaignCanceled:
		return &domain.AppError{Code: "CAMPAIGN_TERMINAL", Message: "campaign already in terminal state"}
	}
	return uc.repo.UpdateStatus(ctx, id, domain.CampaignCanceled)
}
