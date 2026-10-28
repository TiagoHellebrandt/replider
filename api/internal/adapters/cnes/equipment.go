package cnes

import (
	"fmt"
	"strconv"

	"github.com/replider/api/internal/core/domain"
)

// ParseEquipmentDBC turns a raw EQ{UF}{AAMM}.dbc into equipment rows.
// uf is taken from the caller (it names the file; the records only carry
// the municipality code).
func ParseEquipmentDBC(raw []byte, uf string) ([]domain.CNESEquipmentRow, error) {
	t, err := decodeDBC(raw)
	if err != nil {
		return nil, err
	}
	idx, err := t.fieldIndex("CNES", "CODUFMUN", "TIPEQUIP", "CODEQUIP",
		"QT_EXIST", "QT_USO", "IND_SUS", "COMPETEN")
	if err != nil {
		return nil, err
	}
	// Administrative facts the file has always carried and this parser always
	// skipped. CPF_CNPJ is the establishment's own number, CNPJ_MAN the
	// maintaining entity's — the key the radar needs to read a public
	// hospital's published contracts, and which the loader was otherwise
	// crawling the open-data API for, one request per establishment at three
	// per second.
	//
	// OPTIONAL, unlike the eight above. They are a bonus riding on this file;
	// the park is what the radar stands on, so the day DATASUS renames one of
	// them the month must still load without it.
	own, hasOwn := t.optionalField("CPF_CNPJ")
	entity, hasEntity := t.optionalField("CNPJ_MAN")
	sphere, hasSphere := t.optionalField("ESFERA_A")

	rows := make([]domain.CNESEquipmentRow, 0, len(t.records))
	for _, rec := range t.records {
		code := equipCode(value(rec, idx["TIPEQUIP"]), value(rec, idx["CODEQUIP"]))
		cnes := value(rec, idx["CNES"])
		comp := value(rec, idx["COMPETEN"])
		if code == "" || cnes == "" || len(comp) != 6 {
			// A malformed row is dropped, not fatal: one bad record must
			// not lose the other 26 thousand.
			continue
		}
		rows = append(rows, domain.CNESEquipmentRow{
			CNES:        cnes,
			EquipCode:   code,
			Competencia: comp,
			CityCode:    value(rec, idx["CODUFMUN"]),
			UF:          uf,
			QtExist:     atoiSafe(value(rec, idx["QT_EXIST"])),
			QtUso:       atoiSafe(value(rec, idx["QT_USO"])),
			SUS:         value(rec, idx["IND_SUS"]) == "1",
			OwnCNPJ:     optCNPJ(rec, own, hasOwn),
			EntityCNPJ:  optCNPJ(rec, entity, hasEntity),
			AdminSphere: optValue(rec, sphere, hasSphere),
		})
	}
	return rows, nil
}

// equipCode joins TIPEQUIP + CODEQUIP into the 4-char dictionary key
// ("04" + "41" → "0441"). The DBF stores both unpadded sometimes.
func equipCode(tip, cod string) string {
	if tip == "" || cod == "" {
		return ""
	}
	return fmt.Sprintf("%02s%02s", tip, cod)
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// cnpjOrEmpty keeps a registry number only when it is one.
//
// The DBF pads the column and fills an absent value with zeros, so "" and
// "00000000000000" both mean "not published" — and the second one, passed
// through, would be looked up as a real CNPJ and find nothing, which reads
// on screen as "this buyer has no contracts" rather than "we have no key".
func cnpjOrEmpty(raw string) string {
	digits := make([]rune, 0, len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) != 14 {
		return ""
	}
	for _, r := range digits {
		if r != '0' {
			return string(digits)
		}
	}
	return ""
}

// optValue reads a field that may not exist in this month's layout.
func optValue(rec []byte, f dbfField, ok bool) string {
	if !ok {
		return ""
	}
	return value(rec, f)
}

func optCNPJ(rec []byte, f dbfField, ok bool) string {
	return cnpjOrEmpty(optValue(rec, f, ok))
}
