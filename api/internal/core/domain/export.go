package domain

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Universal export (§6.18, I3).
//
// Every list in the product exports the same way: same filters, same formats,
// same audit trail. One pipeline rather than an endpoint per screen, because the
// alternative is what the codebase already had — two hand-written CSV handlers
// with their own column lists, their own caps and no record of who ran them.
//
// The audit is not decoration. A commercial export is the customer list, the
// prices and the pipeline in one file; a company that cannot answer "who
// downloaded that, and when?" has no answer for a departing salesperson either.
// It also happens to satisfy LGPD Art. 18's portability duty, which is a
// by-product of doing the first thing properly.

// ExportEntity is what is being exported. A closed set: each entity is one query
// with one column list, and an unknown value must be refused rather than
// resolved into "everything".
type ExportEntity string

const (
	ExportLeads         ExportEntity = "leads"
	ExportContacts      ExportEntity = "contacts"
	ExportSales         ExportEntity = "sales"
	ExportConversations ExportEntity = "conversations"
	ExportSignals       ExportEntity = "signals"
)

// ValidExportEntity guards a value arriving from a client.
func ValidExportEntity(v string) bool {
	switch ExportEntity(v) {
	case ExportLeads, ExportContacts, ExportSales, ExportConversations, ExportSignals:
		return true
	}
	return false
}

// ExportFormat is how the file is written.
type ExportFormat string

const (
	// ExportCSV is the interoperable one. Written with a UTF-8 BOM and
	// semicolons, because that is what Brazilian Excel opens as a spreadsheet
	// rather than as one column of garbled text.
	ExportCSV ExportFormat = "csv"
	// ExportXLSX is what a sales team actually works in.
	ExportXLSX ExportFormat = "xlsx"
)

// ValidExportFormat guards the value.
func ValidExportFormat(v string) bool {
	switch ExportFormat(v) {
	case ExportCSV, ExportXLSX:
		return true
	}
	return false
}

// ContentType and Extension for the response headers.
func (f ExportFormat) ContentType() string {
	if f == ExportXLSX {
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	return "text/csv; charset=utf-8"
}

func (f ExportFormat) Extension() string {
	if f == ExportXLSX {
		return "xlsx"
	}
	return "csv"
}

// Export bounds. A commercial export is the whole customer list in one file, so
// the ceiling is a deliberate decision rather than a page size.
const (
	// DefaultExportLimit is what an unspecified request gets.
	DefaultExportLimit = 5000
	// MaxExportLimit is the hard ceiling. Fifty thousand rows is a ~10MB file
	// and a query the database can still serve; past that the honest answer is
	// a narrower filter, not a slower download that times out halfway.
	MaxExportLimit = 50000
)

// ExportRequest is one export.
type ExportRequest struct {
	AccountID string
	UserID    string

	Entity ExportEntity
	Format ExportFormat

	// Filters are the SCREEN's active filters, passed through verbatim. The
	// promise the feature makes is "what you are looking at, in a file" — an
	// export that quietly ignores a filter hands somebody a list they did not
	// ask for and, worse, did not notice they did not ask for.
	Filters map[string]string

	// From/To bound the period when the entity has a date to bound on.
	From *time.Time
	To   *time.Time

	Limit int
}

// Normalize validates and clamps the request.
func (r *ExportRequest) Normalize() error {
	r.Entity = ExportEntity(strings.TrimSpace(string(r.Entity)))
	r.Format = ExportFormat(strings.ToLower(strings.TrimSpace(string(r.Format))))

	if !ValidExportEntity(string(r.Entity)) {
		return &AppError{Code: "EXPORT_INVALID_ENTITY", Message: "entidade inválida para exportação"}
	}
	if r.Format == "" {
		r.Format = ExportCSV
	}
	if !ValidExportFormat(string(r.Format)) {
		return &AppError{Code: "EXPORT_INVALID_FORMAT", Message: "formato inválido: use csv ou xlsx"}
	}
	if r.Limit <= 0 {
		r.Limit = DefaultExportLimit
	}
	if r.Limit > MaxExportLimit {
		r.Limit = MaxExportLimit
	}
	if r.From != nil && r.To != nil && r.To.Before(*r.From) {
		return &AppError{Code: "EXPORT_INVALID_PERIOD", Message: "período inválido: fim antes do início"}
	}
	return nil
}

// Filename is what the browser saves it as.
//
// Carries the entity and the date, because a downloads folder with four files
// called "export.csv" is a downloads folder nobody can use.
func (r ExportRequest) Filename(now time.Time) string {
	return string(r.Entity) + "_" + now.Format("2006-01-02") + "." + r.Format.Extension()
}

// ExportTable is the rendered result: a header row and the data.
//
// Deliberately strings all the way down. The alternative — typed cells — would
// need a type per column per entity, and the one place typing matters (a date
// Excel should recognise) is handled by writing an ISO-8601 string, which every
// spreadsheet parses.
type ExportTable struct {
	Columns []string
	Rows    [][]string
}

// RowCount is what the audit records.
func (t ExportTable) RowCount() int { return len(t.Rows) }

// Truncated reports an export that hit the ceiling.
//
// Surfaced rather than silent: somebody exporting "all leads" and receiving
// exactly 50,000 rows has to know the file is not all of them, or they will
// reconcile it against a dashboard and conclude the dashboard is wrong.
func (t ExportTable) Truncated(limit int) bool { return len(t.Rows) >= limit }

// ExportAudit is the record of one export having happened.
type ExportAudit struct {
	ID        string
	AccountID string
	UserID    string
	// UserEmail is stored rather than joined at read time: this row has to
	// survive the person leaving, and "who exported the customer list" is
	// exactly the question asked after somebody leaves.
	UserEmail string

	Entity  ExportEntity
	Format  ExportFormat
	Filters map[string]string
	Rows    int
	// Truncated says the file was cut at the ceiling, so an auditor reading
	// "50000 rows" knows it was not the whole table.
	Truncated bool

	IP        string
	UserAgent string
	CreatedAt time.Time
}

// FillMetaFromContext stamps the request origin, mirroring AuditLog's helper.
//
// An export is traceable to a machine and not only to a login: the question
// asked after a leak is "from where", and a login alone does not answer it.
// Explicit values are never overwritten — a caller that knows better wins.
func (a *ExportAudit) FillMetaFromContext(ctx context.Context) {
	meta := GetAuditMeta(ctx)
	if a.IP == "" {
		a.IP = meta.IP
	}
	if a.UserAgent == "" {
		a.UserAgent = meta.UserAgent
	}
}

// FilterSummary renders the filters deterministically for storage and display.
//
// Sorted, because a JSON object's key order is not stable in Go and an audit
// row whose text changes between two identical exports is an audit row nobody
// trusts.
func FilterSummary(filters map[string]string) string {
	if len(filters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(filters))
	for k, v := range filters {
		if strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+filters[k])
	}
	return strings.Join(parts, ", ")
}

// ExportRepository reads the rows and records the audit.
type ExportRepository interface {
	// Rows renders one entity into a table, applying the request's filters.
	Rows(ctx context.Context, req ExportRequest) (ExportTable, error)
	// RecordAudit appends the "who exported what" row.
	RecordAudit(ctx context.Context, audit *ExportAudit) error
	// ListAudits is the admin read: who has been downloading the customer
	// list.
	ListAudits(ctx context.Context, accountID string, limit, offset int) ([]*ExportAudit, error)
}
