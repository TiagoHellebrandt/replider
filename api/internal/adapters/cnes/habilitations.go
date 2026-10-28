package cnes

import (
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Capability the Ministry FUNDS, which is a harder signal than capability the
// hospital declares.
//
// A bed in the census is what the establishment says it has. A habilitation is
// an ordinance: the Ministry pays against it, names it, dates it, and the
// hospital is required to hold the equipment that entitles it. Measured over
// HB/MT on 2026-09-18: 429 rows across 191 hospitals, 548 funded beds, 120 of
// them neonatal.

// ParseHabilitationsDBC turns a raw HB{UF}{AAMM}.dbc into habilitation rows.
func ParseHabilitationsDBC(raw []byte, uf string) ([]domain.CNESHabilitationRow, error) {
	t, err := decodeDBC(raw)
	if err != nil {
		return nil, err
	}
	// Loud on a missing column, like every other load here: a layout change
	// must break rather than ingest zeros.
	idx, err := t.fieldIndex("CNES", "CODUFMUN", "SGRUPHAB", "COMPETEN")
	if err != nil {
		return nil, err
	}
	// The rest are optional. They enrich the row; none of them is the row.
	beds, hasBeds := t.optionalField("NULEITOS")
	from, hasFrom := t.optionalField("CMPT_INI")
	to, hasTo := t.optionalField("CMPT_FIM")
	ord, hasOrd := t.optionalField("PORTARIA")

	rows := make([]domain.CNESHabilitationRow, 0, len(t.records))
	for _, rec := range t.records {
		cnes := value(rec, idx["CNES"])
		code := value(rec, idx["SGRUPHAB"])
		comp := value(rec, idx["COMPETEN"])
		if cnes == "" || code == "" || len(comp) != 6 {
			continue
		}
		rows = append(rows, domain.CNESHabilitationRow{
			CNES:        cnes,
			Code:        code,
			Competencia: comp,
			CityCode:    value(rec, idx["CODUFMUN"]),
			UF:          uf,
			Beds:        leadingInt(optValue(rec, beds, hasBeds)),
			ValidFrom:   optValue(rec, from, hasFrom),
			ValidTo:     optValue(rec, to, hasTo),
			Ordinance:   optValue(rec, ord, hasOrd),
		})
	}
	return rows, nil
}

// leadingInt reads the first whole number out of a column DATASUS does not
// always write as one.
//
// NULEITOS arrives as "2 1" in real files — the field is three characters
// wide and the writer pads oddly. `strconv.Atoi` on that returns zero, which
// would report a funded unit as having no beds; taking the leading number
// reports two, which is what the ordinance says.
func leadingInt(s string) int {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0
	}
	return atoiSafe(f[0])
}
