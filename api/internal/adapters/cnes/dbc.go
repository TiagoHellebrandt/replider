// Package cnes reads the DATASUS public data behind the installed-park
// feature: the monthly EQ{UF}{AAMM}.dbc files (equipment per establishment)
// from the DATASUS FTP, and the ministry's open-data API for establishment
// records. See docs/spike-cnes.md for the field evidence.
package cnes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/JoshVarga/blast"
)

// A .dbc file is a dBase (DBF) table whose BODY is compressed with the
// PKWare DCL "implode" algorithm ("blast"): the DBF header travels in
// clear (its size is the uint16 at offset 8), followed by a 4-byte CRC32
// and then the compressed records. Decompressing and concatenating yields
// a plain DBF. Format reference: danicat/read.dbc DBC_FORMAT.md.

// dbfTable is a minimally parsed DBF: field layout + raw records.
type dbfTable struct {
	fields []dbfField
	// records holds each non-deleted record's raw bytes (record size,
	// including the leading deletion flag).
	records [][]byte
}

type dbfField struct {
	name   string
	offset int // within the record, first byte after the deletion flag is offset 0
	length int
}

// decodeDBC turns a raw .dbc into a parsed table.
func decodeDBC(raw []byte) (*dbfTable, error) {
	if len(raw) < 32 {
		return nil, fmt.Errorf("dbc too short: %d bytes", len(raw))
	}
	headerSize := int(binary.LittleEndian.Uint16(raw[8:10]))
	if headerSize < 32 || headerSize+4 > len(raw) {
		return nil, fmt.Errorf("dbc header size out of range: %d", headerSize)
	}
	// Skip the 4-byte CRC between the clear header and the blast stream.
	br, err := blast.NewReader(bytes.NewReader(raw[headerSize+4:]))
	if err != nil {
		return nil, fmt.Errorf("blast reader: %w", err)
	}
	defer br.Close() //nolint:errcheck // read-side close of an in-memory stream
	body, err := io.ReadAll(br)
	if err != nil {
		return nil, fmt.Errorf("blast decompress: %w", err)
	}
	return parseDBF(append(append([]byte{}, raw[:headerSize]...), body...))
}

// parseDBF parses a plain DBF byte stream.
func parseDBF(dbf []byte) (*dbfTable, error) {
	if len(dbf) < 32 {
		return nil, fmt.Errorf("dbf too short")
	}
	recordCount := int(binary.LittleEndian.Uint32(dbf[4:8]))
	headerSize := int(binary.LittleEndian.Uint16(dbf[8:10]))
	recordSize := int(binary.LittleEndian.Uint16(dbf[10:12]))
	if headerSize > len(dbf) || recordSize <= 0 {
		return nil, fmt.Errorf("dbf header/record size out of range (%d/%d)", headerSize, recordSize)
	}

	var fields []dbfField
	offset := 0
	for pos := 32; pos+32 <= headerSize && dbf[pos] != 0x0D; pos += 32 {
		desc := dbf[pos : pos+32]
		name := strings.TrimRight(string(bytes.TrimRight(desc[:11], "\x00")), " ")
		length := int(desc[16])
		fields = append(fields, dbfField{name: name, offset: offset, length: length})
		offset += length
	}
	if offset+1 != recordSize {
		return nil, fmt.Errorf("dbf field lengths (%d+flag) disagree with record size %d", offset, recordSize)
	}

	t := &dbfTable{fields: fields}
	pos := headerSize
	for i := 0; i < recordCount && pos+recordSize <= len(dbf); i++ {
		rec := dbf[pos : pos+recordSize]
		pos += recordSize
		if rec[0] == '*' { // deleted record
			continue
		}
		t.records = append(t.records, rec)
	}
	return t, nil
}

// fieldIndex maps field names to their layout, erroring on missing ones so
// a DATASUS schema change fails loudly instead of ingesting zeros.
func (t *dbfTable) fieldIndex(names ...string) (map[string]dbfField, error) {
	byName := make(map[string]dbfField, len(t.fields))
	for _, f := range t.fields {
		byName[f.name] = f
	}
	out := make(map[string]dbfField, len(names))
	for _, n := range names {
		f, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("dbf is missing expected field %q", n)
		}
		out[n] = f
	}
	return out, nil
}

// value reads one field of one record as a trimmed latin-1 string.
func value(rec []byte, f dbfField) string {
	raw := rec[1+f.offset : 1+f.offset+f.length]
	// TrimSpace covers both left-padded numerics and right-padded characters.
	return strings.TrimSpace(decodeLatin1(raw))
}

// decodeLatin1 turns DATASUS's text encoding into UTF-8.
//
// The comment this replaces said latin-1 high bytes "only appear in
// name-bearing tables, which we don't read here" — true until the
// habilitation register was read, whose PORTARIA column is free text. The
// first real load failed on it: `invalid byte sequence for encoding "UTF8":
// 0xb0`, which is the degree sign in "Portaria n° 1234". Beds and services
// came through untouched because their columns are codes and digits; the
// moment a column carries prose, Postgres rejects the whole batch.
//
// Converted only when the bytes are NOT already valid UTF-8. Pure ASCII — the
// overwhelming majority of every column here — is valid UTF-8 and passes
// through byte-identical, so this cannot mangle what already worked, and it
// cannot double-encode a file that arrives in UTF-8 one day.
func decodeLatin1(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	out := make([]rune, 0, len(raw))
	for _, b := range raw {
		out = append(out, rune(b))
	}
	return string(out)
}

// optionalField looks a column up without failing when it is absent.
//
// fieldIndex is deliberately fatal — a renamed column in a REQUIRED field
// must break the load rather than ingest zeros. This is for the columns that
// are a bonus riding on the same file: losing one of those must not cost the
// month.
func (t *dbfTable) optionalField(name string) (dbfField, bool) {
	for _, f := range t.fields {
		if f.name == name {
			return f, true
		}
	}
	return dbfField{}, false
}
