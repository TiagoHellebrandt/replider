package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
)

// FunnelUseCase covers the account-scoped CRUD for sales funnels (US-006).
// Ownership is enforced on every read/mutation by comparing the resource's
// account_id against the caller's account_id. The dynamic metadata schema is
// validated on every write via domain.ValidateMetadataSchema.
type FunnelUseCase struct {
	repo          domain.SalesFunnelRepository
	webhookTester FunnelWebhookTester
	// Plan enforcement (US-M013). Optional wiring: unwired, funnel
	// creation behaves exactly as before rather than blocking every
	// customer because a dependency was forgotten.
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
}

// SetPlanLimits wires the per-tier funnel cap (US-M013).
func (uc *FunnelUseCase) SetPlanLimits(accountRepo domain.AccountRepository, planRepo domain.PlanRepository) {
	uc.accountRepo = accountRepo
	uc.planRepo = planRepo
}

// ErrFunnelLimitReached is the tier cap on funnels.
var ErrFunnelLimitReached = &domain.AppError{
	Code:    "PLAN_LIMIT_EXCEEDED_FUNNELS",
	Message: "seu plano não permite mais funis — faça upgrade para criar outro",
}

// checkFunnelLimit enforces the plan's funnel cap.
//
// Reads EffectiveMaxFunnels, never the raw plan column: a contract plan
// stores 0 and would otherwise block the customers paying the most.
func (uc *FunnelUseCase) checkFunnelLimit(ctx context.Context, accountID string) error {
	if uc.accountRepo == nil || uc.planRepo == nil {
		return nil
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.PlanID == nil {
		// A lookup failure must not become an outage on funnel creation;
		// the cap is a commercial limit, not a security boundary.
		return nil
	}
	plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
	if err != nil || plan == nil {
		return nil
	}
	limit := account.EffectiveMaxFunnels(plan)
	if limit <= 0 {
		// 0 on a self-serve plan means the tier predates the cap (the old
		// attendance ladder), not "no funnels allowed".
		return nil
	}
	existing, err := uc.repo.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil
	}
	if len(existing) >= limit {
		return ErrFunnelLimitReached
	}
	return nil
}

func NewFunnelUseCase(repo domain.SalesFunnelRepository) *FunnelUseCase {
	return &FunnelUseCase{repo: repo}
}

// FunnelWebhookTester sends a synthetic, signed test event to an explicit
// URL+secret and returns the consumer's HTTP status + response body. Satisfied
// by *services.OutboundWebhookDispatcher.DeliverFunnelTest. Kept as an interface
// so the funnel webhook-test use case (US-017) can be unit-tested without the
// HTTP machinery, and so the dispatcher dependency stays optional.
type FunnelWebhookTester interface {
	DeliverFunnelTest(ctx context.Context, accountID, url, secret string) (int, string, error)
}

// SetWebhookTester wires the optional dispatcher used by TestWebhook (US-017).
// nil leaves TestWebhook returning ErrFunnelWebhookTesterUnavailable. main.go
// injects the shared OutboundWebhookDispatcher.
func (uc *FunnelUseCase) SetWebhookTester(t FunnelWebhookTester) {
	uc.webhookTester = t
}

// FunnelWebhookTestResult is the outcome of a funnel webhook test ping returned
// to the dashboard: the consumer's HTTP status and its (truncated) response body.
type FunnelWebhookTestResult struct {
	StatusCode   int    `json:"status_code"`
	ResponseBody string `json:"response_body"`
}

// TestWebhook fires a synthetic signed event at the funnel's configured webhook
// (US-017) so an admin can validate the URL/secret before relying on the real
// lead.qualified/lead.buy_intent triggers. Ownership is enforced; a funnel with
// no webhook URL returns ErrFunnelWebhookNotSet. A transport failure (DNS,
// timeout, refused) surfaces as ErrFunnelWebhookTestFailed rather than a 500 —
// it's the consumer's endpoint that's broken, not Replider.
func (uc *FunnelUseCase) TestWebhook(ctx context.Context, accountID, id string) (*FunnelWebhookTestResult, error) {
	f, err := uc.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(f.WebhookURL) == "" {
		return nil, domain.ErrFunnelWebhookNotSet
	}
	if uc.webhookTester == nil {
		return nil, domain.ErrFunnelWebhookTesterUnavailable
	}
	status, respBody, err := uc.webhookTester.DeliverFunnelTest(ctx, accountID, f.WebhookURL, f.WebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrFunnelWebhookTestFailed, err.Error())
	}
	return &FunnelWebhookTestResult{StatusCode: status, ResponseBody: respBody}, nil
}

// validateAndNormalizeSchema decodes the raw metadata schema, validates it
// (snake_case unique keys, allowed types, enum options, max field count) and
// returns a normalized, non-nil JSON array ("[]" when empty).
func validateAndNormalizeSchema(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("[]"), nil
	}
	var fields []domain.MetadataFieldDef
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, domain.ErrInvalidMetadataSchema
	}
	if err := domain.ValidateMetadataSchema(fields); err != nil {
		return nil, err
	}
	// Re-marshal from the typed slice to drop unknown fields and keep storage
	// canonical.
	normalized, err := json.Marshal(fields)
	if err != nil {
		return nil, domain.ErrInvalidMetadataSchema
	}
	return normalized, nil
}

type CreateFunnelDTO struct {
	AccountID                 string
	Name                      string
	ProductID                 *string
	AgentID                   *string
	ChannelID                 *string
	PersonaPrompt             string
	MetadataSchema            json.RawMessage
	WebhookURL                string
	WebhookSecret             string
	TriggerOnRequiredComplete bool
	TriggerOnBuyIntent        bool
	Active                    *bool
}

// validateFunnelWebhookURL applies the SSRF guard to the per-funnel webhook
// (US-F011). Empty = no webhook, which is valid. Until now this field was
// persisted unvalidated — the only user-supplied URL in the product that
// reached the dispatcher without a check.
func validateFunnelWebhookURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if err := httpsafe.ValidateURL(raw); err != nil {
		return &domain.AppError{
			Code:    "WEBHOOK_URL_BLOCKED",
			Message: "webhook URL não pode apontar para a rede interna",
		}
	}
	return nil
}

func (uc *FunnelUseCase) Create(ctx context.Context, dto CreateFunnelDTO) (*domain.SalesFunnel, error) {
	if err := uc.checkFunnelLimit(ctx, dto.AccountID); err != nil {
		return nil, err
	}
	schema, err := validateAndNormalizeSchema(dto.MetadataSchema)
	if err != nil {
		return nil, err
	}
	if err := validateFunnelWebhookURL(dto.WebhookURL); err != nil {
		return nil, err
	}
	active := true
	if dto.Active != nil {
		active = *dto.Active
	}

	now := time.Now()
	f := &domain.SalesFunnel{
		ID:                        ulid.Make().String(),
		AccountID:                 dto.AccountID,
		Name:                      strings.TrimSpace(dto.Name),
		ProductID:                 dto.ProductID,
		AgentID:                   dto.AgentID,
		ChannelID:                 dto.ChannelID,
		PersonaPrompt:             dto.PersonaPrompt,
		MetadataSchema:            schema,
		WebhookURL:                strings.TrimSpace(dto.WebhookURL),
		WebhookSecret:             dto.WebhookSecret,
		TriggerOnRequiredComplete: dto.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        dto.TriggerOnBuyIntent,
		Active:                    active,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	if err := uc.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// List returns the account's funnels, and it never returns none: the
// default funnel is created here if the account somehow lacks one (an
// account older than the migration whose default was deleted by SQL, a
// registration whose funnel insert failed). The Clientes tab reads this
// list, and "you have no funnel yet" is a sentence the product no longer
// says.
func (uc *FunnelUseCase) List(ctx context.Context, accountID string) ([]*domain.SalesFunnel, error) {
	if _, err := uc.repo.EnsureDefault(ctx, accountID); err != nil {
		return nil, err
	}
	return uc.repo.ListByAccountID(ctx, accountID)
}

// Default returns the account's implicit funnel, creating it if needed.
func (uc *FunnelUseCase) Default(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	return uc.repo.EnsureDefault(ctx, accountID)
}

func (uc *FunnelUseCase) Get(ctx context.Context, accountID, id string) (*domain.SalesFunnel, error) {
	f, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if f.AccountID != accountID {
		return nil, domain.ErrUnauthorized
	}
	return f, nil
}

type UpdateFunnelDTO struct {
	AccountID                 string
	ID                        string
	Name                      string
	ProductID                 *string
	AgentID                   *string
	ChannelID                 *string
	PersonaPrompt             string
	MetadataSchema            json.RawMessage
	WebhookURL                string
	WebhookSecret             string
	TriggerOnRequiredComplete bool
	TriggerOnBuyIntent        bool
	Active                    *bool
}

func (uc *FunnelUseCase) Update(ctx context.Context, dto UpdateFunnelDTO) (*domain.SalesFunnel, error) {
	f, err := uc.repo.GetByID(ctx, dto.ID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if f.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}

	schema, err := validateAndNormalizeSchema(dto.MetadataSchema)
	if err != nil {
		return nil, err
	}
	if err := validateFunnelWebhookURL(dto.WebhookURL); err != nil {
		return nil, err
	}

	f.Name = strings.TrimSpace(dto.Name)
	f.ProductID = dto.ProductID
	f.AgentID = dto.AgentID
	// PATCH semantics for the channel: a nil ChannelID means "not sent", not
	// "clear". The app's funnel form never carries the channel, so a rename
	// used to erase the link FunnelChannelLinker / MCP had made and every
	// approval went back to LEAD_FUNNEL_NO_CHANNEL. Clearing a channel is
	// not possible through this route — nobody needs it.
	if dto.ChannelID != nil {
		f.ChannelID = dto.ChannelID
	}
	f.PersonaPrompt = dto.PersonaPrompt
	f.MetadataSchema = schema
	f.WebhookURL = strings.TrimSpace(dto.WebhookURL)
	f.WebhookSecret = dto.WebhookSecret
	f.TriggerOnRequiredComplete = dto.TriggerOnRequiredComplete
	f.TriggerOnBuyIntent = dto.TriggerOnBuyIntent
	if dto.Active != nil {
		f.Active = *dto.Active
	}
	f.UpdatedAt = time.Now()

	if err := uc.repo.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (uc *FunnelUseCase) Delete(ctx context.Context, accountID, id string) error {
	f, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if f == nil {
		return domain.ErrSalesFunnelNotFound
	}
	if f.AccountID != accountID {
		return domain.ErrUnauthorized
	}
	// The default is where leads land when nobody chose a funnel; deleting
	// it would leave the account with nowhere for its customers to be. It
	// can be renamed and reconfigured, not removed.
	if f.IsDefault {
		return domain.ErrCannotDeleteDefault
	}
	return uc.repo.Delete(ctx, id)
}
