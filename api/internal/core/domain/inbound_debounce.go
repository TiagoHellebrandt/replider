package domain

import (
	"context"
	"strings"
	"time"
)

// Inbound debounce (§6.15, K1).
//
// People type in bursts: "oi" / "tudo bem?" / "qual o preço do comodato?" in
// three messages over eight seconds. Answering each one costs three model calls
// and three paid messages, and produces three replies to what was one question —
// which is also the tell that nobody is really there.
//
// So the AI waits for a moment of silence. The length of that wait is the whole
// design problem, because the two failure modes are asymmetric:
//
//   - Waiting too long on someone ready to buy loses the sale. Speed IS the
//     product here.
//   - Answering too fast on a lukewarm chat costs money and looks robotic.
//
// Hence an ADAPTIVE window rather than one constant: buy intent answers now,
// a question waits the default, and small talk waits longer.

// DebounceDefaultSeconds is the platform default silence window.
//
// 40s: long enough to catch the "oi" → "tudo bem?" → the actual question burst
// (which in observed WhatsApp traffic lands inside ~15s), short enough that
// somebody who sent one complete question is not left staring at a read receipt.
const DebounceDefaultSeconds = 40

// DebounceIdleMultiplier stretches the window for small talk.
//
// A "bom dia" on its own is almost always followed by the real message. Waiting
// 40s there answers the greeting and then has to answer again — two calls and a
// conversation that reads like a switchboard.
const DebounceIdleMultiplier = 4

// DebounceMaxSeconds caps any single window. Five minutes, from the PRD: past
// that, "waiting for them to finish" is indistinguishable from ignoring them.
const DebounceMaxSeconds = 300

// DebounceMaxTotalWait is the ceiling on the WHOLE burst, measured from the
// first message.
//
// Without it a customer typing continuously defers the answer forever: each new
// message pushes `respond_at` out, and the reply never comes. Three minutes, then
// the AI answers whatever is on the table.
const DebounceMaxTotalWait = 3 * time.Minute

// DebounceConfig is one account's setting.
type DebounceConfig struct {
	// Seconds is the account's silence window. Nil means the platform
	// default; an explicit 0 means answer immediately — the pre-debounce
	// behaviour, which an account must be able to ask for without the column
	// being read as unset.
	Seconds *int
}

// Enabled reports whether the account debounces at all.
func (c DebounceConfig) Enabled() bool {
	return c.Seconds == nil || *c.Seconds > 0
}

// window resolves the configured window, clamped.
func (c DebounceConfig) window() time.Duration {
	seconds := DebounceDefaultSeconds
	if c.Seconds != nil {
		seconds = *c.Seconds
	}
	if seconds < 0 {
		seconds = 0
	}
	if seconds > DebounceMaxSeconds {
		seconds = DebounceMaxSeconds
	}
	return time.Duration(seconds) * time.Second
}

// DebounceDelay decides how long THIS message should wait.
//
// Deterministic and keyword-based on purpose. The alternative — asking a model
// how urgent a message is — would spend a call to decide whether to spend a
// call, and would make the delay unexplainable to the operator watching the
// banner.
func DebounceDelay(text string, cfg DebounceConfig) time.Duration {
	if !cfg.Enabled() {
		return 0
	}
	// Buy intent answers NOW. A person asking to buy is the one moment where
	// every second of silence is a chance for them to close the tab, and the
	// cost of one extra model call is trivially less than the deal.
	if HasBuyIntentSignal(text) {
		return 0
	}
	window := cfg.window()
	if IsSmallTalkOpener(text) {
		idle := window * DebounceIdleMultiplier
		if max := DebounceMaxSeconds * time.Second; idle > max {
			idle = max
		}
		return idle
	}
	return window
}

// buyIntentPhrases are the ones that mean "I want to buy", normalized
// (upper-cased, accent-folded).
//
// SUBSTRING matching here, unlike the opt-out list's exact match, and for the
// opposite reason: an opt-out false positive silences a customer who did not ask
// for it, so it must be conservative — whereas a buy-intent false positive costs
// one fast reply, which is the behaviour we want anyway when in doubt.
var buyIntentPhrases = []string{
	"QUERO COMPRAR", "QUERO FECHAR", "VOU FECHAR", "FECHAR PEDIDO",
	"QUANTO CUSTA", "QUAL O PRECO", "QUAL O VALOR", "ME MANDA O ORCAMENTO",
	"MANDA O ORCAMENTO", "FAZER O PEDIDO", "QUERO CONTRATAR", "COMO FACO PARA COMPRAR",
	"PODE FATURAR", "EMITE A NOTA", "MANDA O LINK DE PAGAMENTO", "COMO PAGO",
	"TEM DISPONIVEL", "TEM EM ESTOQUE", "PRAZO DE ENTREGA",
}

// HasBuyIntentSignal reports a message that should be answered immediately.
func HasBuyIntentSignal(text string) bool {
	normalized := normalizeDebounceText(text)
	if normalized == "" {
		return false
	}
	for _, phrase := range buyIntentPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

// smallTalkOpeners are complete messages that carry no question — the ones
// almost always followed by the real one.
//
// EXACT match on the trimmed message, so "bom dia" waits but "bom dia, vocês
// entregam em Cuiabá?" does not: the second one is a question, and a question
// deserves the normal window.
var smallTalkOpeners = map[string]struct{}{
	"OI": {}, "OII": {}, "OIE": {}, "OLA": {}, "OPA": {}, "EAE": {}, "E AI": {},
	"BOM DIA": {}, "BOA TARDE": {}, "BOA NOITE": {},
	"HEY": {}, "HELLO": {}, "HI": {},
	"TUDO BEM": {}, "TUDO BOM": {}, "COMO VAI": {}, "BELEZA": {},
	"OBRIGADO": {}, "OBRIGADA": {}, "VALEU": {}, "OK": {}, "OKAY": {}, "BLZ": {},
	"CERTO": {}, "ENTENDI": {}, "PERFEITO": {}, "TA BOM": {}, "TA": {},
}

// IsSmallTalkOpener reports a message that is only a greeting or an
// acknowledgement.
func IsSmallTalkOpener(text string) bool {
	normalized := normalizeDebounceText(text)
	if normalized == "" {
		return false
	}
	// A question mark is never small talk, whatever the words are.
	if strings.Contains(normalized, "?") {
		return false
	}
	_, ok := smallTalkOpeners[normalized]
	return ok
}

// normalizeDebounceText upper-cases, folds accents and collapses punctuation and
// runs of whitespace, so "Olá!!" , "ola" and " OLÁ " are one thing.
func normalizeDebounceText(text string) string {
	folded := strings.ToUpper(FoldAccents(strings.TrimSpace(text)))

	var b strings.Builder
	lastSpace := false
	for _, r := range folded {
		switch {
		case r == '?':
			// Kept: it is the strongest "this is a question" signal there is.
			b.WriteRune(r)
			lastSpace = false
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSpace = false
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if !lastSpace && b.Len() > 0 {
				b.WriteRune(' ')
				lastSpace = true
			}
		default:
			// Punctuation and emoji are dropped: "oi!!!" and "oi 👋" are "oi".
		}
	}
	return strings.TrimSpace(b.String())
}

// FoldAccents strips the Portuguese diacritics a customer types, so a keyword
// match is accent-insensitive without a Unicode normalization dependency.
//
// Lives in the domain because two independent keyword matchers need it (the
// opt-out list and the debounce signals), and two copies of a fold table is two
// places for "Ç" to be forgotten.
func FoldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToUpper(s) {
		switch r {
		case 'Á', 'À', 'Â', 'Ã', 'Ä':
			b.WriteRune('A')
		case 'É', 'È', 'Ê', 'Ë':
			b.WriteRune('E')
		case 'Í', 'Ì', 'Î', 'Ï':
			b.WriteRune('I')
		case 'Ó', 'Ò', 'Ô', 'Õ', 'Ö':
			b.WriteRune('O')
		case 'Ú', 'Ù', 'Û', 'Ü':
			b.WriteRune('U')
		case 'Ç':
			b.WriteRune('C')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// PendingInbound is one conversation's queued burst.
type PendingInbound struct {
	ConversationID string
	AccountID      string
	ChannelID      string
	AgentID        string

	RespondAt     time.Time
	FirstQueuedAt time.Time
	MessageCount  int
	Content       string
	ClaimedAt     *time.Time
}

// DeadlineFor is the latest moment this burst may still be deferred.
//
// A method rather than a stored column so lowering DebounceMaxTotalWait takes
// effect for rows already waiting, instead of only for new ones.
func (p PendingInbound) DeadlineFor() time.Time {
	return p.FirstQueuedAt.Add(DebounceMaxTotalWait)
}

// ClampRespondAt keeps a newly-extended wait inside the burst's ceiling.
//
// This is the "customer keeps typing" guard, and it lives in the domain rather
// than the SQL so the rule is testable without a database and stated once.
func (p PendingInbound) ClampRespondAt(proposed time.Time) time.Time {
	deadline := p.DeadlineFor()
	if proposed.After(deadline) {
		return deadline
	}
	return proposed
}

// InboundDebounceRepository persists the queue.
type InboundDebounceRepository interface {
	// Enqueue creates the pending run or EXTENDS the existing one, bumping the
	// message count and replacing the trigger text. Returns the row as it
	// stands after the write, so the caller can tell the dashboard when the AI
	// will answer.
	//
	// Never extends past the burst ceiling, and never touches a row a worker
	// has already claimed — a burst whose run is in flight belongs to the next
	// burst, not this one.
	Enqueue(ctx context.Context, pending *PendingInbound) (*PendingInbound, error)
	// Get reads one conversation's pending run, or (nil, nil).
	Get(ctx context.Context, conversationID string) (*PendingInbound, error)
	// ClaimDue atomically claims up to limit due rows. The claim is what makes
	// two pods safe.
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]*PendingInbound, error)
	// Release clears a claim so the row is retried — used when the run could
	// not start.
	Release(ctx context.Context, conversationID string) error
	// Delete removes a finished run.
	Delete(ctx context.Context, conversationID string) error
	// AnswerNow pulls a pending run's respond_at to now, for the attendant's
	// "responder agora". Returns false when there was nothing pending.
	AnswerNow(ctx context.Context, accountID, conversationID string, now time.Time) (bool, error)
	// ReleaseStaleClaims re-queues rows a dead worker left claimed.
	ReleaseStaleClaims(ctx context.Context, olderThan time.Time) (int64, error)
}
