package domain

import (
	"errors"
	"strings"
	"time"
)

// CNES installed park (tese-agente-autonomo). Public DATASUS data: the
// monthly EQ{UF}{AAMM}.dbc file says which equipment each health
// establishment declares, and the ministry's open-data API says who the
// establishment is. None of it is account-scoped — accounts read the shared
// national tables through their own territory.

// ErrCNESCompetenciaNotPublished says the asked month's file is not on the
// FTP yet — callers walk back a month; it is not a failure.
var ErrCNESCompetenciaNotPublished = errors.New("cnes: competencia not published yet")

// CNESEquipmentRow is one establishment x equipment type x competencia.
type CNESEquipmentRow struct {
	CNES        string `json:"cnes"`
	EquipCode   string `json:"equip_code"` // TIPEQUIP zero-padded + CODEQUIP zero-padded, matches cnes_equipment_types.code
	Competencia string `json:"competencia"`
	CityCode    string `json:"city_code"`
	UF          string `json:"uf"`
	QtExist     int    `json:"qt_exist"`
	QtUso       int    `json:"qt_uso"`
	SUS         bool   `json:"sus"`
	// Administrative facts the EQ file carries on every row. They belong to
	// the ESTABLISHMENT, not to the equipment line — the loader lifts them
	// out and folds them into the establishment cache, which is why the
	// entity CNPJ no longer needs a per-id API crawl to arrive.
	OwnCNPJ     string
	EntityCNPJ  string
	AdminSphere string
}

// CNESEstablishment is the cached open-data record of who a CNES id is.
type CNESEstablishment struct {
	CNES      string  `json:"cnes"`
	TradeName string  `json:"trade_name"`
	LegalName string  `json:"legal_name"`
	CNPJ      *string `json:"cnpj,omitempty"`
	// EntityCNPJ is the MAINTAINING entity's CNPJ — who signs contracts for
	// this establishment when it has none of its own (a public hospital is
	// usually a unit of a secretaria, not a legal person).
	//
	// Never a fallback into CNPJ. Seven of Mato Grosso's twenty biggest parks
	// share 57252971000146; treating that as the hospital's own id would make
	// seven buyers look like one, and would attribute the secretaria's
	// contracts to each hospital individually.
	EntityCNPJ        *string  `json:"entity_cnpj,omitempty"`
	Address           string   `json:"address"`
	District          string   `json:"district"`
	CityCode          *string  `json:"city_code,omitempty"`
	Phone             *string  `json:"phone,omitempty"`
	Email             *string  `json:"email,omitempty"`
	Latitude          *float64 `json:"latitude,omitempty"`
	Longitude         *float64 `json:"longitude,omitempty"`
	UnitTypeCode      *string  `json:"unit_type_code,omitempty"`
	HasSurgicalCenter *bool    `json:"has_surgical_center,omitempty"`
	HasHospitalCare   *bool    `json:"has_hospital_care,omitempty"`
	// HasNeonatalCentre and HasObstetricCentre are the qualifiers a neonatal
	// catalogue is sold against. Published in the same record as the surgical
	// centre, which was stored while these two were dropped.
	HasNeonatalCentre  *bool `json:"has_neonatal_centre,omitempty"`
	HasObstetricCentre *bool `json:"has_obstetric_centre,omitempty"`
	// AdminSphere is MUNICIPAL / ESTADUAL / FEDERAL / PRIVADA — the fact that
	// decides whether this buyer buys through a published tender or decides
	// on its own.
	AdminSphere string `json:"admin_sphere,omitempty"`
	// DisabledReason is why the record was deactivated; empty means
	// operating. Equipment and bed rows outlive a closure, so without this
	// the hunt keeps offering hospitals that are shut.
	DisabledReason string     `json:"disabled_reason,omitempty"`
	CEP            string     `json:"cep,omitempty"`
	FetchedAt      *time.Time `json:"fetched_at,omitempty"`
}

// CNESBedRow is one line of the bed census as the LT file publishes it.
type CNESBedRow struct {
	CNES        string
	BedType     string // TP_LEITO, the ministry's coarse grouping
	BedCode     string // CODLEITO, the specialty; joins cnes_bed_types
	Competencia string
	CityCode    string
	UF          string
	QtExist     int
	QtSUS       int
	QtContr     int
}

// CNESHabilitationRow is one line of the HB file: capability the Ministry
// funds, against a named ordinance and a validity window.
type CNESHabilitationRow struct {
	CNES        string
	Code        string
	Competencia string
	CityCode    string
	UF          string
	// Beds the ordinance funds. Zero is common — many habilitations are not
	// bed-based — and must never be read as "no capacity".
	Beds      int
	ValidFrom string
	// ValidTo is the ministry's end competencia. Open-ended is written
	// "999999" — 403 of Mato Grosso's 429 habilitations, measured 2026-09-22.
	//
	// Migration 000243's comment says "399999" and is WRONG: that reading
	// came from a probe that sliced records one byte short, before the
	// deletion flag was accounted for. The migration is applied and is not
	// edited; this is where the fact lives, because this is what code reads.
	ValidTo   string
	Ordinance string
}

// CNESHabilitationItem is one habilitation joined with the ministry's
// dictionary, as a screen renders it.
type CNESHabilitationItem struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Beds       int    `json:"beds,omitempty"`
	IsICU      bool   `json:"is_icu"`
	IsNeonatal bool   `json:"is_neonatal"`
}

// CNESServiceRow is one line of the SR file: a service the establishment says
// it performs.
type CNESServiceRow struct {
	CNES        string
	ServiceCode string
	ClassCode   string
	Competencia string
	CityCode    string
	UF          string
	SUS         bool
}

// CNESServiceItem is one service joined with the ministry's dictionary.
type CNESServiceItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
	SUS  bool   `json:"sus"`
}

// CNESBedItem is one line of a hospital's bed census, joined with the
// ministry's dictionary — what the panel and the dossier render.
//
// ICU and Neonatal ride ON the row rather than being re-derived by the
// caller, because deriving them is exactly what went wrong twice while this
// was being built: TP_LEITO 4 reads as intensive care and means obstetric,
// and code 86 reads as neonatal and means coronary. The dictionary that ships
// with the data answers both; a constant in a caller drifts from it.
type CNESBedItem struct {
	BedCode    string `json:"bed_code"`
	Name       string `json:"name"`
	QtExist    int    `json:"qt_exist"`
	QtSUS      int    `json:"qt_sus"`
	IsICU      bool   `json:"is_icu"`
	IsNeonatal bool   `json:"is_neonatal"`
}

// CNESParkItem is one line of an establishment's park, joined with the
// dictionary — what the dossier, the MCP tool and the future report render.
type CNESParkItem struct {
	EquipCode string `json:"equip_code"`
	Name      string `json:"name"`
	GroupName string `json:"group_name"`
	QtExist   int    `json:"qt_exist"`
	QtUso     int    `json:"qt_uso"`
	SUS       bool   `json:"sus"`
}

// CNESParkRepository is the port the worker writes through and the product
// reads through. Implemented by postgres.CNESParkRepository (raw SQL).
type CNESParkRepository interface {
	// HasRun answers whether (uf, competencia) was already ingested.
	HasRun(uf, competencia string) (bool, error)
	// ReplaceCompetencia upserts every row of one UF+competencia load and
	// stamps the ingest run, in one transaction.
	ReplaceCompetencia(uf, competencia string, rows []CNESEquipmentRow) error
	// PruneOldCompetencias keeps the newest `keep` competencias per UF.
	PruneOldCompetencias(uf string, keep int) error

	// ParkByCNES returns the newest-competencia park of one establishment.
	ParkByCNES(cnes string) ([]CNESParkItem, string, error)
	// ParkForCNESIn is ParkByCNES for MANY establishments in one query,
	// restricted to the given equipment codes (empty = every code). The
	// radar shows a page of hospitals and has to say what EACH one holds;
	// one round trip per hospital would be twenty-five queries a paint.
	// Keyed by CNES id; an establishment with nothing matching is absent
	// rather than present-and-empty.
	ParkForCNESIn(cnesIDs, equipCodes []string) (map[string][]CNESParkItem, string, error)

	// UpsertEstablishment caches one open-data record.
	UpsertEstablishment(e *CNESEstablishment) error
	// GetEstablishment returns the cached record or nil.
	GetEstablishment(cnes string) (*CNESEstablishment, error)
	// SearchEstablishments finds cached establishments by name fragment.
	SearchEstablishments(query string, limit int) ([]CNESEstablishment, error)
	// MissingEstablishments lists CNES ids present in cnes_equipment for the
	// UF but absent (or stale) in the establishment cache.
	MissingEstablishments(uf string, staleBefore time.Time, limit int) ([]string, error)

	// FindEstablishmentByPhoneDigits matches a cached establishment by the
	// digits of its phone (the lead↔CNES bridge: WhatsApp gives us digits,
	// the registry gives us a formatted landline).
	FindEstablishmentByPhoneDigits(digits string) (*CNESEstablishment, error)
	// ResolveEquipmentCodes finds dictionary codes whose name matches a
	// fragment ("ventilador" → 0564).
	ResolveEquipmentCodes(nameQuery string) ([]CNESEquipmentType, error)
	// EstablishmentsWithEquipment lists the UF's establishments holding
	// ≥minCount units across the given codes, biggest park first, joined
	// with the cached record when one exists.
	EstablishmentsWithEquipment(uf string, equipCodes []string, minCount, limit int) ([]CNESEquipmentHolder, error)

	// ResolveEquipmentCodesAny is ResolveEquipmentCodes over many words in
	// ONE query (the radar resolves a whole catalogue per request).
	ResolveEquipmentCodesAny(words []string) ([]CNESEquipmentType, error)
	// CityCodesByName turns territory city NAMES into the 6-digit IBGE
	// codes cnes_equipment.city_code carries (accent/case-insensitive).
	// Unknown names are silently absent — the caller decides whether an
	// unresolvable territory falls back to the whole UF.
	CityCodesByName(uf string, names []string) ([]string, error)
	// EstablishmentsWithEquipmentIn is the hunt restricted to city codes
	// (empty = whole UF) and also answers WHICH competencia was counted.
	// offset is where in the park ranking to read from. A sweep that only
	// ever reads the top cannot reach past it, whatever the territory holds.
	EstablishmentsWithEquipmentIn(uf string, cityCodes, equipCodes []string, minCount, limit, offset int) ([]CNESEquipmentHolder, string, error)
	// CountEquipmentIn is CountEquipment restricted to city codes (empty =
	// whole UF).
	CountEquipmentIn(uf string, cityCodes, equipCodes []string) (establishments, units int, err error)
}

// CompetenciaYearMonth renders a DATASUS competencia ("202607") as the
// ISO-ish "2026-07" the API exposes. Anything that is not AAAAMM comes
// back unchanged — better a raw stamp than a fabricated date.
func CompetenciaYearMonth(comp string) string {
	if len(comp) != 6 {
		return comp
	}
	return comp[:4] + "-" + comp[4:]
}

// CNESUnknownCodeName is how an equipment code with no dictionary row is
// rendered everywhere: the raw code, never a guessed name. The 000219
// group-only rows use "Código NNNN" as name; IsCNESRawCodeName recognises
// both so views can add the group instead of pretending it is a name.
func CNESUnknownCodeName(code string) string { return "código CNES " + code }

// IsCNESRawCodeName says whether a park item's Name is a raw code
// placeholder (unknown to the dictionary) rather than an equipment name.
func IsCNESRawCodeName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(n, "código cnes ") || strings.HasPrefix(n, "código ")
}

// CNESEquipmentType is one dictionary row.
type CNESEquipmentType struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	GroupName string `json:"group_name"`
}

// CNESEquipmentHolder is one hunt result: an establishment and how much of
// the asked equipment it declares. Establishment is nil when the cache has
// not caught up with that CNES id yet.
type CNESEquipmentHolder struct {
	CNES          string             `json:"cnes"`
	CityCode      string             `json:"city_code"`
	Total         int                `json:"total"`
	Establishment *CNESEstablishment `json:"establishment,omitempty"`
}
