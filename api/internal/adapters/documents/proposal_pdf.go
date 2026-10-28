// Package documents renders the files a salesperson actually hands over.
//
// It exists because the fastest way to lose a deal in this market is to be
// second with the price. The proposal screen could compose lines and totals
// and then produced nothing to send: the salesperson finished the work and
// went back to Word.
package documents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"

	"github.com/replider/api/internal/core/domain"
)

// ProposalDocument is everything the page prints. Assembled by the caller so
// this package never reaches for a repository — it renders, nothing else.
type ProposalDocument struct {
	// Seller is the company sending it, and the name it may be signed with.
	Seller string
	// Signature is the sign-off block from the brand kit, when there is one.
	Signature string
	// Customer is who it is addressed to.
	Customer string
	// Reference is the proposal's own id, so a customer replying three weeks
	// later can be matched to the right offer.
	Reference string
	Currency  string
	Items     []domain.ProposalItem
	// Body is the salesperson's own words. Printed above the table because a
	// price list with no sentence is a quote, and a quote is not an offer.
	Body      string
	CreatedAt time.Time
	// ValidUntil is when the prices stop standing. Optional, and deliberately
	// so: a validity nobody decided is worse than none, because the customer
	// will hold you to whatever the paper says.
	ValidUntil *time.Time
}

// pageWidth/pageHeight are A4 in points, and margin is the left rule
// everything aligns to. Fixed rather than configurable: one layout that is
// right beats six that are adjustable.
const (
	marginX     = 56.0
	topY        = 64.0
	lineHeight  = 15.0
	tableWidth  = 483.0
	bodyMaxCols = 92
)

// RenderProposal turns the offer into a one-page PDF a buyer can read and a
// purchasing department can file.
//
// Built through pdfcpu's own page description rather than by hand-writing PDF
// operators: the library is already a dependency (it reads supplier brochures),
// and a generated page that pdfcpu itself produced is a page pdfcpu can always
// reopen.
func RenderProposal(doc ProposalDocument) ([]byte, error) {
	if len(doc.Items) == 0 {
		// A priced offer with no lines is not a document, it is an empty
		// promise with a letterhead.
		return nil, fmt.Errorf("proposal pdf: no lines to print")
	}

	y := topY
	texts := []map[string]any{
		textAt("PROPOSTA COMERCIAL", marginX, y, "Helvetica-Bold", 15),
	}
	y += 22
	if s := strings.TrimSpace(doc.Seller); s != "" {
		texts = append(texts, textAt(s, marginX, y, "Helvetica-Bold", 11))
		y += lineHeight
	}
	texts = append(texts, textAt(headerLine(doc), marginX, y, "Helvetica", 9))
	y += 24

	if c := strings.TrimSpace(doc.Customer); c != "" {
		texts = append(texts, textAt("Para: "+c, marginX, y, "Helvetica-Bold", 11))
		y += 20
	}

	// The salesperson's sentence, wrapped by hand. pdfcpu will not reflow a
	// paragraph for us, and a line that runs off the page is worse than a
	// conservative wrap.
	for _, line := range wrapText(doc.Body, bodyMaxCols) {
		texts = append(texts, textAt(line, marginX, y, "Helvetica", 10))
		y += lineHeight
	}
	if len(doc.Body) > 0 {
		y += 10
	}

	rows := make([][]string, 0, len(doc.Items))
	var total int64
	for _, it := range doc.Items {
		line := int64(it.Quantity) * it.UnitPriceCents
		total += line
		rows = append(rows, []string{
			it.Name,
			fmt.Sprint(it.Quantity),
			domain.FormatMoneyCents(it.UnitPriceCents, doc.Currency),
			domain.FormatMoneyCents(line, doc.Currency),
		})
	}

	tableY := y
	// Table height, so what follows never lands on top of it: header plus one
	// row each, at the line height pdfcpu draws them with.
	y += float64(len(rows)+1)*18 + 18

	texts = append(texts,
		textAt("TOTAL: "+domain.FormatMoneyCents(total, doc.Currency),
			marginX, y, "Helvetica-Bold", 12))
	y += 26

	if doc.ValidUntil != nil {
		texts = append(texts, textAt(
			"Proposta válida até "+doc.ValidUntil.Format("02/01/2006"),
			marginX, y, "Helvetica", 9))
		y += lineHeight
	}
	for _, line := range wrapText(doc.Signature, bodyMaxCols) {
		texts = append(texts, textAt(line, marginX, y, "Helvetica", 9))
		y += lineHeight
	}

	page := map[string]any{
		"content": map[string]any{
			"text": texts,
			"table": []map[string]any{{
				"pos":        []float64{marginX, tableY},
				"width":      tableWidth,
				"rows":       len(rows),
				"cols":       4,
				"lheight":    14,
				"grid":       true,
				"colWidths":  []int{52, 10, 19, 19},
				"colAnchors": []string{"Left", "Center", "Right", "Right"},
				"header": map[string]any{
					"values":  []string{"Item", "Qtd", "Unitário", "Total"},
					"lheight": 14,
					"font":    map[string]any{"name": "Helvetica-Bold", "size": 9},
				},
				"values": rows,
				"font":   map[string]any{"name": "Helvetica", "size": 9},
			}},
		},
	}

	spec := map[string]any{
		"paper":  "A4",
		"origin": "UpperLeft",
		"pages":  map[string]any{"1": page},
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("proposal pdf: describe page: %w", err)
	}

	var out bytes.Buffer
	if err := api.Create(nil, bytes.NewReader(raw), &out, nil); err != nil {
		return nil, fmt.Errorf("proposal pdf: render: %w", err)
	}
	return out.Bytes(), nil
}

func textAt(value string, x, y float64, font string, size float64) map[string]any {
	return map[string]any{
		"value": sanitize(value),
		"pos":   []float64{x, y},
		"font":  map[string]any{"name": font, "size": size},
	}
}

func headerLine(doc ProposalDocument) string {
	parts := []string{"Proposta " + shortRef(doc.Reference)}
	if !doc.CreatedAt.IsZero() {
		parts = append(parts, doc.CreatedAt.Format("02/01/2006"))
	}
	return strings.Join(parts, "  ·  ")
}

// shortRef prints the tail of the ULID, not the head.
//
// The head of a ULID is a timestamp, so every proposal created the same
// afternoon shares it — a reference that cannot distinguish two offers is not
// a reference. Same reasoning as the WhatsApp approval codes.
func shortRef(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

// sanitize keeps the page renderable.
//
// pdfcpu's standard fonts are single-byte encodings, and a stray newline in a
// textbox value ends the line silently rather than wrapping. Both are the kind
// of failure that shows up as a blank area on a customer's screen, which is the
// one place it must not.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

// wrapText breaks a paragraph on word boundaries.
//
// Column-counted rather than measured: the fonts here are proportional, so a
// column count is an approximation — a deliberately conservative one, because
// a short line looks composed and an overflowing one looks broken.
func wrapText(s string, cols int) []string {
	s = sanitize(s)
	if s == "" {
		return nil
	}
	words := strings.Fields(s)
	lines := make([]string, 0, 4)
	cur := ""
	for _, w := range words {
		switch {
		case cur == "":
			cur = w
		case len(cur)+1+len(w) <= cols:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
