package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The MCP tools behind a PAT are actor-scoped (a vendedor's list_funnel_leads
// is their own leads), so every per-account role may mint one. Before this
// the PAT endpoints were admin/attendant only — the one role the sales
// tools were written for could not connect their own Claude.
func TestE2E_MCPTokens_SalesRolesCanMintAPAT(t *testing.T) {
	cleanDB(t)
	registerUser(t, "PAT Floor", "pat-floor@test.com", "password123")

	var accountID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "pat-floor@test.com").Scan(&accountID))

	for _, role := range []domain.Role{domain.RoleVendedor, domain.RoleGestor} {
		userID := seedRBACUser(t, accountID, string(role))
		jwt := tokenForUser(t, accountID, userID, string(role))

		w := doRequest("POST", "/v1/me/mcp-tokens", map[string]any{"name": "Claude do " + string(role)}, jwt)
		require.Equal(t, 201, w.Code, "%s: %s", role, w.Body.String())
		var created map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
		assert.True(t, strings.HasPrefix(created["token"].(string), "rpl_mcp_"), role)

		w = doRequest("GET", "/v1/me/mcp-tokens", nil, jwt)
		require.Equal(t, 200, w.Code, role)
		var list []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
		assert.Len(t, list, 1, "%s sees only their own PAT", role)
	}
}
