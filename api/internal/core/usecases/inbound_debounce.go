package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Inbound debounce, the pipeline half (§6.15, K1).
//
// `deferInbound` is one decision at the top of ProcessAI, and `RunPending` is
// what the worker calls when the wait is over. Everything else about the burst —
// how long, how many, when it stops being deferrable — is in the domain and the
// repository.

// deferInbound queues the message and reports whether the reply should wait.
//
// Returns false — answer now — in every case where waiting would be worse than
// spending a call:
//
//   - The queue is not wired (a deployment that has not enabled the feature).
//   - The account set the window to 0.
//   - The message reads as buy intent. Speed is the product.
//   - The queue write failed. A debounce that swallows a customer's question is
//     a far worse bug than one extra model call, so a broken queue degrades to
//     the old behaviour rather than to silence.
func (uc *InboundWebhookUseCase) deferInbound(
	ctx context.Context, partial *InboundPartialResult, content string,
) bool {
	if uc.debounce == nil || partial.ConversationID == "" || partial.AgentID == nil {
		return false
	}

	delay := domain.DebounceDelay(content, uc.debounceConfig(ctx, partial.AccountID))
	if delay <= 0 {
		// Immediate. Anything already queued for this conversation is now
		// covered by the reply about to be generated, so the row goes — leaving
		// it would fire a second answer minutes later to a question already
		// answered.
		if err := uc.debounce.Delete(ctx, partial.ConversationID); err != nil {
			slog.WarnContext(ctx, "inbound debounce: could not clear queue before immediate reply",
				"conversation_id", partial.ConversationID, "error", err)
		}
		return false
	}

	now := time.Now()
	pending, err := uc.debounce.Enqueue(ctx, &domain.PendingInbound{
		ConversationID: partial.ConversationID,
		AccountID:      partial.AccountID,
		ChannelID:      partial.ChannelID,
		AgentID:        *partial.AgentID,
		RespondAt:      now.Add(delay),
		FirstQueuedAt:  now,
		Content:        content,
	})
	if err != nil {
		slog.ErrorContext(ctx, "inbound debounce: enqueue failed, answering immediately",
			"conversation_id", partial.ConversationID, "error", err)
		return false
	}
	if pending == nil {
		// A run is already claimed and in flight. Answering now would produce
		// two replies to overlapping content; the message is persisted and the
		// in-flight run reads history from the database, so it is seen.
		slog.DebugContext(ctx, "inbound debounce: run already in flight, message folded into it",
			"conversation_id", partial.ConversationID)
		return true
	}

	slog.DebugContext(ctx, "inbound debounce: waiting for the customer to finish",
		"conversation_id", partial.ConversationID,
		"respond_at", pending.RespondAt,
		"messages", pending.MessageCount)
	if uc.debounceNotifier != nil {
		// Tells the dashboard when the AI will answer, so the attendant sees
		// "IA responde em ~40s" instead of a bot that looks stuck.
		uc.debounceNotifier.NotifyDebounce(partial.AccountID, partial.ConversationID,
			pending.RespondAt, pending.MessageCount)
	}
	return true
}

// debounceConfig reads the account's window.
//
// A lookup failure falls back to the PLATFORM DEFAULT rather than to zero: the
// point of the feature is not spending three calls on one question, and a
// database hiccup should not turn it off silently for whoever is affected.
func (uc *InboundWebhookUseCase) debounceConfig(ctx context.Context, accountID string) domain.DebounceConfig {
	if uc.accountRepo == nil {
		return domain.DebounceConfig{}
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		slog.WarnContext(ctx, "inbound debounce: account lookup failed, using the default window",
			"account_id", accountID, "error", err)
		return domain.DebounceConfig{}
	}
	return domain.DebounceConfig{Seconds: account.InboundDebounceSeconds}
}

// DebounceNotifier tells the dashboard about a pending run.
//
// A narrow port rather than the hub itself: the use case has no business knowing
// about WebSocket envelopes, and the notification is best-effort decoration on a
// pipeline that must work without it.
type DebounceNotifier interface {
	NotifyDebounce(accountID, conversationID string, respondAt time.Time, messageCount int)
}

// SetDebounceNotifier wires the dashboard banner (K1). Optional.
func (uc *InboundWebhookUseCase) SetDebounceNotifier(n DebounceNotifier) {
	uc.debounceNotifier = n
}

// RunPending answers a burst whose wait is over.
//
// The worker owns the row: this deletes it BEFORE generating, not after. If it
// deleted afterwards, a run that crashed mid-generation would leave a claimed row
// that the stale sweep re-queues — and the customer would get the same reply
// twice. Deleting first means a crash loses the reply instead, which the
// conversation still shows as an unanswered message an attendant can act on. A
// missing reply is visible; a duplicate one is a bot talking to itself.
func (uc *InboundWebhookUseCase) RunPending(ctx context.Context, pending *domain.PendingInbound) {
	if uc.debounce == nil || pending == nil {
		return
	}
	if err := uc.debounce.Delete(ctx, pending.ConversationID); err != nil {
		// Could not take ownership: leave the claim for the stale sweep rather
		// than generating a reply whose row might fire again.
		slog.ErrorContext(ctx, "inbound debounce: could not claim ownership of the run",
			"conversation_id", pending.ConversationID, "error", err)
		return
	}

	// Re-read the conversation so a takeover, a pause or a deleted thread during
	// the wait is respected. The window is up to three minutes — plenty of time
	// for an attendant to have picked the conversation up, and answering over
	// them is exactly what the takeover feature exists to prevent.
	partial, ok := uc.pendingPartial(ctx, pending)
	if !ok {
		return
	}

	if uc.debounceNotifier != nil {
		// Clears the banner: the wait is over either way.
		uc.debounceNotifier.NotifyDebounce(pending.AccountID, pending.ConversationID,
			time.Time{}, 0)
	}

	// Straight into the shared pipeline via processAIAfterDefer — NOT ProcessAI.
	// The debounce decision is already made (this run IS the wait being over);
	// calling ProcessAI would hit deferInbound again and re-enqueue the row we
	// just deleted, looping forever. Everything downstream — the serializer, the
	// plan limits, the business-hours gate, the takeover cancellation — behaves
	// identically whether the reply was immediate or waited.
	result := uc.processAIAfterDefer(ctx, partial, pending.Content)
	if uc.debounceDispatcher != nil && result != nil {
		uc.debounceDispatcher.DispatchDebounced(ctx, partial, result)
	}
}

// pendingPartial rebuilds the pipeline input and re-checks what may have changed
// during the wait.
func (uc *InboundWebhookUseCase) pendingPartial(
	ctx context.Context, pending *domain.PendingInbound,
) (*InboundPartialResult, bool) {
	conv, err := uc.convRepo.GetByID(ctx, pending.ConversationID, pending.AccountID)
	if err != nil || conv == nil {
		slog.InfoContext(ctx, "inbound debounce: conversation gone during the wait, dropping",
			"conversation_id", pending.ConversationID, "error", err)
		return nil, false
	}
	if conv.IsBotPaused {
		slog.InfoContext(ctx, "inbound debounce: a human took over during the wait, dropping the AI reply",
			"conversation_id", pending.ConversationID)
		return nil, false
	}

	agentID := pending.AgentID
	return &InboundPartialResult{
		AccountID:      pending.AccountID,
		ConversationID: pending.ConversationID,
		ChannelID:      pending.ChannelID,
		AgentID:        &agentID,
		ContactID:      conv.ContactID,
	}, true
}

// DebouncedDispatcher sends the reply a deferred run produced.
//
// The webhook handlers own the send (they hold the channel adapters), so the
// worker hands the result back to them through this port instead of the use case
// growing a second copy of the send switch.
type DebouncedDispatcher interface {
	DispatchDebounced(ctx context.Context, partial *InboundPartialResult, result *ProcessAIResult)
}

// SetDebouncedDispatcher wires the send path for deferred replies (K1).
//
// Without it a deferred reply is generated and persisted but never delivered,
// which is why main.go always wires it — the setter exists so the pipeline's
// tests do not need a channel adapter.
func (uc *InboundWebhookUseCase) SetDebouncedDispatcher(d DebouncedDispatcher) {
	uc.debounceDispatcher = d
}
