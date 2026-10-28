package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

type ProcessMessageUseCase struct {
	usageRepo domain.UsageRepository
	// meter appends this reply's spend to the AI cost ledger. Optional:
	// nil keeps the pipeline working, it just costs nothing on paper.
	meter *AIMeter
	// spend is the budget-only gate (customer ceiling / platform default),
	// derived from the meter's ledger in SetAIMeter. Nil allows.
	spend        *spendGate
	planRepo     domain.PlanRepository
	agentRepo    domain.AgentRepository
	accountRepo  domain.AccountRepository
	convRepo     domain.ConversationRepository
	msgRepo      domain.MessageRepository
	calendarRepo domain.CalendarRepository
	userRepo     domain.UserRepository

	aiFactory    domain.AIFactory
	agentRunner  *services.AgentRunner
	calendar     domain.CalendarAdapter
	emailService email.EmailService
	appBaseURL   string
	usageGuard   services.UsageChecker
}

// NewProcessMessageUseCase wires the inbound-AI pipeline. usageGuard is a
// REQUIRED dependency (US-021): it gates every AI run against the account's
// active-subscription, conversation, and token limits. It used to be an
// optional setter, which meant a wiring mistake silently disabled all
// quota enforcement at runtime — uncapped spend with no signal. Taking it
// as a constructor arg and panicking on nil turns that into a loud
// fail-fast at startup instead.
func NewProcessMessageUseCase(
	usageRepo domain.UsageRepository,
	planRepo domain.PlanRepository,
	agentRepo domain.AgentRepository,
	accountRepo domain.AccountRepository,
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	calendarRepo domain.CalendarRepository,
	userRepo domain.UserRepository,
	aiFactory domain.AIFactory,
	agentRunner *services.AgentRunner,
	calendar domain.CalendarAdapter,
	emailService email.EmailService,
	appBaseURL string,
	usageGuard services.UsageChecker,
) *ProcessMessageUseCase {
	if usageGuard == nil {
		panic("ProcessMessageUseCase: usageGuard is required (nil would disable all quota enforcement)")
	}
	return &ProcessMessageUseCase{
		usageRepo:    usageRepo,
		planRepo:     planRepo,
		agentRepo:    agentRepo,
		accountRepo:  accountRepo,
		convRepo:     convRepo,
		msgRepo:      msgRepo,
		calendarRepo: calendarRepo,
		userRepo:     userRepo,
		aiFactory:    aiFactory,
		agentRunner:  agentRunner,
		calendar:     calendar,
		emailService: emailService,
		appBaseURL:   appBaseURL,
		usageGuard:   usageGuard,
	}
}

type ProcessMessageDTO struct {
	AccountID      string
	AgentID        string
	ConversationID string
	Message        string
}

// UsageAlertInfo is returned after streaming completes with usage threshold info.
type UsageAlertInfo struct {
	PercentUsed    float64 `json:"percent_used"`
	TokensUsed     int64   `json:"tokens_used"`
	EffectiveLimit int64   `json:"effective_limit"`
	// Per-call token breakdown — stamped on the outbound message
	// metadata so the A/B results aggregation can attribute spend
	// back to a specific variant. Both default to 0 when the provider
	// didn't report usage.
	InputTokensThisCall  int64 `json:"-"`
	OutputTokensThisCall int64 `json:"-"`
}

// ProcessMessageResult wraps the streaming channel and a post-stream usage alert channel.
type ProcessMessageResult struct {
	Chunks     <-chan string
	UsageAlert <-chan *UsageAlertInfo // receives one value (possibly nil) after stream ends
	// Variant + ExperimentID propagate the A/B routing decision from
	// AgentRunner up to the caller (inbound_webhook) so the saved
	// outbound message can carry attribution metadata. Both nil when
	// no running experiment buckets this conversation.
	Variant      *string
	ExperimentID *string
}

// Execute processes an incoming message and returns a streaming result with the AI response
// and a post-stream usage alert channel.
func (uc *ProcessMessageUseCase) Execute(ctx context.Context, dto ProcessMessageDTO) (*ProcessMessageResult, error) {
	// Bound the whole pipeline with a 30s deadline derived from the
	// caller's context (US-032). Every downstream call below already
	// threads this ctx — the DB lookups, the AgentRunner prompt build,
	// and crucially the AI provider's GenerateResponse/GenerateStream,
	// which is the realistic place to hang. Deriving from the incoming
	// ctx (rather than context.Background) means a client disconnect or
	// a parent cancellation propagates down and kills the in-flight AI
	// call instead of orphaning a goroutine that keeps burning provider
	// tokens.
	//
	// Ownership of cancel() is conditional: on the SUCCESS path Execute
	// returns a streaming channel that the caller drains AFTER we return,
	// so cancelling on return would truncate the stream — instead the
	// forwarding goroutine owns cancel() and releases it when streaming
	// completes (or the 30s deadline fires). On every early-return error
	// path the stream goroutine never starts, so the deferred guard
	// cancels to avoid leaking the timer.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	streamStarted := false
	defer func() {
		if !streamStarted {
			cancel()
		}
	}()

	// 1. Fetch Agent
	agent, err := uc.agentRepo.GetByID(ctx, dto.AgentID)
	if err != nil || agent == nil {
		return nil, domain.ErrAgentNotFound
	}
	if agent.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}

	// 3. Check conversation bot-pause state (if ConversationID is provided)
	var conv *domain.Conversation
	if dto.ConversationID != "" {
		conv, err = uc.convRepo.GetByID(ctx, dto.ConversationID, dto.AccountID)
		if err != nil || conv == nil {
			return nil, domain.ErrConversationNotFound
		}
		if conv.IsBotPaused {
			return nil, domain.ErrBotPaused
		}
		_ = uc.convRepo.UpdateLastCustomerMessageAt(ctx, dto.ConversationID, dto.AccountID)
	}

	// 4. Fetch Account & Plan — check grace period
	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return nil, domain.ErrUnauthorized
	}
	// Owner-panel suspension trumps every other gate. Same fallback as
	// ErrAccountSuspended (the past-due variant) — message stamped on
	// the dashboard, nothing sent to the channel.
	if account.IsSuspended() {
		return nil, domain.ErrAccountSuspended
	}
	// TrialBypass short-circuits the past_due-grace gate so owner +
	// Meta App Reviewer accounts keep receiving AI replies without a
	// live Stripe subscription. Plan-limit checks below still apply
	// (per the comment on Account.TrialBypass) — bumping those needs
	// a separate decision.
	if !account.TrialBypass {
		if err := uc.usageGuard.CheckAccountActive(account); err != nil {
			return nil, err
		}
		// Trial expiry used to be enforced ONLY by the HTTP middleware, so an
		// expired trial kept getting AI replies through the webhook path.
		if err := accountMayUseAI(account, time.Now()); err != nil {
			return nil, err
		}
	}
	var plan *domain.Plan
	if account.PlanID == nil {
		plan = domain.TrialPlan()
	} else {
		plan, err = uc.planRepo.GetByID(ctx, *account.PlanID)
		if err != nil || plan == nil {
			// Don't silently downgrade a plan-holding account to trial
			// limits; the token-cap logic below dereferences plan, so a
			// nil plan would panic. Fail loud with 500 (US-043).
			slog.Error("plan fetch failed for account with PlanID; refusing to run AI",
				"account_id", dto.AccountID, "plan_id", *account.PlanID, "error", err)
			return nil, domain.ErrPlanMissing
		}
	}

	// 5. Fetch usage and enforce plan limits (conversations + tokens)
	usage, err := uc.usageRepo.GetByAccountID(ctx, dto.AccountID)
	if err != nil || usage == nil {
		return nil, fmt.Errorf("could not fetch usage details")
	}
	if err := uc.usageGuard.Check(account, plan, usage, "conversations"); err != nil {
		return nil, err
	}
	if err := uc.usageGuard.Check(account, plan, usage, "tokens"); err != nil {
		return nil, err
	}
	// Per-account daily AI token cap (US-025). Bounds a single runaway day
	// (looping integration / abuse) to a fraction of the monthly allowance.
	// TrialBypass accounts are exempt, consistent with the active-sub gate.
	if !account.TrialBypass {
		dailyCap := plan.EffectiveMaxDailyAITokens()
		if dailyUsed := usage.EffectiveDailyTokens(time.Now()); dailyUsed >= dailyCap {
			// ERROR level so it surfaces in the ops alert pipeline (the
			// admin-facing alert per AC). An email per message would spam
			// without a per-day "alerted" flag, so we alert via logs.
			slog.Error("daily AI token cap reached, skipping AI reply",
				"alert", "daily_token_cap", "account_id", dto.AccountID,
				"plan", plan.Name, "daily_used", dailyUsed, "cap", dailyCap)
			return nil, domain.ErrDailyTokenLimitExceeded
		}
	}
	aiTokensUsed := int64(0)
	if usage.AITokensUsed != nil {
		aiTokensUsed = *usage.AITokensUsed
	}
	addonTokens := int64(0)
	if usage.AddonTokens != nil {
		addonTokens = *usage.AddonTokens
	}
	// EffectiveMax, not the raw column: a contract plan stores 0 and
	// relies on the account's override, so reading the raw value would
	// hard-block AI for exactly the customers paying the most.
	effectiveLimit := account.EffectiveMaxAITokensPerMonth(plan) + addonTokens

	// 6. Build system prompt using AgentRunner (single source of truth).
	// Detect reply language server-side so the prompt carries an explicit
	// REPLY LANGUAGE tag — the old model-only detection produced English
	// fallbacks against Portuguese customers.
	replyLanguage := services.DetectReplyLanguage(dto.Message)
	// Conversation-aware build: when an A/B experiment is running on
	// this agent, the runner buckets the conversation into a sticky
	// variant and returns the variant's body in place of
	// agent.system_prompt. PromptResult.Variant / ExperimentID are
	// stamped onto the outbound message metadata below for attribution.
	// Channel + contact let the runner resolve a bound sales funnel + lead so
	// it can inject the record_lead_data tool (US-011). Empty for preview chat.
	channelID, contactID := "", ""
	if conv != nil {
		channelID, contactID = conv.ChannelID, conv.ContactID
	}
	promptResult, err := uc.agentRunner.BuildSystemPromptForConversation(
		ctx, agent, dto.Message, account.Name, replyLanguage, dto.ConversationID, channelID, contactID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build system prompt: %w", err)
	}

	// 7. Get AI provider — BYOK-aware. Contract plans (Partner /
	// Enterprise) supply their own provider key; everyone else falls
	// through to the platform's env key.
	// The customer's monthly ceiling (or the platform default) applies to
	// the conversational reply too — it is the highest-volume AI call.
	if err := uc.spend.ensureAnyFor(ctx, account); err != nil {
		return nil, err
	}
	aiPrv, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		return nil, err
	}

	// 8. Prepare history — replay only the last N turns so the prompt size
	// (and token cost) stays bounded regardless of how long the
	// conversation runs (US-024). N is plan-configurable; older turns are
	// folded into conv.Summary and injected as a single context line by
	// buildBoundedHistory so context isn't lost.
	maxHistory := plan.EffectiveMaxHistoryMessages()
	var history []domain.ChatMessage
	var windowFull bool
	if dto.ConversationID != "" {
		prevMsgs, listErr := uc.msgRepo.ListByConversationID(ctx, dto.ConversationID, int32(maxHistory), 0) //nolint:gosec // G115: maxHistory is a small plan-bounded int
		if listErr == nil {
			// A full window means older turns exist beyond it — flag so the
			// post-stream step can refresh the rolling summary.
			windowFull = len(prevMsgs) >= maxHistory
			var summary *string
			if conv != nil {
				summary = conv.Summary
			}
			history = buildBoundedHistory(prevMsgs, dto.Message, summary, maxHistory)
		}
	}
	// Defensive: if there was no conversation context (preview chat) or the
	// list failed, at least send the current message.
	if len(history) == 0 {
		history = []domain.ChatMessage{{Role: "user", Content: dto.Message}}
	}

	// 9. Booking tool call handling (check_availability before streaming)
	if len(promptResult.Tools) > 0 {
		resp, err := aiPrv.GenerateResponse(ctx, promptResult.SystemPrompt, history, promptResult.Tools)
		if err == nil && len(resp.ToolCalls) > 0 {
			// US-011: invisibly persist any lead data the model captured this
			// turn. RecordLeadData filters resp.ToolCalls to record_lead_data,
			// validates each against the funnel schema, de-dups, and writes new
			// values to leads.collected_metadata. No-op without a lead binding.
			if promptResult.LeadContext != nil {
				if n := uc.agentRunner.RecordLeadData(ctx, promptResult.LeadContext, resp.ToolCalls); n > 0 {
					slog.Debug("recorded lead data", "conversation_id", dto.ConversationID, "fields", n)
				}
				// US-012: silently stamp leads.buy_intent_at the first time the
				// model flags purchase intent, arming the funnel trigger (US-016).
				buyIntentJustFlagged := uc.agentRunner.FlagBuyIntent(ctx, promptResult.LeadContext, resp.ToolCalls)
				if buyIntentJustFlagged {
					slog.Debug("flagged buy intent", "conversation_id", dto.ConversationID, "lead_id", promptResult.LeadContext.Lead.ID)
				}
				// US-016: evaluate the funnel's webhook triggers against the lead's
				// freshly-updated state. Fires lead.qualified (required fields all
				// collected, once) and/or lead.buy_intent to the funnel's webhook.
				contactName := ""
				if conv != nil && conv.ContactName != nil {
					contactName = *conv.ContactName
				}
				uc.agentRunner.EvaluateFunnelTriggers(ctx, promptResult.LeadContext, contactName, contactID, buyIntentJustFlagged)
			}
			// Cap how many tool calls this single turn may process (US-023).
			// Each check_availability fans out to Google Calendar, so an
			// over-long list — from a buggy or adversarial model — would be
			// an unbounded burst of slow/paid external calls.
			processedToolCalls := 0
			// Process tool calls up to the cap.
			for _, tc := range resp.ToolCalls {
				// US-044: bail at each tool-call boundary if the context was
				// canceled — a human assuming the conversation mid-pipeline
				// must abort the reply before it fans out to slow/paid
				// external calls (Google Calendar) and before streaming.
				if err := ctx.Err(); err != nil {
					slog.Info("context canceled during tool-call processing; aborting reply",
						"agent_id", dto.AgentID, "conversation_id", dto.ConversationID, "error", err)
					return nil, err
				}
				if processedToolCalls >= services.MaxToolCallIterations {
					slog.Warn("tool-call iteration cap reached; dropping remaining tool calls",
						"agent_id", dto.AgentID, "cap", services.MaxToolCallIterations, "requested", len(resp.ToolCalls))
					history = append(history, domain.ChatMessage{Role: "user", Content: "System: " + services.ToolCallCapNotice})
					break
				}
				if tc.FunctionName != "check_availability" {
					continue
				}
				processedToolCalls++
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
					slog.Warn("failed to parse tool call arguments", "agent_id", dto.AgentID, "error", err)
					continue
				}
				dateStr, _ := args["date"].(string)
				requestedCalID, _ := args["calendar_id"].(string)
				pDate, parseErr := time.Parse("2006-01-02", dateStr)
				if parseErr != nil {
					slog.Warn("invalid date in tool call", "date", dateStr, "error", parseErr)
					history = append(history, domain.ChatMessage{Role: "model", Content: "Não consegui interpretar a data solicitada."})
					continue
				}

				// Fetch agent's calendars to get Google Calendar IDs
				calendars, calErr := uc.calendarRepo.ListByAgentID(ctx, dto.AgentID)
				if calErr != nil || len(calendars) == 0 {
					continue
				}
				var slotsText string

				// If a specific calendar was requested, filter to it
				if requestedCalID != "" {
					for _, cal := range calendars {
						if cal.ID == requestedCalID && cal.IsActive && cal.GoogleCalendarID != nil {
							if !calendarAvailableOn(cal, pDate) {
								break
							}
							slots, err := uc.calendar.ListFreeSlots(ctx, dto.AccountID, *cal.GoogleCalendarID, pDate)
							if err == nil {
								slotsText = formatSlots(cal.Name, slots, cal.SlotDurationMinutes)
							}
							break
						}
					}
				} else {
					// Check all active calendars
					for _, cal := range calendars {
						if !cal.IsActive || cal.GoogleCalendarID == nil || !calendarAvailableOn(cal, pDate) {
							continue
						}
						slots, err := uc.calendar.ListFreeSlots(ctx, dto.AccountID, *cal.GoogleCalendarID, pDate)
						if err != nil {
							continue
						}
						slotsText += formatSlots(cal.Name, slots, cal.SlotDurationMinutes)
					}
				}

				if slotsText == "" {
					slotsText = "Nenhum horário disponível para esta data."
				}
				history = append(history, domain.ChatMessage{Role: "model", Content: fmt.Sprintf("Verificando disponibilidade para %s...", dateStr)})
				history = append(history, domain.ChatMessage{Role: "user", Content: "System: " + slotsText})
			}
		}
	}

	// 10. Generate streaming response
	streamResult, err := aiPrv.GenerateStream(ctx, promptResult.SystemPrompt, history, promptResult.Tools)
	if err != nil {
		return nil, err
	}

	// 11. Wrap stream and update usage asynchronously, then check thresholds
	outStream := make(chan string)
	alertCh := make(chan *UsageAlertInfo, 1)

	// Capture warning flags before streaming starts
	alreadyWarned80 := usage.Warned80Percent
	alreadyWarned100 := usage.Warned100Percent
	planName := plan.Name
	accountID := dto.AccountID

	streamStarted = true
	go func() {
		// This goroutine owns the request ctx's cancel: it outlives
		// Execute's return (the caller drains outStream afterwards), so
		// releasing the 30s timer here — not on Execute return — keeps
		// the deadline valid for the whole stream without truncating it.
		defer cancel()
		defer close(outStream)
		defer close(alertCh)
		// Forward chunks while honoring ctx so a stalled provider
		// doesn't wedge this goroutine (and the caller draining
		// outStream) indefinitely.
	forwardLoop:
		for {
			select {
			case chunk, ok := <-streamResult.Chunks:
				if !ok {
					break forwardLoop
				}
				select {
				case outStream <- chunk:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
		promptTokens, completionTokens := streamResult.GetUsage()
		// math.Round (not bare int64 cast) on each multiplier so the
		// fractional part isn't truncated. With InputMultiplier=0.75 and
		// promptTokens=1001 the old `int64(float64*mult)` produced 750
		// instead of 751 — silently under-counting ~0.5% per call. Across
		// 100k messages a month, that's tens of thousands of free
		// tokens that bypass plan-limit enforcement and Stripe metering.
		normalizedCreditsUsed := int64(math.Round(float64(promptTokens)*plan.InputMultiplier)) +
			int64(math.Round(float64(completionTokens)*plan.OutputMultiplier))
		go func(credits, input, output int64, plan *domain.Plan) {
			// US-033 justified-Background: detached on purpose. Token
			// accounting must persist even if the client disconnected
			// mid-stream — binding this to the request ctx would lose
			// usage (and Stripe metering) on every cancelled stream.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// Token accounting only — the conversations_count column is
			// bumped at conversation-create time (SaveInbound), not per
			// AI turn. The legacy `+1` here made every reply count as a
			// new conversation, so a 3-turn thread showed up as "3/1000"
			// on the trial banner.
			if err := uc.usageRepo.AddUsage(ctx, accountID, credits, input, output, 0); err != nil {
				slog.Error("failed to update usage after AI response", "account_id", accountID, "credits", credits, "error", err)
			}
			// ...and into the COST ledger. AddUsage feeds plan limits, which
			// is a different question from what this account costs us. The
			// conversational reply is the highest-volume AI call in an inbox
			// product and it was absent from ai_costs entirely, so per-account
			// margin was computed without the biggest line item in it.
			uc.meter.Record(ctx, accountID, "agent_reply", plan.AIProvider, plan.AIModel,
				domain.AIUsage{PromptTokens: input, CompletionTokens: output})
		}(normalizedCreditsUsed, promptTokens, completionTokens, plan)

		// Check usage thresholds for alerts
		newTotal := aiTokensUsed + normalizedCreditsUsed
		if effectiveLimit > 0 {
			percentUsed := float64(newTotal) / float64(effectiveLimit) * 100
			if percentUsed >= 80 {
				alertCh <- &UsageAlertInfo{
					PercentUsed:          percentUsed,
					TokensUsed:           newTotal,
					EffectiveLimit:       effectiveLimit,
					InputTokensThisCall:  promptTokens,
					OutputTokensThisCall: completionTokens,
				}
			} else {
				// Always emit the per-call token breakdown so the
				// outbound message stamping path has counts available
				// even when the account isn't near a threshold.
				alertCh <- &UsageAlertInfo{
					InputTokensThisCall:  promptTokens,
					OutputTokensThisCall: completionTokens,
				}
			}

			// Send usage warning emails (once per billing cycle)
			upgradeURL := uc.appBaseURL + "/billing"
			if percentUsed >= 100 && !alreadyWarned100 {
				go uc.sendUsageLimitNotification(accountID, planName, upgradeURL)
			} else if percentUsed >= 80 && !alreadyWarned80 {
				go uc.sendUsageWarningNotification(accountID, int(percentUsed), planName, upgradeURL)
			}
		}
	}()

	// US-024: once the conversation has grown past the prompt window, fold
	// the aged-out turns into conv.summary so future turns keep context
	// without us replaying the whole history. Fire-and-forget and throttled
	// (see refreshSummaryAsync) so it never blocks the reply and costs at
	// most one LLM call per conversation per summaryRefreshInterval.
	if windowFull && conv != nil {
		go uc.refreshSummaryAsync(context.WithoutCancel(ctx), conv, plan, aiPrv, maxHistory)
	}

	return &ProcessMessageResult{
		Chunks:       outStream,
		UsageAlert:   alertCh,
		Variant:      promptResult.Variant,
		ExperimentID: promptResult.ExperimentID,
	}, nil
}

// calendarAvailableOn enforces the calendar's available_days (0=Sunday
// … 6=Saturday) before any Google lookup — a free Google agenda on a
// Saturday must not produce slots when the calendar is weekdays-only.
// An empty available_days list means "no day restriction" (legacy rows).
func calendarAvailableOn(cal *domain.Calendar, date time.Time) bool {
	if len(cal.AvailableDays) == 0 {
		return true
	}
	weekday := int32(date.Weekday())
	for _, d := range cal.AvailableDays {
		if d == weekday {
			return true
		}
	}
	return false
}

func formatSlots(calendarName string, slots []time.Time, durationMinutes int32) string {
	if len(slots) == 0 {
		return fmt.Sprintf("\n%s: Sem horários disponíveis.", calendarName)
	}
	text := fmt.Sprintf("\n%s (consultas de %d min): ", calendarName, durationMinutes)
	for _, s := range slots {
		text += s.Format("15:04") + " "
	}
	return text
}

// sendUsageWarningNotification finds the account admin and sends a usage warning email.
// Launched with `go` (fire-and-forget) — US-033 justified-Background: the
// email send must complete regardless of the originating request's
// lifetime, so it owns its own 15s deadline rather than the request ctx.
func (uc *ProcessMessageUseCase) sendUsageWarningNotification(accountID string, usagePercent int, planName string, upgradeURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Mark as warned first to avoid duplicate sends
	if err := uc.usageRepo.SetUsageWarned80(ctx, accountID); err != nil {
		slog.Error("failed to set usage warned 80%", "account_id", accountID, "error", err)
		return
	}

	adminUser, err := uc.findAccountAdmin(ctx, accountID)
	if err != nil || adminUser == nil {
		slog.Error("failed to find admin for usage warning email", "account_id", accountID, "error", err)
		return
	}

	locale := emailLocaleFor(ctx, uc.accountRepo, accountID)
	if err := uc.emailService.SendUsageWarningEmail(adminUser.Email, adminUser.Name, usagePercent, planName, upgradeURL, locale); err != nil {
		slog.Error("failed to send usage warning email", "email", adminUser.Email, "error", err)
	}
}

// sendUsageLimitNotification finds the account admin and sends a usage limit email.
// Launched with `go` (fire-and-forget) — US-033 justified-Background: same
// rationale as sendUsageWarningNotification, owns its own 15s deadline.
func (uc *ProcessMessageUseCase) sendUsageLimitNotification(accountID string, planName string, upgradeURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Mark as warned first to avoid duplicate sends
	if err := uc.usageRepo.SetUsageWarned100(ctx, accountID); err != nil {
		slog.Error("failed to set usage warned 100%", "account_id", accountID, "error", err)
		return
	}

	adminUser, err := uc.findAccountAdmin(ctx, accountID)
	if err != nil || adminUser == nil {
		slog.Error("failed to find admin for usage limit email", "account_id", accountID, "error", err)
		return
	}

	locale := emailLocaleFor(ctx, uc.accountRepo, accountID)
	if err := uc.emailService.SendUsageLimitEmail(adminUser.Email, adminUser.Name, planName, upgradeURL, locale); err != nil {
		slog.Error("failed to send usage limit email", "email", adminUser.Email, "error", err)
	}
}

// findAccountAdmin retrieves the admin user for the given account.
func (uc *ProcessMessageUseCase) findAccountAdmin(ctx context.Context, accountID string) (*domain.User, error) {
	return uc.userRepo.GetAdminByAccountID(ctx, accountID)
}

// SetAIMeter wires the cost ledger. Optional so the constructor signature
// and its call sites stay untouched; nil simply records nothing.
func (uc *ProcessMessageUseCase) SetAIMeter(m *AIMeter) {
	uc.meter = m
	uc.spend = budgetGateFromMeter(uc.accountRepo, m)
}
