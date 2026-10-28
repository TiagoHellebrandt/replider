package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pieces existed and never met: co-purchase patterns, purchase history,
// signals and registered activity each sat in their own table while the
// approach generator wrote from the funnel's persona and one product.

type stubCrossSell struct {
	bought    []string
	pairs     []domain.CoPurchasePair
	boughtErr error
	pairsErr  error
}

func (s stubCrossSell) Pairs(context.Context, string) ([]domain.CoPurchasePair, error) {
	return s.pairs, s.pairsErr
}

func (s stubCrossSell) ItemsBoughtBy(context.Context, string, string) ([]string, error) {
	return s.bought, s.boughtErr
}

type stubPortfolio struct {
	customer *domain.PortfolioCustomer
	err      error
}

func (s stubPortfolio) List(context.Context, string, int, int) ([]*domain.PortfolioCustomer, error) {
	return nil, nil
}

func (s stubPortfolio) Get(context.Context, string, string) (*domain.PortfolioCustomer, error) {
	return s.customer, s.err
}

func (s stubPortfolio) DueForReorder(context.Context, time.Time, int) ([]*domain.ReorderCandidate, error) {
	return nil, nil
}

type stubSignals struct {
	list []*domain.Signal
	err  error
}

func (s stubSignals) Upsert(context.Context, *domain.Signal) error { return nil }
func (s stubSignals) List(context.Context, string, int, int) ([]*domain.Signal, error) {
	return s.list, s.err
}
func (s stubSignals) ListForRadar(context.Context, string, int, int) ([]*domain.Signal, error) {
	return s.list, s.err
}
func (s stubSignals) CountByAccount(context.Context, string) (int, error) { return len(s.list), s.err }
func (s stubSignals) Get(context.Context, string, string) (*domain.Signal, error) {
	return nil, nil
}
func (s stubSignals) Dismiss(context.Context, string, string, time.Time) error { return nil }
func (s stubSignals) LinkLead(context.Context, string, string, string) error   { return nil }
func (s stubSignals) ListWatches(context.Context, string) ([]*domain.SignalWatch, error) {
	return nil, nil
}
func (s stubSignals) ListActiveWatches(context.Context) ([]*domain.SignalWatch, error) {
	return nil, nil
}
func (s stubSignals) UpsertWatch(context.Context, *domain.SignalWatch) error { return nil }
func (s stubSignals) DeleteWatch(context.Context, string, string) error      { return nil }

func leadWith(t *testing.T, enrichment, collected map[string]any) *domain.Lead {
	t.Helper()
	lead := &domain.Lead{ID: "lead-1", AccountID: "acc-1", ContactID: "contact-1"}
	if enrichment != nil {
		b, err := json.Marshal(enrichment)
		require.NoError(t, err)
		lead.Enrichment = b
	}
	if collected != nil {
		b, err := json.Marshal(collected)
		require.NoError(t, err)
		lead.CollectedMetadata = b
	}
	return lead
}

// The pattern the account already learned from its own sales, with the numbers
// behind it. "They probably need masks" is a guess in a confident voice.
func TestDossier_CoPurchaseTravelsWithItsEvidence(t *testing.T) {
	b := NewLeadDossierBuilder().SetCrossSell(stubCrossSell{
		bought: []string{"luva"},
		pairs: []domain.CoPurchasePair{
			{Product: "luva", Partner: "mascara", Customers: 8, ProductCustomers: 11},
		},
	})

	d := b.Build(context.Background(), "acc-1", leadWith(t, nil, nil), nil)
	out := d.Render()

	assert.Contains(t, out, "mascara")
	assert.Contains(t, out, "8 clientes que compram luva também levam (73%)")
}

// A customer who has bought nothing gives the pattern no anchor, and a
// suggestion with no anchor is the "everybody also buys gloves" noise the
// confidence floor exists to keep out.
func TestDossier_NoPurchaseHistoryMeansNoSuggestion(t *testing.T) {
	b := NewLeadDossierBuilder().SetCrossSell(stubCrossSell{
		pairs: []domain.CoPurchasePair{
			{Product: "luva", Partner: "mascara", Customers: 8, ProductCustomers: 11},
		},
	})

	d := b.Build(context.Background(), "acc-1", leadWith(t, nil, nil), nil)

	for _, f := range d.Facts {
		assert.NotEqual(t, domain.DossierPattern, f.Kind)
	}
}

// The reorder date is what turns "we sell masks" into "yours run out around the
// 12th" — the difference between a pitch and a reason to reply.
func TestDossier_CarriesWhenTheStockRunsOut(t *testing.T) {
	// Slack on purpose: DaysUntilDue truncates, so an exact 72h read a
	// millisecond later is 2.99 days and floors to 2. The test is about the
	// phrase, not about the domain's rounding.
	due := time.Now().Add(72*time.Hour + time.Minute)
	b := NewLeadDossierBuilder().SetPortfolio(stubPortfolio{
		customer: &domain.PortfolioCustomer{Purchases: 4, NextDueAt: &due},
	})

	out := b.Build(context.Background(), "acc-1", leadWith(t, nil, nil), nil).Render()

	assert.Contains(t, out, "Previsão de acabar o estoque")
	assert.Contains(t, out, "em cerca de 3 dias")
}

// Overdue is a different sentence, and a better reason to write today.
func TestDossier_OverdueSaysSo(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour)
	b := NewLeadDossierBuilder().SetPortfolio(stubPortfolio{
		customer: &domain.PortfolioCustomer{Purchases: 1, NextDueAt: &past},
	})

	out := b.Build(context.Background(), "acc-1", leadWith(t, nil, nil), nil).Render()

	assert.Contains(t, out, "já passou do previsto")
}

// Only signals already linked to THIS lead. An unlinked signal that merely
// mentions a similar company would have the message claim a tender the
// recipient never published.
func TestDossier_OnlyLinkedSignalsTravel(t *testing.T) {
	b := NewLeadDossierBuilder().SetSignals(stubSignals{list: []*domain.Signal{
		{Title: "Pregão da nossa lead", Source: "compras.gov", LeadID: "lead-1"},
		{Title: "Pregão de outra empresa", Source: "compras.gov", LeadID: "lead-9"},
		{Title: "Pregão sem dono", Source: "compras.gov"},
	}})

	out := b.Build(context.Background(), "acc-1", leadWith(t, nil, nil), nil).Render()

	assert.Contains(t, out, "Pregão da nossa lead")
	assert.NotContains(t, out, "Pregão de outra empresa")
	assert.NotContains(t, out, "Pregão sem dono")
}

// A registered activity code and a name a model read off a page used to arrive
// with identical authority, in one undifferentiated dump.
func TestDossier_SplitsWhatIsRegisteredFromWhatWasResearched(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"cnae_label":          "Atividade médica ambulatorial",
		"decision_maker_name": "Ana Souza",
		"sources":             []any{"https://www.linkedin.com/in/ana"},
	}, nil)

	out := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil).Render()

	assert.Contains(t, out, "Registro público (verificável)")
	assert.Contains(t, out, "Atividade médica ambulatorial")
	assert.Contains(t, out, "cite como leitura, não como certeza")
	assert.Contains(t, out, "Ana Souza")
	assert.Contains(t, out, "linkedin.com")
}

// What the customer answered themselves outranks everything inferred: they said
// it, so the message may state it back.
func TestDossier_WhatTheCustomerSaidOutranksTheRest(t *testing.T) {
	lead := leadWith(t,
		map[string]any{"decision_maker_name": "Ana"},
		map[string]any{"orcamento_previsto": "R$ 50 mil"})

	out := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil).Render()

	said := indexOf(out, "O cliente disse")
	read := indexOf(out, "Pesquisa na web")
	assert.Greater(t, read, said, "the customer's own words come first")
	assert.Contains(t, out, "Orcamento previsto")
	assert.Contains(t, out, "R$ 50 mil")
}

// Live data stores a collected field either bare or wrapped with its timestamp.
func TestDossier_ReadsBothShapesOfACollectedField(t *testing.T) {
	lead := leadWith(t, nil, map[string]any{
		"cidade":   "Sorocaba",
		"leitos":   map[string]any{"value": "120", "collected_at": "2026-07-01"},
		"ativo":    true,
		"unidades": float64(3),
	})

	out := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil).Render()

	assert.Contains(t, out, "Sorocaba")
	assert.Contains(t, out, "120")
	assert.Contains(t, out, "sim")
	assert.Contains(t, out, "3")
}

// The generator has to tell "no decision maker" from "decision maker unknown":
// the first invites an invented name, the second invites a question.
func TestDossier_NamesTheGapsItFoundInsteadOfHidingThem(t *testing.T) {
	d := NewLeadDossierBuilder().Build(
		context.Background(), "acc-1", leadWith(t, nil, nil), nil)

	assert.Contains(t, d.Missing, "quem decide a compra")
	assert.Contains(t, d.Missing, "o nome de quem vai ler a mensagem")
	assert.Contains(t, d.Render(), "não invente")
}

func TestDossier_AKnownDecisionMakerIsNotAGap(t *testing.T) {
	lead := leadWith(t, map[string]any{"decision_maker_name": "Ana"}, nil)

	d := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil)

	assert.NotContains(t, d.Missing, "quem decide a compra")
}

// Nothing here is worth failing a draft over: a salesperson staring at an error
// instead of a draft has lost more than the missing context was worth.
func TestDossier_ASourceThatFailsIsSkippedNotFatal(t *testing.T) {
	b := NewLeadDossierBuilder().
		SetCrossSell(stubCrossSell{boughtErr: errors.New("db down")}).
		SetPortfolio(stubPortfolio{err: errors.New("db down")}).
		SetSignals(stubSignals{err: errors.New("db down")})

	lead := leadWith(t, map[string]any{"cnae_label": "Comércio"}, nil)
	out := b.Build(context.Background(), "acc-1", lead, nil).Render()

	// Everything that still worked is there.
	assert.Contains(t, out, "Comércio")
}

// Unwired sources behave as they did before the dossier existed.
func TestDossier_WithNothingWiredItIsSilent(t *testing.T) {
	d := NewLeadDossierBuilder().Build(context.Background(), "acc-1", nil, nil)

	assert.True(t, d.Empty())
	assert.Empty(t, d.Render())
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// --- Per-fact provenance ---------------------------------------------------

// The point of the deeper research. Two facts read from two pages used to arrive
// stamped with BOTH pages, so somebody checking "quem decide" landed on the page
// that supported the company's activity instead.
func TestDossier_EachResearchedFactCitesItsOwnPage(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"findings": []any{
			map[string]any{
				"field": "decision_maker_name", "value": "Ana Souza",
				"source": "https://www.linkedin.com/in/ana", "confident": true,
			},
			map[string]any{
				"field": "what_they_do", "value": "Clínica de fisioterapia",
				"source": "https://clinicaboavista.com.br/sobre", "confident": true,
			},
		},
		"sources": []any{
			"https://www.linkedin.com/in/ana",
			"https://clinicaboavista.com.br/sobre",
		},
	}, nil)

	d := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil)

	byValue := map[string]string{}
	for _, f := range d.Facts {
		byValue[f.Value] = f.Source
	}
	assert.Equal(t, "linkedin.com", byValue["Ana Souza"])
	assert.Equal(t, "clinicaboavista.com.br", byValue["Clínica de fisioterapia"])
}

// A fact the model would not attribute, or was not sure about, still travels —
// it makes a good question. What must never happen is it reaching a stranger
// worded as established.
func TestDossier_AnUncitedFindingIsHedgedNotDropped(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"findings": []any{
			map[string]any{
				"field": "current_supplier", "value": "compra da Distribuidora X",
				"confident": true,
			},
			map[string]any{
				"field": "pain_signal", "value": "reclama de prazo de entrega",
				"source": "https://reclameaqui.com.br/x", "confident": false,
			},
		},
	}, nil)

	out := NewLeadDossierBuilder().Build(
		context.Background(), "acc-1", lead, nil).Render()

	assert.Contains(t, out, "compra da Distribuidora X")
	assert.Contains(t, out, "reclama de prazo de entrega")
	// Both are marked: one has no page, the other the model itself doubted.
	assert.Equal(t, 2, strings.Count(out, "Indício a confirmar"))
}

// The prompt invites facts nobody enumerated; discarding them here would make
// that invitation a lie.
func TestDossier_KeepsAFactNobodyEnumerated(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"findings": []any{
			map[string]any{
				"field": "numero_de_leitos", "value": "120",
				"source": "https://hospital.com.br", "confident": true,
			},
		},
	}, nil)

	out := NewLeadDossierBuilder().Build(
		context.Background(), "acc-1", lead, nil).Render()

	assert.Contains(t, out, "Numero de leitos")
	assert.Contains(t, out, "120")
}

// An enrichment written before findings existed still renders — with the union
// of sources, which is the most that answer can honestly support.
func TestDossier_OlderEnrichmentFallsBackToTheLegacyFields(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"decision_maker_name": "Ana",
		"sources":             []any{"https://www.linkedin.com/in/ana"},
	}, nil)

	out := NewLeadDossierBuilder().Build(
		context.Background(), "acc-1", lead, nil).Render()

	assert.Contains(t, out, "Ana")
	assert.Contains(t, out, "linkedin.com")
	assert.NotContains(t, out, "Indício a confirmar")
}

// Findings present means findings win: rendering both shapes would print the
// decision maker twice, once cited to its page and once to the bibliography.
func TestDossier_FindingsReplaceTheLegacyFieldsRatherThanJoinThem(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"decision_maker_name": "Ana Souza",
		"findings": []any{
			map[string]any{
				"field": "decision_maker_name", "value": "Ana Souza",
				"source": "https://www.linkedin.com/in/ana", "confident": true,
			},
		},
	}, nil)

	out := NewLeadDossierBuilder().Build(
		context.Background(), "acc-1", lead, nil).Render()

	assert.Equal(t, 1, strings.Count(out, "Ana Souza"))
}

// A finding with no value teaches the model that a field exists and is unknown,
// which is what Missing says better.
func TestDossier_IgnoresEmptyAndMalformedFindings(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"findings": []any{
			map[string]any{"field": "what_they_do", "value": "   "},
			map[string]any{"field": "", "value": "sem campo"},
			"não é um objeto",
			map[string]any{
				"field": "region", "value": "Sorocaba/SP",
				"source": "https://clinica.com.br", "confident": true,
			},
		},
	}, nil)

	d := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil)

	var researched int
	for _, f := range d.Facts {
		if f.Kind == domain.DossierResearch {
			researched++
		}
	}
	assert.Equal(t, 1, researched)
	assert.Contains(t, d.Render(), "Sorocaba/SP")
}

// A decision maker found through the findings is not a gap either.
func TestDossier_AFoundDecisionMakerInFindingsIsNotAGap(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"findings": []any{
			map[string]any{
				"field": "decision_maker_name", "value": "Ana",
				"source": "https://www.linkedin.com/in/ana", "confident": true,
			},
		},
	}, nil)

	d := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil)

	assert.NotContains(t, d.Missing, "quem decide a compra")
}

// ---- installed park (CNES) ----

type stubCNESPark struct {
	byPhone map[string]*domain.CNESEstablishment
	byName  []domain.CNESEstablishment
	park    []domain.CNESParkItem
	comp    string
}

// GetEstablishment: the key-first path. Nil here keeps these tests on the
// phone/name guesses they were written for.
func (s stubCNESPark) GetEstablishment(string) (*domain.CNESEstablishment, error) {
	return nil, nil
}

func (s stubCNESPark) FindEstablishmentByPhoneDigits(digits string) (*domain.CNESEstablishment, error) {
	return s.byPhone[digits], nil
}
func (s stubCNESPark) SearchEstablishments(string, int) ([]domain.CNESEstablishment, error) {
	return s.byName, nil
}
func (s stubCNESPark) ParkByCNES(string) ([]domain.CNESParkItem, string, error) {
	return s.park, s.comp, nil
}

func TestLeadDossier_ParkArrivesByPhoneMatch(t *testing.T) {
	b := NewLeadDossierBuilder().SetCNESPark(stubCNESPark{
		byPhone: map[string]*domain.CNESEstablishment{
			"6521216363": {CNES: "3062694", TradeName: "CADIM"},
		},
		park: []domain.CNESParkItem{
			{EquipCode: "0111", Name: "Tomógrafo Computadorizado", QtExist: 2},
			{EquipCode: "0564", Name: "Respirador/Ventilador", QtExist: 12},
		},
		comp: "202607",
	})
	// WhatsApp shape: country code + bare digits. The bridge strips the 55.
	lead := &domain.Lead{ID: "l1", ContactID: "556521216363"}

	d := b.Build(context.Background(), "acc1", lead, nil)

	rendered := d.Render()
	assert.Contains(t, rendered, "CADIM")
	assert.Contains(t, rendered, "2 Tomógrafo Computadorizado")
	assert.Contains(t, rendered, "12 Respirador/Ventilador")
	assert.Contains(t, rendered, "CNES/DATASUS 3062694, competência 2026-07")
	// Registry section: checkable, may be stated plainly.
	assert.Contains(t, rendered, "Registro público")
}

func TestLeadDossier_UnknownCodesStayCodesNeverGuessedNames(t *testing.T) {
	b := NewLeadDossierBuilder().SetCNESPark(stubCNESPark{
		byPhone: map[string]*domain.CNESEstablishment{
			"6521216363": {CNES: "3062694", TradeName: "CADIM"},
		},
		park: []domain.CNESParkItem{
			// What ParkByCNES yields for a code with no dictionary row.
			{EquipCode: "1103", Name: "código CNES 1103", QtExist: 2},
			// A 000219 group-only row: official group, no official name.
			{EquipCode: "0565", Name: "Código 0565", GroupName: "Equipamentos De Manutenção Da Vida", QtExist: 240},
			{EquipCode: "0111", Name: "Tomógrafo Computadorizado", QtExist: 1},
		},
		comp: "202607",
	})
	lead := &domain.Lead{ID: "l1", ContactID: "556521216363"}

	rendered := b.Build(context.Background(), "acc1", lead, nil).Render()

	assert.Contains(t, rendered, "2 código CNES 1103", "an unnamed code is still counted, as a code")
	assert.Contains(t, rendered, "240 código CNES 0565 (Equipamentos De Manutenção Da Vida)",
		"the sourced group rides along; the name is never invented")
	assert.Contains(t, rendered, "1 Tomógrafo Computadorizado")
}

func TestLeadDossier_AmbiguousNameNeverGuesses(t *testing.T) {
	b := NewLeadDossierBuilder().SetCNESPark(stubCNESPark{
		byName: []domain.CNESEstablishment{
			{CNES: "1", TradeName: "Hospital Santa Casa Cuiabá"},
			{CNES: "2", TradeName: "Hospital Santa Casa Rondonópolis"},
		},
		park: []domain.CNESParkItem{{EquipCode: "0111", Name: "Tomógrafo", QtExist: 1}},
		comp: "202607",
	})
	name := "Hospital Santa Casa"
	lead := &domain.Lead{ID: "l1", ContactID: "place-id-without-phone"}
	contact := &domain.Contact{Name: &name}

	d := b.Build(context.Background(), "acc1", lead, contact)

	assert.NotContains(t, d.Render(), "Tomógrafo",
		"two name hits means no match: asserting a guessed hospital's park is an invented fact")
}

func TestLeadDossier_UnambiguousNameMatches(t *testing.T) {
	b := NewLeadDossierBuilder().SetCNESPark(stubCNESPark{
		byName: []domain.CNESEstablishment{{CNES: "3062694", TradeName: "CADIM MEDICINA"}},
		park:   []domain.CNESParkItem{{EquipCode: "0111", Name: "Tomógrafo", QtExist: 2}},
		comp:   "202607",
	})
	name := "CADIM MEDICINA"
	lead := &domain.Lead{ID: "l1", ContactID: "no-phone"}

	d := b.Build(context.Background(), "acc1", lead, &domain.Contact{Name: &name})

	assert.Contains(t, d.Render(), "Tomógrafo")
}

func TestLeadDossier_NoParkWiredNoSection(t *testing.T) {
	d := NewLeadDossierBuilder().Build(context.Background(), "acc1",
		&domain.Lead{ID: "l1", ContactID: "556521216363"}, nil)
	assert.NotContains(t, d.Render(), "Parque de equipamentos")
}

// ── Mailbox history (issue #292) ────────────────────────────────────────────
//
// A lead the customer has emailed for years must not be approached like a
// stranger — and the draft must not invent what those emails agreed either.

type stubMailboxHistory struct {
	found []*domain.DiscoveredContact
	err   error
}

func (s stubMailboxHistory) GetByAddresses(context.Context, string, []string) ([]*domain.DiscoveredContact, error) {
	return s.found, s.err
}

func TestDossier_MailboxRelationshipTravelsAsFact(t *testing.T) {
	first := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := NewLeadDossierBuilder().SetMailboxHistory(stubMailboxHistory{
		found: []*domain.DiscoveredContact{{
			Address:     "compras@hospital.com.br",
			SentCount:   37,
			FirstSentAt: &first,
			LastSentAt:  &last,
			LastSubject: "Manutenção preventiva do tomógrafo",
		}},
	})
	lead := &domain.Lead{ID: "l1", AccountID: "acc-1", ContactID: "compras@hospital.com.br"}

	d := b.Build(context.Background(), "acc-1", lead, nil)
	rendered := d.Render()
	assert.Contains(t, rendered, "37 mensagem")
	assert.Contains(t, rendered, "2024-03-10")
	assert.Contains(t, rendered, "Manutenção preventiva do tomógrafo")
	// The epistemic guard rides in the label: envelope facts, never content.
	assert.Contains(t, rendered, "nunca do conteúdo")
}

// An address never seen in the mailbox adds nothing — no empty section, no
// "0 mensagens" line pretending to be information.
func TestDossier_NoMailboxRelationshipAddsNothing(t *testing.T) {
	b := NewLeadDossierBuilder().SetMailboxHistory(stubMailboxHistory{})
	lead := &domain.Lead{ID: "l1", AccountID: "acc-1", ContactID: "novo@hospital.com.br"}

	d := b.Build(context.Background(), "acc-1", lead, nil)
	assert.NotContains(t, d.Render(), "Relação por e-mail")
}

// A phone-keyed lead (WhatsApp) has no address to look up; the reader must
// not be called with garbage and the dossier must stay clean.
func TestDossier_PhoneLeadSkipsMailboxLookup(t *testing.T) {
	b := NewLeadDossierBuilder().SetMailboxHistory(stubMailboxHistory{
		err: errors.New("must not be called"),
	})
	lead := &domain.Lead{ID: "l1", AccountID: "acc-1", ContactID: "5565999990000"}

	d := b.Build(context.Background(), "acc-1", lead, nil)
	assert.NotContains(t, d.Render(), "Relação por e-mail")
}

// The tender is the reason the message exists, so it leads the dossier.
//
// This is the fix for nine production offers, every one of them born from a
// published tender and not one of them naming it. The facts were in the prompt
// the whole time — object, deadline, value, all four of them — ranked at the
// weight of background trivia, below the registry and below inferred patterns.
// A model reading top-down wrote the generic opener the ranking asked for.
func TestDossier_TheTenderLeadsBecauseItIsWhyWeAreWriting(t *testing.T) {
	lead := leadWith(t, map[string]any{
		"cnae_label":            "Atividade médica ambulatorial",
		"decision_maker_name":   "Ana Souza",
		"edital":                "Aquisição de monitores multiparamétricos",
		"edital_prazo":          "2026-09-24",
		"edital_valor_estimado": "250.000,00 BRL",
		"edital_id":             "03507415002864-1-000124/2026",
		"edital_url":            "https://pncp.gov.br/app/editais/1",
	}, map[string]any{"orcamento_previsto": "R$ 50 mil"})

	out := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil).Render()

	tender := indexOf(out, "LICITAÇÃO QUE ESTE ÓRGÃO PUBLICOU")
	require.GreaterOrEqual(t, tender, 0, "the tender must have its own section")
	assert.Less(t, tender, indexOf(out, "O cliente disse"))
	assert.Less(t, tender, indexOf(out, "Registro público"))
	assert.Less(t, tender, indexOf(out, "Pesquisa na web"))

	// And the instruction that makes it actionable rather than decorative.
	assert.Contains(t, out, "abrir citando o objeto e o prazo")
	assert.Contains(t, out, "Aquisição de monitores multiparamétricos")
	assert.Contains(t, out, "2026-09-24")
	assert.Contains(t, out, "250.000,00 BRL")
	// The salesperson reviewing the draft can open what it answers.
	assert.Contains(t, out, "https://pncp.gov.br/app/editais/1")
}

// A lead with no tender must not grow an empty tender heading — an announced
// section over silence teaches the model that a notice exists and is unknown.
func TestDossier_NoTenderMeansNoTenderSection(t *testing.T) {
	lead := leadWith(t, map[string]any{"cnae_label": "Atividade médica ambulatorial"}, nil)

	out := NewLeadDossierBuilder().Build(context.Background(), "acc-1", lead, nil).Render()

	assert.NotContains(t, out, "LICITAÇÃO QUE ESTE ÓRGÃO PUBLICOU")
}
