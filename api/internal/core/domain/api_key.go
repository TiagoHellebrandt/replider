package domain

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Machine API keys (§6.19, J1).
//
// Everything the product authenticates today belongs to a PERSON: a 24h user JWT,
// or a personal access token minted for one person's MCP client. Neither is a
// credential a customer's ERP can hold — a JWT expires overnight and a PAT dies
// with the employee who created it.
//
// So: a key that belongs to the ACCOUNT, with explicit scopes. It rides the same
// `api_tokens` table as the PATs (same hash, same prefix, same revocation), which
// means the middleware, the "last used" bookkeeping and the expiry sweep all
// already work — the only new things are the type discriminator and the scopes.

// APITokenTypeAPIKey is the machine-key discriminator.
const APITokenTypeAPIKey = "api_key" //nolint:gosec // G101: a token *type*, not a credential

// TokenPrefixAPIKey is what the plaintext key starts with.
//
// A distinct prefix from the PAT's on purpose: a key pasted into the wrong field
// should fail with "wrong kind of credential" rather than with a generic 401, and
// a leaked string in a log is identifiable at a glance.
const TokenPrefixAPIKey = "rpk_" //nolint:gosec // G101: a prefix, not a credential

// APIScope is one permission a key may carry.
//
// Resource plus verb, deliberately coarse: an ERP either reads leads or writes
// them. A per-field permission model would be a lot of surface for a distinction
// no integration has ever asked for, and every extra scope is one more thing a
// customer has to get right to make their integration work.
type APIScope string

const (
	ScopeLeadsRead     APIScope = "leads:read"
	ScopeLeadsWrite    APIScope = "leads:write"
	ScopeContactsRead  APIScope = "contacts:read"
	ScopeContactsWrite APIScope = "contacts:write"
	ScopeSalesRead     APIScope = "sales:read"
	ScopeSalesWrite    APIScope = "sales:write"
	ScopeConversations APIScope = "conversations:read"
	ScopeAnalyticsRead APIScope = "analytics:read"
	ScopeExportRead    APIScope = "export:read"
	ScopeWebhooksWrite APIScope = "webhooks:write"
)

// APIScopeCatalogue is every scope a key may be minted with, with what it grants.
//
// Described rather than merely listed for the same reason the webhook events are:
// somebody choosing scopes is deciding what a third party may do with their
// customer list, and a bare "sales:write" does not say whether it can delete.
func APIScopeCatalogue() []struct {
	Scope       APIScope
	Description string
} {
	return []struct {
		Scope       APIScope
		Description string
	}{
		{ScopeLeadsRead, "Ler leads e seus dados coletados."},
		{ScopeLeadsWrite, "Criar e atualizar leads. NÃO envia mensagem: envio continua exigindo aprovação humana."},
		{ScopeContactsRead, "Ler contatos."},
		{ScopeContactsWrite, "Criar e atualizar contatos."},
		{ScopeSalesRead, "Ler vendas e carteira."},
		{ScopeSalesWrite, "Registrar vendas."},
		{ScopeConversations, "Ler conversas e mensagens."},
		{ScopeAnalyticsRead, "Ler os relatórios."},
		{ScopeExportRead, "Exportar listas. Toda exportação fica registrada com o nome da chave."},
		{ScopeWebhooksWrite, "Criar e alterar assinaturas de webhook."},
	}
}

// ValidAPIScope guards a scope arriving from a client.
func ValidAPIScope(v string) bool {
	scope := APIScope(strings.TrimSpace(v))
	for _, entry := range APIScopeCatalogue() {
		if entry.Scope == scope {
			return true
		}
	}
	return false
}

// NormalizeAPIScopes validates, de-duplicates and sorts.
//
// Sorted because the set is stored as a single string: two keys granted the same
// permissions must produce the same stored value, or "which keys can write sales?"
// becomes a substring search over every ordering somebody happened to send.
func NormalizeAPIScopes(scopes []string) ([]APIScope, error) {
	if len(scopes) == 0 {
		// A key with no scopes authenticates and can do nothing — which
		// presents as a broken integration rather than as a configuration
		// mistake, so it is refused at creation.
		return nil, &AppError{
			Code:    "API_KEY_NO_SCOPES",
			Message: "escolha ao menos um escopo para a chave",
		}
	}
	seen := map[APIScope]struct{}{}
	out := make([]APIScope, 0, len(scopes))
	for _, raw := range scopes {
		trimmed := strings.TrimSpace(raw)
		if !ValidAPIScope(trimmed) {
			return nil, &AppError{
				Code:    "API_KEY_UNKNOWN_SCOPE",
				Message: "escopo desconhecido: " + trimmed,
			}
		}
		scope := APIScope(trimmed)
		if _, dup := seen[scope]; dup {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// EncodeAPIScopes renders the set for the `scope` column.
func EncodeAPIScopes(scopes []APIScope) string {
	parts := make([]string, 0, len(scopes))
	for _, s := range scopes {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, " ")
}

// DecodeAPIScopes reads the column back.
func DecodeAPIScopes(encoded string) []APIScope {
	fields := strings.Fields(encoded)
	out := make([]APIScope, 0, len(fields))
	for _, f := range fields {
		out = append(out, APIScope(f))
	}
	return out
}

// HasAPIScope reports whether a granted set covers the required scope.
//
// Exact match only — no hierarchy, no wildcard. A `leads:write` that implicitly
// granted `leads:read` would be convenient and would also mean a customer
// auditing "what can this key read?" has to know the rule; making it explicit
// costs one more checkbox and removes the guessing.
func HasAPIScope(granted []APIScope, required APIScope) bool {
	for _, scope := range granted {
		if scope == required {
			return true
		}
	}
	return false
}

// ErrAPIScopeMissing is the refusal when a key lacks the scope for a route.
//
// Names the scope it needed: "403" alone sends somebody to re-read documentation,
// while "precisa de sales:write" is a one-line fix.
func ErrAPIScopeMissing(required APIScope) *AppError {
	return &AppError{
		Code:    "API_KEY_SCOPE_MISSING",
		Message: "esta chave não tem o escopo necessário: " + string(required),
	}
}

// MaxActiveAPIKeysPerAccount bounds how many live keys an account may hold.
//
// Twenty: enough for a per-integration key (which is the point — one key per
// consumer means revoking one does not break the others), few enough that a
// runaway script cannot fill the table.
const MaxActiveAPIKeysPerAccount = 20

// ErrAPIKeyLimitReached is the refusal at that ceiling.
var ErrAPIKeyLimitReached = &AppError{
	Code:    "API_KEY_LIMIT_REACHED",
	Message: "limite de chaves ativas atingido — revogue uma antes de criar outra",
}

// APIKeyInfo is a key as the dashboard lists it — never the hash, never the
// plaintext.
type APIKeyInfo struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []APIScope `json:"scopes"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// ResolvedAPIKey is what the middleware needs to authorize a machine request.
//
// No user id: the key belongs to the ACCOUNT, and pretending it acts as some
// person would put a name on an audit row that never touched the request.
type ResolvedAPIKey struct {
	KeyID     string
	AccountID string
	// Name identifies WHICH integration acted, which is what an audit row needs
	// in place of a person.
	Name   string
	Scopes []APIScope
}

// APIKeyRepository reads and revokes machine keys. Creation goes through the
// shared APITokenRepository, so there is one place that writes a token hash.
type APIKeyRepository interface {
	ListByAccount(ctx context.Context, accountID string) ([]*APIKeyInfo, error)
	CountActive(ctx context.Context, accountID string) (int64, error)
	Revoke(ctx context.Context, accountID, id string) (bool, error)
	// ResolveKey returns (nil, nil) for anything that is not a live key.
	ResolveKey(ctx context.Context, tokenHash string) (*ResolvedAPIKey, error)
	TouchLastUsed(ctx context.Context, keyID string)
}
