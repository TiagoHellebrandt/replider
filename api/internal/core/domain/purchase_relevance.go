package domain

import (
	"regexp"
	"strings"
)

// Is this contract a purchase of the KIND of thing I sell?
//
// The health gate (signal_health_context.go) answers a different and easier
// question — "is this about health at all" — and the buying calendar proved it
// is not enough on its own. Measured over 307 in-window contracts for one
// account on 2026-09-14, matching the catalogue's words plus that gate
// returned three rows and all three were wrong: two were "prestação de
// serviços médicos especializados em cirurgia vascular" (a service, matched on
// "cirúrgico") and one was spare parts for a municipal laboratory.
//
// Two more tests fix that, and both come from how Brazilian procurement is
// actually written.

// goodsRe recognises a purchase of THINGS.
//
// A supplier of equipment cannot bid on a contract for medical labour, however
// medical the words in it are. "Aquisição", "fornecimento" and "registro de
// preços" name a purchase; the service forms name work.
var goodsRe = regexp.MustCompile(`(?:^|[^a-z0-9])(aquisicao|aquisicoes|compra|fornecimento|registro de precos)`)

// serviceRe recognises work being bought instead of goods.
//
// Checked FIRST and independently, because a notice often says both: "registro
// de preços para contratação de serviços de manutenção" is a service with a
// purchase word in it.
var serviceRe = regexp.MustCompile(`(?:^|[^a-z0-9])(prestacao de servic|contratacao de servic|servicos medicos|credenciamento|locacao|manutencao|reparo)`)

// otherTradeRe names trades that are not ours however medical the buyer is.
//
// A hospital buys computers, furniture, fuel and food, and it buys them with
// exactly the vocabulary a catalogue shares: "microcomputadores, notebooks e
// MONITORES" matched a supplier of multiparameter monitors and would have been
// the first thing a salesperson saw. Two of fourteen matches across 2.625
// stored contracts were that line; it is the only class the goods/service test
// does not already remove.
var otherTradeRe = regexp.MustCompile(`(?:^|[^a-z0-9])(microcomputador|notebook|impressora|computador|informatica|software|mobiliario|veiculo|combustivel|alimenticio|merenda|generos)`)

// IsGoodsPurchaseInOurTrade reports whether a published contract is a purchase
// of goods, in health, and not in an adjacent trade that happens to share a
// word with the catalogue.
//
// Deliberately three cheap regexes and no model call: the question is about
// the WORDS a notice uses, and a notice that has to be interpreted is a notice
// the salesperson should read themselves.
func IsGoodsPurchaseInOurTrade(object, buyerName string) bool {
	text := strings.ToLower(FoldAccents(object))
	if serviceRe.MatchString(text) || !goodsRe.MatchString(text) {
		return false
	}
	if otherTradeRe.MatchString(text) {
		return false
	}
	return LooksLikeHealthPurchase(object, buyerName)
}
