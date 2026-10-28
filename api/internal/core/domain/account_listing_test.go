package domain

import (
	"encoding/json"
	"testing"
)

// TestAccountListing_JSONShape locks the owner-panel wire contract. The
// Flutter OwnerAccountListing model deserializes with FieldRename.snake,
// so the API must emit snake_case keys and a nested `account` object. A
// regression here (e.g. dropping the json tags) makes the dashboard
// probe throw and silently hide the owner panel from authorized
// operators.
func TestAccountListing_JSONShape(t *testing.T) {
	l := AccountListing{
		Account:            &Account{ID: "acc-1", Name: "Raphaela"},
		PlanName:           "Trial",
		OwnerEmail:         "owner@example.com",
		OwnerName:          "Owner",
		TokensUsed:         1312,
		AddonTokens:        0,
		ConversationsCount: 0,
		PlanMaxTokens:      0,
	}

	b, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"account", "plan_name", "owner_email", "owner_name",
		"tokens_used", "addon_tokens", "conversations_count", "plan_max_tokens",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing snake_case key %q in %s", key, b)
		}
	}

	// Guard against the Go-default PascalCase leaking back.
	for _, bad := range []string{"Account", "OwnerEmail", "PlanName", "TokensUsed"} {
		if _, ok := got[bad]; ok {
			t.Errorf("unexpected PascalCase key %q in %s", bad, b)
		}
	}

	// The nested account must be an object keyed `account` with snake fields.
	var acc map[string]json.RawMessage
	if err := json.Unmarshal(got["account"], &acc); err != nil {
		t.Fatalf("nested account not an object: %v", err)
	}
	if _, ok := acc["id"]; !ok {
		t.Errorf("nested account missing id: %s", got["account"])
	}
}

// TestOwnerConversationRow_JSONShape locks the owner-panel "recent
// conversations" contract. The Flutter OwnerConversationRow model uses
// FieldRename.snake, so a missing json tag (PascalCase leak) makes the
// section silently render empty even when the account has conversations.
func TestOwnerConversationRow_JSONShape(t *testing.T) {
	row := OwnerConversationRow{ID: "c-1", ChannelID: "ch-1", ContactName: "X"}
	got := marshalKeys(t, row)
	for _, key := range []string{
		"id", "channel_id", "agent_id", "contact_name", "status",
		"is_bot_paused", "channel_type", "message_count",
		"last_customer_message_at", "created_at",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing snake_case key %q", key)
		}
	}
	for _, bad := range []string{"ID", "ChannelID", "ContactName", "IsBotPaused"} {
		if _, ok := got[bad]; ok {
			t.Errorf("unexpected PascalCase key %q", bad)
		}
	}
}

// TestAccountTokenUsage_JSONShape locks the "biggest spenders" widget
// contract — note the Flutter TopAccountUsage model keys the account
// id/name as `id`/`name`, NOT `account_id`/`account_name`.
func TestAccountTokenUsage_JSONShape(t *testing.T) {
	row := AccountTokenUsage{AccountID: "a-1", AccountName: "Acme"}
	got := marshalKeys(t, row)
	for _, key := range []string{
		"id", "name", "plan_id", "plan_name", "tokens_used",
		"conversations_count", "suspended_at",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing snake_case key %q", key)
		}
	}
	for _, bad := range []string{"AccountID", "AccountName", "account_id", "account_name"} {
		if _, ok := got[bad]; ok {
			t.Errorf("unexpected key %q (model wants id/name)", bad)
		}
	}
}

func marshalKeys(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return got
}
