package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Copilot tool surface (US-M026).
//
// Every tool delegates to the use case the SCREENS call, so ownership
// rules, plan limits and instrumentation apply to the copilot exactly as
// they apply to a click. There is no copilot-only path into the pipeline.
//
// The catalogue is a hand-picked projection of the MCP surface
// (adapters/mcp): the MCP adapter cannot be imported from here (adapters
// depend on usecases, never the reverse), so the names and semantics are
// kept identical by convention — get_pending_actions, list_signals,
// draft_lead_approach and list_funnels mean the same thing in both.
//
// Notably absent: anything that sends. See the note on CopilotUseCase.
// Also absent since the SDR pivot: Google Places prospecting — the radar
// (CNES park + PNCP signals) replaced it, and the screen that offered it
// is gone.

func (uc *CopilotUseCase) toolDefs() []domain.AITool {
	var tools []domain.AITool

	if uc.commandCenter != nil {
		tools = append(tools, domain.AITool{
			Name: "get_pending_actions",
			Description: "O que precisa de uma pessoa agora, na mesma ordem da tela de início: " +
				"prazo vencido primeiro, depois valor em jogo, depois os mais antigos. Use para 'por onde começo?'.",
			// Zero-argument tools send NO schema: Gemini rejects a function
			// declaration whose parameters is an object with empty
			// properties, and rejects the whole request with it — so one
			// argument-less tool silently broke every answer.
			Parameters: nil,
		})
	}
	if uc.funnels != nil {
		tools = append(tools, domain.AITool{
			Name:        "list_funnels",
			Description: "Lista os funis da conta (id e nome). Use antes de qualquer ação que precise de um funil.",
			Parameters:  nil,
		})
	}
	if uc.leads != nil {
		tools = append(tools,
			domain.AITool{
				Name:        "pipeline_overview",
				Description: "Números do funil: leads ativos, aguardando aprovação, sem dono, total.",
				Parameters:  nil,
			},
			domain.AITool{
				Name:        "list_approval_queue",
				Description: "Lista as abordagens aguardando aprovação humana (quem recebe e o texto).",
				Parameters:  nil,
			},
		)
	}
	if uc.signals != nil {
		tools = append(tools, domain.AITool{
			Name: "list_signals",
			Description: "Licitações abertas (PNCP) que interessam à conta, prazo mais próximo primeiro. " +
				"Já vêm sem as vencidas.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "quantos sinais no máximo (padrão 10)"},
				},
			},
		})
	}
	if uc.approach != nil {
		tools = append(tools,
			domain.AITool{
				Name: "draft_lead_approach",
				Description: "Gera a oferta por IA para UM lead (e-mail) e coloca na FILA DE APROVAÇÃO. " +
					"Não envia nada — quem envia é o vendedor, aprovando na fila.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"lead_id": map[string]any{"type": "string", "description": "id do lead"},
					},
					"required": []string{"lead_id"},
				},
			},
			domain.AITool{
				Name: "draft_approaches",
				Description: "Gera abordagens por IA para os leads do funil e coloca na FILA DE APROVAÇÃO. " +
					"Não envia nada — quem envia é o vendedor, aprovando na fila.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"funnel_id": map[string]any{"type": "string"},
						"limit":     map[string]any{"type": "integer", "description": "quantos leads no máximo (padrão 10)"},
					},
					"required": []string{"funnel_id"},
				},
			},
		)
	}
	if uc.caseMetrics != nil {
		tools = append(tools, domain.AITool{
			Name:        "case_report",
			Description: "Relatório do período: leads gerados, abordagens enviadas, respostas, reuniões, taxa de resposta.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"days": map[string]any{"type": "integer", "description": "janela em dias (padrão 30)"}},
			},
		})
	}
	return tools
}

// defaultCopilotSignalLimit is how many tenders the chat lists when nobody
// said a number: enough to act on, few enough to read on a phone.
const defaultCopilotSignalLimit = 10

// runTool executes one call and returns (result for the model, optional
// UI action). Errors come back as TEXT for the model rather than aborting
// the turn: "I could not, because X" is a better answer than a spinner
// that never resolves.
func (uc *CopilotUseCase) runTool(ctx context.Context, actor domain.Actor, call domain.ToolCall) (string, *CopilotAction) {
	args := map[string]any{}
	if call.Arguments != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			slog.WarnContext(ctx, "copilot: bad tool arguments",
				"tool", call.FunctionName, "error", err)
		}
	}
	str := func(k string) string {
		if v, ok := args[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	num := func(k string, def int) int {
		if v, ok := args[k].(float64); ok && v > 0 {
			return int(v)
		}
		return def
	}

	// Every case re-checks its dependency. A model can name a tool that
	// this deployment does not wire — because it hallucinated one, or
	// because the transcript came from a deployment that did — and a bare
	// switch would nil-deref that into a 500 mid-conversation.
	unavailable := func() (string, *CopilotAction) {
		return "essa ação não está disponível nesta conta", nil
	}

	switch call.FunctionName {
	case "get_pending_actions":
		if uc.commandCenter == nil {
			return unavailable()
		}
		out, err := uc.commandCenter.Execute(ctx, actor)
		if err != nil {
			return "erro ao ler as pendências: " + err.Error(), nil
		}
		if len(out.Actions) == 0 {
			return "nada esperando por você agora", nil
		}
		var b strings.Builder
		for _, a := range out.Actions {
			fmt.Fprintf(&b, "- [%s] %s", a.Kind, a.CustomerName)
			if a.Detail != "" {
				fmt.Fprintf(&b, ": %s", a.Detail)
			}
			if a.MoneyAtStakeCents > 0 {
				fmt.Fprintf(&b, " (%s)", formatCentsForChat(a.MoneyAtStakeCents, a.Currency))
			}
			b.WriteString("\n")
		}
		// The rows are a cut, and saying so is the difference between the
		// model reporting the backlog and reporting what it happened to see.
		if out.TotalActions > len(out.Actions) {
			fmt.Fprintf(&b, "(mostrando %d de %d pendências)\n", len(out.Actions), out.TotalActions)
		}
		return b.String(), &CopilotAction{Label: "Ver pendências", Route: "/"}

	case "list_funnels":
		if uc.funnels == nil {
			return unavailable()
		}
		funnels, err := uc.funnels.List(ctx, actor.AccountID)
		if err != nil {
			return "erro ao listar funis: " + err.Error(), nil
		}
		if len(funnels) == 0 {
			return "a conta não tem nenhum funil ainda", nil
		}
		var b strings.Builder
		for _, f := range funnels {
			fmt.Fprintf(&b, "- %s (id: %s)\n", f.Name, f.ID)
		}
		return b.String(), nil

	case "pipeline_overview":
		if uc.leads == nil {
			return unavailable()
		}
		out, err := uc.leads.Overview(ctx, actor)
		if err != nil {
			return "erro ao ler o funil: " + err.Error(), nil
		}
		unowned := "n/d"
		if out.Unowned != nil {
			unowned = fmt.Sprintf("%d", *out.Unowned)
		}
		summary := fmt.Sprintf("total %d, ativos %d, aguardando aprovação %d, sem dono %s",
			out.Total, out.Active, out.PendingApproval, unowned)
		return summary, pipelineOverviewAction(out.PendingApproval, out.Total)

	case "list_approval_queue":
		if uc.leads == nil {
			return unavailable()
		}
		page, err := uc.leads.ApprovalQueue(ctx, actor, ApprovalQueueOptions{}, 20, 0)
		if err != nil {
			return "erro ao ler a fila: " + err.Error(), nil
		}
		if len(page.Items) == 0 {
			return "a fila de aprovação está vazia", nil
		}
		var b strings.Builder
		for _, l := range page.Items {
			fmt.Fprintf(&b, "- %s: %s\n", l.ContactLabel, truncate(l.ApproachDraft, 120))
		}
		// The 20 read here is a page, and saying so is the difference between
		// the model reporting the backlog and reporting what it happened to
		// see — an operator told "you have 20 drafts" when 340 are waiting has
		// been misinformed by a number that was technically present.
		if page.Total > int64(len(page.Items)) {
			fmt.Fprintf(&b, "(mostrando %d de %d na fila)\n", len(page.Items), page.Total)
		}
		return b.String(), &CopilotAction{Label: "Ver fila", Route: "/prospect?tab=queue"}

	case "list_signals":
		if uc.signals == nil {
			return unavailable()
		}
		list, err := uc.signals.List(ctx, actor.AccountID, num("limit", defaultCopilotSignalLimit), 0)
		if err != nil {
			return "erro ao ler os sinais: " + err.Error(), nil
		}
		if len(list) == 0 {
			return "nenhuma licitação aberta para os interesses da conta agora", nil
		}
		now := nowUTC()
		var b strings.Builder
		for _, sig := range list {
			fmt.Fprintf(&b, "- %s — %s", sig.Organization, truncate(sig.Title, 100))
			if sig.City != "" || sig.State != "" {
				fmt.Fprintf(&b, " (%s)", strings.TrimSpace(strings.TrimPrefix(sig.City+"/"+sig.State, "/")))
			}
			if sig.ValueCents > 0 {
				fmt.Fprintf(&b, ", %s", formatCentsForChat(sig.ValueCents, "BRL"))
			}
			if d := sig.DaysLeft(now); d != nil {
				fmt.Fprintf(&b, ", %d dia(s) de prazo", *d)
			}
			fmt.Fprintf(&b, " (id: %s)\n", sig.ID)
		}
		return b.String(), &CopilotAction{Label: "Ver radar", Route: "/prospect?tab=radar"}

	case "draft_lead_approach":
		if uc.approach == nil {
			return unavailable()
		}
		leadID := str("lead_id")
		if leadID == "" {
			return "faltou lead_id", nil
		}
		lead, err := uc.approach.Draft(ctx, actor.AccountID, leadID)
		if err != nil {
			return "não consegui gerar a oferta: " + err.Error(), nil
		}
		return fmt.Sprintf("oferta rascunhada para o lead %s e colocada na fila de aprovação. Nada foi enviado.\nTexto: %s",
				lead.ID, truncate(lead.ApproachDraft, 400)),
			&CopilotAction{Label: "Ver fila", Route: "/prospect?tab=queue"}

	case "draft_approaches":
		if uc.approach == nil {
			return unavailable()
		}
		funnelID := str("funnel_id")
		if funnelID == "" {
			return "faltou funnel_id", nil
		}
		res, err := uc.approach.DraftBatch(ctx, actor.AccountID, funnelID, num("limit", 10))
		if err != nil {
			return "não consegui gerar as abordagens: " + err.Error(), nil
		}
		return fmt.Sprintf("%d abordagens na fila de aprovação (%d puladas por opt-out, %d por erro). Nada foi enviado.",
				res.Drafted, res.SkippedOptedOut, res.SkippedError),
			&CopilotAction{Label: "Ver fila", Route: "/prospect?tab=queue"}

	case "case_report":
		if uc.caseMetrics == nil {
			return unavailable()
		}
		days := num("days", 30)
		to := nowUTC()
		report, err := uc.caseMetrics.Report(ctx, actor.AccountID, to.AddDate(0, 0, -days), to, 0)
		if err != nil {
			return "erro ao montar o relatório: " + err.Error(), nil
		}
		c := report.Current
		rate := "n/d"
		if c.ReplyRate != nil {
			rate = fmt.Sprintf("%.0f%%", *c.ReplyRate*100)
		}
		return fmt.Sprintf("em %d dias: %d leads, %d abordagens, %d respostas (taxa %s), %d reuniões",
				days, c.LeadsCreated, c.ApproachesSent, c.RepliesReceived, rate, c.MeetingsBooked),
			&CopilotAction{Label: "Ver relatório", Route: "/case-report"}
	}

	return "ferramenta desconhecida: " + call.FunctionName, nil
}

func nowUTC() time.Time { return time.Now().UTC() }

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// formatCentsForChat renders an amount for the model to quote. Plain
// "R$ 12.345,00" for BRL; other currencies keep their code, because the
// account's money is not always reais.
func formatCentsForChat(cents int64, currency string) string {
	whole := cents / 100
	frac := cents % 100
	if frac < 0 {
		frac = -frac
	}
	digits := fmt.Sprintf("%d", whole)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 && r != '-' {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	amount := fmt.Sprintf("%s,%02d", b.String(), frac)
	if currency == "" || strings.EqualFold(currency, "BRL") {
		return "R$ " + amount
	}
	return strings.ToUpper(currency) + " " + amount
}

// pipelineOverviewAction picks where the overview should send the user.
//
// Every other tool hands back a way to act on what it just said. This one
// reported drafts waiting on a human and then left them to find the queue
// themselves — the one step in the loop that cannot happen without them.
//
// Drafts waiting beat everything: that is the only number in the summary
// that is blocked ON the reader. An empty pipeline gets no button at all,
// because there is nothing to go and look at.
func pipelineOverviewAction(pendingApproval, total int64) *CopilotAction {
	switch {
	case pendingApproval > 0:
		return &CopilotAction{Label: "Ver fila", Route: "/prospect?tab=queue"}
	case total > 0:
		return &CopilotAction{Label: "Ver funil", Route: "/funnel"}
	default:
		return nil
	}
}
