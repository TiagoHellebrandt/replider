package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// RecordLeadDataToolName is the function name the model calls to invisibly
// persist a single declared lead field during the conversation (US-011).
const RecordLeadDataToolName = "record_lead_data"

// FlagBuyIntentToolName is the function name the model calls to silently flag
// that the lead has signalled purchase / closing intent (US-012). It carries no
// arguments and exposes nothing to the customer; the handler stamps
// leads.buy_intent_at the first time it fires.
const FlagBuyIntentToolName = "flag_buy_intent"

// LeadDataContext is the resolved binding between a conversation and a sales
// funnel + lead. When present, the AgentRunner injects the record_lead_data
// tool and the collection rules so the AI can silently capture the funnel's
// declared metadata fields as the customer volunteers them.
type LeadDataContext struct {
	// Funnel is the active sales funnel bound to the conversation's channel. It
	// carries the webhook URL/secret + trigger toggles the funnel webhook
	// evaluation (US-016) reads after metadata/buy-intent updates.
	Funnel *domain.SalesFunnel
	Lead   *domain.Lead
	Fields []domain.MetadataFieldDef
}

// CollectedField is the stored shape of one value the AI (or a human) captured
// for a funnel metadata field. Persisted as the value of leads.collected_metadata[key].
type CollectedField struct {
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CollectedSourceAI marks a field value captured automatically by the AI.
const CollectedSourceAI = "ai"

// CollectedSourceHuman marks a field value entered or corrected by a human
// attendant via the contact panel (US-014). Once a human sets a value the
// dashboard renders a "humano" origin badge instead of "IA".
const CollectedSourceHuman = "human"

// SetSalesCollection wires the optional sales funnel + lead repositories used
// to resolve a conversation's lead binding and persist invisibly-collected
// metadata (US-011). When either repo is nil, lead collection is a no-op: no
// tool is injected and RecordLeadData does nothing. main.go injects the
// Postgres-backed implementations; tests can leave them unset.
func (r *AgentRunner) SetSalesCollection(funnelRepo domain.SalesFunnelRepository, leadRepo domain.LeadRepository) {
	r.salesFunnelRepo = funnelRepo
	r.leadRepo = leadRepo
}

// resolveLeadContext returns the funnel+lead binding for a conversation, or
// nil when there's no active funnel on the channel, no lead for the contact,
// or the funnel declares no metadata fields. Conservative on errors: any
// lookup failure logs and falls back to nil so a sales-collection hiccup never
// breaks a normal reply.
func (r *AgentRunner) resolveLeadContext(ctx context.Context, channelID, contactID string) *LeadDataContext {
	if r.salesFunnelRepo == nil || r.leadRepo == nil || channelID == "" || contactID == "" {
		return nil
	}
	funnel, err := r.salesFunnelRepo.GetByChannelID(ctx, channelID)
	if err != nil {
		slog.Warn("lead collection: funnel lookup failed", "channel_id", channelID, "error", err)
		return nil
	}
	if funnel == nil {
		return nil
	}
	fields, err := funnel.MetadataFields()
	if err != nil || len(fields) == 0 {
		return nil
	}
	lead, err := r.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contactID)
	if err != nil {
		slog.Warn("lead collection: lead lookup failed", "funnel_id", funnel.ID, "contact_id", contactID, "error", err)
		return nil
	}
	if lead == nil {
		return nil
	}
	return &LeadDataContext{Funnel: funnel, Lead: lead, Fields: fields}
}

// buildRecordLeadDataTool builds the dynamic record_lead_data tool from a
// funnel schema. field_key is constrained to the funnel's declared keys via a
// JSON-schema enum so the model can't invent new keys; value is a free string.
func buildRecordLeadDataTool(fields []domain.MetadataFieldDef) domain.AITool {
	keys := make([]string, 0, len(fields))
	var keyDesc strings.Builder
	keyDesc.WriteString("The field to record. Allowed keys:")
	for _, f := range fields {
		keys = append(keys, f.Key)
		keyDesc.WriteString(fmt.Sprintf(" %s (%s)", f.Key, f.Label))
		if f.Type == domain.MetadataFieldEnum && len(f.EnumOptions) > 0 {
			keyDesc.WriteString(" [" + strings.Join(f.EnumOptions, "/") + "]")
		}
		keyDesc.WriteString(";")
	}
	return domain.AITool{
		Name:        RecordLeadDataToolName,
		Description: "Silently records ONE piece of data the customer explicitly volunteered. Never ask for data just to fill this; only call it after the customer states a value. Do not mention this tool to the customer.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field_key": map[string]interface{}{
					"type":        "string",
					"enum":        keys,
					"description": keyDesc.String(),
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "The exact value the customer stated, normalized minimally. Never invent or infer it.",
				},
			},
			"required": []string{"field_key", "value"},
		},
	}
}

// buildFlagBuyIntentTool builds the parameter-less flag_buy_intent tool. The
// model calls it when the customer signals they want to buy / close — the
// handler stamps leads.buy_intent_at so the funnel trigger (US-016) can fire.
func buildFlagBuyIntentTool() domain.AITool {
	return domain.AITool{
		Name:        FlagBuyIntentToolName,
		Description: "Silently flags that the customer has signalled clear purchase or closing intent (e.g. wants to buy, asks how to pay, confirms the deal). Call it once when that happens. Takes no arguments, exposes nothing to the customer, and must never be mentioned to them.",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

// buyIntentPromptSection returns the instruction telling the AI when to call
// flag_buy_intent. Kept silent and one-shot, mirroring the invisible-collection
// posture: detecting intent must never change what the customer is told.
func buyIntentPromptSection() string {
	var b strings.Builder
	b.WriteString("\n\nBUY INTENT DETECTION:\n")
	b.WriteString("- Call the ")
	b.WriteString(FlagBuyIntentToolName)
	b.WriteString(" tool when the customer signals clear intent to buy or close")
	b.WriteString(" (e.g. \"I want to buy\", \"how do I pay?\", \"let's close it\", \"send me the link\").\n")
	b.WriteString("- This is silent: do NOT tell the customer anything about it and do NOT mention the tool.\n")
	b.WriteString("- Call it at most once; do not re-flag intent you already flagged.\n")
	return b.String()
}

// hasFlagBuyIntentCall reports whether the turn's tool calls include a
// flag_buy_intent invocation. The tool takes no arguments, so presence is all
// that matters.
func hasFlagBuyIntentCall(calls []domain.ToolCall) bool {
	for _, tc := range calls {
		if tc.FunctionName == FlagBuyIntentToolName {
			return true
		}
	}
	return false
}

// leadCollectionPromptSection returns the anti-hallucination rules plus the
// list of fields still worth collecting. The rules are deliberately rigid:
// the AI must never invent, infer, or batch-ask for data, must never present a
// form, and must record values only when the customer states them explicitly.
func leadCollectionPromptSection(ctx *LeadDataContext) string {
	var b strings.Builder
	b.WriteString("\n\n---\n\nINVISIBLE DATA COLLECTION (highest priority for tool use):\n")
	b.WriteString("You may quietly collect the fields below by calling the ")
	b.WriteString(RecordLeadDataToolName)
	b.WriteString(" tool. STRICT RULES — follow them exactly:\n")
	b.WriteString("- Ask for AT MOST ONE missing field per reply, and ONLY when it arises naturally in the conversation. Never interrogate.\n")
	b.WriteString("- NEVER deduce, guess or infer a value. Record a field ONLY when the customer states it explicitly.\n")
	b.WriteString("- NEVER ask again for a field the customer already answered (see ALREADY COLLECTED).\n")
	b.WriteString("- NEVER present these as a form, list, or questionnaire. The customer must not feel surveyed.\n")
	b.WriteString("- Recording is silent: do NOT tell the customer you saved anything, and do NOT mention the tool.\n")

	collected := decodeCollected(ctx.Lead)

	b.WriteString("\nFIELDS TO COLLECT:\n")
	for _, f := range ctx.Fields {
		b.WriteString("- ")
		b.WriteString(f.Key)
		b.WriteString(" (")
		b.WriteString(f.Label)
		b.WriteString(", ")
		b.WriteString(string(f.Type))
		if f.Required {
			b.WriteString(", required")
		}
		b.WriteString(")")
		if strings.TrimSpace(f.AIHint) != "" {
			b.WriteString(" — ")
			b.WriteString(f.AIHint)
		}
		b.WriteString("\n")
	}

	if len(collected) > 0 {
		b.WriteString("\nALREADY COLLECTED (do NOT ask for these again):\n")
		for _, f := range ctx.Fields {
			if cf, ok := collected[f.Key]; ok && strings.TrimSpace(cf.Value) != "" {
				b.WriteString("- " + f.Key + ": " + cf.Value + "\n")
			}
		}
	}

	return b.String()
}

// decodeCollected parses a lead's collected_metadata into the typed map. A
// nil/empty/invalid blob yields an empty map (best-effort — collection never
// hard-fails on a malformed prior value).
func decodeCollected(lead *domain.Lead) map[string]CollectedField {
	out := map[string]CollectedField{}
	if lead == nil || len(lead.CollectedMetadata) == 0 {
		return out
	}
	if err := json.Unmarshal(lead.CollectedMetadata, &out); err != nil {
		slog.Warn("lead collection: malformed collected_metadata, starting fresh", "lead_id", lead.ID, "error", err)
		return map[string]CollectedField{}
	}
	return out
}

// mergeRecordLeadData applies a turn's record_lead_data tool calls onto the
// existing collected map. It returns the merged map and the number of NEW
// writes (a write is new when the value differs from what's stored). Calls are
// validated against the allowed key set and a non-empty value, and de-duplicated
// within the turn: the same (field_key, value) seen twice counts once.
func mergeRecordLeadData(
	existing map[string]CollectedField,
	fields []domain.MetadataFieldDef,
	calls []domain.ToolCall,
	now time.Time,
) (map[string]CollectedField, int) {
	allowed := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		allowed[f.Key] = struct{}{}
	}

	merged := make(map[string]CollectedField, len(existing)+len(calls))
	for k, v := range existing {
		merged[k] = v
	}

	seen := make(map[string]struct{}, len(calls))
	writes := 0
	for _, tc := range calls {
		if tc.FunctionName != RecordLeadDataToolName {
			continue
		}
		var args struct {
			FieldKey string `json:"field_key"`
			Value    string `json:"value"`
		}
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			slog.Warn("lead collection: failed to parse record_lead_data args", "error", err)
			continue
		}
		key := strings.TrimSpace(args.FieldKey)
		value := strings.TrimSpace(args.Value)
		// Ignore unknown keys (don't create new fields) and empty values.
		if _, ok := allowed[key]; !ok || value == "" {
			continue
		}
		// Anti-loop: same key+value twice in one turn is recorded once.
		dedupKey := key + "\x00" + value
		if _, dup := seen[dedupKey]; dup {
			continue
		}
		seen[dedupKey] = struct{}{}
		// No-op when the stored value is already identical.
		if cur, ok := merged[key]; ok && cur.Value == value {
			continue
		}
		merged[key] = CollectedField{Value: value, Source: CollectedSourceAI, UpdatedAt: now}
		writes++
	}
	return merged, writes
}

// RecordLeadData applies the record_lead_data tool calls from a single AI turn
// to the conversation's lead, persisting any new field values to
// leads.collected_metadata via LeadRepository.UpdateMetadata. It is a no-op
// when there's no lead context, no repo wired, or no new values were captured.
// Enrichment is passed through unchanged. Returns the number of fields written.
func (r *AgentRunner) RecordLeadData(ctx context.Context, leadCtx *LeadDataContext, calls []domain.ToolCall) int {
	if leadCtx == nil || leadCtx.Lead == nil || r.leadRepo == nil || len(calls) == 0 {
		return 0
	}
	existing := decodeCollected(leadCtx.Lead)
	merged, writes := mergeRecordLeadData(existing, leadCtx.Fields, calls, time.Now())
	if writes == 0 {
		return 0
	}
	blob, err := json.Marshal(merged)
	if err != nil {
		slog.Error("lead collection: failed to marshal collected_metadata", "lead_id", leadCtx.Lead.ID, "error", err)
		return 0
	}
	if err := r.leadRepo.UpdateMetadata(ctx, leadCtx.Lead.ID, blob, leadCtx.Lead.Enrichment); err != nil {
		slog.Error("lead collection: failed to persist collected_metadata", "lead_id", leadCtx.Lead.ID, "error", err)
		return 0
	}
	// Keep the in-memory lead consistent so a later step in the same turn
	// (e.g. the prompt's ALREADY COLLECTED list on a retry) sees the writes.
	leadCtx.Lead.CollectedMetadata = blob
	return writes
}

// FlagBuyIntent stamps leads.buy_intent_at when the AI turn includes a
// flag_buy_intent tool call (US-012). It is idempotent and silent: a no-op when
// there's no lead context, no repo wired, no such call, or the lead already had
// intent flagged. Returns true only when it persisted a fresh timestamp.
func (r *AgentRunner) FlagBuyIntent(ctx context.Context, leadCtx *LeadDataContext, calls []domain.ToolCall) bool {
	if leadCtx == nil || leadCtx.Lead == nil || r.leadRepo == nil {
		return false
	}
	if !hasFlagBuyIntentCall(calls) {
		return false
	}
	// Already flagged: nothing to persist (the SQL guard would also no-op, but
	// short-circuiting avoids a needless write and keeps the return honest).
	if leadCtx.Lead.BuyIntentAt != nil {
		return false
	}
	if err := r.leadRepo.MarkBuyIntent(ctx, leadCtx.Lead.ID); err != nil {
		slog.Error("lead collection: failed to mark buy intent", "lead_id", leadCtx.Lead.ID, "error", err)
		return false
	}
	// Keep the in-memory lead consistent for any later step in this turn.
	now := time.Now()
	leadCtx.Lead.BuyIntentAt = &now
	return true
}
