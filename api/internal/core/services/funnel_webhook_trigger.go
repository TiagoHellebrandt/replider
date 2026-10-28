package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// FunnelWebhookDispatcher is the minimal dispatch surface the funnel trigger
// needs (satisfied by *OutboundWebhookDispatcher). Keeping it an interface lets
// tests assert the dispatched event/payload without the full HTTP machinery.
type FunnelWebhookDispatcher interface {
	EmitToEndpoint(rateKey, accountID, url, secret, event string, payload any)
}

// SetFunnelWebhookDispatcher wires the optional dispatcher used to fire a
// funnel's webhook when a lead qualifies or signals buy intent (US-016). nil
// leaves EvaluateFunnelTriggers a no-op. main.go injects the shared
// OutboundWebhookDispatcher.
func (r *AgentRunner) SetFunnelWebhookDispatcher(d FunnelWebhookDispatcher) {
	r.funnelWebhookDispatcher = d
}

// EvaluateFunnelTriggers checks the funnel's trigger conditions against the
// lead's post-update state and dispatches the funnel webhook for any that fire.
// Called once per AI turn, after RecordLeadData / FlagBuyIntent:
//
//   - lead.buy_intent: when buy intent was freshly flagged this turn
//     (buyIntentJustFlagged) and the funnel opted into TriggerOnBuyIntent.
//   - lead.qualified: when every required field now has a value and the funnel
//     opted into TriggerOnRequiredComplete. Idempotent — MarkQualified stamps
//     qualified_at exactly once, so the event fires once no matter how many
//     further turns arrive after the gate completes.
//
// No-op when there's no lead/funnel binding, no dispatcher wired, or the funnel
// has no webhook URL. Best-effort: a persistence error logs and skips dispatch
// rather than breaking the reply.
func (r *AgentRunner) EvaluateFunnelTriggers(ctx context.Context, leadCtx *LeadDataContext, contactName, contactPhone string, buyIntentJustFlagged bool) {
	if leadCtx == nil || leadCtx.Lead == nil || leadCtx.Funnel == nil || r.funnelWebhookDispatcher == nil || r.leadRepo == nil {
		return
	}
	funnel := leadCtx.Funnel
	lead := leadCtx.Lead
	if strings.TrimSpace(funnel.WebhookURL) == "" {
		return
	}

	// Buy intent: fire once, the turn intent was freshly flagged.
	if buyIntentJustFlagged && funnel.TriggerOnBuyIntent {
		r.dispatchFunnelEvent(funnel, lead, contactName, contactPhone, EventLeadBuyIntent)
	}

	// Qualified: all required fields collected. The qualified_at guard +
	// MarkQualified's fresh-stamp return make this exactly-once.
	if funnel.TriggerOnRequiredComplete && lead.QualifiedAt == nil && requiredFieldsComplete(leadCtx.Fields, lead) {
		fired, err := r.leadRepo.MarkQualified(ctx, lead.ID)
		if err != nil {
			slog.Error("funnel trigger: mark qualified failed", "lead_id", lead.ID, "error", err)
			return
		}
		if !fired {
			return
		}
		// Keep the in-memory lead consistent for any later step this turn.
		now := time.Now()
		lead.QualifiedAt = &now
		lead.Status = domain.LeadStatusQualified
		r.dispatchFunnelEvent(funnel, lead, contactName, contactPhone, EventLeadQualified)
	}
}

// requiredFieldsComplete reports whether every required field in the schema has
// a non-empty collected value. Returns false when the schema declares no
// required field — an empty gate must never auto-qualify a lead.
func requiredFieldsComplete(fields []domain.MetadataFieldDef, lead *domain.Lead) bool {
	collected := decodeCollected(lead)
	hasRequired := false
	for _, f := range fields {
		if !f.Required {
			continue
		}
		hasRequired = true
		cf, ok := collected[f.Key]
		if !ok || strings.TrimSpace(cf.Value) == "" {
			return false
		}
	}
	return hasRequired
}

// dispatchFunnelEvent builds the structured funnel webhook payload and hands it
// to the dispatcher. The payload carries the funnel/lead ids, contact identity,
// lead status and the full collected metadata so the consumer (CRM/automation)
// has everything it needs without a callback. event/timestamp are duplicated
// inside data for consumers that read only the data object.
func (r *AgentRunner) dispatchFunnelEvent(funnel *domain.SalesFunnel, lead *domain.Lead, contactName, contactPhone, event string) {
	payload := map[string]any{
		"funnel_id": funnel.ID,
		"lead_id":   lead.ID,
		"status":    string(lead.Status),
		"contact": map[string]any{
			"name":  contactName,
			"phone": contactPhone,
		},
		"collected_metadata": decodeCollected(lead),
		"event":              event,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
	}
	r.funnelWebhookDispatcher.EmitToEndpoint(
		"funnel:"+funnel.ID, lead.AccountID, funnel.WebhookURL, funnel.WebhookSecret, event, payload,
	)
}
