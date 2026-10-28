package domain

import (
	"errors"
	"strings"
	"testing"
)

// Machine API keys (J1).
//
// The scope set is what a customer is deciding when they hand a third party
// access to their customer list, so the tests are about the decisions being
// explicit: no hierarchy, no wildcards, nothing granted by accident.

func TestNormalizeAPIScopes_RefusesAnEmptySet(t *testing.T) {
	// A key with no scopes authenticates and can do nothing, which presents as
	// a broken integration rather than as a configuration mistake.
	if _, err := NormalizeAPIScopes(nil); err == nil {
		t.Fatal("a key with no scopes must be refused at creation")
	}
}

func TestNormalizeAPIScopes_NamesAnUnknownScope(t *testing.T) {
	_, err := NormalizeAPIScopes([]string{"leads:read", "leads:delete"})

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("err = %v, want an AppError", err)
	}
	if !strings.Contains(appErr.Message, "leads:delete") {
		t.Fatalf("message %q does not name the offender", appErr.Message)
	}
}

func TestNormalizeAPIScopes_IsDeterministic(t *testing.T) {
	first, err := NormalizeAPIScopes([]string{"sales:write", "leads:read", "sales:write"})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	second, err := NormalizeAPIScopes([]string{"leads:read", "sales:write"})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	// Stored as one string, so two keys with the same permissions must produce
	// the same value — otherwise "which keys can write sales?" is a substring
	// search over every ordering somebody happened to send.
	if EncodeAPIScopes(first) != EncodeAPIScopes(second) {
		t.Fatalf("encodings differ: %q vs %q",
			EncodeAPIScopes(first), EncodeAPIScopes(second))
	}
	if len(first) != 2 {
		t.Fatalf("scopes = %v, want the duplicate collapsed", first)
	}
}

func TestAPIScopes_RoundTripThroughTheColumn(t *testing.T) {
	scopes, err := NormalizeAPIScopes([]string{"leads:read", "export:read"})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	decoded := DecodeAPIScopes(EncodeAPIScopes(scopes))

	if len(decoded) != len(scopes) {
		t.Fatalf("decoded %v, want %v", decoded, scopes)
	}
	for i := range scopes {
		if decoded[i] != scopes[i] {
			t.Fatalf("decoded[%d] = %q, want %q", i, decoded[i], scopes[i])
		}
	}
	// An empty column decodes to nothing, not to a phantom scope.
	if got := DecodeAPIScopes(""); len(got) != 0 {
		t.Fatalf("empty column decoded to %v", got)
	}
}

func TestHasAPIScope_HasNoHierarchy(t *testing.T) {
	granted := []APIScope{ScopeLeadsWrite}

	if !HasAPIScope(granted, ScopeLeadsWrite) {
		t.Fatal("the granted scope must match itself")
	}
	// A write that implicitly granted read would be convenient and would mean a
	// customer auditing "what can this key read?" has to know the rule.
	if HasAPIScope(granted, ScopeLeadsRead) {
		t.Fatal("leads:write must NOT imply leads:read")
	}
	// And nothing resembling a wildcard.
	if HasAPIScope(granted, APIScope("leads:*")) {
		t.Fatal("wildcards must not resolve")
	}
	if HasAPIScope(nil, ScopeLeadsRead) {
		t.Fatal("an empty set grants nothing")
	}
}

func TestErrAPIScopeMissing_NamesWhatIsNeeded(t *testing.T) {
	err := ErrAPIScopeMissing(ScopeSalesWrite)

	// "403" alone sends somebody back to the documentation; the name is a
	// one-line fix.
	if !strings.Contains(err.Message, "sales:write") {
		t.Fatalf("message %q does not name the scope", err.Message)
	}
}

func TestAPIScopeCatalogue_EveryScopeExplainsItself(t *testing.T) {
	seen := map[APIScope]bool{}
	for _, entry := range APIScopeCatalogue() {
		if strings.TrimSpace(entry.Description) == "" {
			t.Fatalf("%q has no description", entry.Scope)
		}
		if seen[entry.Scope] {
			t.Fatalf("%q listed twice", entry.Scope)
		}
		seen[entry.Scope] = true
	}
}

func TestAPIKeyPrefix_IsDistinctFromThePAT(t *testing.T) {
	// A user token pasted into a machine integration should fail as "wrong kind
	// of credential" rather than as a mysterious 401 — and a leaked string in a
	// log should be identifiable at a glance.
	if TokenPrefixAPIKey == TokenPrefixPAT {
		t.Fatal("the machine key and the PAT must not share a prefix")
	}
	if !strings.HasSuffix(TokenPrefixAPIKey, "_") {
		t.Fatalf("prefix %q should end with a separator", TokenPrefixAPIKey)
	}
}
