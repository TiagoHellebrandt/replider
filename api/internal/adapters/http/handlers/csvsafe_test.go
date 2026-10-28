package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// A formula typed into a free-text field by an outsider must not execute
// when an admin opens the export in Excel. The audit log stands in for all
// five CSV writers: they share domain.CSVSafeRow.
func TestAuditExportCSV_NeutralisesFormulaInjection(t *testing.T) {
	repo := new(MockAuditLogRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	payload := `=HYPERLINK("http://evil.example/x","click")`
	repo.On("ListFiltered", mock.Anything, mock.Anything).Return([]*domain.AuditLog{{
		ID: "log-1", AccountID: testAccountID, Action: "+cmd|' /C calc'!A0",
		EntityType: "lead", EntityID: "@SUM(1)", Payload: []byte(payload),
	}}, nil)

	router := setupAuditLogRouter(repo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/audit/export.csv", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `'+cmd|' /C calc'!A0`)
	assert.Contains(t, body, `'@SUM(1)`)
	assert.Contains(t, body, `'=HYPERLINK(`)
	// The unprefixed formula never appears at the start of a cell.
	for _, line := range strings.Split(body, "\n") {
		for _, cell := range strings.Split(line, ",") {
			cell = strings.TrimPrefix(cell, `"`)
			assert.False(t, strings.HasPrefix(cell, "="), "raw formula cell: %q", cell)
		}
	}
}
