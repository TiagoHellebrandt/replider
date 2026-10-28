package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// CNES installed park (migration 000202): the repo the worker writes and
// the product reads. These pin the atomic load+stamp, the dictionary join
// (against the REAL seeded types), the prune window and the territory scan.

func cnesRows(comp string) []domain.CNESEquipmentRow {
	return []domain.CNESEquipmentRow{
		{CNES: "2655411", EquipCode: "0111", Competencia: comp, CityCode: "510340", UF: "MT", QtExist: 2, QtUso: 1, SUS: true},
		{CNES: "2655411", EquipCode: "0564", Competencia: comp, CityCode: "510340", UF: "MT", QtExist: 12, QtUso: 12, SUS: false},
	}
}

func TestCNESPark_LoadJoinsTheSeededDictionary(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", cnesRows("202607")))

	done, err := repo.HasRun("MT", "202607")
	require.NoError(t, err)
	assert.True(t, done, "the stamp and the rows commit together")

	park, comp, err := repo.ParkByCNES("2655411")
	require.NoError(t, err)
	assert.Equal(t, "202607", comp)
	require.Len(t, park, 2)

	byCode := map[string]domain.CNESParkItem{}
	for _, p := range park {
		byCode[p.EquipCode] = p
	}
	// Names come from the migration's CNV seed — the real dictionary, not a
	// test fixture, so a broken seed fails here.
	assert.Contains(t, byCode["0111"].Name, "Tom")
	assert.Contains(t, byCode["0564"].Name, "Ventilador")
	assert.Equal(t, 12, byCode["0564"].QtExist)
}

func TestCNESPark_PruneKeepsTheNewestCompetencias(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202605", cnesRows("202605")))
	require.NoError(t, repo.ReplaceCompetencia("MT", "202606", cnesRows("202606")))
	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", cnesRows("202607")))
	require.NoError(t, repo.PruneOldCompetencias("MT", 2))

	var comps []string
	rows, err := testPool.Query(context.Background(),
		`SELECT DISTINCT competencia FROM cnes_equipment WHERE uf = 'MT' ORDER BY competencia`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		comps = append(comps, c)
	}
	assert.Equal(t, []string{"202606", "202607"}, comps)

	// The run ledger follows, so a pruned month can be re-ingested if it
	// ever needs to come back.
	done, err := repo.HasRun("MT", "202605")
	require.NoError(t, err)
	assert.False(t, done)
}

func TestCNESPark_EstablishmentCacheRoundTrip(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)
	phone := "65 2121-6363"

	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "3062694", TradeName: "CADIM MEDICINA DIAGNOSTICA",
		LegalName: "CADIM LTDA", Address: "Rua Aclimação, 100", Phone: &phone,
	}))

	got, err := repo.GetEstablishment("3062694")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "CADIM MEDICINA DIAGNOSTICA", got.TradeName)
	require.NotNil(t, got.Phone)
	assert.Equal(t, phone, *got.Phone)

	found, err := repo.SearchEstablishments("cadim", 5)
	require.NoError(t, err)
	require.Len(t, found, 1)

	missing, err := repo.GetEstablishment("0000000")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestCNESPark_MissingEstablishmentsAndTerritory(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", cnesRows("202607")))

	ids, err := repo.MissingEstablishments("MT", time.Now().AddDate(0, -3, 0), 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"2655411"}, ids, "equipment without a cached record is missing")

	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{CNES: "2655411"}))
	ids, err = repo.MissingEstablishments("MT", time.Now().AddDate(0, -3, 0), 10)
	require.NoError(t, err)
	assert.Empty(t, ids, "a fresh cache row leaves the list")

	// Territory scan reads accounts.territory.
	accountID := seedIdentityAccount(t)
	_, err = testPool.Exec(ctx,
		`UPDATE accounts SET territory = '{"uf":"mt","cities":["Cuiabá"]}' WHERE id = $1`, accountID)
	require.NoError(t, err)
	ufs, err := repo.TerritoryUFs()
	require.NoError(t, err)
	assert.Equal(t, []string{"MT"}, ufs, "upper-cased, deduped")
}

func TestCNESPark_HuntAndPhoneBridge(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	rows := []domain.CNESEquipmentRow{
		{CNES: "1111111", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 12},
		{CNES: "2222222", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 3},
		{CNES: "3333333", EquipCode: "0111", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 2},
		// An OLD competencia with a huge count must not leak into the hunt.
		{CNES: "4444444", EquipCode: "0564", Competencia: "202606", CityCode: "510340", UF: "MT", QtExist: 99},
	}
	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", rows[:3]))
	require.NoError(t, repo.ReplaceCompetencia("MT", "202606", rows[3:]))
	phone := "65 2121-6363"
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "1111111", TradeName: "HOSPITAL A", Phone: &phone,
	}))

	// The dictionary resolves the human word to codes.
	types, err := repo.ResolveEquipmentCodes("ventilador")
	require.NoError(t, err)
	require.NotEmpty(t, types)
	codes := make([]string, 0, len(types))
	hasVent := false
	for _, ty := range types {
		codes = append(codes, ty.Code)
		if ty.Code == "0564" {
			hasVent = true
		}
	}
	assert.True(t, hasVent, "seed maps 'ventilador' to 0564")

	holders, err := repo.EstablishmentsWithEquipment("MT", codes, 1, 10)
	require.NoError(t, err)
	require.Len(t, holders, 2, "only the newest competencia counts")
	assert.Equal(t, "1111111", holders[0].CNES, "biggest park first")
	assert.Equal(t, 12, holders[0].Total)
	require.NotNil(t, holders[0].Establishment, "cached record rides along")
	assert.Equal(t, "HOSPITAL A", holders[0].Establishment.TradeName)
	assert.Nil(t, holders[1].Establishment, "uncached id still listed")

	// min_count filters.
	holders, err = repo.EstablishmentsWithEquipment("MT", codes, 5, 10)
	require.NoError(t, err)
	require.Len(t, holders, 1)

	// The phone bridge: WhatsApp digits (sans 55) meet the registry's
	// formatted landline.
	est, err := repo.FindEstablishmentByPhoneDigits("6521216363")
	require.NoError(t, err)
	require.NotNil(t, est)
	assert.Equal(t, "HOSPITAL A", est.TradeName)

	est, err = repo.FindEstablishmentByPhoneDigits("9999999999")
	require.NoError(t, err)
	assert.Nil(t, est)
}

func TestCNESPark_UnknownCodesRenderAsCodesAndGapRowsCarryTheGroup(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", []domain.CNESEquipmentRow{
		// 0565: in the 000219 gap seed (group sourced, name not).
		{CNES: "2655411", EquipCode: "0565", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 240},
		// 1103: TIPEQUIP 11 is in no official table at all.
		{CNES: "2655411", EquipCode: "1103", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 2},
	}))

	park, _, err := repo.ParkByCNES("2655411")
	require.NoError(t, err)
	require.Len(t, park, 2, "unknown codes are counted, never dropped")
	byCode := map[string]domain.CNESParkItem{}
	for _, p := range park {
		byCode[p.EquipCode] = p
	}
	assert.Equal(t, "Código 0565", byCode["0565"].Name)
	assert.Equal(t, "Equipamentos De Manutenção Da Vida", byCode["0565"].GroupName)
	assert.True(t, domain.IsCNESRawCodeName(byCode["0565"].Name))
	assert.Equal(t, "código CNES 1103", byCode["1103"].Name, "no dictionary row → the raw code, not a guess")
	assert.Equal(t, "", byCode["1103"].GroupName)

	// The group-only row lets a hunt by GROUP name find the holder.
	types, err := repo.ResolveEquipmentCodes("manutenção da vida")
	require.NoError(t, err)
	codes := []string{}
	for _, ty := range types {
		codes = append(codes, ty.Code)
	}
	assert.Contains(t, codes, "0565")
}

func TestCNESPark_MissingEstablishmentsNamesTheBiggestParkFirst(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", []domain.CNESEquipmentRow{
		{CNES: "1111111", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 3},
		{CNES: "2222222", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 50},
		{CNES: "3333333", EquipCode: "0111", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 9},
		{CNES: "4444444", EquipCode: "0111", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 1},
	}))
	// An OLD month's giant must not jump the queue.
	require.NoError(t, repo.ReplaceCompetencia("MT", "202606", []domain.CNESEquipmentRow{
		{CNES: "1111111", EquipCode: "0564", Competencia: "202606", CityCode: "510340", UF: "MT", QtExist: 999},
	}))
	// 4444444 is cached WITH a name but stale: it goes after every unnamed id.
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{CNES: "4444444", TradeName: "HOSPITAL D"}))
	_, err := testPool.Exec(context.Background(),
		`UPDATE cnes_establishments SET fetched_at = NOW() - INTERVAL '200 days' WHERE cnes = '4444444'`)
	require.NoError(t, err)

	ids, err := repo.MissingEstablishments("MT", time.Now().AddDate(0, -3, 0), 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"2222222", "3333333", "1111111", "4444444"}, ids,
		"unnamed ids by newest-month park size, then the stale-but-named by cnes")

	ids, err = repo.MissingEstablishments("MT", time.Now().AddDate(0, -3, 0), 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"2222222", "3333333"}, ids, "the cap keeps the biggest parks")
}

func TestCNESPark_TerritoryCitiesScopeTheHuntAndTheCounts(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", []domain.CNESEquipmentRow{
		{CNES: "1111111", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 12}, // Cuiabá
		{CNES: "2222222", EquipCode: "0564", Competencia: "202607", CityCode: "510840", UF: "MT", QtExist: 3},  // Várzea Grande
		{CNES: "3333333", EquipCode: "0564", Competencia: "202607", CityCode: "510760", UF: "MT", QtExist: 40}, // Sinop
	}))

	// Names the seller typed, in any casing/accenting, meet the IBGE seed.
	codes, err := repo.CityCodesByName("MT", []string{"cuiaba", "VÁRZEA GRANDE", "Xyzville"})
	require.NoError(t, err)
	assert.Equal(t, []string{"510340", "510840"}, codes, "unknown names are simply absent")

	holders, comp, err := repo.EstablishmentsWithEquipmentIn("MT", codes, []string{"0564"}, 1, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, "202607", comp)
	require.Len(t, holders, 2, "Sinop's bigger park is outside the territory")
	assert.Equal(t, "1111111", holders[0].CNES)

	holders, comp, err = repo.EstablishmentsWithEquipmentIn("MT", nil, []string{"0564"}, 1, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, "202607", comp)
	assert.Len(t, holders, 3, "no cities = the whole UF, as before")

	// Paging is what lets the nightly sweep WALK a territory instead of
	// re-reading its first page every night. Production on 2026-09-17: the
	// hunt held 60 hospitals out of 2.818 matching establishments and could
	// not reach past them.
	page2, _, err := repo.EstablishmentsWithEquipmentIn("MT", nil, []string{"0564"}, 1, 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1, "the second page is what the first one did not show")
	assert.Equal(t, "2222222", page2[0].CNES, "smallest park last, and reachable")

	estabs, units, err := repo.CountEquipmentIn("MT", codes, []string{"0564"})
	require.NoError(t, err)
	assert.Equal(t, 2, estabs)
	assert.Equal(t, 15, units)

	// A filter that matches nobody still cites the UF's month.
	holders, comp, err = repo.EstablishmentsWithEquipmentIn("MT", []string{"999999"}, []string{"0564"}, 1, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, holders)
	assert.Equal(t, "202607", comp)
}

func TestCNESPark_ResolveEquipmentCodesAnyIsOneQueryOverAllWords(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	types, err := repo.ResolveEquipmentCodesAny([]string{"tomógrafo", "ventilador", "  ", "luva"})
	require.NoError(t, err)
	codes := []string{}
	for _, ty := range types {
		codes = append(codes, ty.Code)
	}
	assert.Contains(t, codes, "0111")
	assert.Contains(t, codes, "0564")

	types, err = repo.ResolveEquipmentCodesAny(nil)
	require.NoError(t, err)
	assert.Empty(t, types)
}

func TestCNESPark_CountEquipmentAggregates(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", []domain.CNESEquipmentRow{
		{CNES: "1111111", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 12},
		{CNES: "1111111", EquipCode: "0111", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 2},
		{CNES: "2222222", EquipCode: "0564", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 3},
	}))

	// Two codes, two establishments — the DISTINCT count must not double
	// the hospital that holds both.
	estabs, units, err := repo.CountEquipment("MT", []string{"0564", "0111"})
	require.NoError(t, err)
	assert.Equal(t, 2, estabs)
	assert.Equal(t, 17, units)

	comp, err := repo.NewestCompetencia("MT")
	require.NoError(t, err)
	assert.Equal(t, "202607", comp)

	comp, err = repo.NewestCompetencia("AC")
	require.NoError(t, err)
	assert.Equal(t, "", comp, "no data reads as empty, not an error")
}

// The bed census, end to end — and the dictionary that keeps it honest.
//
// This audit read the ministry's codes wrong twice before opening the file
// that defines them: TP_LEITO 4 was taken for intensive care when it means
// obstetric, and code 86 was counted as neonatal when it is coronary
// intensive care. Both errors are cheap in a probe and expensive on a
// seller's screen, so the dictionary ships as a seeded table and this asserts
// the two codes that caught me.
func TestCNESBeds_CensusAndDictionary(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceBedsCompetencia("MT", "202608", []domain.CNESBedRow{
		{CNES: "2396866", BedType: "3", BedCode: "81", Competencia: "202608",
			UF: "MT", CityCode: "510760", QtExist: 20, QtSUS: 18},
		{CNES: "2396866", BedType: "3", BedCode: "86", Competencia: "202608",
			UF: "MT", CityCode: "510760", QtExist: 6, QtSUS: 6},
		{CNES: "2396866", BedType: "1", BedCode: "03", Competencia: "202608",
			UF: "MT", CityCode: "510760", QtExist: 40, QtSUS: 30},
	}))

	beds, comp, err := repo.BedsForCNESIn([]string{"2396866"})
	require.NoError(t, err)
	assert.Equal(t, "202608", comp)
	rows := beds["2396866"]
	require.Len(t, rows, 3)

	byCode := map[string]domain.CNESBedItem{}
	for _, r := range rows {
		byCode[r.BedCode] = r
	}

	// 81 is UTI NEONATAL TIPO II: intensive care AND neonatal.
	assert.Equal(t, "UTI NEONATAL - TIPO II", byCode["81"].Name)
	assert.True(t, byCode["81"].IsICU)
	assert.True(t, byCode["81"].IsNeonatal)
	assert.Equal(t, 20, byCode["81"].QtExist)

	// 86 is UTI CORONARIANA TIPO III — intensive care, NOT neonatal. Counting
	// it as neonatal inflated this state's neonatal capacity in the audit
	// that preceded this table.
	assert.True(t, byCode["86"].IsICU)
	assert.False(t, byCode["86"].IsNeonatal,
		"86 is coronary intensive care, never neonatal")

	// And a plain surgical bed is neither.
	assert.False(t, byCode["03"].IsICU)
	assert.False(t, byCode["03"].IsNeonatal)

	// Newest month only, like the park: an older census must not surface
	// beside a newer one.
	require.NoError(t, repo.ReplaceBedsCompetencia("MT", "202609", []domain.CNESBedRow{
		{CNES: "2396866", BedType: "3", BedCode: "81", Competencia: "202609",
			UF: "MT", CityCode: "510760", QtExist: 24, QtSUS: 20},
	}))
	beds, comp, err = repo.BedsForCNESIn([]string{"2396866"})
	require.NoError(t, err)
	assert.Equal(t, "202609", comp)
	require.Len(t, beds["2396866"], 1)
	assert.Equal(t, 24, beds["2396866"][0].QtExist)
}

// A deactivated establishment keeps its equipment rows for months after it
// closes. Without the registry's reason, the hunt goes on offering a hospital
// that is shut — and the seller finds out on the phone.
//
// Unknown is kept: a NULL reason means the registry has not told us yet, and
// dropping those would hide real ground on the strength of a blank column.
func TestCNESPark_ClosedEstablishmentsLeaveTheHunt(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceCompetencia("MT", "202608", []domain.CNESEquipmentRow{
		{CNES: "1111111", EquipCode: "0564", Competencia: "202608", CityCode: "510340", UF: "MT", QtExist: 10},
		{CNES: "2222222", EquipCode: "0564", Competencia: "202608", CityCode: "510340", UF: "MT", QtExist: 9},
		{CNES: "3333333", EquipCode: "0564", Competencia: "202608", CityCode: "510340", UF: "MT", QtExist: 8},
	}))
	// 1111111 is shut; 2222222 is open and says so; 3333333 has never been
	// read, which is every row until the refresh reaches it.
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "1111111", TradeName: "FECHADO", DisabledReason: "05"}))
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "2222222", TradeName: "ABERTO"}))

	holders, _, err := repo.EstablishmentsWithEquipmentIn("MT", nil, []string{"0564"}, 1, 10, 0)
	require.NoError(t, err)

	var got []string
	for _, h := range holders {
		got = append(got, h.CNES)
	}
	assert.Equal(t, []string{"2222222", "3333333"}, got,
		"the closed one leaves; the unread one stays")
}

// The three facts the record published and the loader dropped, round-tripped.
func TestCNESPark_EstablishmentProfileSurvivesTheRoundTrip(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	yes, no := true, false
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "9209352", TradeName: "HOSPITAL MUNICIPAL",
		AdminSphere: "MUNICIPAL", CEP: "78010100",
		HasNeonatalCentre: &yes, HasObstetricCentre: &no,
	}))

	got, err := repo.GetEstablishment("9209352")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "MUNICIPAL", got.AdminSphere)
	assert.Equal(t, "78010100", got.CEP)
	require.NotNil(t, got.HasNeonatalCentre)
	assert.True(t, *got.HasNeonatalCentre)
	require.NotNil(t, got.HasObstetricCentre)
	assert.False(t, *got.HasObstetricCentre,
		"declared-false and never-declared are different answers")
	assert.Empty(t, got.DisabledReason, "no reason means operating")
}

// The entity CNPJ arrives with the equipment month, and never overwrites the
// richer record the open-data API brings.
func TestCNESPark_IdentityFillsBlanksWithoutClobbering(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	// Already known through the API, with a name and its own CNPJ.
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "2396866", TradeName: "SANTA CASA DE RONDONOPOLIS",
		CNPJ: cnesStr("03099157000104"),
	}))

	n, err := repo.FillEstablishmentIdentity([]domain.CNESEquipmentRow{
		// Same hospital, three equipment rows — the file's real shape.
		{CNES: "2396866", OwnCNPJ: "99999999999999", AdminSphere: "0M"},
		{CNES: "2396866", OwnCNPJ: "99999999999999"},
		{CNES: "2396866", OwnCNPJ: "99999999999999"},
		// Never seen before: arrives as a stub.
		{CNES: "9209352", EntityCNPJ: "15084338000146", AdminSphere: "0M"},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, n, "collapsed to one write per establishment")

	known, err := repo.GetEstablishment("2396866")
	require.NoError(t, err)
	require.NotNil(t, known.CNPJ)
	assert.Equal(t, "03099157000104", *known.CNPJ,
		"the API's record is richer and must win")
	assert.Equal(t, "SANTA CASA DE RONDONOPOLIS", known.TradeName)
	assert.Equal(t, "0M", known.AdminSphere, "a blank is still filled")

	fresh, err := repo.GetEstablishment("9209352")
	require.NoError(t, err)
	require.NotNil(t, fresh)
	require.NotNil(t, fresh.EntityCNPJ)
	assert.Equal(t, "15084338000146", *fresh.EntityCNPJ)
	assert.Empty(t, fresh.TradeName,
		"a stub carries no name — the API top-up is still what brings it")
}

func cnesStr(s string) *string { return &s }

// What the Ministry FUNDS, and what the hospital says it performs — the two
// families the audit found and nothing had ever opened.
func TestCNESHabilitationsAndServices_RoundTripThroughTheDictionaries(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewCNESParkRepository(testPool)

	require.NoError(t, repo.ReplaceHabilitationsCompetencia("MT", "202608",
		[]domain.CNESHabilitationRow{
			{CNES: "2396866", Code: "2602", Competencia: "202608", UF: "MT",
				CityCode: "510760", Beds: 10, ValidFrom: "202201",
				ValidTo: "999999", Ordinance: "24243/GM/MS"},
			// A code the dictionary does not name still shows, with its raw
			// number — the rule every CNES surface here follows.
			{CNES: "2396866", Code: "ZZ99", Competencia: "202608", UF: "MT"},
		}))
	require.NoError(t, repo.ReplaceServicesCompetencia("MT", "202608",
		[]domain.CNESServiceRow{
			{CNES: "2396866", ServiceCode: "122", ClassCode: "011",
				Competencia: "202608", UF: "MT", SUS: true},
		}))

	habs, comp, err := repo.HabilitationsForCNESIn([]string{"2396866"})
	require.NoError(t, err)
	assert.Equal(t, "202608", comp)
	rows := habs["2396866"]
	require.Len(t, rows, 2)

	// Biggest funded capacity first.
	assert.Equal(t, "2602", rows[0].Code)
	assert.Equal(t, "UTI II NEONATAL", rows[0].Name)
	assert.Equal(t, 10, rows[0].Beds)
	assert.True(t, rows[0].IsNeonatal)
	assert.True(t, rows[0].IsICU)
	assert.Contains(t, rows[1].Name, "ZZ99", "an unnamed code keeps its number")

	servs, comp, err := repo.ServicesForCNESIn([]string{"2396866"})
	require.NoError(t, err)
	assert.Equal(t, "202608", comp)
	require.Len(t, servs["2396866"], 1)
	assert.Contains(t, servs["2396866"][0].Name, "POTENCIAIS EVOCADOS")
	assert.True(t, servs["2396866"][0].SUS)

	// Each family answers for ITSELF. The equipment stamp predates all of
	// them and must never be the thing asked.
	has, err := repo.HasHabilitations("MT", "202608")
	require.NoError(t, err)
	assert.True(t, has)
	has, err = repo.HasServices("MT", "202609")
	require.NoError(t, err)
	assert.False(t, has)
}
