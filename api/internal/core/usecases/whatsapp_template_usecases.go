package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/oklog/ulid/v2"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
)

// templateVarRegex matches Meta's positional placeholders ({{1}}, {{2}}…)
// in a template body so we can record how many parameters the send adapter
// (US-019) must bind.
var templateVarRegex = regexp.MustCompile(`\{\{\s*(\d+)\s*\}\}`)

// SyncTemplatesUseCase pulls a WhatsApp channel's message templates from
// the Meta Graph API and upserts them into the local cache
// (whatsapp_templates), so the dashboard + outbound send pipeline don't
// roundtrip to Graph on every render. Idempotent per
// (channel_id, name, language).
type SyncTemplatesUseCase struct {
	templateRepo domain.WhatsAppTemplateRepository
	channelRepo  WAChannelResolver
}

func NewSyncTemplatesUseCase(t domain.WhatsAppTemplateRepository, c WAChannelResolver) *SyncTemplatesUseCase {
	return &SyncTemplatesUseCase{templateRepo: t, channelRepo: c}
}

// Execute syncs templates for the given channel and returns the resulting
// cached set (with meta_status). Returns CHANNEL_NOT_FOUND when the channel
// doesn't belong to the account, and CHANNEL_MISSING_CONFIG when the channel
// has no waba_id/access_token (the "credentials absent" path).
func (uc *SyncTemplatesUseCase) Execute(ctx context.Context, accountID, channelID string) ([]*domain.WhatsAppTemplate, error) {
	ch, err := uc.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if ch == nil || ch.AccountID != accountID {
		return nil, &domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"}
	}
	wabaID, _, accessToken, err := extractWAChannelCreds(ch)
	if err != nil {
		return nil, err
	}

	metaTemplates, err := waadapter.ListTemplates(ctx, wabaID, accessToken)
	if err != nil {
		return nil, fmt.Errorf("list meta templates: %w", err)
	}

	for _, mt := range metaTemplates {
		body := extractTemplateBody(mt.Components)
		variables := extractTemplateVariables(body)
		components := marshalComponents(mt.Components)

		row := &domain.WhatsAppTemplate{
			ID:             ulid.Make().String(),
			AccountID:      accountID,
			ChannelID:      channelID,
			MetaTemplateID: mt.ID,
			Name:           mt.Name,
			Language:       mt.Language,
			Category:       mt.Category,
			MetaStatus:     mt.Status,
			Body:           body,
			Components:     components,
			Variables:      variables,
			RejectedReason: mt.RejectedReason,
		}
		if err := uc.templateRepo.Upsert(ctx, row); err != nil {
			return nil, fmt.Errorf("persist template %q: %w", mt.Name, err)
		}
	}

	return uc.templateRepo.ListByChannel(ctx, channelID)
}

// extractTemplateBody flattens the BODY component's text out of Meta's
// heterogeneous components array. Returns "" when there's no body
// component (e.g. authentication templates).
func extractTemplateBody(components []interface{}) string {
	for _, raw := range components {
		c, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		typ, _ := c["type"].(string)
		if typ != "BODY" {
			continue
		}
		if text, ok := c["text"].(string); ok {
			return text
		}
	}
	return ""
}

// extractTemplateVariables returns the sorted, de-duplicated list of
// positional placeholders found in the body ("{{1}}" -> "1") as a JSONB
// array. Always non-nil ("[]" when there are no placeholders).
func extractTemplateVariables(body string) json.RawMessage {
	matches := templateVarRegex.FindAllStringSubmatch(body, -1)
	seen := map[int]struct{}{}
	nums := make([]int, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		nums = append(nums, n)
	}
	sort.Ints(nums)
	vars := make([]string, 0, len(nums))
	for _, n := range nums {
		vars = append(vars, strconv.Itoa(n))
	}
	out, err := json.Marshal(vars)
	if err != nil {
		return json.RawMessage("[]")
	}
	return out
}

// marshalComponents serializes the verbatim Meta components array for
// storage. Falls back to "[]" on a marshal error so the column is never
// left as invalid JSON.
func marshalComponents(components []interface{}) json.RawMessage {
	if len(components) == 0 {
		return json.RawMessage("[]")
	}
	out, err := json.Marshal(components)
	if err != nil {
		return json.RawMessage("[]")
	}
	return out
}
