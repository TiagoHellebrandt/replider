package domain

import (
	"regexp"
	"strings"
)

// Is this tender about health at all?
//
// The question exists because a derived watch's keywords are the account's
// own product words, and outside a hospital most of them mean something else.
// The gate looks for a term that only a health purchase uses — and takes it
// from WHAT IS BEING BOUGHT, not from who is paying: a city hall names its
// health department as the budget owner while buying televisions, so "saúde"
// on its own keeps almost everything.

// healthProductPrefixes are terms long enough that a prefix match is safe:
// `odontolog` reaches "odontológicas", `cirurgic` reaches "cirúrgicos".
var healthProductPrefixes = []string{
	"hospitalar", "medicinal", "enfermagem", "paciente", "ambulanc",
	"odontolog", "cirurgic", "cirurgia", "neonatal", "obstetric", "farmac",
	"medicamento", "autoclave", "esteriliz", "laboratori", "tomograf",
	"ultrassom", "oxigenoterapia", "hemodialise", "internacao", "ambulatorial",
	"radiolog", "fisioterap", "hemodinamic", "oximetr", "desfibrilad",
	"eletrocardiograf", "incubadora", "anestesi", "endoscop", "mamograf",

	// Device names that exist in no other trade.
	//
	// Added when the buying calendar started using this gate and dropped
	// genuine purchases: "AQUISIÇÃO DE MONITORES MULTIPARAMÉTRICOS" and
	// "VENTILADORES PULMONARES" carry no word from the list above, and the
	// bodies buying them are named "Estado de Mato Grosso". Rejecting those is
	// the opposite failure from the one this gate was built for, and the more
	// expensive one: a false positive wastes a glance, a false negative loses
	// the deal silently.
	//
	// Every term here is unambiguous on purpose. None of them re-admits any of
	// the four false positives measured in Mato Grosso — a heated pass-through,
	// a drinking fountain, a canteen scale and a bundle of assorted equipment.
	"multiparametr", "bilirrubin", "transcutane", "reanimador", "nebulizad",
	"pulmonar", "capnograf", "laringoscop", "fototerap", "eletrocirurg",
	"monitorizacao", "berco aquecido", "arco cirurgico",
}

// healthProductWords must match as WHOLE words. Every one is a prefix of an
// ordinary Portuguese word, and prefix-matching them is how a pest-control
// contract read as intensive care: `uti` hides inside "UTILIZAÇÃO".
var healthProductWords = []string{
	"uti", "ubs", "upa", "samu", "leito", "leitos", "raio-x", "raio x",
}

// healthOrgTerms identify a body whose purchases are health purchases even
// when the object is written badly ("valor que se empenha para atender…").
var healthOrgTerms = []string{
	"hospital", "santa casa", "maternidade", "policlinica", "pronto socorro",
	"pronto atendimento", "consorcio regional de saude",
	"secretaria municipal de saude", "secretaria de estado de saude",
	"fundacao de saude", "instituto de saude", "upa", "samu",
}

var (
	healthPrefixRe = regexp.MustCompile(`(?:^|[^a-z0-9])(` +
		strings.Join(healthProductPrefixes, "|") + `)`)
	healthWordRe = regexp.MustCompile(`(?:^|[^a-z0-9])(` +
		strings.Join(healthProductWords, "|") + `)(?:[^a-z0-9]|$)`)
	healthOrgRe = regexp.MustCompile(`(?:^|[^a-z0-9])(` +
		strings.Join(healthOrgTerms, "|") + `)`)
)

// LooksLikeHealthTender reports whether a tender is plausibly a health
// purchase — from its object, or from the body that published it.
//
// Exported for measurement and tests; Matches uses it through the unexported
// wrapper so the gate has exactly one caller in production code.
func LooksLikeHealthTender(s *Signal) bool { return looksLikeHealthTender(s) }

// LooksLikeHealthPurchase asks the same question of any published purchase,
// not just a tender.
//
// The buying calendar matches the SAME catalogue-derived keywords the radar
// matches and had no such gate, so it inherited the whole failure the radar
// was given this gate to fix. Measured over the 362 contracts collected for
// Mato Grosso on 2026-09-14: four matched the keywords and ALL FOUR were
// wrong — a heated pass-through for a student restaurant ("aquecido"), an
// industrial drinking fountain ("reservatório"), an electronic scale for a
// campus canteen ("balança"), and a bundle of assorted equipment
// ("ventilador", almost certainly ceiling fans).
//
// None of them happened to fall inside the calendar's window, which is the
// only reason nobody has seen a food cart presented as a buying opportunity.
func LooksLikeHealthPurchase(object, organization string) bool {
	return looksLikeHealthTender(&Signal{Title: object, Organization: organization})
}

func looksLikeHealthTender(s *Signal) bool {
	if s == nil {
		return false
	}
	object := strings.ToLower(FoldAccents(s.Title + " " + s.Description))
	if healthPrefixRe.MatchString(object) || healthWordRe.MatchString(object) {
		return true
	}
	// The publisher is the fallback, not the primary test. A hospital buying
	// something badly described is still a hospital; a city hall's health
	// department buying televisions is not a health purchase.
	return healthOrgRe.MatchString(strings.ToLower(FoldAccents(s.Organization)))
}
