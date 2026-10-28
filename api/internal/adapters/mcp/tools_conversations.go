package mcp

import (
	"context"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

const maxConversationPageSize = 50

type conversationSummary struct {
	ID             string     `json:"id"`
	ContactName    string     `json:"contact_name"`
	ChannelType    string     `json:"channel_type"`
	Status         string     `json:"status"`
	BotPaused      bool       `json:"bot_paused"`
	AssignedToName string     `json:"assigned_to_name,omitempty"`
	Priority       string     `json:"priority,omitempty"`
	LastCustomerAt *time.Time `json:"last_customer_message_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type messageSummary struct {
	ID        string    `json:"id"`
	Direction string    `json:"direction"` // inbound (cliente) | outbound (bot/atendente)
	Sender    string    `json:"sender"`    // "bot", ULID de atendente, ou id do cliente
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func registerConversationTools(s *mcpsdk.Server, deps Deps) {
	type listConversationsInput struct {
		Status string `json:"status,omitempty" jsonschema:"filtro: open ou closed (vazio = todas)"`
		Limit  int    `json:"limit,omitempty" jsonschema:"máximo de conversas (padrão 20, máx 50)"`
		Offset int    `json:"offset,omitempty"`
	}
	type listConversationsOutput struct {
		Conversations []conversationSummary `json:"conversations"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_conversations",
		Description: "Lista conversas com clientes, mais recentes primeiro, com status, canal e atribuição.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listConversationsInput) (*mcpsdk.CallToolResult, *listConversationsOutput, error) {
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
		offset := in.Offset
		if offset < 0 {
			offset = 0
		}
		convs, err := deps.ConvRepo.ListEnriched(ctx, id.AccountID, in.Status, int32(limit), int32(offset), "", "", "")
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listConversationsOutput{Conversations: make([]conversationSummary, 0, len(convs))}
		for _, c := range convs {
			summary := conversationSummary{
				ID:             c.ID,
				ContactName:    c.ContactName,
				ChannelType:    c.ChannelType,
				Status:         string(c.Status),
				BotPaused:      c.IsBotPaused,
				Priority:       c.Priority,
				LastCustomerAt: c.LastCustomerMessageAt,
				UpdatedAt:      c.UpdatedAt,
			}
			if c.AssignedToName != nil {
				summary.AssignedToName = *c.AssignedToName
			}
			out.Conversations = append(out.Conversations, summary)
		}
		return nil, out, nil
	})

	type getConversationInput struct {
		ConversationID string `json:"conversation_id"`
		Limit          int    `json:"limit,omitempty" jsonschema:"máximo de mensagens (padrão 30, máx 100)"`
	}
	type getConversationOutput struct {
		ContactName string           `json:"contact_name"`
		Status      string           `json:"status"`
		BotPaused   bool             `json:"bot_paused"`
		Messages    []messageSummary `json:"messages"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_conversation",
		Description: "Lê as mensagens de uma conversa (mais recentes primeiro).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in getConversationInput) (*mcpsdk.CallToolResult, *getConversationOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		conv, err := deps.ConvRepo.GetByID(ctx, in.ConversationID, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		if conv == nil {
			return nil, nil, toolError(domain.ErrConversationNotFound)
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 30
		}
		if limit > 100 {
			limit = 100
		}
		msgs, err := deps.MsgRepo.ListByConversationID(ctx, conv.ID, int32(limit), 0)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &getConversationOutput{
			Status:    string(conv.Status),
			BotPaused: conv.IsBotPaused,
			Messages:  make([]messageSummary, 0, len(msgs)),
		}
		if conv.ContactName != nil {
			out.ContactName = *conv.ContactName
		}
		for _, m := range msgs {
			summary := messageSummary{
				ID:        m.ID,
				Direction: m.Direction,
				Sender:    m.From,
				Type:      m.Type,
				CreatedAt: m.Timestamp,
			}
			if m.Content != nil {
				summary.Content = *m.Content
			}
			out.Messages = append(out.Messages, summary)
		}
		return nil, out, nil
	})

	type conversationIDInput struct {
		ConversationID string `json:"conversation_id"`
	}
	type okOutput struct {
		OK      bool   `json:"ok"`
		Message string `json:"message,omitempty"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "assume_conversation",
		Description: "Assume a conversa para atendimento humano: pausa o bot e atribui a conversa ao usuário " +
			"conectado. Necessário antes de send_message_to_customer.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in conversationIDInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if _, err := deps.TakeoverUC.Assume(ctx, in.ConversationID, id.AccountID, id.UserID); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true, Message: "Conversa assumida — o bot está pausado nesta conversa."}, nil
	})

	type sendMessageInput struct {
		ConversationID string `json:"conversation_id"`
		Message        string `json:"message" jsonschema:"texto exato a enviar ao cliente final"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "send_message_to_customer",
		Description: "Envia uma mensagem a um CLIENTE FINAL real (WhatsApp/Telegram/Instagram) em nome do " +
			"usuário conectado. OBRIGATÓRIO: mostre ao usuário o texto exato e a conversa de destino e " +
			"receba confirmação explícita ANTES de chamar. A conversa precisa estar assumida " +
			"(assume_conversation).",
		Annotations: &mcpsdk.ToolAnnotations{OpenWorldHint: boolPtr(true)},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in sendMessageInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		msg, err := deps.TakeoverUC.SendHumanMessage(ctx, usecases.SendHumanMessageDTO{
			ConversationID: in.ConversationID,
			AccountID:      id.AccountID,
			UserID:         id.UserID,
			UserName:       id.UserName,
			Content:        in.Message,
			Type:           "text",
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true, Message: "Mensagem enviada (id " + msg.ID + ")."}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "release_conversation",
		Description: "Devolve a conversa ao bot (retoma o atendimento automático).",
		Annotations: &mcpsdk.ToolAnnotations{IdempotentHint: true},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in conversationIDInput) (*mcpsdk.CallToolResult, *okOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if _, err := deps.TakeoverUC.Release(ctx, in.ConversationID, id.AccountID); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &okOutput{OK: true, Message: "Bot reativado nesta conversa."}, nil
	})
}
