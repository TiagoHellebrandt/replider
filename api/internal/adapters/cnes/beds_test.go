package cnes

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf8"

	"github.com/JoshVarga/blast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildLTDBC mirrors buildEQDBC for the bed census layout — a real
// blast-compressed DBC, so the test exercises the decoder the loader uses
// rather than a hand-fed table.
func buildLTDBC(t *testing.T, cols []string, lens []int, records [][]string) []byte {
	t.Helper()
	recordSize := 1
	for _, l := range lens {
		recordSize += l
	}
	headerSize := 32 + 32*len(cols) + 1

	header := make([]byte, headerSize)
	header[0] = 0x03
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(records)))
	binary.LittleEndian.PutUint16(header[8:10], uint16(headerSize))
	binary.LittleEndian.PutUint16(header[10:12], uint16(recordSize))
	for i, name := range cols {
		desc := header[32+32*i : 32+32*(i+1)]
		copy(desc[:11], name)
		desc[11] = 'C'
		desc[16] = byte(lens[i])
	}
	header[headerSize-1] = 0x0D

	var body bytes.Buffer
	for _, rec := range records {
		body.WriteByte(' ')
		for i := range cols {
			v := rec[i]
			for len(v) < lens[i] {
				v += " "
			}
			body.WriteString(v[:lens[i]])
		}
	}

	var compressed bytes.Buffer
	w := blast.NewWriter(&compressed, blast.Binary, 4096)
	_, err := w.Write(body.Bytes())
	require.NoError(t, err)
	require.NoError(t, w.Close())

	out := append([]byte{}, header...)
	out = append(out, 0, 0, 0, 0) // CRC placeholder — the reader skips it
	out = append(out, compressed.Bytes()...)
	return out
}

var ltCols = []string{"CNES", "CODUFMUN", "TP_LEITO", "CODLEITO",
	"QT_EXIST", "QT_SUS", "QT_CONTR", "COMPETEN"}
var ltLens = []int{7, 6, 1, 2, 4, 4, 4, 6}

// The bed census is the number that sizes an order, and this loader had never
// read the file it arrives in.
func TestParseBedsDBC_ReadsTheCensus(t *testing.T) {
	raw := buildLTDBC(t, ltCols, ltLens, [][]string{
		{"2396866", "510760", "3", "81", "  20", "  18", "   0", "202608"},
		{"2396866", "510760", "1", "3", "  40", "  30", "   0", "202608"},
	})

	rows, err := ParseBedsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "2396866", rows[0].CNES)
	assert.Equal(t, "81", rows[0].BedCode)
	assert.Equal(t, "3", rows[0].BedType)
	assert.Equal(t, 20, rows[0].QtExist)
	assert.Equal(t, 18, rows[0].QtSUS)
	assert.Equal(t, "MT", rows[0].UF)
	// Unpadded in the file, padded to the dictionary's key: "3" is specialty
	// 03 and left bare it joins nothing at all.
	assert.Equal(t, "03", rows[1].BedCode)
}

// One malformed record must not lose the other twelve hundred.
func TestParseBedsDBC_DropsOnlyTheBadRow(t *testing.T) {
	raw := buildLTDBC(t, ltCols, ltLens, [][]string{
		{"", "510760", "3", "81", "  10", "   0", "   0", "202608"},
		{"2396866", "510760", "3", "81", "  10", "   0", "   0", "2026"},
		{"2396866", "510760", "3", "82", "   5", "   5", "   0", "202608"},
	})

	rows, err := ParseBedsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "82", rows[0].BedCode)
}

// A DATASUS layout change must break the load loudly rather than ingest
// zeros — the same posture the equipment load takes.
func TestParseBedsDBC_MissingColumnIsFatal(t *testing.T) {
	raw := buildLTDBC(t, []string{"CNES", "COMPETEN"}, []int{7, 6},
		[][]string{{"2396866", "202608"}})
	_, err := ParseBedsDBC(raw, "MT")
	require.Error(t, err)
}

var eqCols = []string{"CNES", "CODUFMUN", "TIPEQUIP", "CODEQUIP", "QT_EXIST",
	"QT_USO", "IND_SUS", "COMPETEN", "CPF_CNPJ", "CNPJ_MAN", "ESFERA_A"}
var eqLens = []int{7, 6, 2, 3, 4, 4, 1, 6, 14, 14, 2}

// The entity CNPJ was being crawled out of the open-data API one
// establishment at a time — 5.266 requests at three per second for Mato
// Grosso — while every row of the monthly equipment file already carried it.
func TestParseEquipmentDBC_CarriesTheAdministrativeIdentity(t *testing.T) {
	raw := buildLTDBC(t, eqCols, eqLens, [][]string{
		{"9209352", "510340", "01", "11", "   2", "   1", "1", "202608",
			"00000000000000", "15084338000146", "0M"},
	})

	rows, err := ParseEquipmentDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "15084338000146", rows[0].EntityCNPJ)
	assert.Equal(t, "0M", rows[0].AdminSphere)
	// An all-zero column is the file saying "not published". Passed through,
	// it would be looked up as a real CNPJ, find nothing, and render as "this
	// buyer has no contracts" instead of "we have no key".
	assert.Empty(t, rows[0].OwnCNPJ)
}

func TestParseEquipmentDBC_RejectsAMalformedCNPJ(t *testing.T) {
	raw := buildLTDBC(t, eqCols, eqLens, [][]string{
		{"9209352", "510340", "01", "11", "   2", "   1", "1", "202608",
			"1234", "", "  "},
	})

	rows, err := ParseEquipmentDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Empty(t, rows[0].OwnCNPJ, "four digits is not a CNPJ")
	assert.Empty(t, rows[0].EntityCNPJ)
}

// The administrative columns are a bonus riding on this file; the park is what
// the radar stands on. The day DATASUS renames one of them, the month must
// still load without it.
func TestParseEquipmentDBC_ALayoutWithoutTheExtrasStillLoads(t *testing.T) {
	raw := buildLTDBC(t,
		[]string{"CNES", "CODUFMUN", "TIPEQUIP", "CODEQUIP", "QT_EXIST", "QT_USO", "IND_SUS", "COMPETEN"},
		[]int{7, 6, 2, 3, 4, 4, 1, 6},
		[][]string{{"9209352", "510340", "01", "11", "   2", "   1", "1", "202608"}})

	rows, err := ParseEquipmentDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].QtExist)
	assert.Empty(t, rows[0].EntityCNPJ)
}

var hbCols = []string{"CNES", "CODUFMUN", "SGRUPHAB", "NULEITOS", "CMPT_INI",
	"CMPT_FIM", "PORTARIA", "COMPETEN"}
var hbLens = []int{7, 6, 4, 3, 6, 6, 50, 6}

// A habilitation is capability the Ministry FUNDS — an ordinance, a validity
// window, and beds it pays for. Harder than a declared bed, and never read.
func TestParseHabilitationsDBC_ReadsWhatTheMinistryFunds(t *testing.T) {
	raw := buildLTDBC(t, hbCols, hbLens, [][]string{
		{"2396866", "510760", "2602", " 10", "202201", "999999", "24243/GM/MS", "202608"},
	})

	rows, err := ParseHabilitationsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2602", rows[0].Code) // UTI II NEONATAL
	assert.Equal(t, 10, rows[0].Beds)
	assert.Equal(t, "202201", rows[0].ValidFrom)
	assert.Equal(t, "999999", rows[0].ValidTo, "DATASUS writes open-ended this way")
	assert.Equal(t, "24243/GM/MS", rows[0].Ordinance)
}

// NULEITOS arrives as "2 1" in real files — the column is three wide and the
// writer pads oddly. Atoi returns zero on that, which would report a funded
// unit as having no beds.
func TestParseHabilitationsDBC_ReadsAnOddlyPaddedBedCount(t *testing.T) {
	raw := buildLTDBC(t, hbCols, hbLens, [][]string{
		{"2396866", "510760", "2602", "2 1", "202201", "999999", "X", "202608"},
	})

	rows, err := ParseHabilitationsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].Beds)
}

// The extras enrich the row; none of them IS the row. A layout without them
// must still name what the Ministry funded.
func TestParseHabilitationsDBC_SurvivesWithoutTheExtras(t *testing.T) {
	raw := buildLTDBC(t, []string{"CNES", "CODUFMUN", "SGRUPHAB", "COMPETEN"},
		[]int{7, 6, 4, 6},
		[][]string{{"2396866", "510760", "2602", "202608"}})

	rows, err := ParseHabilitationsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2602", rows[0].Code)
	assert.Zero(t, rows[0].Beds)
}

var srCols = []string{"CNES", "CODUFMUN", "SERV_ESP", "CLASS_SR", "AMB_SUS",
	"HOSP_SUS", "COMPETEN"}
var srLens = []int{7, 6, 3, 3, 1, 1, 6}

func TestParseServicesDBC_ReadsWhatTheHospitalPerforms(t *testing.T) {
	raw := buildLTDBC(t, srCols, srLens, [][]string{
		{"2396866", "510760", "122", "011", "0", "1", "202608"},
		// No classification is a real answer the dictionary names as 000 —
		// an empty column would join nothing.
		{"2396866", "510760", "123", "", "0", "0", "202608"},
	})

	rows, err := ParseServicesDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "122", rows[0].ServiceCode)
	assert.Equal(t, "011", rows[0].ClassCode)
	assert.True(t, rows[0].SUS, "hospital SUS counts as SUS")
	assert.Equal(t, "000", rows[1].ClassCode)
	assert.False(t, rows[1].SUS)
}

// THE BUG THE NEW LOG CAUGHT, in production, on the first tick after deploy:
//
//	cnes habilitations: load failed ... invalid byte sequence for encoding
//	"UTF8": 0xb0
//	catch-up loaded  beds=1232  habilitations=0  services=27371
//
// DATASUS writes these files in latin-1. Every column read until then was a
// code or a digit — pure ASCII, valid UTF-8 by accident — so the encoding
// never mattered. The habilitation register carries PORTARIA, free text, and
// "Portaria n° 1234" put a 0xb0 straight into a Postgres batch that rejected
// the whole month.
func TestParseDBC_LatinOneTextSurvivesTheTripToPostgres(t *testing.T) {
	// 0xb0 is "°" in latin-1; 0xc7/0xc3 are "Ç" and "Ã".
	portaria := string([]byte{'N', 0xb0, ' ', 'A', 0xc7, 0xc3, 'O'})
	raw := buildLTDBC(t, hbCols, hbLens, [][]string{
		{"2396866", "510760", "2602", " 10", "202201", "999999", portaria, "202608"},
	})

	rows, err := ParseHabilitationsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, utf8.ValidString(rows[0].Ordinance),
		"latin-1 must be decoded or Postgres rejects the whole batch")
	assert.Equal(t, "N° AÇÃO", rows[0].Ordinance)
}

// ASCII is valid UTF-8 and must pass through byte-identical — the conversion
// must not touch what already worked.
func TestParseDBC_ASCIIIsUntouched(t *testing.T) {
	raw := buildLTDBC(t, hbCols, hbLens, [][]string{
		{"2396866", "510760", "2602", " 10", "202201", "999999", "24243/GM/MS", "202608"},
	})

	rows, err := ParseHabilitationsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "24243/GM/MS", rows[0].Ordinance)
}

// A file that arrives already in UTF-8 must not be double-encoded.
func TestParseDBC_ValidUTF8IsNotReEncoded(t *testing.T) {
	raw := buildLTDBC(t, hbCols, hbLens, [][]string{
		{"2396866", "510760", "2602", " 10", "202201", "999999", "AÇÃO", "202608"},
	})

	rows, err := ParseHabilitationsDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "AÇÃO", rows[0].Ordinance)
}
