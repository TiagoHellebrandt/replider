package mcp

import (
	"context"
	"strconv"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Contracts (§6.11, G4).
//
// Admin-only: a contract carries the price, the penalties and the exclusivity a
// competitor would pay for. The HTTP layer redacts those for the other roles,
// and the co-pilot's answer would be a way around that redaction — so the tools
// simply do not exist for anybody else.
func registerContractTools(s *mcpsdk.Server, deps Deps) {
	if deps.ContractUC == nil {
		return
	}

	type contractSummaryOut struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		// Version is 1 for the original and 2+ for each aditivo.
		Version    int    `json:"version"`
		Amendments int    `json:"amendments"`
		Subject    string `json:"subject,omitempty"`

		Parties []string `json:"parties,omitempty"`

		ValueCents int64  `json:"value_cents"`
		StartsOn   string `json:"starts_on,omitempty"`
		EndsOn     string `json:"ends_on,omitempty"`
		// DaysUntilEnd is NEGATIVE once the contract has expired, which is the
		// state that costs money and must not read as zero.
		DaysUntilEnd *int `json:"days_until_end,omitempty"`

		AutoRenewal bool `json:"auto_renewal"`
		NoticeDays  int  `json:"notice_days"`
		// NoticeDeadline is the last day to say "do not renew". Absent when the
		// contract does not renew itself or states no window — never invented.
		NoticeDeadline string `json:"notice_deadline,omitempty"`

		AdjustmentIndex string `json:"adjustment_index,omitempty"`
		AdjustmentMonth int    `json:"adjustment_month,omitempty"`

		SLA         string   `json:"sla,omitempty"`
		Penalties   []string `json:"penalties,omitempty"`
		Exclusivity bool     `json:"exclusivity"`

		// ExtractionStatus and Confirmed are the honesty fields: an unconfirmed
		// reading is a PROPOSAL, and the co-pilot must say so before anybody
		// acts on a renewal date a model guessed.
		ExtractionStatus string `json:"extraction_status"`
		Confirmed        bool   `json:"confirmed"`
	}

	summarize := func(c *domain.Contract) *contractSummaryOut {
		out := &contractSummaryOut{
			ID: c.ID, Title: c.Title, Version: c.Version,
			Amendments: len(c.Amendments), Subject: c.Subject,
			ValueCents: c.ValueCents, AutoRenewal: c.AutoRenewal,
			NoticeDays: c.NoticeDays, AdjustmentIndex: c.AdjustmentIndex,
			SLA: c.SLAText, Exclusivity: c.Exclusivity,
			ExtractionStatus: string(c.ExtractionStatus), Confirmed: c.Confirmed(),
		}
		for _, p := range c.Parties {
			label := p.Name
			if p.Role != "" {
				label += " (" + p.Role + ")"
			}
			out.Parties = append(out.Parties, label)
		}
		for _, p := range c.Penalties {
			out.Penalties = append(out.Penalties, penaltyLine(p))
		}
		if c.StartsOn != nil {
			out.StartsOn = c.StartsOn.Format("2006-01-02")
		}
		if c.EndsOn != nil {
			out.EndsOn = c.EndsOn.Format("2006-01-02")
		}
		out.DaysUntilEnd = c.DaysUntilEnd(time.Now())
		if deadline := c.NoticeDeadline(); deadline != nil {
			out.NoticeDeadline = deadline.Format("2006-01-02")
		}
		if c.AdjustmentMonth != nil {
			out.AdjustmentMonth = *c.AdjustmentMonth
		}
		return out
	}

	type listOut struct {
		Contracts []contractSummaryOut `json:"contracts"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_contract_summary",
		Description: "Resumo dos contratos da conta, ou de um contrato específico por id: partes, " +
			"vigência, renovação automática com o prazo de aviso, índice de reajuste, SLA, " +
			"multas e exclusividade. SEMPRE diga se a leitura ainda NÃO foi confirmada por " +
			"uma pessoa (confirmed=false) antes de afirmar uma data — nesse caso é proposta " +
			"da IA, não fato.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		ContractID string `json:"contract_id,omitempty" jsonschema:"id do contrato; vazio lista os contratos"`
		Limit      int    `json:"limit,omitempty" jsonschema:"máximo de contratos (padrão 20)"`
	}) (*mcpsdk.CallToolResult, *listOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}

		if strings.TrimSpace(in.ContractID) != "" {
			contract, err := deps.ContractUC.Get(ctx, id.AccountID, in.ContractID)
			if err != nil {
				return nil, nil, toolError(err)
			}
			out := &listOut{Contracts: []contractSummaryOut{*summarize(contract)}}
			for _, amendment := range contract.Amendments {
				out.Contracts = append(out.Contracts, *summarize(amendment))
			}
			return nil, out, nil
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > maxConversationPageSize {
			limit = maxConversationPageSize
		}
		list, err := deps.ContractUC.List(ctx, id.AccountID, limit, 0)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listOut{Contracts: make([]contractSummaryOut, 0, len(list))}
		for _, contract := range list {
			out.Contracts = append(out.Contracts, *summarize(contract))
		}
		return nil, out, nil
	})

	type uploadOut struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		// ExtractionStatus is "pending" when the reading was queued. The fields
		// are NOT in this answer on purpose: they do not exist yet, and a
		// co-pilot that reported them would be reporting the request, not the
		// contract.
		ExtractionStatus string `json:"extraction_status"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upload_contract",
		Description: "Arquiva um contrato e manda a IA ler. Passe `text` com o teor colado, ou " +
			"`object_path` de um PDF já enviado pelo app. A leitura roda em segundo plano: " +
			"consulte depois com get_contract_summary, e diga ao usuário que os campos são " +
			"PROPOSTA até alguém confirmar. Para um aditivo, informe parent_contract_id.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		Title string `json:"title" jsonschema:"como o time chama este contrato"`
		// Text is how a co-pilot supplies a document: MCP is a text protocol and
		// cannot carry bytes, so the alternative is a path the app already
		// uploaded to.
		Text             string `json:"text,omitempty" jsonschema:"teor do contrato colado (use isto OU object_path)"`
		ObjectPath       string `json:"object_path,omitempty" jsonschema:"caminho do PDF já enviado via /contracts/upload-url"`
		ContactID        string `json:"contact_id,omitempty" jsonschema:"cliente a que o contrato se refere"`
		ParentContractID string `json:"parent_contract_id,omitempty" jsonschema:"id do contrato-pai, se este for um aditivo"`
	}) (*mcpsdk.CallToolResult, *uploadOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if strings.TrimSpace(in.Text) == "" && strings.TrimSpace(in.ObjectPath) == "" {
			return nil, nil, &domain.AppError{
				Code:    "CONTRACT_NO_DOCUMENT",
				Message: "informe o teor do contrato em `text` ou o `object_path` do PDF enviado",
			}
		}

		contract, err := deps.ContractUC.Create(ctx, id.AccountID, id.UserID,
			usecases.ContractInput{
				Title:            in.Title,
				Text:             in.Text,
				ObjectPath:       in.ObjectPath,
				ContactID:        in.ContactID,
				ParentContractID: in.ParentContractID,
			})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &uploadOut{
			ID: contract.ID, Title: contract.Title,
			ExtractionStatus: string(contract.ExtractionStatus),
		}, nil
	})
}

// penaltyLine writes one multa the way somebody says it out loud.
func penaltyLine(p domain.ContractPenalty) string {
	var b strings.Builder
	if p.Description != "" {
		b.WriteString(p.Description)
	}
	switch {
	case p.Percent > 0:
		if b.Len() > 0 {
			b.WriteString(" — ")
		}
		b.WriteString(trimFloat(p.Percent) + "%")
	case p.AmountCents > 0:
		if b.Len() > 0 {
			b.WriteString(" — ")
		}
		b.WriteString(brlFromCents(p.AmountCents))
	}
	if p.Trigger != "" {
		b.WriteString(" (" + p.Trigger + ")")
	}
	return b.String()
}

// trimFloat drops a trailing ".0" so a flat 10% does not read as "10.0%".
func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}

// brlFromCents writes a fixed penalty the way it is read aloud.
func brlFromCents(cents int64) string {
	return "R$ " + strconv.FormatInt(cents/100, 10) + "," +
		leftPad2(cents%100)
}

func leftPad2(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// The clause deadlines (§6.11, G5/G6).
//
// set_clause_alert is the co-pilot half of the 1-click "agendar": somebody reading
// a contract out loud with the assistant should be able to say "me lembra disso" and
// have a dated task exist. list_expiring_contracts is the 8am question — what is
// about to renew itself, and what is it worth.
func registerContractAlertTools(s *mcpsdk.Server, deps Deps) {
	if deps.ContractAlertUC == nil {
		return
	}

	type flagOut struct {
		Kind     string `json:"kind"`
		Severity string `json:"severity"`
		Title    string `json:"title"`
		DueOn    string `json:"due_on,omitempty"`
		// MoneyAtStakeCents is what acting on it is worth. Zero for a constraint
		// (exclusivity) rather than an amount.
		MoneyAtStakeCents int64 `json:"money_at_stake_cents"`
		// Scheduled says a task already exists, so the co-pilot offers to open it
		// instead of creating a second one.
		Scheduled bool `json:"scheduled"`
	}
	type attentionOut struct {
		// Summary is the contract in plain sentences, composed from the confirmed
		// fields — never a second model reading, so it cannot disagree with them.
		Summary []string `json:"summary"`
		// Confirmed is the honesty gate: false means NO flags were derived, and the
		// answer must say the reading is unconfirmed rather than "no red flags",
		// which reads as a clean contract.
		Confirmed         bool      `json:"confirmed"`
		Flags             []flagOut `json:"flags"`
		TotalAtStakeCents int64     `json:"total_at_stake_cents"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_contract_red_flags",
		Description: "Resumo em linguagem simples e as cláusulas de atenção de um contrato: renovação " +
			"automática com o prazo de aviso, reajuste com o acumulado do índice, vencimento, " +
			"saldo de ata, multas e exclusividade. Se confirmed=false, a leitura ainda NÃO foi " +
			"confirmada por uma pessoa e a lista vem vazia — diga isso, não diga que o contrato " +
			"está limpo.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		ContractID string `json:"contract_id" jsonschema:"id do contrato"`
	}) (*mcpsdk.CallToolResult, *attentionOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		attention, err := deps.ContractAlertUC.Attention(ctx, id.AccountID, in.ContractID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &attentionOut{
			Summary: attention.Summary, Confirmed: attention.Confirmed,
			TotalAtStakeCents: attention.TotalAtStakeCents,
			Flags:             make([]flagOut, 0, len(attention.Flags)),
		}
		for _, f := range attention.Flags {
			item := flagOut{
				Kind: string(f.Kind), Severity: string(f.Severity), Title: f.Title,
				MoneyAtStakeCents: f.MoneyAtStakeCents, Scheduled: f.Scheduled,
			}
			if f.DueOn != nil {
				item.DueOn = f.DueOn.Format("2006-01-02")
			}
			out.Flags = append(out.Flags, item)
		}
		return nil, out, nil
	})

	type alertOut struct {
		ID    string `json:"id"`
		Kind  string `json:"kind"`
		Title string `json:"title"`
		DueOn string `json:"due_on"`
		// DaysUntilDue is NEGATIVE once the deadline passed — the state that costs
		// money, which must not read as zero.
		DaysUntilDue      int    `json:"days_until_due"`
		MoneyAtStakeCents int64  `json:"money_at_stake_cents"`
		ContractID        string `json:"contract_id"`
		ContractTitle     string `json:"contract_title,omitempty"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "set_clause_alert",
		Description: "Agenda um alerta para uma cláusula do contrato. Use o `kind` que veio de " +
			"get_contract_red_flags (renewal_notice, adjustment, expiry, registry_balance) — a " +
			"data e o texto vêm da própria cláusula, não do que você escrever. Para um lembrete " +
			"que não está no contrato, use kind=custom com due_on e title.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		ContractID string `json:"contract_id" jsonschema:"id do contrato"`
		Kind       string `json:"kind" jsonschema:"renewal_notice, adjustment, expiry, registry_balance ou custom"`
		DueOn      string `json:"due_on,omitempty" jsonschema:"só para custom: data AAAA-MM-DD"`
		Title      string `json:"title,omitempty" jsonschema:"só para custom: o que precisa ser feito"`
	}) (*mcpsdk.CallToolResult, *alertOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		var due *time.Time
		if raw := strings.TrimSpace(in.DueOn); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return nil, nil, &domain.AppError{
					Code: "CONTRACT_ALERT_INVALID", Message: "data inválida, use AAAA-MM-DD"}
			}
			due = &parsed
		}

		alert, err := deps.ContractAlertUC.Schedule(ctx, id.AccountID, id.UserID,
			usecases.ScheduleInput{
				ContractID: in.ContractID,
				Kind:       domain.ContractAlertKind(strings.TrimSpace(in.Kind)),
				DueOn:      due, Title: in.Title,
			})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &alertOut{
			ID: alert.ID, Kind: string(alert.Kind), Title: alert.Title,
			DueOn:             alert.DueOn.Format("2006-01-02"),
			DaysUntilDue:      alert.DaysUntilDue(time.Now()),
			MoneyAtStakeCents: alert.MoneyAtStakeCents,
			ContractID:        alert.ContractID, ContractTitle: alert.ContractTitle,
		}, nil
	})

	type expiringOut struct {
		Alerts []alertOut `json:"alerts"`
		// TotalAtStakeCents is the whole list's worth — the number that decides
		// whether the user acts today.
		TotalAtStakeCents int64 `json:"total_at_stake_cents"`
		Overdue           int   `json:"overdue"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_expiring_contracts",
		Description: "O que vence, renova sozinho ou está com reajuste pendente, mais próximo primeiro, " +
			"com o valor em jogo. Comece pelos atrasados (days_until_due negativo): esses já " +
			"estão custando dinheiro.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		HorizonDays int `json:"horizon_days,omitempty" jsonschema:"quantos dias à frente olhar (padrão 90)"`
		Limit       int `json:"limit,omitempty" jsonschema:"máximo de alertas (padrão 20)"`
	}) (*mcpsdk.CallToolResult, *expiringOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > maxConversationPageSize {
			limit = maxConversationPageSize
		}

		alerts, err := deps.ContractAlertUC.Due(ctx, id.AccountID, in.HorizonDays, limit)
		if err != nil {
			return nil, nil, toolError(err)
		}
		now := time.Now()
		out := &expiringOut{Alerts: make([]alertOut, 0, len(alerts))}
		for _, a := range alerts {
			if a.Overdue(now) {
				out.Overdue++
			}
			out.TotalAtStakeCents += a.MoneyAtStakeCents
			out.Alerts = append(out.Alerts, alertOut{
				ID: a.ID, Kind: string(a.Kind), Title: a.Title,
				DueOn: a.DueOn.Format("2006-01-02"), DaysUntilDue: a.DaysUntilDue(now),
				MoneyAtStakeCents: a.MoneyAtStakeCents,
				ContractID:        a.ContractID, ContractTitle: a.ContractTitle,
			})
		}
		return nil, out, nil
	})
}
