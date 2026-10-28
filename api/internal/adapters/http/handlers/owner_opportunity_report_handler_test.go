package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatCentsBRL(t *testing.T) {
	assert.Equal(t, "R$ 1.234.567,89", formatCentsBRL(123456789))
	assert.Equal(t, "R$ 100,00", formatCentsBRL(10000))
	assert.Equal(t, "valor não divulgado", formatCentsBRL(0))
}

func TestOpportunityReportTemplate_RendersSelfContained(t *testing.T) {
	phone := "65 2121-6363"
	r := &usecases.OpportunityReport{
		UF: "MT", GeneratedAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
		Competencia:         "202607",
		TotalEstablishments: 40, TotalUnits: 210,
		Categories: []usecases.OpportunityCategory{
			{Query: "ventilador", Establishments: 40, Units: 210},
		},
		Sample: []usecases.OpportunitySample{
			{CNES: "1111111", Name: "HOSPITAL A", Address: "Rua X", Phone: &phone, Units: 30},
		},
		HiddenTargets: 39,
		Tenders: []usecases.OpportunityTender{
			{Title: "Aquisição de ventiladores", Organization: "SES-MT", ValueCents: 10000000},
		},
		TendersTotal: 1, TendersSumCents: 10000000, TenderWindowDays: 30,
	}

	var sb strings.Builder
	require.NoError(t, opportunityReportTmpl.Execute(&sb, reportView(r)))
	html := sb.String()

	assert.Contains(t, html, "HOSPITAL A")
	assert.Contains(t, html, "65 2121-6363")
	assert.Contains(t, html, "Os outros 39")
	assert.Contains(t, html, "R$ 100.000,00")
	assert.Contains(t, html, "competência 202607")
	assert.NotContains(t, html, "http://", "self-contained: no external assets")
	assert.NotContains(t, strings.ReplaceAll(html, "https://pncp", ""), "src=", "no external assets")
}
