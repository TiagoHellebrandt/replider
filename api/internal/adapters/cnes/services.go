package cnes

import (
	"github.com/replider/api/internal/core/domain"
)

// What the establishment says it PERFORMS.
//
// The park says what it owns, the census how many beds it runs, the
// habilitation what the Ministry funds — and this says which services it
// actually delivers. It is the qualifier that separates a hospital running a
// neonatal service from one that happens to have a cot. Measured over SR/MT
// on 2026-09-18: 26.917 rows across 5.483 establishments.

// ParseServicesDBC turns a raw SR{UF}{AAMM}.dbc into service rows.
func ParseServicesDBC(raw []byte, uf string) ([]domain.CNESServiceRow, error) {
	t, err := decodeDBC(raw)
	if err != nil {
		return nil, err
	}
	idx, err := t.fieldIndex("CNES", "CODUFMUN", "SERV_ESP", "CLASS_SR", "COMPETEN")
	if err != nil {
		return nil, err
	}
	// Which budget pays for it. Optional: a layout without them still names
	// the service, which is the point of the row.
	ambSUS, hasAmb := t.optionalField("AMB_SUS")
	hospSUS, hasHosp := t.optionalField("HOSP_SUS")

	rows := make([]domain.CNESServiceRow, 0, len(t.records))
	for _, rec := range t.records {
		cnes := value(rec, idx["CNES"])
		serv := value(rec, idx["SERV_ESP"])
		class := value(rec, idx["CLASS_SR"])
		comp := value(rec, idx["COMPETEN"])
		if cnes == "" || serv == "" || len(comp) != 6 {
			continue
		}
		if class == "" {
			// "no classification" is a real answer the dictionary names as
			// 000; an empty column would join nothing.
			class = "000"
		}
		rows = append(rows, domain.CNESServiceRow{
			CNES:        cnes,
			ServiceCode: serv,
			ClassCode:   class,
			Competencia: comp,
			CityCode:    value(rec, idx["CODUFMUN"]),
			UF:          uf,
			SUS: optValue(rec, ambSUS, hasAmb) == "1" ||
				optValue(rec, hospSUS, hasHosp) == "1",
		})
	}
	return rows, nil
}
