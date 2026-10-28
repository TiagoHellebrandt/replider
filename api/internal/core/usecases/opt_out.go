package usecases

import (
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// optOutConfirmationText is the short acknowledgment sent back to a contact
// the moment they opt out via keyword (US-025). Kept terse on purpose: a long
// reply to a STOP message reads as engagement-baiting and risks the very
// quality back-pressure the opt-out is meant to avoid.
const optOutConfirmationText = "Pronto! Você não receberá mais mensagens deste número. Se mudar de ideia, é só responder por aqui."

// optOutKeywords are the case-insensitive exact words that trigger an
// automatic opt-out when a customer sends them inbound. We match on the
// TRIMMED full message (not substring) so that "PARAR" triggers opt-out but
// "Posso parar de receber às 18h?" does not — the latter is a conversation
// that the operator should handle manually.
//
// Keys are stored already normalized (upper-cased, accent-folded) so the
// lookup in IsOptOutMessage can compare against a normalized copy of the
// inbound text. Sources: LGPD Art. 18 + industry practice (CAN-SPAM, Meta
// WhatsApp Commerce Policy). Extend this list with tenant-specific aliases in
// a follow-up (store per-account in DB).
var optOutKeywords = map[string]struct{}{
	"PARAR":        {},
	"SAIR":         {},
	"SAI":          {},
	"CANCELAR":     {},
	"DESCADASTRAR": {},
	"STOP":         {},
	"UNSUBSCRIBE":  {},
}

// IsOptOutMessage returns true when the inbound text should be interpreted
// as an opt-out signal. Conservative by design: exact match on a trimmed,
// upper-cased, accent-folded copy of the message — no substring matching, no
// regex. The normalization means "Sair", "SAÍR" and " sair. " all match.
//
// Keeping this deterministic (and small) makes it easy to test, easy to
// audit, and avoids false positives on conversational text.
func IsOptOutMessage(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	// Strip trailing punctuation (.!?) a customer might append.
	trimmed = strings.TrimRight(trimmed, ".!?")
	_, ok := optOutKeywords[normalizeOptOutWord(strings.TrimSpace(trimmed))]
	return ok
}

// normalizeOptOutWord upper-cases and strips common Portuguese diacritics so
// the keyword match is accent-insensitive. The fold table lives in the domain
// (domain.FoldAccents) because the debounce signal matcher needs the same one,
// and two copies is two places for "Ç" to be forgotten.
func normalizeOptOutWord(s string) string {
	return domain.FoldAccents(s)
}
