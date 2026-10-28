package services

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
)

// CampaignSender abstracts the act of sending one template message and
// returning the provider-side message id (e.g. WhatsApp wamid). Kept narrow
// so the worker can be unit-tested without a real adapter.
type CampaignSender interface {
	SendTemplateAndGetID(
		ctx context.Context,
		channel *domain.Channel,
		to string,
		templateName, languageCode string,
		components []interface{},
	) (string, error)
}

// CampaignWorkerConfig tunes the worker's pacing.
type CampaignWorkerConfig struct {
	// PollInterval is how often the worker checks for due campaigns.
	PollInterval time.Duration
	// SendDelay is the gap between consecutive sends within a campaign,
	// keeping us under the Meta rate limit (~80/s per WABA at low tier).
	// 50ms = 20 msg/s — safe starting point. Raise per WABA quality rating.
	SendDelay time.Duration
	// BatchSize is how many recipients are processed per cycle.
	BatchSize int32
	// MaxConcurrentCampaigns caps parallel campaigns. 1 keeps things simple
	// and predictable; the worker will still process every due campaign,
	// just sequentially.
	MaxConcurrentCampaigns int
}

func DefaultCampaignWorkerConfig() CampaignWorkerConfig {
	return CampaignWorkerConfig{
		PollInterval:           5 * time.Second,
		SendDelay:              50 * time.Millisecond,
		BatchSize:              200,
		MaxConcurrentCampaigns: 1,
	}
}

// CampaignWorker drains scheduled broadcast campaigns asynchronously.
//
// Lifecycle:
//   - On every PollInterval tick, fetch up to MaxConcurrentCampaigns
//     campaigns whose status='scheduled' and (scheduled_at IS NULL OR <= NOW())
//   - For each: flip to 'sending', drain its pending recipients in BatchSize
//     chunks, send via the WhatsApp adapter, mark each recipient sent/failed
//     and increment counters
//   - When the recipient list is fully drained, flip to 'done' (or 'failed'
//     if every recipient failed)
//
// Crash safety:
//   - Idempotent on resumption: status='sending' campaigns are picked up by
//     the same Drain logic; pending recipients are processed in order.
//   - We never lose a recipient — the only state that matters is per-
//     recipient (pending/sent/failed) and per-campaign counters are derived.
//
// Concurrency:
//   - Single goroutine per worker process. If you run multiple replicas you
//     need to add row-level locking on campaigns (SELECT ... FOR UPDATE
//     SKIP LOCKED). Today we run one API replica so this is OK.
type CampaignWorker struct {
	repo        domain.CampaignRepository
	channelRepo domain.ChannelRepository
	// Optional — when set, the worker re-checks the broadcast budget at
	// dispatch time (defense against plan downgrades between the pre-flight
	// check at campaign create and the actual send).
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	usageRepo   domain.UsageRepository
	sender      CampaignSender
	cfg         CampaignWorkerConfig
	running     atomic.Bool
}

func NewCampaignWorker(
	repo domain.CampaignRepository,
	channelRepo domain.ChannelRepository,
	sender CampaignSender,
	cfg CampaignWorkerConfig,
) *CampaignWorker {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 200
	}
	if cfg.MaxConcurrentCampaigns <= 0 {
		cfg.MaxConcurrentCampaigns = 1
	}
	return &CampaignWorker{
		repo:        repo,
		channelRepo: channelRepo,
		sender:      sender,
		cfg:         cfg,
	}
}

// WithBudgetCheck wires the repositories needed for the dispatch-time
// defensive budget re-check. Without this call the worker skips the check
// (useful in tests where the pre-flight at create time is enough).
func (w *CampaignWorker) WithBudgetCheck(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
) *CampaignWorker {
	w.accountRepo = accountRepo
	w.planRepo = planRepo
	w.usageRepo = usageRepo
	return w
}

// Start runs the worker loop until ctx is cancelled. Safe to call once;
// subsequent calls are no-ops to prevent accidental double-start in main.
func (w *CampaignWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("campaign worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)

	slog.Info("campaign worker started",
		"poll_interval", w.cfg.PollInterval,
		"send_delay", w.cfg.SendDelay,
		"batch_size", w.cfg.BatchSize,
	)

	// Run one tick immediately on startup so deploys process recently
	// scheduled campaigns without waiting for the first tick.
	w.tick(ctx)

	t := time.NewTicker(w.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("campaign worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// tick processes one cycle: pick up due campaigns and drain each.
func (w *CampaignWorker) tick(ctx context.Context) {
	due, err := w.repo.ListScheduledDue(ctx, int32(w.cfg.MaxConcurrentCampaigns))
	if err != nil {
		slog.Error("campaign worker: list scheduled due failed", "error", err)
		return
	}
	for _, c := range due {
		if ctx.Err() != nil {
			return
		}
		if err := w.processCampaign(ctx, c); err != nil {
			// processCampaign already logs detail; we just continue with the next one.
			slog.Error("campaign processing failed", "campaign_id", c.ID, "error", err)
		}
	}
}

// processCampaign drains pending recipients of a single campaign.
func (w *CampaignWorker) processCampaign(ctx context.Context, c *domain.Campaign) error {
	// 1. Resolve the channel (need its config + access token).
	channel, err := w.channelRepo.GetByID(ctx, c.ChannelID)
	if err != nil || channel == nil {
		_ = w.repo.UpdateStatus(ctx, c.ID, domain.CampaignFailed)
		return errors.New("campaign channel not found")
	}
	if channel.Type != "whatsapp" {
		_ = w.repo.UpdateStatus(ctx, c.ID, domain.CampaignFailed)
		return errors.New("campaign channel is not whatsapp")
	}
	if !channel.IsActive {
		// Channel was disabled after scheduling — fail the campaign so the
		// admin sees an explicit reason instead of silent stalling.
		_ = w.repo.UpdateStatus(ctx, c.ID, domain.CampaignFailed)
		return errors.New("campaign channel is disabled")
	}

	// 2. Defensive budget re-check. Protects against:
	//   (a) plan downgraded AFTER the campaign was scheduled;
	//   (b) another concurrent campaign consuming the budget between pre-
	//       flight and now (even though reservation is atomic on create,
	//       admin-level edits could zero it out);
	//   (c) campaigns that were in 'sending' when the process crashed —
	//       we must re-authorize dispatch rather than blindly resume.
	// If the check fails we mark the campaign as failed; partial sends
	// that already went out stay accounted for in the recipient rows.
	if w.accountRepo != nil && w.planRepo != nil && w.usageRepo != nil {
		if reason := w.checkBudget(ctx, c); reason != "" {
			slog.Warn("campaign worker: budget check failed at dispatch",
				"campaign_id", c.ID, "reason", reason)
			_ = w.repo.UpdateStatus(ctx, c.ID, domain.CampaignFailed)
			return errors.New("budget check failed: " + reason)
		}
	}

	// 3. Mark as sending (idempotent if already sending).
	if c.Status != domain.CampaignSending {
		if err := w.repo.UpdateStatus(ctx, c.ID, domain.CampaignSending); err != nil {
			return err
		}
	}

	// 3. Drain in batches until no pending remain.
	totalSent, totalFailed := 0, 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		batch, err := w.repo.ListPendingRecipients(ctx, c.ID, w.cfg.BatchSize)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, r := range batch {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			ok := w.sendOne(ctx, channel, c, r)
			if ok {
				totalSent++
			} else {
				totalFailed++
			}
			if w.cfg.SendDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(w.cfg.SendDelay):
				}
			}
		}
	}

	// 4. Final status: done if at least one sent, failed if all failed.
	finalStatus := domain.CampaignDone
	if totalSent == 0 && totalFailed > 0 {
		finalStatus = domain.CampaignFailed
	}
	if err := w.repo.UpdateStatus(ctx, c.ID, finalStatus); err != nil {
		slog.Error("campaign worker: failed to set terminal status",
			"campaign_id", c.ID, "status", finalStatus, "error", err)
	}
	slog.Info("campaign drained",
		"campaign_id", c.ID,
		"sent", totalSent,
		"failed", totalFailed,
		"final_status", finalStatus,
	)
	return nil
}

// checkBudget verifies the campaign's account still has the plan
// entitlement and monthly budget to finish dispatch. Returns "" when the
// campaign may proceed, or a short reason string when it must abort.
//
// The check is intentionally lenient: we count recipients still PENDING
// (the work remaining), not the campaign's total, because partial sends
// that already fired at the previous plan should not be "refunded" from
// the new budget.
func (w *CampaignWorker) checkBudget(ctx context.Context, c *domain.Campaign) string {
	acc, err := w.accountRepo.GetByID(ctx, c.AccountID)
	if err != nil {
		// Transient DB failure — don't fail the campaign for it; next tick
		// will retry. Returning empty lets processing continue.
		return ""
	}
	var planID string
	if acc != nil && acc.PlanID != nil {
		planID = *acc.PlanID
	}

	var plan *domain.Plan
	if planID != "" {
		plan, _ = w.planRepo.GetByID(ctx, planID)
	}
	if plan == nil {
		plan = domain.TrialPlan()
	}

	// Per-account overrides (Partner / Enterprise contracts) trump the
	// plan-row default — without the accessor a contract account with a
	// 0 plan-row cap would be wrongly rejected here.
	broadcastCap := 0
	if acc != nil {
		broadcastCap = acc.EffectiveMaxBroadcastMessagesPerMonth(plan)
	} else {
		broadcastCap = plan.MaxBroadcastMessagesPerMonth
	}
	if broadcastCap <= 0 {
		return "plan does not allow broadcasts"
	}

	// Count the work that still has to fire.
	pending, err := w.repo.ListPendingRecipients(ctx, c.ID, int32(c.TotalRecipients+1))
	if err != nil || len(pending) == 0 {
		return ""
	}
	usage, err := w.usageRepo.GetByAccountID(ctx, c.AccountID)
	if err != nil {
		return ""
	}
	used := 0
	if usage != nil && usage.BroadcastMessagesUsed != nil {
		used = int(*usage.BroadcastMessagesUsed)
	}
	// The pre-flight at CreateCampaign already reserved usage for the whole
	// campaign. Subtract the pending remainder from `used` to avoid double-
	// counting (the reserved-but-not-yet-sent portion is already in `used`).
	effectivelyUsed := used - len(pending)
	if effectivelyUsed < 0 {
		effectivelyUsed = 0
	}
	if effectivelyUsed+len(pending) > broadcastCap {
		return "monthly budget exceeded after replanning"
	}
	return ""
}

// sendOne sends one template message for one recipient and updates state.
// Returns true on success, false on failure. Does not propagate errors —
// per-recipient failures are accounting events, not fatal worker errors.
func (w *CampaignWorker) sendOne(
	ctx context.Context,
	channel *domain.Channel,
	camp *domain.Campaign,
	rec *domain.CampaignRecipient,
) bool {
	vars := mergeVars(camp.TemplateVars, rec.Vars)
	components := buildBodyComponents(vars)

	wamid, err := w.sender.SendTemplateAndGetID(
		ctx, channel, rec.Recipient, camp.TemplateName, camp.TemplateLang, components,
	)
	if err != nil {
		_ = w.repo.MarkRecipientFailed(ctx, rec.ID, truncate(err.Error(), 500))
		_ = w.repo.IncrementCounter(ctx, camp.ID, domain.RecipientFailed)
		return false
	}
	_ = w.repo.MarkRecipientSent(ctx, rec.ID, wamid)
	_ = w.repo.IncrementCounter(ctx, camp.ID, domain.RecipientSent)
	return true
}

// mergeVars overlays per-recipient overrides on top of campaign-level vars
// (positional). The longer slice wins on length; recipient values override
// when both have a value at the same index.
func mergeVars(campaign, recipient []string) []string {
	if len(recipient) == 0 {
		return campaign
	}
	maxLen := len(campaign)
	if len(recipient) > maxLen {
		maxLen = len(recipient)
	}
	out := make([]string, maxLen)
	for i := 0; i < maxLen; i++ {
		if i < len(recipient) && recipient[i] != "" {
			out[i] = recipient[i]
		} else if i < len(campaign) {
			out[i] = campaign[i]
		}
	}
	return out
}

// buildBodyComponents converts positional vars into the Meta template
// "components" payload shape. Returns nil when there are no vars (template
// has no parameters).
func buildBodyComponents(vars []string) []interface{} {
	if len(vars) == 0 {
		return nil
	}
	params := make([]map[string]string, len(vars))
	for i, v := range vars {
		params[i] = map[string]string{"type": "text", "text": v}
	}
	return []interface{}{
		map[string]interface{}{
			"type":       "body",
			"parameters": params,
		},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Ensure the WhatsAppAdapter satisfies CampaignSender at compile time.
// This keeps wiring honest if the adapter signature drifts.
var _ CampaignSender = (*waadapter.WhatsAppAdapter)(nil)
