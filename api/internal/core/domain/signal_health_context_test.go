package domain

import "testing"

// Every fixture below is a VERBATIM object from a tender the production feed
// stored for a hospital-equipment account on 2026-09-09. One row in
// twenty-five was actually for that account, and the two loudest — by value
// and by deadline — were a locksmith contract and inflatable playground toys.
func TestLooksLikeHealthTender_RealTendersFromProduction(t *testing.T) {
	cases := []struct {
		want bool
		org  string
		obj  string
	}{
		// --- the ones that are actually this account's business -----------
		{true, "MUNICIPIO DE UNIAO DO SUL",
			"CONTRATAÇÃO DE EMPRESA ESPECIALIZADA NO FORNECIMENTO DE OXIGÊNIO MEDICINAL E AR COMPRIMIDO"},
		{true, "COMANDO DO EXERCITO",
			"02 (dois) Reparos técnico de cadeiras odontológicas (Olsen, dentemed)"},
		{true, "MUNICIPIO DE ITIQUIRA",
			"AQUISIÇÃO DE PEÇAS DE REPOSIÇÃO ORIGINAIS PARA ATENDER ÀS NECESSIDADES DO LABORATÓRIO"},
		{true, "MUNICIPIO DE SAO JOSE DOS QUATRO MARCOS",
			"Aquisição de gás oxigênio medicinal e fluxometros para atender as necessidades da secretaria municipal de Saúde"},

		// --- the noise that reached a salesperson's morning ---------------
		{false, "ESTADO DE MATO GROSSO",
			"prestação de serviços de chaveiro, abrangendo a confecção de chaves (simples, multiponto, tetra e pantográficas), abertura técnica de portas, cofres"},
		{false, "MUNICIPIO DE PORTO ESPERIDIAO",
			"REGISTRO DE PREÇOS PARA LOCAÇÃO DE BRINQUEDOS INFLÁVEIS E OUTROS EQUIPAMENTOS RECREATIVOS, MESAS E CADEIRAS"},
		// "ventilador" is in the catalogue as a pulmonary ventilator.
		{false, "MUNICIPIO DE JACIARA",
			"Aquisição de aparelhos de televisão e ventiladores, destinados ao atendimento das necessidades da Villa Aconchego"},
		// "bomba" is in the catalogue as an infusion pump.
		{false, "COMANDO DO EXERCITO",
			"Serviço especializado de manutenção corretiva em bomba de abastecimento de combustível"},
		// "termômetro" is a clinical product in the catalogue.
		{false, "SECRETARIA DE ESTADO DE MEIO AMBIENTE",
			"Aquisição de termômetros de ambiente digitais e controles remotos compatíveis com o ar condicionado"},
		// "cilindro" is in the catalogue as a gas cylinder.
		{false, "CAMARA MUNICIPAL DE SAPEZAL",
			"AQUISICAO DE CARTUCHOS DE CILINDRO DR219C"},
		{false, "MUNICIPIO DE RONDONOPOLIS",
			"CONTRATAÇÃO DE SERVIÇO DE LOCAÇÃO DE CARRINHO DE PICOLÉ"},
		{false, "PROCURADORIA GERAL DE JUSTICA",
			"Aquisição de conjunto de microfones sem fio para unidades ministeriais"},
		{false, "JUSTICA FEDERAL DE PRIMEIRA INSTANCIA",
			"aquisição/instalação de 03 gradis de contenção paralelo com as catracas eletrônicas"},
		{false, "CONSELHO REGIONAL DE EDUCACAO FISICA",
			"Aquisição MATERIAL DE EXPEDIENTE"},
	}

	for _, c := range cases {
		got := LooksLikeHealthTender(&Signal{
			Organization: c.org, Title: c.obj,
		})
		if got != c.want {
			t.Errorf("got %v want %v for %q / %q", got, c.want, c.org, shortObj(c.obj))
		}
	}
}

func shortObj(s string) string {
	if len(s) > 58 {
		return s[:58]
	}
	return s
}

// The gate that let a pest-control contract through: `uti` is a prefix of
// UTILIZAÇÃO, UTILIDADE, USUFRUTO. A short abbreviation has to be a whole
// word on both sides.
func TestLooksLikeHealthTender_ShortAbbreviationsAreWholeWords(t *testing.T) {
	for _, obj := range []string{
		"CONTRATAÇÃO PARA UTILIZAÇÃO DE SISTEMA DE GESTÃO",
		"aquisição de material de utilidade doméstica",
		"serviços de dedetização e controle sanitário integrado",
	} {
		if LooksLikeHealthTender(&Signal{Organization: "CAMARA MUNICIPAL", Title: obj}) {
			t.Errorf("matched a non-health tender on a substring: %q", obj)
		}
	}
	// The real abbreviation still counts.
	if !LooksLikeHealthTender(&Signal{Organization: "MUNICIPIO X", Title: "aquisição de 10 leitos de UTI"}) {
		t.Error("a real UTI tender must pass")
	}
}

// The publisher is the fallback: a hospital buying something described as
// "valor que se empenha para atender às despesas" is still a hospital.
func TestLooksLikeHealthTender_ThePublisherIsAFallback(t *testing.T) {
	if !LooksLikeHealthTender(&Signal{
		Organization: "HOSPITAL MUNICIPAL DE CUIABA",
		Title:        "VALOR QUE SE EMPENHA PARA ATENDER ÀS DESPESAS",
	}) {
		t.Error("a hospital's own purchase must pass even with an empty object")
	}
	// But a city hall's health DEPARTMENT is not a health purchase by
	// itself — it is the budget owner, and it buys televisions too.
	if LooksLikeHealthTender(&Signal{
		Organization: "MUNICIPIO DE JACIARA",
		Title:        "Aquisição de aparelhos de televisão para a secretaria",
	}) {
		t.Error("a municipality buying televisions must not pass")
	}
}
