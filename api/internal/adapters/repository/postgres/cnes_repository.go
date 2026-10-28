package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// CNESParkRepository is a raw-SQL repo (NOT sqlc) over the shared pool for
// the public DATASUS tables (migration 000202). Raw SQL because the tables
// are not account-scoped and the write path is a bulk worker load — none of
// the sqlc repos' tenancy conventions apply.
//
// It takes the POOL (not DBTX): ReplaceCompetencia needs its own
// transaction so a half-loaded month can never be stamped as ingested.
type CNESParkRepository struct {
	pool *pgxpool.Pool
}

func NewCNESParkRepository(pool *pgxpool.Pool) *CNESParkRepository {
	return &CNESParkRepository{pool: pool}
}

func (r *CNESParkRepository) HasRun(uf, competencia string) (bool, error) {
	var one int
	err := r.pool.QueryRow(context.Background(),
		`SELECT 1 FROM cnes_ingest_runs WHERE uf = $1 AND competencia = $2`,
		uf, competencia).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ReplaceCompetencia loads one UF+competencia atomically: rows upserted in
// batches AND the ingest-run stamp written in the same transaction, so a
// crash mid-load re-runs the whole month instead of half-counting it.
func (r *CNESParkRepository) ReplaceCompetencia(uf, competencia string, rows []domain.CNESEquipmentRow) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	batch := &pgx.Batch{}
	flush := func() error {
		if batch.Len() == 0 {
			return nil
		}
		br := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close() //nolint:errcheck,gosec // surfacing the exec error, not the close
				return err
			}
		}
		batch = &pgx.Batch{}
		return br.Close()
	}
	for _, row := range rows {
		batch.Queue(`INSERT INTO cnes_equipment
			(cnes, equip_code, competencia, city_code, uf, qt_exist, qt_uso, sus, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			ON CONFLICT (cnes, equip_code, competencia) DO UPDATE
			SET qt_exist = EXCLUDED.qt_exist, qt_uso = EXCLUDED.qt_uso,
			    sus = EXCLUDED.sus, city_code = EXCLUDED.city_code,
			    updated_at = NOW()`,
			row.CNES, row.EquipCode, row.Competencia, row.CityCode, row.UF,
			row.QtExist, row.QtUso, row.SUS)
		if batch.Len() >= 500 {
			if err := flush(); err != nil {
				return fmt.Errorf("cnes equipment batch: %w", err)
			}
		}
	}
	if err := flush(); err != nil {
		return fmt.Errorf("cnes equipment batch: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO cnes_ingest_runs (uf, competencia, rows_loaded)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (uf, competencia) DO UPDATE
		 SET rows_loaded = EXCLUDED.rows_loaded, finished_at = NOW()`,
		uf, competencia, len(rows)); err != nil {
		return fmt.Errorf("cnes ingest stamp: %w", err)
	}
	return tx.Commit(ctx)
}

// PruneOldCompetencias keeps the newest `keep` competencias of one UF —
// two are enough for "the hospital gained a tomograph" deltas later.
func (r *CNESParkRepository) PruneOldCompetencias(uf string, keep int) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `
		DELETE FROM cnes_equipment
		WHERE uf = $1 AND competencia NOT IN (
			SELECT DISTINCT competencia FROM cnes_equipment
			WHERE uf = $1 ORDER BY competencia DESC LIMIT $2
		)`, uf, keep)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		DELETE FROM cnes_ingest_runs
		WHERE uf = $1 AND competencia NOT IN (
			SELECT DISTINCT competencia FROM cnes_ingest_runs
			WHERE uf = $1 ORDER BY competencia DESC LIMIT $2
		)`, uf, keep)
	return err
}

// ParkByCNES returns the establishment's park at its newest loaded
// competencia, joined with the dictionary. Unknown codes still show, with
// the raw code as name — a new ministry code must not hide equipment.
func (r *CNESParkRepository) ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error) {
	ctx := context.Background()
	var comp string
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(competencia) FROM cnes_equipment WHERE cnes = $1`, cnes).Scan(&comp)
	if err == pgx.ErrNoRows || comp == "" {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT e.equip_code, COALESCE(t.name, 'código CNES ' || e.equip_code), COALESCE(t.group_name, ''),
		       e.qt_exist, e.qt_uso, e.sus
		FROM cnes_equipment e
		LEFT JOIN cnes_equipment_types t ON t.code = e.equip_code
		WHERE e.cnes = $1 AND e.competencia = $2
		ORDER BY COALESCE(t.group_name, ''), e.equip_code`, cnes, comp)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []domain.CNESParkItem
	for rows.Next() {
		var it domain.CNESParkItem
		if err := rows.Scan(&it.EquipCode, &it.Name, &it.GroupName, &it.QtExist, &it.QtUso, &it.SUS); err != nil {
			return nil, "", err
		}
		out = append(out, it)
	}
	return out, comp, rows.Err()
}

// ParkForCNESIn reads the park of many establishments at once. The
// competencia is taken per-UF-wide MAX over the requested ids rather than
// per establishment: the radar cites ONE month on the whole page, and a
// straggler hospital that only ever reported an older month must not drag
// the page's label backwards — it simply contributes no row.
func (r *CNESParkRepository) ParkForCNESIn(cnesIDs, equipCodes []string) (map[string][]domain.CNESParkItem, string, error) {
	if len(cnesIDs) == 0 {
		return map[string][]domain.CNESParkItem{}, "", nil
	}
	ctx := context.Background()
	var comp *string
	if err := r.pool.QueryRow(ctx,
		`SELECT MAX(competencia) FROM cnes_equipment WHERE cnes = ANY($1)`, cnesIDs).Scan(&comp); err != nil {
		if err == pgx.ErrNoRows {
			return map[string][]domain.CNESParkItem{}, "", nil
		}
		return nil, "", err
	}
	if comp == nil || *comp == "" {
		return map[string][]domain.CNESParkItem{}, "", nil
	}
	if equipCodes == nil {
		equipCodes = []string{}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT e.cnes, e.equip_code, COALESCE(t.name, 'código CNES ' || e.equip_code),
		       COALESCE(t.group_name, ''), e.qt_exist, e.qt_uso, e.sus
		FROM cnes_equipment e
		LEFT JOIN cnes_equipment_types t ON t.code = e.equip_code
		WHERE e.cnes = ANY($1) AND e.competencia = $2
		  AND (cardinality($3::text[]) = 0 OR e.equip_code = ANY($3))
		  AND e.qt_exist > 0
		ORDER BY e.cnes, e.qt_exist DESC, e.equip_code`, cnesIDs, *comp, equipCodes)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := map[string][]domain.CNESParkItem{}
	for rows.Next() {
		var cnes string
		var it domain.CNESParkItem
		if err := rows.Scan(&cnes, &it.EquipCode, &it.Name, &it.GroupName,
			&it.QtExist, &it.QtUso, &it.SUS); err != nil {
			return nil, "", err
		}
		out[cnes] = append(out[cnes], it)
	}
	return out, *comp, rows.Err()
}

func (r *CNESParkRepository) UpsertEstablishment(e *domain.CNESEstablishment) error {
	_, err := r.pool.Exec(context.Background(), `
		INSERT INTO cnes_establishments
			(cnes, trade_name, legal_name, cnpj, cnpj_entidade, address, district,
			 city_code, phone, email, latitude, longitude, unit_type_code,
			 has_surgical_center, has_hospital_care, has_neonatal_centre,
			 has_obstetric_centre, admin_sphere, disabled_reason, cep, fetched_at)
		VALUES ($1,$2,$3,$4,$15,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$16,$17,$18,$19,$20,NOW())
		ON CONFLICT (cnes) DO UPDATE SET
			trade_name = EXCLUDED.trade_name, legal_name = EXCLUDED.legal_name,
			cnpj = EXCLUDED.cnpj, cnpj_entidade = EXCLUDED.cnpj_entidade,
			address = EXCLUDED.address,
			district = EXCLUDED.district, city_code = EXCLUDED.city_code,
			phone = EXCLUDED.phone, email = EXCLUDED.email,
			latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude,
			unit_type_code = EXCLUDED.unit_type_code,
			has_surgical_center = EXCLUDED.has_surgical_center,
			has_hospital_care = EXCLUDED.has_hospital_care,
			has_neonatal_centre = EXCLUDED.has_neonatal_centre,
			has_obstetric_centre = EXCLUDED.has_obstetric_centre,
			admin_sphere = EXCLUDED.admin_sphere,
			disabled_reason = EXCLUDED.disabled_reason,
			cep = EXCLUDED.cep,
			fetched_at = NOW()`,
		e.CNES, e.TradeName, e.LegalName, e.CNPJ, e.Address, e.District,
		e.CityCode, e.Phone, e.Email, e.Latitude, e.Longitude,
		e.UnitTypeCode, e.HasSurgicalCenter, e.HasHospitalCare, e.EntityCNPJ,
		e.HasNeonatalCentre, e.HasObstetricCentre, e.AdminSphere,
		e.DisabledReason, e.CEP)
	return err
}

func (r *CNESParkRepository) GetEstablishment(cnes string) (*domain.CNESEstablishment, error) {
	row := r.pool.QueryRow(context.Background(), `
		SELECT cnes, trade_name, legal_name, cnpj, cnpj_entidade, address, district,
		       city_code, phone, email, latitude, longitude, unit_type_code,
		       has_surgical_center, has_hospital_care, has_neonatal_centre,
		       has_obstetric_centre, COALESCE(admin_sphere, ''),
		       COALESCE(disabled_reason, ''), COALESCE(cep, ''), fetched_at
		FROM cnes_establishments WHERE cnes = $1`, cnes)
	e, err := scanEstablishment(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return e, err
}

func (r *CNESParkRepository) SearchEstablishments(query string, limit int) ([]domain.CNESEstablishment, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := r.pool.Query(context.Background(), `
		SELECT cnes, trade_name, legal_name, cnpj, cnpj_entidade, address, district,
		       city_code, phone, email, latitude, longitude, unit_type_code,
		       has_surgical_center, has_hospital_care, has_neonatal_centre,
		       has_obstetric_centre, COALESCE(admin_sphere, ''),
		       COALESCE(disabled_reason, ''), COALESCE(cep, ''), fetched_at
		FROM cnes_establishments
		WHERE trade_name ILIKE '%' || $1 || '%' OR legal_name ILIKE '%' || $1 || '%'
		ORDER BY trade_name LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CNESEstablishment
	for rows.Next() {
		e, err := scanEstablishment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// MissingEstablishments lists CNES ids that have equipment loaded for the
// UF but whose establishment record is absent or older than staleBefore.
//
// Order matters: the cache fills a bounded batch per day, so a big UF
// takes weeks. Ids that still have NO name come first, biggest park in
// the newest competencia first — those are the Radar's top rows, and
// "Estabelecimento CNES 1234567" is exactly what the seller must not see
// for weeks. Stale-but-named rows follow, by cnes.
func (r *CNESParkRepository) MissingEstablishments(uf string, staleBefore time.Time, limit int) ([]string, error) {
	rows, err := r.pool.Query(context.Background(), `
		WITH newest AS (
			SELECT MAX(competencia) AS comp FROM cnes_equipment WHERE uf = $1
		)
		SELECT e.cnes
		FROM cnes_equipment e
		CROSS JOIN newest
		LEFT JOIN cnes_establishments s ON s.cnes = e.cnes
		WHERE e.uf = $1 AND (s.cnes IS NULL OR s.fetched_at < $2)
		GROUP BY e.cnes, s.cnes, s.trade_name
		ORDER BY
			CASE WHEN s.cnes IS NULL OR COALESCE(s.trade_name, '') = '' THEN 0 ELSE 1 END,
			CASE WHEN s.cnes IS NULL OR COALESCE(s.trade_name, '') = ''
			     THEN -COALESCE(SUM(e.qt_exist) FILTER (WHERE e.competencia = newest.comp), 0)
			     ELSE 0 END,
			e.cnes
		LIMIT $3`, uf, staleBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindEstablishmentByPhoneDigits matches on the digits of the cached phone.
// The registry stores "65 21216363"; a WhatsApp contact arrives as
// 556521216363 — the caller strips the country code, this strips the
// formatting, and the comparison happens digit-to-digit.
func (r *CNESParkRepository) FindEstablishmentByPhoneDigits(digits string) (*domain.CNESEstablishment, error) {
	if len(digits) < 8 {
		return nil, nil // too short to be a phone; never match the world
	}
	row := r.pool.QueryRow(context.Background(), `
		SELECT cnes, trade_name, legal_name, cnpj, cnpj_entidade, address, district,
		       city_code, phone, email, latitude, longitude, unit_type_code,
		       has_surgical_center, has_hospital_care, has_neonatal_centre,
		       has_obstetric_centre, COALESCE(admin_sphere, ''),
		       COALESCE(disabled_reason, ''), COALESCE(cep, ''), fetched_at
		FROM cnes_establishments
		WHERE phone IS NOT NULL
		  AND regexp_replace(phone, '\D', '', 'g') = $1
		LIMIT 1`, digits)
	e, err := scanEstablishment(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// FindEstablishmentByCNPJ bridges a published tender to the registry.
//
// A notice names its buyer by CNPJ and nothing else — no phone, no address,
// no idea what the buyer already owns. The registry files 2.714 of Mato
// Grosso's establishments under a CNPJ, so for a buyer that IS an
// establishment the two records are the same organization and the park, the
// phone and the e-mail come along.
//
// A CNPJ carried by more than one establishment answers NOTHING. A city hall
// registers a dozen units under its own number, and picking one of them would
// attach one clinic's equipment to a tender the whole município published —
// the same rule that stops the dossier guessing between two "Santa Casa"s.
func (r *CNESParkRepository) FindEstablishmentByCNPJ(cnpj string) (*domain.CNESEstablishment, error) {
	digits := onlyDigits(cnpj)
	if len(digits) != 14 {
		return nil, nil
	}
	rows, err := r.pool.Query(context.Background(), `
		SELECT cnes, trade_name, legal_name, cnpj, cnpj_entidade, address, district,
		       city_code, phone, email, latitude, longitude, unit_type_code,
		       has_surgical_center, has_hospital_care, has_neonatal_centre,
		       has_obstetric_centre, COALESCE(admin_sphere, ''),
		       COALESCE(disabled_reason, ''), COALESCE(cep, ''), fetched_at
		FROM cnes_establishments
		WHERE regexp_replace(COALESCE(cnpj, ''), '\D', '', 'g') = $1
		LIMIT 2`, digits)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.CNESEstablishment
	for rows.Next() {
		e, err := scanEstablishment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) != 1 {
		return nil, nil
	}
	return out[0], nil
}

// onlyDigits keeps the digits of a string, for registry identifiers written
// with punctuation.
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ResolveEquipmentCodes turns a human word into dictionary codes.
func (r *CNESParkRepository) ResolveEquipmentCodes(nameQuery string) ([]domain.CNESEquipmentType, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT code, name, group_name FROM cnes_equipment_types
		WHERE name ILIKE '%' || $1 || '%' OR group_name ILIKE '%' || $1 || '%'
		ORDER BY code`, nameQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CNESEquipmentType
	for rows.Next() {
		var t domain.CNESEquipmentType
		if err := rows.Scan(&t.Code, &t.Name, &t.GroupName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ResolveEquipmentCodesAny resolves many words in ONE round trip — the
// radar used to fire one ILIKE per catalogue word per request.
func (r *CNESParkRepository) ResolveEquipmentCodesAny(words []string) ([]domain.CNESEquipmentType, error) {
	clean := make([]string, 0, len(words))
	for _, w := range words {
		if w = strings.TrimSpace(w); w != "" {
			clean = append(clean, w)
		}
	}
	if len(clean) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(context.Background(), `
		SELECT t.code, t.name, t.group_name FROM cnes_equipment_types t
		WHERE EXISTS (
			SELECT 1 FROM unnest($1::text[]) AS w
			WHERE t.name ILIKE '%' || w || '%' OR t.group_name ILIKE '%' || w || '%'
		)
		ORDER BY t.code`, clean)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CNESEquipmentType
	for rows.Next() {
		var t domain.CNESEquipmentType
		if err := rows.Scan(&t.Code, &t.Name, &t.GroupName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CityCodesByName bridges accounts.territory.cities (names the seller
// typed) to cnes_equipment.city_code (6-digit IBGE) through the
// ibge_municipalities seed (migration 000220). Matching is on the folded
// name, so "Cuiaba", "CUIABÁ" and "Cuiabá" are the same city.
func (r *CNESParkRepository) CityCodesByName(uf string, names []string) ([]string, error) {
	norm := make([]string, 0, len(names))
	for _, n := range names {
		if f := foldCityName(n); f != "" {
			norm = append(norm, f)
		}
	}
	if len(norm) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(context.Background(), `
		SELECT code FROM ibge_municipalities
		WHERE uf = $1 AND name_norm = ANY($2)
		ORDER BY code`, strings.ToUpper(strings.TrimSpace(uf)), norm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, strings.TrimSpace(c))
	}
	return out, rows.Err()
}

// foldCityName lower-cases, folds Portuguese accents and collapses
// whitespace — the SAME table the 000220 generator applied to name_norm,
// so both sides of the comparison agree without a DB extension.
func foldCityName(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(strings.TrimSpace(s)) {
		if r, ok := cityFold[ch]; ok {
			ch = r
		}
		b.WriteRune(ch)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var cityFold = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c',
}

// EstablishmentsWithEquipment is the territory hunt: who holds the most of
// what the account sells. Newest competencia per UF, biggest park first.
func (r *CNESParkRepository) EstablishmentsWithEquipment(uf string, equipCodes []string, minCount, limit int) ([]domain.CNESEquipmentHolder, error) {
	out, _, err := r.EstablishmentsWithEquipmentIn(uf, nil, equipCodes, minCount, limit, 0)
	return out, err
}

// EstablishmentsWithEquipmentIn is the hunt restricted to city codes when
// the territory names cities (empty = whole UF). It also returns the
// competencia counted, so every caller can say WHICH month the data is.
func (r *CNESParkRepository) EstablishmentsWithEquipmentIn(uf string, cityCodes, equipCodes []string, minCount, limit, offset int) ([]domain.CNESEquipmentHolder, string, error) {
	if len(equipCodes) == 0 {
		return nil, "", nil
	}
	if minCount < 1 {
		minCount = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	if cityCodes == nil {
		cityCodes = []string{}
	}
	// ORDER BY is total DESC, cnes — total alone is not a total order (parks
	// tie constantly at small counts) and an unstable tail would make paging
	// skip and repeat establishments.
	rows, err := r.pool.Query(context.Background(), `
		WITH newest AS (
			SELECT MAX(competencia) AS comp FROM cnes_equipment WHERE uf = $1
		)
		SELECT e.cnes, MIN(e.city_code), SUM(e.qt_exist)::int AS total, MIN(newest.comp)
		FROM cnes_equipment e, newest
		WHERE e.uf = $1 AND e.competencia = newest.comp
		  AND e.equip_code = ANY($2)
		  AND (cardinality($5::text[]) = 0 OR e.city_code = ANY($5))
		  -- A deactivated establishment keeps its equipment rows and its bed
		  -- census long after it closes, so without this the hunt goes on
		  -- offering a hospital that is shut. UNKNOWN IS KEPT: a NULL reason
		  -- is the registry not having told us yet (every row is NULL until
		  -- the refresh of migration 000242 reaches it), and dropping those
		  -- would hide real ground on the strength of a blank column.
		  AND NOT EXISTS (
			SELECT 1 FROM cnes_establishments s
			WHERE s.cnes = e.cnes AND COALESCE(s.disabled_reason, '') <> ''
		  )
		GROUP BY e.cnes
		HAVING SUM(e.qt_exist) >= $3
		ORDER BY total DESC, e.cnes
		LIMIT $4 OFFSET $6`, uf, equipCodes, minCount, limit, cityCodes, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []domain.CNESEquipmentHolder
	comp := ""
	for rows.Next() {
		var h domain.CNESEquipmentHolder
		var c *string
		if err := rows.Scan(&h.CNES, &h.CityCode, &h.Total, &c); err != nil {
			return nil, "", err
		}
		if c != nil {
			comp = *c
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if comp == "" {
		// No holder in the filter still has a month to cite (or none at
		// all when the UF was never ingested).
		if c, err := r.NewestCompetencia(uf); err == nil {
			comp = c
		}
	}
	// Second pass for the cached records: N is ≤100 and the hunt is an
	// interactive query, not a hot path.
	for i := range out {
		e, err := r.GetEstablishment(out[i].CNES)
		if err != nil {
			return nil, "", err
		}
		out[i].Establishment = e
	}
	return out, comp, nil
}

// CountEquipment aggregates the UF's newest competencia over a code set:
// how many distinct establishments hold any of them, and how many units.
// The opportunity report's headline numbers.
func (r *CNESParkRepository) CountEquipment(uf string, equipCodes []string) (establishments, units int, err error) {
	return r.CountEquipmentIn(uf, nil, equipCodes)
}

// CountEquipmentIn is CountEquipment inside a set of city codes (empty =
// whole UF) — the territory's own numbers, not the state's.
func (r *CNESParkRepository) CountEquipmentIn(uf string, cityCodes, equipCodes []string) (establishments, units int, err error) {
	if len(equipCodes) == 0 {
		return 0, 0, nil
	}
	if cityCodes == nil {
		cityCodes = []string{}
	}
	err = r.pool.QueryRow(context.Background(), `
		WITH newest AS (
			SELECT MAX(competencia) AS comp FROM cnes_equipment WHERE uf = $1
		)
		SELECT COUNT(DISTINCT e.cnes), COALESCE(SUM(e.qt_exist), 0)::int
		FROM cnes_equipment e, newest
		WHERE e.uf = $1 AND e.competencia = newest.comp
		  AND e.equip_code = ANY($2)
		  AND (cardinality($3::text[]) = 0 OR e.city_code = ANY($3))`,
		uf, equipCodes, cityCodes).Scan(&establishments, &units)
	return establishments, units, err
}

// NewestCompetencia says which month the UF's park data is from — the
// report cites its source honestly.
func (r *CNESParkRepository) NewestCompetencia(uf string) (string, error) {
	var comp *string
	err := r.pool.QueryRow(context.Background(),
		`SELECT MAX(competencia) FROM cnes_equipment WHERE uf = $1`, uf).Scan(&comp)
	if err != nil || comp == nil {
		return "", err
	}
	return *comp, nil
}

// TerritoryUFs lists the distinct UFs customer accounts sell in — the
// worker only ingests states somebody actually works.
func (r *CNESParkRepository) TerritoryUFs() ([]string, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT DISTINCT UPPER(territory->>'uf')
		FROM accounts
		WHERE territory->>'uf' IS NOT NULL AND territory->>'uf' <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uf string
		if err := rows.Scan(&uf); err != nil {
			return nil, err
		}
		if len(uf) == 2 {
			out = append(out, uf)
		}
	}
	return out, rows.Err()
}

func scanEstablishment(row rowScanner) (*domain.CNESEstablishment, error) {
	var e domain.CNESEstablishment
	var fetched time.Time
	if err := row.Scan(&e.CNES, &e.TradeName, &e.LegalName, &e.CNPJ, &e.EntityCNPJ,
		&e.Address, &e.District, &e.CityCode, &e.Phone, &e.Email, &e.Latitude,
		&e.Longitude, &e.UnitTypeCode, &e.HasSurgicalCenter, &e.HasHospitalCare,
		&e.HasNeonatalCentre, &e.HasObstetricCentre, &e.AdminSphere,
		&e.DisabledReason, &e.CEP, &fetched); err != nil {
		return nil, err
	}
	e.FetchedAt = &fetched
	return &e, nil
}

// ── Bed census (migration 000241) ───────────────────────────────────────────

// ReplaceBedsCompetencia loads one UF+competencia of the bed census
// atomically, the same shape ReplaceCompetencia uses for equipment: a crash
// mid-load re-runs the month instead of half-counting it.
//
// There is deliberately no ingest stamp of its own. The bed census rides the
// equipment load's `(uf, competencia)` decision — same tree, same month, same
// publication — and a second stamp would let the two drift into reporting
// different months for one hospital.
func (r *CNESParkRepository) ReplaceBedsCompetencia(uf, competencia string, rows []domain.CNESBedRow) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	batch := &pgx.Batch{}
	flush := func() error {
		if batch.Len() == 0 {
			return nil
		}
		br := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close() //nolint:errcheck,gosec // surfacing the exec error, not the close
				return err
			}
		}
		batch = &pgx.Batch{}
		return br.Close()
	}
	for _, row := range rows {
		batch.Queue(`INSERT INTO cnes_beds
			(cnes, bed_type, bed_code, competencia, city_code, uf,
			 qt_exist, qt_sus, qt_contr, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
			ON CONFLICT (cnes, bed_type, bed_code, competencia) DO UPDATE
			SET qt_exist = EXCLUDED.qt_exist, qt_sus = EXCLUDED.qt_sus,
			    qt_contr = EXCLUDED.qt_contr, city_code = EXCLUDED.city_code,
			    updated_at = NOW()`,
			row.CNES, row.BedType, row.BedCode, row.Competencia,
			row.CityCode, row.UF, row.QtExist, row.QtSUS, row.QtContr)
		if batch.Len() >= 500 {
			if err := flush(); err != nil {
				return fmt.Errorf("cnes beds batch: %w", err)
			}
		}
	}
	if err := flush(); err != nil {
		return fmt.Errorf("cnes beds batch: %w", err)
	}
	return tx.Commit(ctx)
}

// PruneOldBedCompetencias keeps the newest `keep` months of one UF, matching
// the equipment prune so the two never hold different histories.
func (r *CNESParkRepository) PruneOldBedCompetencias(uf string, keep int) error {
	_, err := r.pool.Exec(context.Background(), `
		DELETE FROM cnes_beds
		WHERE uf = $1 AND competencia NOT IN (
			SELECT DISTINCT competencia FROM cnes_beds
			WHERE uf = $1 ORDER BY competencia DESC LIMIT $2
		)`, uf, keep)
	return err
}

// BedsForCNESIn reads the bed census of many establishments at once, joined
// with the ministry's dictionary.
//
// Competencia is the MAX over the requested ids, the same rule ParkForCNESIn
// follows: the page cites one month, and a straggler hospital that only ever
// reported an older one contributes no row rather than dragging the label
// backwards.
func (r *CNESParkRepository) BedsForCNESIn(cnesIDs []string) (map[string][]domain.CNESBedItem, string, error) {
	if len(cnesIDs) == 0 {
		return map[string][]domain.CNESBedItem{}, "", nil
	}
	ctx := context.Background()
	var comp *string
	if err := r.pool.QueryRow(ctx,
		`SELECT MAX(competencia) FROM cnes_beds WHERE cnes = ANY($1)`, cnesIDs).Scan(&comp); err != nil {
		if err == pgx.ErrNoRows {
			return map[string][]domain.CNESBedItem{}, "", nil
		}
		return nil, "", err
	}
	if comp == nil || *comp == "" {
		return map[string][]domain.CNESBedItem{}, "", nil
	}

	// An unnamed code still shows, with its raw number — the same rule the
	// park follows, so a new ministry code cannot hide a whole unit.
	rows, err := r.pool.Query(ctx, `
		SELECT b.cnes, b.bed_code,
		       COALESCE(t.name, 'código CNES ' || b.bed_code),
		       SUM(b.qt_exist)::int, SUM(b.qt_sus)::int,
		       COALESCE(bool_or(t.is_icu), FALSE),
		       COALESCE(bool_or(t.is_neonatal), FALSE)
		FROM cnes_beds b
		LEFT JOIN cnes_bed_types t ON t.code = b.bed_code
		WHERE b.cnes = ANY($1) AND b.competencia = $2 AND b.qt_exist > 0
		GROUP BY b.cnes, b.bed_code, t.name
		ORDER BY b.cnes, SUM(b.qt_exist) DESC, b.bed_code`, cnesIDs, *comp)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := map[string][]domain.CNESBedItem{}
	for rows.Next() {
		var cnes string
		var it domain.CNESBedItem
		if err := rows.Scan(&cnes, &it.BedCode, &it.Name, &it.QtExist, &it.QtSUS,
			&it.IsICU, &it.IsNeonatal); err != nil {
			return nil, "", err
		}
		out[cnes] = append(out[cnes], it)
	}
	return out, *comp, rows.Err()
}

// FillEstablishmentIdentity folds the administrative columns the EQ file
// already carries into the establishment cache.
//
// The entity CNPJ used to arrive only through the open-data API, one request
// per establishment at three per second — 5.266 of them for Mato Grosso
// alone. The monthly equipment file has the same number on every row and the
// parser was skipping it. This is the same data, in a download the loader
// already makes.
//
// It FILLS BLANKS and never overwrites: the API's record is richer (it is the
// only source of the trade name, the phone and the e-mail), so whatever is
// already stored wins. A row that does not exist yet is inserted as a stub
// with a deliberately ancient stamp, so the API top-up still picks it up for
// the name — this accelerates the identity, it does not replace the crawl.
func (r *CNESParkRepository) FillEstablishmentIdentity(rows []domain.CNESEquipmentRow) (int, error) {
	// The EQ file is one row per equipment — twenty-six thousand for MT, a
	// handful per establishment. Collapse before writing or the batch is
	// mostly the same hospital over and over.
	type ident struct{ own, entity, sphere string }
	seen := make(map[string]ident, len(rows)/4+1)
	for _, row := range rows {
		if row.CNES == "" {
			continue
		}
		if row.OwnCNPJ == "" && row.EntityCNPJ == "" && row.AdminSphere == "" {
			continue
		}
		cur := seen[row.CNES]
		if cur.own == "" {
			cur.own = row.OwnCNPJ
		}
		if cur.entity == "" {
			cur.entity = row.EntityCNPJ
		}
		if cur.sphere == "" {
			cur.sphere = row.AdminSphere
		}
		seen[row.CNES] = cur
	}
	if len(seen) == 0 {
		return 0, nil
	}

	ctx := context.Background()
	batch := &pgx.Batch{}
	for cnes, id := range seen {
		batch.Queue(`INSERT INTO cnes_establishments
			(cnes, trade_name, legal_name, address, district,
			 cnpj, cnpj_entidade, admin_sphere, fetched_at)
			VALUES ($1, '', '', '', '', NULLIF($2,''), NULLIF($3,''), NULLIF($4,''),
			        TIMESTAMPTZ '2000-01-01')
			ON CONFLICT (cnes) DO UPDATE SET
				cnpj = COALESCE(NULLIF(cnes_establishments.cnpj, ''), EXCLUDED.cnpj),
				cnpj_entidade = COALESCE(NULLIF(cnes_establishments.cnpj_entidade, ''), EXCLUDED.cnpj_entidade),
				admin_sphere = COALESCE(NULLIF(cnes_establishments.admin_sphere, ''), EXCLUDED.admin_sphere)`,
			cnes, id.own, id.entity, id.sphere)
	}
	br := r.pool.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			br.Close() //nolint:errcheck,gosec // surfacing the exec error, not the close
			return 0, fmt.Errorf("cnes identity batch: %w", err)
		}
	}
	return len(seen), br.Close()
}

// HasBeds reports whether the bed census for this UF+competencia is already
// loaded.
//
// The equipment ingest stamp cannot answer this. It was written before the
// census existed, so on every month already stamped it says "done" for the
// whole tick — which is exactly how the census shipped green and stayed empty
// in production: the stamp locked out work the stamp predates.
func (r *CNESParkRepository) HasBeds(uf, competencia string) (bool, error) {
	var one int
	err := r.pool.QueryRow(context.Background(),
		`SELECT 1 FROM cnes_beds WHERE uf = $1 AND competencia = $2 LIMIT 1`,
		uf, competencia).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ── Habilitations and specialized services (migration 000243) ──────────────

// ReplaceHabilitationsCompetencia loads one UF+competencia of the
// habilitation register, atomically, like every other family here.
func (r *CNESParkRepository) ReplaceHabilitationsCompetencia(uf, competencia string, rows []domain.CNESHabilitationRow) error {
	return r.replaceBatch(func(batch *pgx.Batch) {
		for _, row := range rows {
			batch.Queue(`INSERT INTO cnes_habilitations
				(cnes, code, competencia, uf, city_code, beds, valid_from, valid_to,
				 ordinance, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NOW())
				ON CONFLICT (cnes, code, competencia) DO UPDATE
				SET beds = EXCLUDED.beds, valid_from = EXCLUDED.valid_from,
				    valid_to = EXCLUDED.valid_to, ordinance = EXCLUDED.ordinance,
				    city_code = EXCLUDED.city_code, updated_at = NOW()`,
				row.CNES, row.Code, row.Competencia, row.UF, row.CityCode,
				row.Beds, row.ValidFrom, row.ValidTo, row.Ordinance)
		}
	}, "cnes habilitations batch")
}

// ReplaceServicesCompetencia loads one UF+competencia of the specialized
// service register.
func (r *CNESParkRepository) ReplaceServicesCompetencia(uf, competencia string, rows []domain.CNESServiceRow) error {
	return r.replaceBatch(func(batch *pgx.Batch) {
		for _, row := range rows {
			batch.Queue(`INSERT INTO cnes_services
				(cnes, service_code, class_code, competencia, uf, city_code, sus, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
				ON CONFLICT (cnes, service_code, class_code, competencia) DO UPDATE
				SET sus = EXCLUDED.sus, city_code = EXCLUDED.city_code, updated_at = NOW()`,
				row.CNES, row.ServiceCode, row.ClassCode, row.Competencia,
				row.UF, row.CityCode, row.SUS)
		}
	}, "cnes services batch")
}

// replaceBatch is the transaction-and-flush shape every family load shares:
// queue in chunks, commit once, so a crash mid-load re-runs the month instead
// of half-counting it. Extracted at the third copy.
func (r *CNESParkRepository) replaceBatch(queue func(*pgx.Batch), what string) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	batch := &pgx.Batch{}
	queue(batch)
	if batch.Len() > 0 {
		br := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close() //nolint:errcheck,gosec // surfacing the exec error
				return fmt.Errorf("%s: %w", what, err)
			}
		}
		if err := br.Close(); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return tx.Commit(ctx)
}

// PruneOldHabilitations and PruneOldServices keep the newest months, matching
// the equipment and census prunes so no two families hold different history.
func (r *CNESParkRepository) PruneOldHabilitations(uf string, keep int) error {
	_, err := r.pool.Exec(context.Background(), `
		DELETE FROM cnes_habilitations
		WHERE uf = $1 AND competencia NOT IN (
			SELECT DISTINCT competencia FROM cnes_habilitations
			WHERE uf = $1 ORDER BY competencia DESC LIMIT $2)`, uf, keep)
	return err
}

func (r *CNESParkRepository) PruneOldServices(uf string, keep int) error {
	_, err := r.pool.Exec(context.Background(), `
		DELETE FROM cnes_services
		WHERE uf = $1 AND competencia NOT IN (
			SELECT DISTINCT competencia FROM cnes_services
			WHERE uf = $1 ORDER BY competencia DESC LIMIT $2)`, uf, keep)
	return err
}

// HasHabilitations reports whether this UF+competencia is already loaded —
// the same question HasBeds answers, and for the same reason: the equipment
// ingest stamp predates these families and must not answer for them.
func (r *CNESParkRepository) HasHabilitations(uf, competencia string) (bool, error) {
	return r.hasRowFor("cnes_habilitations", uf, competencia)
}

func (r *CNESParkRepository) HasServices(uf, competencia string) (bool, error) {
	return r.hasRowFor("cnes_services", uf, competencia)
}

func (r *CNESParkRepository) hasRowFor(table, uf, competencia string) (bool, error) {
	// The table name is a package-local constant at every call site, never
	// input — the two callers above are the whole set.
	var one int
	err := r.pool.QueryRow(context.Background(),
		fmt.Sprintf(`SELECT 1 FROM %s WHERE uf = $1 AND competencia = $2 LIMIT 1`, table),
		uf, competencia).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// HabilitationsForCNESIn reads what the Ministry funds at these
// establishments, joined with its own dictionary.
func (r *CNESParkRepository) HabilitationsForCNESIn(cnesIDs []string) (map[string][]domain.CNESHabilitationItem, string, error) {
	out := map[string][]domain.CNESHabilitationItem{}
	if len(cnesIDs) == 0 {
		return out, "", nil
	}
	ctx := context.Background()
	comp, err := r.newestFor("cnes_habilitations", cnesIDs)
	if err != nil || comp == "" {
		return out, "", err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT h.cnes, h.code,
		       COALESCE(t.name, 'habilitação ' || h.code),
		       h.beds, COALESCE(t.is_icu, FALSE), COALESCE(t.is_neonatal, FALSE)
		FROM cnes_habilitations h
		LEFT JOIN cnes_habilitation_types t ON t.code = h.code
		WHERE h.cnes = ANY($1) AND h.competencia = $2
		ORDER BY h.cnes, h.beds DESC, h.code`, cnesIDs, comp)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var cnes string
		var it domain.CNESHabilitationItem
		if err := rows.Scan(&cnes, &it.Code, &it.Name, &it.Beds, &it.IsICU, &it.IsNeonatal); err != nil {
			return nil, "", err
		}
		out[cnes] = append(out[cnes], it)
	}
	return out, comp, rows.Err()
}

// ServicesForCNESIn reads what these establishments say they perform.
func (r *CNESParkRepository) ServicesForCNESIn(cnesIDs []string) (map[string][]domain.CNESServiceItem, string, error) {
	out := map[string][]domain.CNESServiceItem{}
	if len(cnesIDs) == 0 {
		return out, "", nil
	}
	ctx := context.Background()
	comp, err := r.newestFor("cnes_services", cnesIDs)
	if err != nil || comp == "" {
		return out, "", err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT s.cnes, s.service_code || s.class_code,
		       COALESCE(t.name, 'serviço ' || s.service_code || '/' || s.class_code),
		       s.sus
		FROM cnes_services s
		LEFT JOIN cnes_service_types t ON t.code = s.service_code || s.class_code
		WHERE s.cnes = ANY($1) AND s.competencia = $2
		ORDER BY s.cnes, s.service_code, s.class_code`, cnesIDs, comp)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var cnes string
		var it domain.CNESServiceItem
		if err := rows.Scan(&cnes, &it.Code, &it.Name, &it.SUS); err != nil {
			return nil, "", err
		}
		out[cnes] = append(out[cnes], it)
	}
	return out, comp, rows.Err()
}

// newestFor is the one-month rule every reader here follows: the page cites a
// single competencia, and a straggler establishment that only ever reported
// an older one contributes no row rather than dragging the label backwards.
func (r *CNESParkRepository) newestFor(table string, cnesIDs []string) (string, error) {
	var comp *string
	err := r.pool.QueryRow(context.Background(),
		fmt.Sprintf(`SELECT MAX(competencia) FROM %s WHERE cnes = ANY($1)`, table),
		cnesIDs).Scan(&comp)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if comp == nil {
		return "", nil
	}
	return *comp, nil
}
