package domain

import (
	"regexp"
	"strings"
)

// Promise detection for AI-drafted outbound (US-020 follow-up).
//
// The meta-prompt already tells the model not to promise. It is not enough:
// measured across real generations, "garantir" survived in most drafts even
// with an explicit ban. A prompt is a request; this is a check.
//
// Why it matters more than tone. A pilot user in medical-equipment service
// put it plainly: a message promising fast guaranteed repair on a hospital
// ventilator is not an awkward sentence, it is a commitment made in her name
// to a customer, before any engineer saw it. The person approving is the one
// who answers for it — so the least this system can do is tell them the
// sentence is there.
//
// Deliberately NOT auto-rewriting: the human owns the words. This flags.
// ApproachFinding is one flagged fragment and WHAT KIND of problem it is.
// The kind matters: telling someone "você prometeu" when the real problem is
// "you asserted something you do not know" sends them to fix the wrong sentence.
type ApproachFinding struct {
	Kind  string `json:"kind"`
	Quote string `json:"quote"`
}

// Placeholders that survived into a message about to be sent.
//
// Kept SEPARATE from the promise patterns because the consequence is
// different in kind. A promise is a judgement call — the sender may have
// meant it, and a human approving can decide. "Olá {{nome}}" is never
// intentional: it is a broken message, and letting it through burns a lead
// for a reason nobody chose.
var placeholderPatterns = []*regexp.Regexp{
	// {{nome}}, {{ empresa }}, {nome}
	regexp.MustCompile(`\{\{?\s*[a-zA-Z0-9_.]+\s*\}?\}`),
	// [Seu Nome], [nome do cliente], <nome>
	regexp.MustCompile(`(?i)\[\s*(seu|sua|nome|empresa|cliente|produto|cargo|cidade)[^\]]{0,40}\]`),
	regexp.MustCompile(`(?i)<\s*(seu|sua|nome|empresa|cliente|produto)[^>]{0,40}>`),
	// XXX / XXXX used as a fill-me marker, and the classic TODO.
	regexp.MustCompile(`\bX{3,}\b`),
	regexp.MustCompile(`(?i)\b(TODO|PREENCHER|INSERIR AQUI)\b`),
}

// PlaceholderFindings returns the unfilled placeholders in a message.
//
// Deterministic on purpose: the AI reviewer already lists `placeholder` as a
// category, but that pass costs a model call and is not always run. A message
// with "{{nome}}" in it must be caught every single time, by something that
// cannot be unavailable.
func PlaceholderFindings(text string) []ApproachFinding {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out []ApproachFinding
	seen := map[string]struct{}{}
	for _, re := range placeholderPatterns {
		for _, m := range re.FindAllString(text, -1) {
			frag := strings.TrimSpace(m)
			key := strings.ToLower(frag)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, ApproachFinding{Kind: "placeholder", Quote: frag})
		}
	}
	return out
}

// attachmentPromiseRe matches a message that says it is carrying a file.
//
// Portuguese first because that is what customers write, with the common
// English forms for a bilingual thread. Matched on the PHRASE rather than on
// the word "anexo" alone: "sem anexo", "não vai anexo" and "aguardo o anexo"
// are all sentences about an attachment that is correctly absent.
var attachmentPromiseRe = regexp.MustCompile(
	`(?i)\b(segue(m)?\s+(em\s+)?anexo|em\s+anexo|anexo\s+(a|o|segue)|` +
		`envio\s+(em\s+)?anexo|anexei|estou\s+anexando|` +
		`please\s+find\s+attached|attached\s+(is|you)|see\s+attached)\b`)

// PromisesAnAttachment reports whether the text tells the reader a file is
// coming with it.
//
// The point is the mismatch: "segue em anexo o termo de referência" with
// nothing attached is a message the recipient will answer with "não veio
// nada", and the salesperson looks careless at exactly the moment they were
// trying not to.
func PromisesAnAttachment(text string) bool {
	return attachmentPromiseRe.MatchString(text)
}

var promisePatterns = []*regexp.Regexp{
	// garantir / garantimos / garantido / garantia de...
	regexp.MustCompile(`(?i)\bgarant(?:e|em|ir|imos|ido|ida|ia)\b`),
	// "atendimento em 24 horas", "resposta em 2h", "em até 48h"
	regexp.MustCompile(`(?i)\bem\s+at[ée]\s+\d+\s*(?:h\b|horas?|dias?|minutos?)`),
	regexp.MustCompile(`(?i)\b(?:atendimento|resposta|retorno|solu[çc][ãa]o)\s+em\s+\d+\s*(?:h\b|horas?|dias?)`),
	// absolutes about capability
	regexp.MustCompile(`(?i)\bresolv(?:emos|e)\s+qualquer\b`),
	regexp.MustCompile(`(?i)\b(?:sempre|nunca)\s+(?:falha|para|quebra)`),
	// claimed knowledge the sender was never given
	regexp.MustCompile(`(?i)\b(?:soube|fiquei sabendo|vi|sei)\s+que\s+(?:voc[êe]s?|a\s|o\s)`),
}

// patternKinds mirrors promisePatterns index-for-index. Kept adjacent so a
// new pattern without a kind fails loudly in the test rather than silently
// inheriting "promise".
var patternKinds = []string{
	"promise", "promise", "promise",
	"promise", "promise",
	"invented_fact",
}

// ApproachFindings is ApproachPromises with the kind attached.
func ApproachFindings(text string) []ApproachFinding {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out []ApproachFinding
	seen := map[string]struct{}{}
	for i, re := range promisePatterns {
		kind := "promise"
		if i < len(patternKinds) {
			kind = patternKinds[i]
		}
		for _, m := range re.FindAllString(text, -1) {
			frag := strings.TrimSpace(m)
			key := strings.ToLower(frag)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, ApproachFinding{Kind: kind, Quote: frag})
		}
	}
	return out
}

// ApproachPromises returns the promise-like fragments found in a drafted
// message, in the order they appear. Empty means nothing to warn about.
func ApproachPromises(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var found []string
	seen := map[string]struct{}{}
	for _, re := range promisePatterns {
		for _, m := range re.FindAllString(text, -1) {
			key := strings.ToLower(strings.TrimSpace(m))
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			found = append(found, strings.TrimSpace(m))
		}
	}
	return found
}

// HasApproachPromise is the boolean form, for callers that only branch.
func HasApproachPromise(text string) bool { return len(ApproachPromises(text)) > 0 }
