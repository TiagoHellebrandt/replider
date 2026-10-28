package usecases

import (
	"bytes"
	"context"
	"encoding/csv"
	"log/slog"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/replider/api/internal/core/domain"
)

// Universal export (§6.18, I3).
//
// One pipeline for every list: the same filters the screen is showing, the same
// two formats, and an audit row every time. What it replaces is an endpoint per
// screen, each with its own column list, its own cap and no record of who ran it.

// ExportUseCase renders and records exports.
type ExportUseCase struct {
	repo  domain.ExportRepository
	users domain.UserRepository
	now   func() time.Time
}

func NewExportUseCase(repo domain.ExportRepository, users domain.UserRepository) *ExportUseCase {
	return &ExportUseCase{repo: repo, users: users, now: time.Now}
}

// ExportResult is the finished file plus what the caller needs to describe it.
type ExportResult struct {
	Filename    string
	ContentType string
	Body        []byte
	Rows        int
	// Truncated says the export hit the row ceiling. Surfaced so the client can
	// say so — somebody who asked for "all leads" and got exactly the ceiling
	// has to know the file is not all of them, or they will reconcile it against
	// a dashboard and conclude the dashboard is wrong.
	Truncated bool
}

// Execute renders the export and records who ran it.
func (uc *ExportUseCase) Execute(ctx context.Context, req domain.ExportRequest) (*ExportResult, error) {
	if err := req.Normalize(); err != nil {
		return nil, err
	}

	table, err := uc.repo.Rows(ctx, req)
	if err != nil {
		return nil, err
	}

	body, err := renderExport(table, req.Format)
	if err != nil {
		return nil, err
	}

	result := &ExportResult{
		Filename:    req.Filename(uc.now()),
		ContentType: req.Format.ContentType(),
		Body:        body,
		Rows:        table.RowCount(),
		Truncated:   table.Truncated(req.Limit),
	}

	uc.audit(ctx, req, result)
	return result, nil
}

// audit records the download.
//
// Best-effort and AFTER the render: failing to write the audit must not deny
// somebody the export they are entitled to. But it is logged at ERROR, because a
// missing row is a gap in the answer to "who downloaded the customer list" —
// which is the question this table exists for.
func (uc *ExportUseCase) audit(ctx context.Context, req domain.ExportRequest, result *ExportResult) {
	audit := &domain.ExportAudit{
		AccountID: req.AccountID,
		UserID:    req.UserID,
		Entity:    req.Entity,
		Format:    req.Format,
		Filters:   req.Filters,
		Rows:      result.Rows,
		Truncated: result.Truncated,
	}
	// The email is stored on the row rather than joined later: this has to
	// survive the person leaving, which is exactly when the question gets asked.
	if uc.users != nil && req.UserID != "" {
		if user, err := uc.users.GetByID(ctx, req.UserID); err == nil && user != nil {
			audit.UserEmail = user.Email
		}
	}
	// IP and user-agent ride the context, stamped by the audit middleware — the
	// same mechanism the admin audit log uses, so an export is traceable to a
	// machine and not only to a login.
	audit.FillMetaFromContext(ctx)

	if err := uc.repo.RecordAudit(ctx, audit); err != nil {
		slog.ErrorContext(ctx, "export: could not record who exported what",
			"entity", req.Entity, "account_id", req.AccountID,
			"user_id", req.UserID, "error", err)
	}
}

// ListAudits is the admin read: who has been downloading the customer list.
func (uc *ExportUseCase) ListAudits(ctx context.Context, accountID string, limit, offset int) ([]*domain.ExportAudit, error) {
	return uc.repo.ListAudits(ctx, accountID, limit, offset)
}

// renderExport writes the table in the requested format.
func renderExport(table domain.ExportTable, format domain.ExportFormat) ([]byte, error) {
	if format == domain.ExportXLSX {
		return renderXLSX(table)
	}
	return renderCSV(table)
}

// utf8BOM is what makes Excel open a UTF-8 CSV correctly.
//
// Without it, "Clínica" arrives as "ClÃ­nica" for every Brazilian user on a
// default Windows Excel — which is the majority of the people this file is for.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// renderCSV writes a semicolon-separated, BOM-prefixed file.
//
// Semicolons, not commas: Excel in a pt-BR locale treats a comma as the decimal
// separator and puts an entire comma-separated row into one cell. The file is
// then not readable by the people it was made for, which is a worse failure than
// being slightly non-standard.
func renderCSV(table domain.ExportTable) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(utf8BOM)

	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.Write(table.Columns); err != nil {
		return nil, err
	}
	for _, row := range table.Rows {
		if err := w.Write(sanitizeExportRow(row)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderXLSX writes a real spreadsheet.
func renderXLSX(table domain.ExportTable) ([]byte, error) {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("export: closing the spreadsheet failed", "error", err)
		}
	}()

	const sheet = "Sheet1"
	for i, column := range table.Columns {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellStr(sheet, cell, column); err != nil {
			return nil, err
		}
	}
	for rowIdx, row := range table.Rows {
		safe := sanitizeExportRow(row)
		for colIdx, value := range safe {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err != nil {
				return nil, err
			}
			// SetCellStr, not SetCellValue: a phone number like "5565999990000"
			// or a CNPJ with leading zeros would otherwise be stored as a number
			// and come back in scientific notation with the zeros gone.
			if err := f.SetCellStr(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sanitizeExportRow defuses spreadsheet formula injection.
//
// A cell beginning with =, +, - or @ is executed as a formula when the file is
// opened. The contact names in this file are typed by strangers on WhatsApp, so
// "=HYPERLINK(...)" in a contact name is a real path from an inbound message to
// code running on a salesperson's laptop. Prefixing with an apostrophe is the
// standard defusal: the cell displays exactly what was typed and executes
// nothing.
// sanitizeExportRow is domain.CSVSafeRow under the name the renderers
// already use; the rule itself lives in the domain so every CSV writer in
// the API shares one implementation.
func sanitizeExportRow(row []string) []string {
	return domain.CSVSafeRow(row)
}

func sanitizeExportCell(value string) string {
	return domain.CSVSafeCell(value)
}

// ExportEntitiesHelp lists what may be exported, for the MCP tool's description
// and for an error message that tells the caller what to try instead.
func ExportEntitiesHelp() string {
	entities := []string{
		string(domain.ExportLeads), string(domain.ExportContacts),
		string(domain.ExportSales), string(domain.ExportConversations),
		string(domain.ExportSignals),
	}
	return strings.Join(entities, " | ")
}
