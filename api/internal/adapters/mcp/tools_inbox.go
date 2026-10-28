package mcp

import (
	"context"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Inbox and email tools (§6.10 / §6.8).
//
// The parity rule from the spec: every action a person can take on a screen
// has an equivalent tool. It is not decoration — the co-pilot is how an
// operator works when they are on a phone, and a feature that exists only in
// the dashboard is a feature they cannot use where they actually are.

func registerInboxTools(s *mcpsdk.Server, deps Deps) {
	registerListInbox(s, deps)
	registerEmailRiskTools(s, deps)
	registerThreadSummary(s, deps)
}

func registerListInbox(s *mcpsdk.Server, deps Deps) {
	type listInboxInput struct {
		Category string `json:"category,omitempty" jsonschema:"filtro de triagem: hot_lead, customer, supplier, financial, newsletter, quarantine, other (vazio = todas)"`
		Status   string `json:"status,omitempty" jsonschema:"open ou closed (vazio = todas)"`
		Limit    int    `json:"limit,omitempty" jsonschema:"máximo de conversas (padrão 20, máx 50)"`
		Offset   int    `json:"offset,omitempty"`
	}
	type inboxItem struct {
		conversationSummary
		Category string `json:"category,omitempty"`
		// Importance is the sort key, exposed so the caller can see WHY the
		// order is what it is instead of having to trust it.
		Importance int `json:"importance"`
	}
	type listInboxOutput struct {
		Conversations []inboxItem `json:"conversations"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_inbox",
		Description: "Lista a caixa de entrada por categoria de triagem, mais importante primeiro " +
			"(lead que respondeu > financeiro > cliente > resto; quarentena no topo). " +
			"Ordem cronológica pura enterra o que importa numa caixa comercial.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listInboxInput) (*mcpsdk.CallToolResult, *listInboxOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if in.Category != "" && !domain.ValidInboxCategory(in.Category) {
			return nil, nil, toolError(&domain.AppError{
				Code:    "INVALID_FILTER",
				Message: "categoria inválida: use hot_lead, customer, supplier, financial, newsletter, quarantine ou other",
			})
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > maxConversationPageSize {
			limit = maxConversationPageSize
		}
		offset := in.Offset
		if offset < 0 {
			offset = 0
		}

		convs, err := listInboxConversations(ctx, deps, id.AccountID, in.Status,
			int32(limit), int32(offset), in.Category) //nolint:gosec // G115: both bounded above.
		if err != nil {
			return nil, nil, toolError(err)
		}

		out := &listInboxOutput{Conversations: make([]inboxItem, 0, len(convs))}
		for _, c := range convs {
			item := inboxItem{
				conversationSummary: conversationSummary{
					ID:             c.ID,
					ContactName:    c.ContactName,
					ChannelType:    c.ChannelType,
					Status:         string(c.Status),
					BotPaused:      c.IsBotPaused,
					Priority:       c.Priority,
					LastCustomerAt: c.LastCustomerMessageAt,
					UpdatedAt:      c.UpdatedAt,
				},
				Category:   c.InboxCategory,
				Importance: c.InboxImportance,
			}
			if c.AssignedToName != nil {
				item.AssignedToName = *c.AssignedToName
			}
			out.Conversations = append(out.Conversations, item)
		}
		return nil, out, nil
	})
}

// listInboxConversations reaches the category-aware query when the wired
// repository has one.
//
// The same narrow-port dance as the HTTP handler: the filter lives on an
// optional interface rather than on domain.ConversationRepository, which
// dozens of test mocks implement.
func listInboxConversations(
	ctx context.Context, deps Deps,
	accountID, status string, limit, offset int32, category string,
) ([]*domain.ConversationSummary, error) {
	if filtered, ok := deps.ConvRepo.(interface {
		ListEnrichedFiltered(ctx context.Context, accountID, status string, limit, offset int32, sort, assignedTo, channelType, category string) ([]*domain.ConversationSummary, error)
	}); ok {
		return filtered.ListEnrichedFiltered(ctx, accountID, status, limit, offset,
			"importance", "", "", category)
	}
	return deps.ConvRepo.ListEnriched(ctx, accountID, status, limit, offset, "", "", "")
}

func registerEmailRiskTools(s *mcpsdk.Server, deps Deps) {
	if deps.EmailRiskUC == nil {
		// Unregistered rather than answering with an empty verdict: a tool
		// that says "sem risco" because it is unwired is worse than no tool.
		return
	}

	type markLegitimateInput struct {
		MessageID string `json:"message_id"`
	}
	type okOutput struct {
		OK bool `json:"ok"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "mark_legitimate",
		Description: "Marca o remetente de uma mensagem como legítimo e o adiciona à allowlist da conta, " +
			"parando os alertas para ele. Só faça isso com confirmação explícita do usuário.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in markLegitimateInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		actor := actorOf(id)
		msg, err := inboundMessage(ctx, deps, actor, in.MessageID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		if err := deps.EmailRiskUC.MarkLegitimate(ctx, actor, msg.ID, msg.From); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true}, nil
	})

	type reportScamInput struct {
		MessageID string `json:"message_id"`
		Reason    string `json:"reason,omitempty" jsonschema:"por que o usuário acha que é golpe (vira exemplo para o classificador)"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "report_scam",
		Description: "Reporta uma mensagem como golpe. A assinatura (domínio, conta bancária, padrão) " +
			"entra na base coletiva: 2+ contas reportando o mesmo domínio o colocam em quarentena " +
			"na plataforma inteira.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in reportScamInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		actor := actorOf(id)
		msg, err := inboundMessage(ctx, deps, actor, in.MessageID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		var body string
		if msg.Content != nil {
			body = *msg.Content
		}
		if _, err := deps.EmailRiskUC.ReportScam(ctx, actor, msg.ID, msg.From, body, in.Reason); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true}, nil
	})
}

func registerThreadSummary(s *mcpsdk.Server, deps Deps) {
	if deps.BriefUC == nil {
		return
	}

	type summaryInput struct {
		ConversationID string `json:"conversation_id"`
	}
	type summaryOutput struct {
		Summary string `json:"summary"`
		// GeneratedAt lets the caller see how fresh the summary is. A brief
		// is regenerated when the thread moves, so an old timestamp means a
		// quiet conversation, not a stale answer.
		GeneratedAt time.Time `json:"generated_at,omitempty"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_thread_summary",
		Description: "Resume uma conversa longa em poucas linhas — o estado da negociação e o próximo " +
			"passo — para quem está pegando a conversa agora.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in summaryInput) (*mcpsdk.CallToolResult, *summaryOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		brief, err := deps.BriefUC.Get(ctx, actorOf(id), in.ConversationID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		if brief == nil {
			// A thread too short to be worth summarising. An empty answer is
			// honest; inventing one from four messages is not.
			return nil, &summaryOutput{}, nil
		}
		return nil, &summaryOutput{Summary: brief.Text, GeneratedAt: brief.GeneratedAt}, nil
	})
}

// actorOf adapts the MCP identity to the domain actor the use cases take.
func actorOf(id Identity) domain.Actor {
	return domain.Actor{UserID: id.UserID, AccountID: id.AccountID, Role: domain.Role(id.Role)}
}

// inboundMessage resolves a message and refuses anything that is not an
// inbound row.
//
// `From` carries the contact's own address on an inbound row — the same key
// the risk checks use. Vouching for an OUTBOUND row would allowlist
// ourselves, which is both useless and quietly wrong.
func inboundMessage(ctx context.Context, deps Deps, actor domain.Actor, messageID string) (*domain.Message, error) {
	if deps.MsgRepo == nil {
		return nil, &domain.AppError{Code: "EMAIL_RISK_UNAVAILABLE", Message: "risk checks are not configured"}
	}
	msg, err := deps.MsgRepo.GetByID(ctx, messageID, actor.AccountID)
	if err != nil {
		return nil, err
	}
	if msg == nil || msg.From == "" || msg.Direction != "inbound" {
		return nil, domain.ErrMessageNotFound
	}
	return msg, nil
}
