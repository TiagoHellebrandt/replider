package cnes

import (
	"github.com/replider/api/internal/core/domain"
)

// The bed census: what a hospital is BUILT to treat.
//
// The installed park says which machines are in the building; this says how
// much care the place is set up to deliver, which is the number that predicts
// demand. Twenty neonatal intensive-care beds means twenty incubators, twenty
// monitors and the ventilators to match, on a replacement cycle. The park
// alone never says that — a hospital can hold six incubators and run a
// twenty-cot unit.
//
// Measured over LT/MT on 2026-09-18: 308 neonatal beds across 29 hospitals.

// ParseBedsDBC turns a raw LT{UF}{AAMM}.dbc into bed rows. uf comes from the
// caller for the same reason it does in the equipment parser: it names the
// file, and the records only carry the municipality code.
func ParseBedsDBC(raw []byte, uf string) ([]domain.CNESBedRow, error) {
	t, err := decodeDBC(raw)
	if err != nil {
		return nil, err
	}
	// Loud on a missing column, like the equipment load: a DATASUS layout
	// change must break the load rather than quietly ingest zeros.
	idx, err := t.fieldIndex("CNES", "CODUFMUN", "TP_LEITO", "CODLEITO",
		"QT_EXIST", "QT_SUS", "QT_CONTR", "COMPETEN")
	if err != nil {
		return nil, err
	}

	rows := make([]domain.CNESBedRow, 0, len(t.records))
	for _, rec := range t.records {
		cnes := value(rec, idx["CNES"])
		comp := value(rec, idx["COMPETEN"])
		bedType := value(rec, idx["TP_LEITO"])
		bedCode := bedSpecialty(value(rec, idx["CODLEITO"]))
		if cnes == "" || bedType == "" || bedCode == "" || len(comp) != 6 {
			// One malformed record must not lose the other twelve hundred.
			continue
		}
		rows = append(rows, domain.CNESBedRow{
			CNES:        cnes,
			BedType:     bedType,
			BedCode:     bedCode,
			Competencia: comp,
			CityCode:    value(rec, idx["CODUFMUN"]),
			UF:          uf,
			QtExist:     atoiSafe(value(rec, idx["QT_EXIST"])),
			QtSUS:       atoiSafe(value(rec, idx["QT_SUS"])),
			QtContr:     atoiSafe(value(rec, idx["QT_CONTR"])),
		})
	}
	return rows, nil
}

// bedSpecialty pads CODLEITO to the dictionary's two-character key.
//
// The DBF stores it unpadded often enough to matter: "3" is specialty 03
// (cirurgia geral), and left as "3" it joins nothing. The same shape the
// equipment code already had to handle.
func bedSpecialty(cod string) string {
	switch len(cod) {
	case 0:
		return ""
	case 1:
		return "0" + cod
	default:
		return cod
	}
}
