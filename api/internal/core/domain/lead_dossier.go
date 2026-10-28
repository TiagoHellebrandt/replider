package domain

import (
	"fmt"
	"sort"
	"strings"
)

// The dossier: everything the account already knows about one lead, in one
// place, with where each piece came from.
//
// The approach generator used to see the funnel's persona, one product, the
// contact's name, the collected fields and the raw enrichment blob. Meanwhile
// the same account already held: which products its customers buy together
// (with evidence), what this customer has bought and when it runs out, the
// buying signals somebody recorded, its own knowledge base, and the company's
// registered activity. None of it reached the message.
//
// So the proposal was written from a fraction of what had already been paid
// for. This type is the rest of it.
//
// Two rules make it usable rather than merely bigger:
//
//  1. EVERY fact carries its provenance. A model handed a sentence with no
//     source will cite it as established, and the one thing a cold approach
//     cannot afford is asserting something the recipient knows is wrong. The
//     rendered payload names the origin of each line so the model can hedge —
//     and so the person reviewing can go check.
//
//  2. A pattern carries its EVIDENCE, not just its conclusion. "They probably
//     need masks" is a guess with a confident voice. "8 of the 11 customers who
//     buy gloves here also buy masks" is a sentence the salesperson can say out
//     loud and defend. CoPurchasePair already computes exactly that; the
//     dossier's job is to stop throwing it away.
//
// Everything is scoped to ONE account, deliberately — same reasoning as
// cross_sell.go. What a distributor sells to whom is their commercial position.

// DossierFactKind names where a fact came from, because that is what decides
// how much weight the message may put on it.
type DossierFactKind string

const (
	// DossierRegistry is public registered data (activity, size, region).
	// Checkable, so it may be stated plainly.
	DossierRegistry DossierFactKind = "registry"
	// DossierResearch came from a grounded web search. Real sources, but a
	// model chose what to read — so it is worth citing, not asserting.
	DossierResearch DossierFactKind = "research"
	// DossierCollected was answered by the customer themselves, in a
	// conversation. The strongest kind: they said it.
	DossierCollected DossierFactKind = "collected"
	// DossierOwnSales is this account's own transaction history. Fact, not
	// inference.
	DossierOwnSales DossierFactKind = "own_sales"
	// DossierOwnMailbox is the shape of the account's own email relationship
	// with this contact, learned from the connected mailbox's Sent folder
	// (envelope-only — issue #292). Fact: the customer really wrote those
	// messages. It says HOW MUCH and ABOUT WHAT the relationship was, never
	// what anyone promised.
	DossierOwnMailbox DossierFactKind = "own_mailbox"
	// DossierPattern is inferred FROM own sales — a co-purchase rate. Always
	// arrives with its numerator and denominator.
	DossierPattern DossierFactKind = "pattern"
	// DossierSignal is a recorded intent-to-buy event.
	DossierSignal DossierFactKind = "signal"
	// DossierTender is the published notice this lead was BORN from — the
	// buyer's own written statement that they intend to buy a named thing by
	// a named date. Separate from DossierSignal because it is not background
	// about the customer: it is the occasion for the message.
	DossierTender DossierFactKind = "tender"
	// DossierKnowledge is a passage from the account's own knowledge base:
	// what the company can actually promise.
	DossierKnowledge DossierFactKind = "knowledge"
)

// Weight orders the kinds by how much a message may lean on them.
//
// Highest first: what the customer said, then what we sold them, then the
// public record, then what we inferred, then what a search read. A message that
// leads with an inference and buries the customer's own words has the ladder
// upside down.
func (k DossierFactKind) Weight() int {
	switch k {
	case DossierTender:
		// Above everything, including the customer's own words to us: a
		// published tender IS their own words, in writing, checkable, and it
		// is the reason this message is being written at all.
		//
		// It used to ride at DossierSignal's weight of 2 — below the registry,
		// below inferred patterns — and the consequence was measurable. Every
		// one of the nine tender-born offers waiting in production opened with
		// "apoiamos unidades de saúde com equipamentos hospitalares" and not
		// one named the notice, the deadline or the value, while all three sat
		// in the same prompt a few hundred tokens further down. The facts were
		// never missing; they were ranked as background.
		return 7
	case DossierCollected:
		return 6
	case DossierOwnSales:
		return 5
	case DossierOwnMailbox:
		return 5
	case DossierKnowledge:
		return 4
	case DossierRegistry:
		return 3
	case DossierPattern:
		return 2
	case DossierSignal:
		return 2
	case DossierResearch:
		return 1
	}
	return 0
}

// DossierFact is one thing known, and how it is known.
type DossierFact struct {
	Kind  DossierFactKind `json:"kind"`
	Label string          `json:"label"`
	Value string          `json:"value"`

	// Evidence is the count behind a pattern, in words the salesperson can
	// repeat: "8 of 11 customers". Empty for facts that are not inferred.
	Evidence string `json:"evidence,omitempty"`
	// Source is a URL or a named origin. Empty when the origin IS the kind
	// (own sales need no citation to the person who made the sale).
	Source string `json:"source,omitempty"`
}

// LeadDossier is the assembled picture. Sections rather than one flat list,
// because the generator needs to treat "what they told us" differently from
// "what we guessed".
type LeadDossier struct {
	Facts []DossierFact `json:"facts"`

	// Missing names what was looked for and not found.
	//
	// An absent section reads as "nothing to say"; a named gap reads as "this
	// is unknown", and those are different instructions to a model deciding
	// whether to make a claim. It is also the list that tells the person which
	// question is worth asking first.
	Missing []string `json:"missing,omitempty"`
}

// Empty reports a dossier with nothing in it, so a caller can skip the section
// entirely rather than rendering a heading over silence.
func (d LeadDossier) Empty() bool { return len(d.Facts) == 0 }

// Add appends a fact, dropping anything with no value: a label with an empty
// value teaches the model that the field exists and is unknown, which is what
// Missing says better.
func (d *LeadDossier) Add(f DossierFact) {
	f.Label = strings.TrimSpace(f.Label)
	f.Value = strings.TrimSpace(f.Value)
	if f.Label == "" || f.Value == "" {
		return
	}
	d.Facts = append(d.Facts, f)
}

// MarkMissing records a gap once.
func (d *LeadDossier) MarkMissing(what string) {
	what = strings.TrimSpace(what)
	if what == "" {
		return
	}
	for _, m := range d.Missing {
		if m == what {
			return
		}
	}
	d.Missing = append(d.Missing, what)
}

// dossierKindLabel is how each origin is announced to the model. Written as a
// claim about EPISTEMIC STATUS rather than as a data-source name: "the customer
// said" tells the model how to use it, "collected_metadata" does not.
var dossierKindLabel = map[DossierFactKind]string{
	DossierTender: "LICITAÇÃO QUE ESTE ÓRGÃO PUBLICOU — é POR ISSO que estamos " +
		"escrevendo. A mensagem PRECISA abrir citando o objeto e o prazo, com " +
		"as palavras do próprio edital. Um texto genérico aqui prova que " +
		"ninguém leu o que eles publicaram",
	DossierCollected: "O cliente disse (pode ser afirmado)",
	DossierOwnSales:  "Histórico de compras com a gente (fato)",
	DossierOwnMailbox: "Relação por e-mail com a gente (fato — do envelope, nunca do conteúdo; " +
		"não afirme o que foi combinado, apenas retome o contato)",
	DossierKnowledge: "O que a nossa empresa pode prometer",
	DossierRegistry:  "Registro público (verificável)",
	DossierPattern:   "Padrão observado nas nossas vendas (cite o número)",
	DossierSignal:    "Sinal de intenção registrado",
	DossierResearch:  "Pesquisa na web (cite como leitura, não como certeza)",
}

// Render turns the dossier into the block the generator reads.
//
// Grouped by origin and ordered by weight, so the strongest material is the
// first thing the model sees. Each group is prefixed with what its facts are
// epistemically — the difference between a message that says "vocês compram
// luva" and one that says "vi que vocês talvez trabalhem com luva" is entirely
// in this labelling, and getting it wrong is how a draft ends up asserting
// something invented to somebody who knows better.
func (d LeadDossier) Render() string {
	if d.Empty() && len(d.Missing) == 0 {
		return ""
	}

	byKind := map[DossierFactKind][]DossierFact{}
	for _, f := range d.Facts {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}
	kinds := make([]DossierFactKind, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	// Stable: same dossier renders identically, so a re-draft is comparable to
	// the previous one and the provider's prefix cache is not thrown away by a
	// map's iteration order.
	sort.Slice(kinds, func(i, j int) bool {
		if kinds[i].Weight() != kinds[j].Weight() {
			return kinds[i].Weight() > kinds[j].Weight()
		}
		return kinds[i] < kinds[j]
	})

	var b strings.Builder
	b.WriteString("O que já sabemos deste lead (use o que sustentar a conversa; " +
		"NUNCA afirme além do que a origem permite):\n")
	for _, k := range kinds {
		label := dossierKindLabel[k]
		if label == "" {
			label = string(k)
		}
		fmt.Fprintf(&b, "\n%s:\n", label)
		for _, f := range byKind[k] {
			fmt.Fprintf(&b, "  - %s: %s", f.Label, f.Value)
			if f.Evidence != "" {
				fmt.Fprintf(&b, " [%s]", f.Evidence)
			}
			if f.Source != "" {
				fmt.Fprintf(&b, " (fonte: %s)", f.Source)
			}
			b.WriteString("\n")
		}
	}
	if len(d.Missing) > 0 {
		// Named rather than omitted: the model must not fill a gap it cannot
		// see, and a good first message often ASKS one of these instead of
		// pretending to know it.
		fmt.Fprintf(&b, "\nAinda não sabemos (não invente; se ajudar, pergunte):\n  - %s\n",
			strings.Join(d.Missing, "\n  - "))
	}
	return b.String()
}
