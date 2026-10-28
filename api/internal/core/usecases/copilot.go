package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/replider/api/internal/core/domain"
)

// The in-app copilot (US-M026).
//
// One chat that does the SDR's job through the same use cases the screens
// call, so there is exactly one implementation of "prospect", "draft" and
// "move a lead" — a copilot with its own copies would drift from the UI
// within a month and the two would disagree in front of the customer.
//
// What it deliberately CANNOT do: send. Approving outbound stays in the
// human queue (US-M020), because the product's whole posture on cold
// outreach is that a person reads the exact text and the recipient before
// it reaches a stranger. A chat that could send on "manda pra eles" would
// undo that in one sentence.

// copilotSystemPrompt frames the assistant. Written in the language the
// user actually works in.
const copilotSystemPrompt = `Você é o copiloto de vendas da Replider, dentro do app de um vendedor B2B de saúde.

Seu trabalho: executar o que o vendedor pede usando as ferramentas, e responder em 1-3 frases dizendo o que FOI FEITO e o que ele precisa fazer agora.

Regras:
- Use as ferramentas para agir. Não invente números: se não chamou a ferramenta, não sabe.
- Quando uma ferramenta devolver números ou uma lista, CITE-OS na resposta. "Está na fila" não ajuda ninguém; "2 abordagens esperando, para Clínica X e Hospital Y" ajuda.
- Você NUNCA envia mensagem para um lead. Você prepara abordagens; quem aprova e envia é o vendedor, na fila de aprovação. Se pedirem para enviar, explique isso e ofereça preparar.
- Antes de gerar abordagens em lote, confirme o funil quando houver mais de um.
- Para "por onde começo?", use get_pending_actions; para licitações abertas, list_signals.
- Português do Brasil, direto, sem floreio. Valores em reais.`

// CopilotAction is something the answer wants the UI to offer — a deep
// link the user can act on without retyping what they just asked for.
type CopilotAction struct {
	Label string `json:"label"`
	// Route is an in-app location (e.g. "/prospect?tab=queue").
	Route string `json:"route"`
}

// CopilotReceipt is what a tool that WRITES reported, word for word, without
// passing through the model.
//
// The loop already asked it to "use these numbers in the answer" and the model
// rewrote them anyway: a prospecting run that created 56 leads was announced as
// "1 of them was added". Asking a text generator for honesty is not a
// mechanism. The receipt is: it leaves the server and the screen prints it.
//
// Only for tools that write. On a read, a wrong paraphrase costs one more
// question; on a write, the user is left not knowing what entered their
// account.
type CopilotReceipt struct {
	Tool string `json:"tool"`
	Text string `json:"text"`
}

// CopilotReply is one assistant turn.
type CopilotReply struct {
	Message string `json:"message"`
	// ContextMessage is the text BEFORE identifiers are stripped, for the client
	// to send back as history on the next question.
	//
	// Taking the ULID out of the message took it out of the conversation too: the
	// model asked "confirm which funnel", the user said "yes, use that one", and
	// the id was nowhere any more — the next call went out with a guessed id and
	// came back SALES_FUNNEL_NOT_FOUND. Whoever reads and whatever continues the
	// conversation are different consumers of the same text.
	//
	// Empty when there was nothing to strip: the client falls back to Message.
	ContextMessage string          `json:"context_message,omitempty"`
	Actions        []CopilotAction `json:"actions,omitempty"`
	// ToolsUsed names what actually ran, so the UI can show the work
	// instead of asking the user to trust a paragraph.
	ToolsUsed []string `json:"tools_used,omitempty"`
	// Receipts carries the literal result of every tool that wrote.
	Receipts []CopilotReceipt `json:"receipts,omitempty"`
}

// toolCallKey identifies a call by tool + arguments.
//
// The arguments go through a JSON round-trip because the same request comes back
// with its keys in different orders between rounds — comparing the raw string
// would let the repeat through. json.Marshal of a map sorts keys, which is
// exactly the normalisation needed. An unreadable argument falls back to the
// original string: better an undetected repeat than two different calls treated
// as one.
func toolCallKey(call domain.ToolCall) string {
	args := strings.TrimSpace(call.Arguments)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err == nil {
		if norm, err := json.Marshal(parsed); err == nil {
			args = string(norm)
		}
	}
	return call.FunctionName + "|" + args
}

// ulidPattern matches a bare ULID in prose, with or without the surrounding
// "(id: ...)". Crockford base32: no I, L, O or U.
var ulidPattern = regexp.MustCompile(
	`\s*\(?\s*(?:id:?\s*)?\b[0-9A-HJKMNP-TV-Z]{26}\b\s*\)?`)

// stripIdentifiers removes ULIDs from the answer.
//
// The copilot wrote 'The only funnel available is "TESTE QA" (id:
// 01KYW9HYFXQK3Y41HAJFTG9CSS)'. The id exists for the tool to make the next
// call, not for a person to read — whoever is on the other side knows the funnel
// by its name, and 26 characters of base32 mid-sentence only get in the way.
//
// Done here rather than by an instruction in the prompt: an instruction the
// model forgets, and this is the same turn in which it rewrote 56 as 1.
func stripIdentifiers(msg string) string {
	locs := ulidPattern.FindAllStringIndex(msg, -1)
	if len(locs) == 0 {
		return msg
	}

	var b strings.Builder
	prev := 0
	for _, loc := range locs {
		b.WriteString(msg[prev:loc[0]])
		// The pattern eats the space BEFORE the id, so the gap needs one put
		// back — but only when both edges are content. Without the condition,
		// "Moved lead X to" becomes "Moved leadto"; with an unconditional space,
		// `funnel "TESTE QA (id: X)"` becomes `funnel "TESTE QA "`.
		if joinsWords(lastRune(msg[:loc[0]]), firstRune(msg[loc[1]:])) {
			b.WriteByte(' ')
		}
		prev = loc[1]
	}
	b.WriteString(msg[prev:])
	return strings.TrimSpace(b.String())
}

// joinsWords informa se as bordas do buraco precisam de um espaço entre si.
func joinsWords(left, right rune) bool {
	word := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	return word(left) && word(right)
}

func lastRune(s string) rune {
	for _, r := range reverseRunes(s) {
		return r
	}
	return 0
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

func reverseRunes(s string) []rune {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return rs
}

// copilotWriteTools names the tools that CHANGE the account — the ones that
// import leads or fill the approval queue. One list, here: a tool that writes
// and is missing from it hands its number over through the model's mouth, which
// is exactly the defect the receipt exists to close.
var copilotWriteTools = map[string]bool{
	"draft_approaches":    true,
	"draft_lead_approach": true,
}

func copilotToolWrites(name string) bool { return copilotWriteTools[name] }

// hasAction reports whether the reply already offers this destination.
func (r *CopilotReply) hasAction(route string) bool {
	for _, a := range r.Actions {
		if a.Route == route {
			return true
		}
	}
	return false
}

// CopilotUseCase runs one turn of the copilot conversation.
type CopilotUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	aiFactory   domain.AIFactory

	funnels     *FunnelUseCase
	leads       *LeadUseCase
	prospecting *ProspectingUseCase
	approach    *ApproachUseCase
	caseMetrics *CaseMetricsUseCase
	// commandCenter and signals back the SDR reads (SetSDRTools): the same
	// "what needs me" list the home screen ranks, and the open tenders.
	commandCenter *CommandCenterUseCase
	signals       domain.SignalRepository
	// meter prices the copilot's own AI spend (US-M015): a chat that
	// burns tokens invisibly would break the "no markup, all visible"
	// promise the same way drafts once did.
	meter *AIMeter
	// spend is the budget-only gate derived from the meter (customer
	// ceiling / platform default). Nil allows.
	spend *spendGate
}

func NewCopilotUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	aiFactory domain.AIFactory,
) *CopilotUseCase {
	return &CopilotUseCase{accountRepo: accountRepo, planRepo: planRepo, aiFactory: aiFactory}
}

// SetTools wires the use cases the copilot drives. Anything left nil
// simply removes that capability from the tool list — the copilot then
// says it cannot do it instead of failing mid-call.
func (uc *CopilotUseCase) SetTools(
	funnels *FunnelUseCase,
	leads *LeadUseCase,
	prospecting *ProspectingUseCase,
	approach *ApproachUseCase,
	caseMetrics *CaseMetricsUseCase,
) {
	uc.funnels = funnels
	uc.leads = leads
	uc.prospecting = prospecting
	uc.approach = approach
	uc.caseMetrics = caseMetrics
}

// SetSDRTools wires the SDR reads. Nil leaves each tool off the list.
func (uc *CopilotUseCase) SetSDRTools(commandCenter *CommandCenterUseCase, signals domain.SignalRepository) {
	uc.commandCenter = commandCenter
	uc.signals = signals
}

// SetAIMeter wires transparent token metering.
func (uc *CopilotUseCase) SetAIMeter(m *AIMeter) {
	uc.meter = m
	uc.spend = budgetGateFromMeter(uc.accountRepo, m)
}

// maxCopilotToolRounds bounds the tool loop. A model that keeps calling
// tools is either confused or looping; either way the user is waiting.
const maxCopilotToolRounds = 4

// Ask runs one turn. History is the prior conversation (oldest first).
func (uc *CopilotUseCase) Ask(ctx context.Context, actor domain.Actor, history []domain.ChatMessage, message string) (*CopilotReply, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "mensagem vazia"}
	}
	// The copilot acts on the pipeline, so it answers to the same gate the
	// pipeline does: an attendant has no business here.
	if !domain.RoleSatisfies(actor.Role, domain.RoleGestor) && actor.Role != domain.RoleVendedor {
		return nil, domain.ErrForbidden
	}

	account, err := uc.accountRepo.GetByID(ctx, actor.AccountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && *account.PlanID != "" && uc.planRepo != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	if err := uc.spend.ensureAnyFor(ctx, account); err != nil {
		return nil, err
	}
	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("copiloto indisponível: %v", err),
		}
	}

	convo := append([]domain.ChatMessage{}, history...)
	convo = append(convo, domain.ChatMessage{Role: "user", Content: message})

	reply := &CopilotReply{}
	tools := uc.toolDefs()
	// The result of each (tool, arguments) already executed this turn.
	done := map[string]string{}

	for round := 0; round < maxCopilotToolRounds; round++ {
		resp, err := provider.GenerateResponse(ctx, copilotSystemPrompt, convo, tools)
		if err != nil || resp == nil {
			return nil, &domain.AppError{
				Code:    domain.ErrAIProviderUnavailable.Code,
				Message: fmt.Sprintf("copiloto falhou: %v", err),
			}
		}
		uc.meter.Record(ctx, actor.AccountID, "copilot", plan.AIProvider, plan.AIModel, resp.Usage)

		if len(resp.ToolCalls) == 0 {
			raw := strings.TrimSpace(resp.Content)
			reply.Message = stripIdentifiers(raw)
			if reply.Message != raw {
				reply.ContextMessage = raw
			}
			if reply.Message == "" {
				reply.Message = "Não consegui responder isso agora."
			}
			return reply, nil
		}

		for _, call := range resp.ToolCalls {
			// The model repeats the SAME call when it is not satisfied with
			// the answer. Seen in production: four identical
			// prospect_businesses in one turn — the first created three
			// leads, the other three created zero (dedup held) and cost
			// three paid Google Places searches. The rounds spent on that
			// are the same ones it then lacked to finish, and the turn ended
			// in "I lost my way".
			//
			// Repeating is not re-executing: it returns what is already known
			// and says it already ran. The side effect happens once a turn.
			key := toolCallKey(call)
			if prev, seen := done[key]; seen {
				convo = append(convo, domain.ChatMessage{
					Role: "user",
					Content: fmt.Sprintf(
						"A ferramenta %s JÁ RODOU neste turno com estes mesmos "+
							"argumentos e não vai rodar de novo. Resultado:\n%s\n"+
							"Responda com estes números.",
						call.FunctionName, prev),
				})
				continue
			}

			result, action := uc.runTool(ctx, actor, call)
			done[key] = result
			reply.ToolsUsed = append(reply.ToolsUsed, call.FunctionName)
			if copilotToolWrites(call.FunctionName) {
				reply.Receipts = append(reply.Receipts, CopilotReceipt{
					Tool: call.FunctionName,
					Text: strings.TrimSpace(result),
				})
			}
			if action != nil && !reply.hasAction(action.Route) {
				// Deduped by destination: two tools that both end at the
				// approval queue must not stack two identical buttons.
				reply.Actions = append(reply.Actions, *action)
			}
			// The tool's output goes back as conversation, which is what
			// lets the model summarise real numbers instead of guessing.
			// Fed back as a user turn labelled as data: as a "model" turn
			// the model treats it as something it already said and
			// paraphrases the numbers away instead of quoting them.
			convo = append(convo, domain.ChatMessage{
				Role: "user",
				Content: fmt.Sprintf(
					"DADOS da ferramenta %s (use estes números na resposta):\n%s",
					call.FunctionName, result),
			})
		}
	}

	// Ran out of rounds with the model still asking for tools.
	reply.Message = "Fiz o que consegui, mas me perdi no meio. Pode repetir de forma mais direta?"
	return reply, nil
}
