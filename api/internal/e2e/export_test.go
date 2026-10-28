package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Universal export against a real database (§6.18, I3).
//
// Two things only a real request proves: the filters the SCREEN is showing
// actually reach the SQL (an export that quietly ignores one hands somebody a
// list they did not ask for), and every download leaves a row saying who took it.

func seedExportLead(t *testing.T, accountID, funnelID, status, source string) string {
	t.Helper()
	contactID := seedSaleContact(t, accountID)
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, source,
		                    created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
		id, accountID, funnelID, contactID, status, source)
	require.NoError(t, err)
	return id
}

func exportCSVLines(t *testing.T, body string) []string {
	t.Helper()
	// The BOM is what makes Excel read it as UTF-8; strip it before parsing.
	body = strings.TrimPrefix(body, "\ufeff")
	lines := []string{}
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func TestExport_RespectsTheScreensFilters(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)

	seedExportLead(t, accountID, funnelID, "new", "pncp")
	seedExportLead(t, accountID, funnelID, "responded", "pncp")
	seedExportLead(t, accountID, funnelID, "responded", "google_places")

	all := doRequest("GET", "/v1/export/leads", nil, token)
	require.Equal(t, http.StatusOK, all.Code, all.Body.String())
	// Header plus three rows.
	assert.Len(t, exportCSVLines(t, all.Body.String()), 4)

	// The promise is "what you are looking at, in a file". An export that
	// silently ignored the filter would hand somebody a list they did not ask
	// for and did not notice they did not ask for.
	filtered := doRequest("GET", "/v1/export/leads?status=responded", nil, token)
	require.Equal(t, http.StatusOK, filtered.Code)
	assert.Len(t, exportCSVLines(t, filtered.Body.String()), 3)

	both := doRequest("GET", "/v1/export/leads?status=responded&source=pncp", nil, token)
	require.Equal(t, http.StatusOK, both.Code)
	assert.Len(t, exportCSVLines(t, both.Body.String()), 2)
}

func TestExport_AnUnknownFilterIsIgnoredRatherThanFailing(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)
	seedExportLead(t, accountID, funnelID, "new", "pncp")

	// A screen gains a filter before the export learns about it. Failing the
	// download is a worse answer than exporting the wider set — and the audit
	// records what was actually applied.
	w := doRequest("GET", "/v1/export/leads?colour=blue", nil, token)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, exportCSVLines(t, w.Body.String()), 2)
}

func TestExport_RefusesWhatItCannotHonour(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	cases := map[string]string{
		// An unknown entity must be refused, never resolved into "everything".
		"unknown entity": "/v1/export/everything",
		"unknown format": "/v1/export/leads?format=pdf",
		// An export silently covering a period nobody asked for is a file that
		// gets forwarded to somebody.
		"unparseable date": "/v1/export/leads?from=ontem",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			w := doRequest("GET", path, nil, token)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestExport_EveryDownloadIsRecorded(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)
	seedExportLead(t, accountID, funnelID, "responded", "pncp")

	w := doRequest("GET", "/v1/export/leads?status=responded", nil, token)
	require.Equal(t, http.StatusOK, w.Code)

	audits := doRequest("GET", "/v1/export/audits", nil, token)
	require.Equal(t, http.StatusOK, audits.Code, audits.Body.String())
	rows := parseJSONArray(t, audits)
	require.Len(t, rows, 1)

	entry, _ := rows[0].(map[string]any)
	assert.Equal(t, "leads", entry["Entity"])
	assert.Equal(t, float64(1), entry["Rows"])
	// The email is stored on the row, not joined: the question "who exported
	// the customer list" is asked precisely after somebody has left.
	assert.NotEmpty(t, entry["UserEmail"])
	// And the filters that were actually applied, so the trail says WHAT was
	// taken rather than only that something was.
	filters, _ := entry["Filters"].(map[string]any)
	require.NotNil(t, filters)
	assert.Contains(t, filters["resumo"], "status=responded")
}

func TestExport_XLSXIsARealSpreadsheet(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)
	seedExportLead(t, accountID, funnelID, "new", "pncp")

	w := doRequest("GET", "/v1/export/leads?format=xlsx", nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "spreadsheetml")
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".xlsx")
	// XLSX is a zip container.
	assert.Equal(t, "PK", w.Body.String()[:2])
}

func TestExport_LeaksNothingAcrossAccounts(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")
	myFunnel := seedCCFunnel(t, mine)
	seedExportLead(t, mine, myFunnel, "new", "pncp")

	theirs := seedIdentityAccount(t)
	theirFunnel := seedCCFunnel(t, theirs)
	for i := 0; i < 3; i++ {
		seedExportLead(t, theirs, theirFunnel, "new", "pncp")
	}

	w := doRequest("GET", "/v1/export/leads", nil, myToken)
	require.Equal(t, http.StatusOK, w.Code)

	// One lead, mine. This file is the customer list — a cross-tenant row here
	// is the worst leak the product can produce.
	assert.Len(t, exportCSVLines(t, w.Body.String()), 2)
}

func TestExport_AnAttendantCannotTakeTheCustomerList(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendantToken := tokenForSeededAccount(t, accountID, "attendant")

	// One file carries the customer list, the prices and the pipeline.
	w := doRequest("GET", "/v1/export/leads", nil, attendantToken)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	audits := doRequest("GET", "/v1/export/audits", nil, attendantToken)
	assert.Equal(t, http.StatusForbidden, audits.Code)
}

func TestExport_TruncationIsAnnounced(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)
	for i := 0; i < 3; i++ {
		seedExportLead(t, accountID, funnelID, "new", "pncp")
	}

	w := doRequest("GET", "/v1/export/leads?limit=2", nil, token)
	require.Equal(t, http.StatusOK, w.Code)

	// The body is a file the browser is already downloading, so the notice has
	// nowhere else to go — and somebody who asked for "all leads" needs to know
	// this is not all of them.
	assert.Equal(t, "true", w.Header().Get("X-Export-Truncated"))
	assert.Equal(t, "2", w.Header().Get("X-Export-Rows"))
}
