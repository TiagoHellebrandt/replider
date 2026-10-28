package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// The two reads behind the buying calendar, against a real database.
//
// They are here and not in a unit test because the failure they had cannot
// happen anywhere else: `BuyersNeedingScan` selected `e.uf` from a table that
// has no `uf` column, and it shipped. Go compiled it, every unit test passed
// against a fake, and production answered one WARN line —
//
//	contract sweep: buyer list failed  error="column e.uf does not exist"
//
// — while the calendar stayed silently empty. A SQL string is only verified by
// a database.
func TestContractCalendar_TheSweepFindsBuyersInTheTerritory(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)

	// The state lives on the municipality, not on the establishment: that is
	// the whole shape this test exists to pin.
	seedMunicipality(t, "510340", "MT", "Cuiabá")
	seedMunicipality(t, "355030", "SP", "São Paulo")
	inTerritory := seedEstablishment(t, "510340", "03507415002864", "HOSPITAL SANTA ROSA")
	seedEstablishment(t, "355030", "60979457000111", "HOSPITAL PAULISTA")
	// No CNPJ: there is nothing to ask PNCP about.
	seedEstablishment(t, "510340", "", "POSTO SEM CNPJ")

	targets, err := repo.BuyersNeedingScan(ctx, []string{"MT"}, nil, time.Now(), 50)
	require.NoError(t, err)

	require.Len(t, targets, 1, "only the buyer in the territory, and only with a CNPJ")
	assert.Equal(t, inTerritory, targets[0].CNPJ)
	assert.Equal(t, "MT", targets[0].UF, "the state comes from the municipality")
	assert.Equal(t, "Cuiabá", targets[0].City)
	assert.True(t, targets[0].LastScannedAt.IsZero(), "never asked about")

	// Once asked, a buyer stays quiet for the freshness window.
	require.NoError(t, repo.MarkScanned(ctx, inTerritory, 3))
	again, err := repo.BuyersNeedingScan(ctx, []string{"MT"}, nil, time.Now().Add(-time.Hour), 50)
	require.NoError(t, err)
	assert.Empty(t, again, "a buyer read a minute ago is not owed another read")
}

// The city narrow uses IBGE codes (the 6-digit CODUFMUN the schema stores) —
// the same ones every other territory read uses — so an account selling only to Cuiabá does not sweep the whole state.
func TestContractCalendar_TheSweepNarrowsByCityCode(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)

	seedMunicipality(t, "510340", "MT", "Cuiabá")
	seedMunicipality(t, "510790", "MT", "Sinop")
	cuiaba := seedEstablishment(t, "510340", "03507415002864", "HOSPITAL SANTA ROSA")
	seedEstablishment(t, "510790", "60979457000111", "HOSPITAL DE SINOP")

	targets, err := repo.BuyersNeedingScan(ctx, []string{"MT"}, []string{"510340"}, time.Now(), 50)
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, cuiaba, targets[0].CNPJ)
}

// The buyers we already met through their own tenders.
//
// CNES lists health ESTABLISHMENTS, so the CNPJ it holds is a hospital's. The
// bodies that run the big purchases — a state health department, a regional
// consortium, EBSERH — are not establishments and are not in that register.
// Measured on 2026-09-14: of the 48 buyers that had already published a tender
// matching one account's words, 47 were absent from CNES, including the state
// of Mato Grosso itself. The sweep would never have asked any of them.
func TestContractCalendar_TheSweepAlsoAsksBuyersWeMetThroughTenders(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)
	accountID := seedIdentityAccount(t)

	// A body with no CNES establishment at all: only a published tender.
	_, err := testPool.Exec(ctx, `
		INSERT INTO signals (id, account_id, source, external_id, organization, cnpj,
		                     title, state, published_at, created_at)
		VALUES ($1, $2, 'pncp', $3, 'ESTADO DE MATO GROSSO', '03507415002864',
		        'Aquisição de monitores', 'MT', NOW(), NOW())`,
		ulid.Make().String(), accountID, "pncp-"+ulid.Make().String())
	require.NoError(t, err)

	targets, err := repo.BuyersNeedingScan(ctx, []string{"MT"}, nil, time.Now(), 50)
	require.NoError(t, err)

	var found bool
	for _, t2 := range targets {
		if t2.CNPJ == "03507415002864" {
			found = true
			assert.Equal(t, "MT", t2.UF)
			assert.Contains(t, t2.Name, "MATO GROSSO")
		}
	}
	assert.True(t, found,
		"a buyer that published a tender for what we sell must be swept")
}

// A tender buyer carries a city NAME, not an IBGE code, so the city narrow
// cannot apply to it — and should not: a body that buys for a whole state is
// not something to filter down to one town.
func TestContractCalendar_ACityNarrowDoesNotHideAStatewideBuyer(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)
	accountID := seedIdentityAccount(t)

	seedMunicipality(t, "510340", "MT", "Cuiabá")
	_, err := testPool.Exec(ctx, `
		INSERT INTO signals (id, account_id, source, external_id, organization, cnpj,
		                     title, state, published_at, created_at)
		VALUES ($1, $2, 'pncp', $3, 'ESTADO DE MATO GROSSO', '03507415002864',
		        'Aquisição de monitores', 'MT', NOW(), NOW())`,
		ulid.Make().String(), accountID, "pncp-"+ulid.Make().String())
	require.NoError(t, err)

	targets, err := repo.BuyersNeedingScan(ctx, []string{"MT"}, []string{"510340"}, time.Now(), 50)
	require.NoError(t, err)

	var found bool
	for _, t2 := range targets {
		if t2.CNPJ == "03507415002864" {
			found = true
		}
	}
	assert.True(t, found, "the state buyer survives a city narrow")
}

// The calendar itself: stored contracts, narrowed to the account's state and
// its words, soonest first.
func TestContractCalendar_ListsWhatExpiresAndMatchesTheWords(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)

	soon := time.Now().AddDate(0, 0, 20)
	later := time.Now().AddDate(0, 0, 120)
	longGone := time.Now().AddDate(-1, 0, 0)

	require.NoError(t, repo.Save(ctx, []domain.PublicContract{
		{
			PNCPID: "c-later", BuyerCNPJ: "03507415002864", BuyerName: "Estado de MT",
			Object: "aquisição de monitores multiparamétricos para o hospital", UF: "MT",
			ExpiresAt: &later, AtaPNCPID: "ata-9", FromAdhesion: true,
		},
		{
			PNCPID: "c-soon", BuyerCNPJ: "03507415002864", BuyerName: "Estado de MT",
			// An ACQUISITION: "manutenção de ventiladores" is a service
			// contract and the trade gate rejects it, correctly — a supplier
			// of equipment cannot bid on somebody's maintenance labour.
			Object: "aquisição de ventiladores pulmonares para o hospital", UF: "MT", ExpiresAt: &soon,
		},
		// Right state, wrong business.
		{
			PNCPID: "c-lunch", BuyerCNPJ: "03507415002864", BuyerName: "Estado de MT",
			Object: "fornecimento de merenda escolar", UF: "MT", ExpiresAt: &soon,
		},
		// Right business, wrong state.
		{
			PNCPID: "c-sp", BuyerCNPJ: "60979457000111", BuyerName: "Estado de SP",
			Object: "aquisição de monitores", UF: "SP", ExpiresAt: &soon,
		},
		// Right everything, expired a year ago: outside the window.
		{
			PNCPID: "c-old", BuyerCNPJ: "03507415002864", BuyerName: "Estado de MT",
			Object: "monitores antigos", UF: "MT", ExpiresAt: &longGone,
		},
	}))

	out, err := repo.ListExpiring(ctx, domain.ContractCalendarFilter{
		States:   []string{"MT"},
		Keywords: []string{"monitor", "ventilador"},
		From:     time.Now().AddDate(0, 0, -30),
		To:       time.Now().AddDate(0, 0, 180),
		Limit:    20,
	})
	require.NoError(t, err)

	ids := make([]string, 0, len(out))
	for _, c := range out {
		ids = append(ids, c.PNCPID)
	}
	assert.Equal(t, []string{"c-soon", "c-later"}, ids,
		"soonest first; school meals, another state and last year's are all out")

	// The two facts PNCP hands over for free and nothing used to read.
	require.Len(t, out, 2)
	assert.True(t, out[1].IsPriceRegistry(), "a year of selling, not one closed deal")
	assert.True(t, out[1].FromAdhesion, "this buyer can buy with no tender at all")
}

// The words alone are not enough, and the radar already learned this the
// expensive way.
//
// Half the keywords a catalogue yields are ordinary Portuguese. Measured over
// Mato Grosso's first 362 collected contracts on 2026-09-14: four matched the
// keywords and ALL FOUR were wrong — a heated pass-through for a student
// restaurant, an industrial drinking fountain, a canteen scale, and a bundle
// of assorted equipment. None fell inside the window, which is the only reason
// nobody has seen a food cart presented as a buying opportunity.
func TestContractCalendar_RestaurantEquipmentIsNotAMedicalPurchase(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)

	soon := time.Now().AddDate(0, 0, 30)
	require.NoError(t, repo.Save(ctx, []domain.PublicContract{
		// The four real false positives, verbatim from production.
		{
			PNCPID: "c-passthrough", BuyerCNPJ: "03507415000144", BuyerName: "IFMT",
			Object: "AQUISIÇÃO DE EQUIPAMENTO (PASS THROUGH AQUECIDO) PARA O RESTAURANTE ESTUDANTIL",
			UF:     "MT", ExpiresAt: &soon,
		},
		{
			PNCPID: "c-bebedouro", BuyerCNPJ: "03507415000144", BuyerName: "IFMT",
			Object: "BEBEDOURO ÁGUA, TIPO INDUSTRIAL, RESERVATÓRIO 4 TORNEIRAS",
			UF:     "MT", ExpiresAt: &soon,
		},
		{
			PNCPID: "c-balanca", BuyerCNPJ: "03507415000144", BuyerName: "IFMT",
			Object: "BALANÇA ELETRÔNICA PARA ATENDER O RESTAURANTE DO IFMT - CAMPUS JUÍNA",
			UF:     "MT", ExpiresAt: &soon,
		},
		{
			PNCPID: "c-diversos", BuyerCNPJ: "03507415000144", BuyerName: "IFMT",
			Object: "EMPENHO PARA AQUISIÇÃO DE MAQUINAS, UTENSILIOS E EQUIPAMENTOS DIVERSOS, VENTILADOR",
			UF:     "MT", ExpiresAt: &soon,
		},
		// And the one that IS the business.
		{
			PNCPID: "c-real", BuyerCNPJ: "03507415000144", BuyerName: "SECRETARIA DE ESTADO DE SAÚDE",
			Object: "AQUISIÇÃO DE 10 VENTILADORES PULMONARES PARA UTI NEONATAL",
			UF:     "MT", ExpiresAt: &soon,
		},
	}))

	out, err := repo.ListExpiring(ctx, domain.ContractCalendarFilter{
		States:   []string{"MT"},
		Keywords: []string{"ventilador", "aquecido", "balança", "reservatório"},
		From:     time.Now().AddDate(0, 0, -30),
		To:       time.Now().AddDate(0, 0, 180),
		Limit:    20,
	})
	require.NoError(t, err)

	require.Len(t, out, 1, "only the medical purchase survives the gate")
	assert.Equal(t, "c-real", out[0].PNCPID)
}

// An account with no words watches nothing. "Every contract in Mato Grosso" is
// thousands of rows about street lighting, and a calendar nobody can act on is
// a calendar nobody opens.
func TestContractCalendar_NoKeywordsMatchesNothing(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPublicContractRepository(testPool)

	soon := time.Now().AddDate(0, 0, 10)
	require.NoError(t, repo.Save(ctx, []domain.PublicContract{{
		PNCPID: "c1", BuyerCNPJ: "03507415002864", Object: "monitores",
		UF: "MT", ExpiresAt: &soon,
	}}))

	out, err := repo.ListExpiring(ctx, domain.ContractCalendarFilter{
		States: []string{"MT"},
		From:   time.Now().AddDate(0, 0, -30),
		To:     time.Now().AddDate(0, 0, 180),
	})
	require.NoError(t, err)
	assert.Empty(t, out)
}

func seedMunicipality(t *testing.T, code, uf, name string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO ibge_municipalities (code, uf, name, name_norm)
		VALUES ($1, $2, $3, upper($3))
		ON CONFLICT (code) DO UPDATE SET uf = EXCLUDED.uf, name = EXCLUDED.name`,
		code, uf, name)
	require.NoError(t, err)
}

// seedEstablishment returns the digits-only CNPJ, which is the key the sweep
// answers with.
func seedEstablishment(t *testing.T, cityCode, cnpj, name string) string {
	t.Helper()
	var cnpjArg *string
	if cnpj != "" {
		// Punctuated on purpose: the registry publishes it that way, and the
		// query is the thing that has to strip it.
		punctuated := cnpj[:2] + "." + cnpj[2:5] + "." + cnpj[5:8] + "/" +
			cnpj[8:12] + "-" + cnpj[12:]
		cnpjArg = &punctuated
	}
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO cnes_establishments (cnes, city_code, cnpj, legal_name, trade_name, fetched_at)
		VALUES ($1, $2, $3, $4, $4, NOW())`,
		// The TAIL of the ULID, not the head: the head is a timestamp, so three
		// establishments seeded in the same millisecond collide on the primary
		// key. Same trap the proposal reference documents.
		tailOf(ulid.Make().String(), 7), cityCode, cnpjArg, name)
	require.NoError(t, err)
	return cnpj
}

func tailOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
