package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"sort"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Case report metrics (US-M011).

// CaseMetricsHandler serves the aggregated instrumentation.
type CaseMetricsHandler struct {
	uc *usecases.CaseMetricsUseCase
}

func NewCaseMetricsHandler(uc *usecases.CaseMetricsUseCase) *CaseMetricsHandler {
	return &CaseMetricsHandler{uc: uc}
}

// defaultCaseWindow is the period a report covers when the caller does not
// say. 30 days because that is the unit a customer thinks in ("this month
// the robot did X") — long enough for a slow B2B reply cycle to close.
const defaultCaseWindow = 30 * 24 * time.Hour

// HandleCaseMetrics serves GET /v1/sales/case-metrics?from=&to=.
//
// Dates are RFC3339 or YYYY-MM-DD. An unparseable date is a 400 rather than
// a silent fallback: a report over a period the caller did not ask for is
// worse than an error, because it will be screenshotted and sent to someone.
func (h *CaseMetricsHandler) HandleCaseMetrics(c *gin.Context) {
	now := time.Now()
	from, ok := parseCaseDate(c, "from", now.Add(-defaultCaseWindow))
	if !ok {
		return
	}
	to, ok := parseCaseDate(c, "to", now)
	if !ok {
		return
	}

	metrics, err := h.uc.Execute(c.Request.Context(), getAccountID(c), from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func parseCaseDate(c *gin.Context, param string, fallback time.Time) (time.Time, bool) {
	raw := c.Query(param)
	if raw == "" {
		return fallback, true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	respondError(c, &domain.AppError{
		Code:    "VALIDATION_ERROR",
		Message: "data inválida em '" + param + "': use YYYY-MM-DD ou RFC3339",
	})
	return time.Time{}, false
}

// HandleCaseReport serves GET /v1/sales/case-report — the same numbers as
// the JSON endpoint, plus the previous period and the work-avoided estimate.
//
// `?format=html` renders the one-page report (US-M012): self-contained,
// print-clean, no external assets, so "export PDF" is the browser's own
// print dialog rather than a PDF engine nobody maintains.
func (h *CaseMetricsHandler) HandleCaseReport(c *gin.Context) {
	now := time.Now()
	from, ok := parseCaseDate(c, "from", now.Add(-defaultCaseWindow))
	if !ok {
		return
	}
	to, ok := parseCaseDate(c, "to", now)
	if !ok {
		return
	}
	// The assumption behind "work avoided" is a query param on purpose: a
	// customer who thinks 8 minutes is wrong can put their own number in
	// and the page shows the arithmetic either way.
	minutes, _ := strconv.Atoi(c.Query("minutes_per_approach"))

	report, err := h.uc.Report(c.Request.Context(), getAccountID(c), from, to, minutes)
	if err != nil {
		respondError(c, err)
		return
	}
	if c.Query("format") != "html" {
		c.JSON(http.StatusOK, report)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := caseReportTemplate.Execute(c.Writer, newCaseReportView(report)); err != nil {
		c.Status(http.StatusInternalServerError)
	}
}

// caseReportView is the rendering shape: everything already formatted, so
// the template holds layout and no arithmetic.
type caseReportView struct {
	Period      string
	PrevPeriod  string
	Rows        []caseReportRow
	Sources     []caseReportBreakdown
	Channels    []caseReportBreakdown
	AvgReply    string
	ReplyRate   string
	HoursSaved  string
	Assumption  string
	GeneratedAt string
}

type caseReportRow struct {
	Label string
	Value string
	Delta string
	// Better marks an improvement, so the page can colour it without the
	// template deciding what "up" means for each metric.
	Better bool
	Worse  bool
}

type caseReportBreakdown struct {
	Label string
	Value int64
}

func newCaseReportView(r *usecases.CaseReport) caseReportView {
	const day = "02/01/2006"
	v := caseReportView{
		Period:      r.Current.From.Format(day) + " – " + r.Current.To.Format(day),
		PrevPeriod:  r.Previous.From.Format(day) + " – " + r.Previous.To.Format(day),
		GeneratedAt: time.Now().Format("02/01/2006 15:04"),
		HoursSaved:  fmt.Sprintf("%.1f h", r.HoursSaved),
		Assumption: fmt.Sprintf("%d aprovações × %d min de trabalho manual por abordagem",
			r.Current.ApproachesSent, r.MinutesPerApproach),
	}

	v.Rows = []caseReportRow{
		caseRow("Leads gerados", r.Current.LeadsCreated, r.LeadsDelta),
		caseRow("Abordagens enviadas", r.Current.ApproachesSent, r.ApproachesDelta),
		caseRow("Respostas recebidas", r.Current.RepliesReceived, r.RepliesDelta),
		caseRow("Reuniões marcadas", r.Current.MeetingsBooked, r.MeetingsDelta),
	}

	// Unknown stays unknown on the page too: a dash is honest, "0s" is not.
	v.AvgReply = "—"
	if r.Current.AvgReplySeconds != nil {
		v.AvgReply = humanDuration(*r.Current.AvgReplySeconds)
	}
	v.ReplyRate = "—"
	if r.Current.ReplyRate != nil {
		v.ReplyRate = fmt.Sprintf("%.0f%%", *r.Current.ReplyRate*100)
	}

	for label, n := range r.Current.LeadsBySource {
		v.Sources = append(v.Sources, caseReportBreakdown{Label: label, Value: n})
	}
	for label, n := range r.Current.ApproachesByChannel {
		v.Channels = append(v.Channels, caseReportBreakdown{Label: label, Value: n})
	}
	sortBreakdown(v.Sources)
	sortBreakdown(v.Channels)
	return v
}

func caseRow(label string, value, delta int64) caseReportRow {
	row := caseReportRow{Label: label, Value: strconv.FormatInt(value, 10), Delta: "—"}
	switch {
	case delta > 0:
		row.Delta = "+" + strconv.FormatInt(delta, 10)
		row.Better = true
	case delta < 0:
		row.Delta = strconv.FormatInt(delta, 10)
		row.Worse = true
	}
	return row
}

func humanDuration(seconds int) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dmin", seconds/60)
	default:
		return fmt.Sprintf("%dh%02d", seconds/3600, (seconds%3600)/60)
	}
}

// sortBreakdown orders by count desc then label, so the page is stable
// across reloads — a report whose rows shuffle looks made up.
func sortBreakdown(items []caseReportBreakdown) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Value != items[j].Value {
			return items[i].Value > items[j].Value
		}
		return items[i].Label < items[j].Label
	})
}

// One page, no external assets: it has to render identically in a browser,
// in a print dialog and in an email client that stripped the internet.
var caseReportTemplate = template.Must(template.New("case").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8">
<title>Relatório de prospecção — Replider</title>
<style>
  :root { color-scheme: light; }
  body { font: 15px/1.5 -apple-system, Segoe UI, Roboto, sans-serif; color: #111; margin: 0; padding: 32px; }
  .wrap { max-width: 720px; margin: 0 auto; }
  h1 { font-size: 22px; margin: 0 0 4px; }
  .period { color: #555; margin-bottom: 24px; }
  table { width: 100%; border-collapse: collapse; margin-bottom: 24px; }
  th, td { text-align: left; padding: 10px 8px; border-bottom: 1px solid #e5e5e5; }
  th { font-size: 12px; text-transform: uppercase; letter-spacing: .04em; color: #666; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; font-weight: 600; }
  td.delta { text-align: right; font-variant-numeric: tabular-nums; color: #666; width: 90px; }
  /* More specific than td.delta, or the neutral grey wins and every
     movement renders the same colour as no movement. */
  td.delta.up { color: #0a7c2f; }
  td.delta.down { color: #b3261e; }
  .grid { display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 24px; }
  .card { flex: 1 1 180px; border: 1px solid #e5e5e5; border-radius: 10px; padding: 14px; }
  .card .v { font-size: 24px; font-weight: 700; }
  .card .l { font-size: 12px; color: #666; }
  .note { font-size: 12px; color: #666; border-top: 1px solid #e5e5e5; padding-top: 12px; }
  @media print { body { padding: 0; } .card, table { break-inside: avoid; } }
</style></head><body><div class="wrap">
  <h1>Relatório de prospecção</h1>
  <div class="period">{{.Period}} · comparado com {{.PrevPeriod}}</div>

  <table>
    <tr><th>Métrica</th><th style="text-align:right">Período</th><th style="text-align:right">vs. anterior</th></tr>
    {{range .Rows}}
    <tr>
      <td>{{.Label}}</td>
      <td class="num">{{.Value}}</td>
      <td class="delta{{if .Better}} up{{end}}{{if .Worse}} down{{end}}">{{.Delta}}</td>
    </tr>
    {{end}}
  </table>

  <div class="grid">
    <div class="card"><div class="v">{{.ReplyRate}}</div><div class="l">Taxa de resposta</div></div>
    <div class="card"><div class="v">{{.AvgReply}}</div><div class="l">Tempo médio de resposta</div></div>
    <div class="card"><div class="v">{{.HoursSaved}}</div><div class="l">Trabalho manual evitado (estimativa)</div></div>
  </div>

  {{if .Sources}}
  <table>
    <tr><th>Origem dos leads</th><th style="text-align:right">Leads</th></tr>
    {{range .Sources}}<tr><td>{{.Label}}</td><td class="num">{{.Value}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Channels}}
  <table>
    <tr><th>Canal das abordagens</th><th style="text-align:right">Envios</th></tr>
    {{range .Channels}}<tr><td>{{.Label}}</td><td class="num">{{.Value}}</td></tr>{{end}}
  </table>
  {{end}}

  <p class="note">
    Estimativa de trabalho evitado: {{.Assumption}}. Métricas sem base de cálculo
    aparecem como “—” em vez de zero. Gerado em {{.GeneratedAt}}.
  </p>
</div></body></html>`))
