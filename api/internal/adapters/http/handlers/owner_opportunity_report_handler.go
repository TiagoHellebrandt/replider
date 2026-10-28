package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// The Opportunity Report endpoint (tese-agente-autonomo §4) — the owner's
// selling tool: generated for a PROSPECT, from public data only, and
// deliberately incomplete. Super-admin only in v1; the public lead-magnet
// version is a later, separate decision.

// SetOpportunityReportUC wires the report (optional setter; nil → 503).
func (h *OwnerHandler) SetOpportunityReportUC(uc *usecases.OpportunityReportUseCase) {
	h.opportunityReportUC = uc
}

// HandleOpportunityReport serves GET /v1/owner/opportunity-report.
//
// Query: uf (required), equipment (comma-separated words, required),
// sample (default 3), days (tender window, default 30), format=html for
// the printable page (browser print → PDF is the delivery).
//
// @Summary      Generate a prospect's lost-opportunity report (public data only)
// @Tags         owner
// @Produce      json
// @Success      200 {object} usecases.OpportunityReport
// @Failure      400 {object} domain.AppError
// @Security     BearerAuth
// @Router       /owner/opportunity-report [get]
func (h *OwnerHandler) HandleOpportunityReport(c *gin.Context) {
	if h.opportunityReportUC == nil {
		respondError(c, &domain.AppError{Code: "REPORT_UNAVAILABLE", Message: "opportunity report not configured"})
		return
	}
	var categories []string
	for _, part := range strings.Split(c.Query("equipment"), ",") {
		if p := strings.TrimSpace(part); p != "" {
			categories = append(categories, p)
		}
	}
	if len(categories) == 0 {
		respondError(c, &domain.AppError{Code: "VALIDATION_ERROR", Message: "equipment is required (comma-separated words)"})
		return
	}
	sample, _ := strconv.Atoi(c.Query("sample"))
	days, _ := strconv.Atoi(c.Query("days"))

	report, err := h.opportunityReportUC.Build(c.Request.Context(), usecases.OpportunityReportInput{
		UF:         c.Query("uf"),
		Categories: categories,
		Sample:     sample,
		TenderDays: days,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	if c.Query("format") == "html" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := opportunityReportTmpl.Execute(c.Writer, reportView(report)); err != nil {
			respondError(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, report)
}

// reportViewModel pre-formats what html/template is clumsy at.
type reportViewModel struct {
	*usecases.OpportunityReport
	TendersSum string
	Values     map[int]string // index → formatted tender value
}

func reportView(r *usecases.OpportunityReport) reportViewModel {
	v := reportViewModel{OpportunityReport: r, Values: map[int]string{}}
	v.TendersSum = formatCentsBRL(r.TendersSumCents)
	for i, t := range r.Tenders {
		v.Values[i] = formatCentsBRL(t.ValueCents)
	}
	return v
}

func formatCentsBRL(cents int64) string {
	if cents <= 0 {
		return "valor não divulgado"
	}
	whole := cents / 100
	// Thousand separators, pt-BR style.
	s := strconv.FormatInt(whole, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return fmt.Sprintf("R$ %s,%02d", strings.Join(parts, "."), cents%100)
}

// The page is self-contained (inline CSS, no assets): it gets printed to
// PDF and mailed, and a report that needs a server alive to render is a
// report that breaks in the prospect's inbox.
var opportunityReportTmpl = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8">
<title>Oportunidades em {{.UF}} — Replider</title>
<style>
body{font-family:-apple-system,Segoe UI,Roboto,sans-serif;color:#111;max-width:720px;margin:40px auto;padding:0 24px;line-height:1.5}
h1{font-size:26px;margin-bottom:4px} h2{font-size:18px;margin-top:32px;border-bottom:2px solid #111;padding-bottom:4px}
.sub{color:#555;font-size:14px}
.big{font-size:40px;font-weight:800;margin:0}
.cards{display:flex;gap:16px;margin:20px 0}
.card{flex:1;border:1px solid #ddd;border-radius:10px;padding:16px}
.card .label{font-size:12px;color:#555;text-transform:uppercase;letter-spacing:.5px}
table{width:100%;border-collapse:collapse;margin-top:12px;font-size:14px}
td,th{text-align:left;padding:8px 6px;border-bottom:1px solid #eee}
.teaser{background:#111;color:#fff;border-radius:10px;padding:18px 20px;margin-top:28px;font-size:16px}
.src{color:#888;font-size:11px;margin-top:36px}
@media print {.noprint{display:none}}
</style></head><body>
<h1>O que está passando na sua porta em {{.UF}}</h1>
<p class="sub">Relatório de oportunidades — dados públicos (CNES/DATASUS e PNCP), gerado em {{.GeneratedAt.Format "02/01/2006"}}.</p>

<div class="cards">
<div class="card"><div class="label">Estabelecimentos com o parque que você atende</div><p class="big">{{.TotalEstablishments}}</p></div>
<div class="card"><div class="label">Unidades de equipamento declaradas</div><p class="big">{{.TotalUnits}}</p></div>
<div class="card"><div class="label">Compras públicas na janela ({{.TenderWindowDays}} dias)</div><p class="big">{{.TendersTotal}}</p><div class="sub">{{.TendersSum}}</div></div>
</div>

<h2>Por categoria</h2>
<table><tr><th>Equipamento</th><th>Estabelecimentos</th><th>Unidades</th></tr>
{{range .Categories}}<tr><td>{{.Query}}</td><td>{{.Establishments}}</td><td>{{.Units}}</td></tr>{{end}}
</table>

{{if .Sample}}<h2>Amostra — alvos reais, com telefone</h2>
<table><tr><th>Estabelecimento</th><th>Endereço</th><th>Telefone</th><th>Unidades</th></tr>
{{range .Sample}}<tr><td>{{.Name}}</td><td>{{.Address}}</td><td>{{if .Phone}}{{.Phone}}{{end}}</td><td>{{.Units}}</td></tr>{{end}}
</table>{{end}}

{{if .Tenders}}<h2>Comprando agora (PNCP)</h2>
<table><tr><th>Objeto</th><th>Órgão</th><th>Valor estimado</th></tr>
{{range $i, $t := .Tenders}}<tr><td>{{$t.Title}}</td><td>{{$t.Organization}}</td><td>{{index $.Values $i}}</td></tr>{{end}}
</table>{{end}}

<div class="teaser">Este relatório mostra {{len .Sample}} alvos. <b>Os outros {{.HiddenTargets}}</b> — e a oferta pronta para cada um, com o produto certo e a objeção já respondida — ficam dentro do Replider.</div>

<p class="src">Fontes: CNES/DATASUS (parque declarado por estabelecimento{{if .Competencia}}, competência {{.Competencia}}{{end}}) e PNCP (contratações públicas dos últimos {{.TenderWindowDays}} dias). Dados públicos; números refletem o que os estabelecimentos declaram ao Ministério da Saúde.</p>
</body></html>`))
