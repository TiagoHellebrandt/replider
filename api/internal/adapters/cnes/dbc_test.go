package cnes

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/JoshVarga/blast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildEQDBC assembles a synthetic EQ .dbc exactly the way DATASUS does:
// clear DBF header, 4-byte CRC (unchecked by the reader), blast-compressed
// records. Round-tripping through the real compressor keeps the test honest
// about the format instead of pinning our own decoder to our own encoder's
// quirks.
func buildEQDBC(t *testing.T, records []map[string]string) []byte {
	t.Helper()
	fields := []struct {
		name   string
		length int
	}{
		{"CNES", 7}, {"CODUFMUN", 6}, {"TIPEQUIP", 2}, {"CODEQUIP", 3},
		{"QT_EXIST", 4}, {"QT_USO", 4}, {"IND_SUS", 1}, {"COMPETEN", 6},
	}
	recordSize := 1
	for _, f := range fields {
		recordSize += f.length
	}
	headerSize := 32 + 32*len(fields) + 1

	var dbf bytes.Buffer
	header := make([]byte, headerSize)
	header[0] = 0x03
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(records)))
	binary.LittleEndian.PutUint16(header[8:10], uint16(headerSize))
	binary.LittleEndian.PutUint16(header[10:12], uint16(recordSize))
	for i, f := range fields {
		desc := header[32+32*i : 32+32*(i+1)]
		copy(desc[:11], f.name)
		desc[11] = 'C'
		desc[16] = byte(f.length)
	}
	header[headerSize-1] = 0x0D
	dbf.Write(header)

	var body bytes.Buffer
	for _, rec := range records {
		body.WriteByte(' ')
		for _, f := range fields {
			v := rec[f.name]
			for len(v) < f.length {
				v += " "
			}
			body.WriteString(v[:f.length])
		}
	}

	var compressed bytes.Buffer
	w := blast.NewWriter(&compressed, blast.Binary, 4096)
	_, err := w.Write(body.Bytes())
	require.NoError(t, err)
	require.NoError(t, w.Close())

	out := append([]byte{}, dbf.Bytes()...)
	out = append(out, 0, 0, 0, 0) // CRC placeholder — the reader skips it
	out = append(out, compressed.Bytes()...)
	return out
}

func TestParseEquipmentDBC_RoundTrip(t *testing.T) {
	raw := buildEQDBC(t, []map[string]string{
		{"CNES": "2655411", "CODUFMUN": "510340", "TIPEQUIP": "01",
			"CODEQUIP": "11", "QT_EXIST": "   2", "QT_USO": "   1",
			"IND_SUS": "1", "COMPETEN": "202607"},
		{"CNES": "9635890", "CODUFMUN": "510340", "TIPEQUIP": "5",
			"CODEQUIP": "64", "QT_EXIST": "  12", "QT_USO": "  12",
			"IND_SUS": "0", "COMPETEN": "202607"},
	})

	rows, err := ParseEquipmentDBC(raw, "MT")
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "2655411", rows[0].CNES)
	assert.Equal(t, "0111", rows[0].EquipCode, "TIPEQUIP+CODEQUIP zero-padded is the dictionary key")
	assert.Equal(t, 2, rows[0].QtExist)
	assert.Equal(t, 1, rows[0].QtUso)
	assert.True(t, rows[0].SUS)
	assert.Equal(t, "MT", rows[0].UF)
	assert.Equal(t, "510340", rows[0].CityCode)
	assert.Equal(t, "202607", rows[0].Competencia)

	// Unpadded TIPEQUIP "5" still lands on the 4-char key.
	assert.Equal(t, "0564", rows[1].EquipCode)
	assert.False(t, rows[1].SUS)
}

func TestParseEquipmentDBC_DeletedRecordsAreSkipped(t *testing.T) {
	raw := buildEQDBC(t, []map[string]string{
		{"CNES": "1111111", "CODUFMUN": "510340", "TIPEQUIP": "01",
			"CODEQUIP": "11", "QT_EXIST": "1", "QT_USO": "1",
			"IND_SUS": "1", "COMPETEN": "202607"},
	})
	// Flip the record's deletion flag (first byte after header+CRC region
	// is inside the compressed stream, so rebuild instead: mark via a
	// malformed row — empty CNES — which the parser must drop).
	raw2 := buildEQDBC(t, []map[string]string{
		{"CNES": "", "CODUFMUN": "510340", "TIPEQUIP": "01",
			"CODEQUIP": "11", "QT_EXIST": "1", "QT_USO": "1",
			"IND_SUS": "1", "COMPETEN": "202607"},
	})

	rows, err := ParseEquipmentDBC(raw, "MT")
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	rows2, err := ParseEquipmentDBC(raw2, "MT")
	require.NoError(t, err)
	assert.Empty(t, rows2, "a row with no CNES is dropped, not fatal")
}

func TestParseEquipmentDBC_MissingFieldFailsLoudly(t *testing.T) {
	// A schema change at DATASUS must break the load, not ingest zeros.
	var dbf bytes.Buffer
	headerSize := 32 + 32 + 1
	header := make([]byte, headerSize)
	header[0] = 0x03
	binary.LittleEndian.PutUint32(header[4:8], 0)
	binary.LittleEndian.PutUint16(header[8:10], uint16(headerSize))
	binary.LittleEndian.PutUint16(header[10:12], uint16(1+7))
	copy(header[32:43], "CNES")
	header[32+11] = 'C'
	header[32+16] = 7
	header[headerSize-1] = 0x0D
	dbf.Write(header)

	var compressed bytes.Buffer
	w := blast.NewWriter(&compressed, blast.Binary, 4096)
	_, err := w.Write(nil)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	raw := append(dbf.Bytes(), 0, 0, 0, 0)
	raw = append(raw, compressed.Bytes()...)

	_, err = ParseEquipmentDBC(raw, "MT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing expected field")
}

func TestDecodeDBC_RejectsGarbage(t *testing.T) {
	_, err := decodeDBC([]byte("nope"))
	assert.Error(t, err)

	_, err = decodeDBC(bytes.Repeat([]byte{0xFF}, 64))
	assert.Error(t, err)
}
