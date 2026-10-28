package domain

// CSVSafeCell neutralises spreadsheet formula injection. A cell that starts
// with one of the characters Excel, LibreOffice and Google Sheets treat as
// the start of a formula (=, +, -, @) or a control character that lets a
// formula hide behind a line break (tab, carriage return) is prefixed with
// an apostrophe, which the spreadsheet renders as literal text.
//
// This is the ONE implementation every CSV/XLSX writer must go through:
// the export use case, the audit-log, analytics, conversation and
// commission exports. A lead name typed by a stranger ("=HYPERLINK(...)")
// ends up in all of them.
func CSVSafeCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}
	return value
}

// CSVSafeRow applies CSVSafeCell to every cell of a row, returning a new
// slice so the caller's data is left untouched.
func CSVSafeRow(row []string) []string {
	out := make([]string, len(row))
	for i, value := range row {
		out[i] = CSVSafeCell(value)
	}
	return out
}
