package mcp

import (
	"context"
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// channelSummary is a lean projection of domain.Channel — the raw
// Config carries credentials and is deliberately not exposed over MCP.
type channelSummary struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Active  bool   `json:"active"`
	AgentID string `json:"agent_id,omitempty"`
}

func toChannelSummary(ch *domain.Channel) channelSummary {
	out := channelSummary{
		ID:     ch.ID,
		Type:   ch.Type,
		Name:   ch.Name,
		Active: ch.IsActive,
	}
	if ch.AgentID != nil {
		out.AgentID = *ch.AgentID
	}
	return out
}

type listChannelsOutput struct {
	Channels []channelSummary `json:"channels"`
}

func registerChannelTools(s *mcpsdk.Server, deps Deps, admin bool) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_channels",
		Description: "Lista os canais de atendimento conectados (WhatsApp, Telegram, Instagram, webchat) e qual agente responde em cada um.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, *listChannelsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		channels, err := deps.ListChannelsUC.Execute(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listChannelsOutput{Channels: make([]channelSummary, 0, len(channels))}
		for _, ch := range channels {
			out.Channels = append(out.Channels, toChannelSummary(ch))
		}
		return nil, out, nil
	})

	if !admin {
		return
	}

	// Telegram bots answered customers in v1; the SDR product sends e-mail.
	// Kept behind Deps.LegacyTools.
	if deps.LegacyTools {
		type createTelegramInput struct {
			Name     string `json:"name" jsonschema:"nome do canal, ex.: Suporte Telegram"`
			BotToken string `json:"bot_token" jsonschema:"token do bot obtido no @BotFather"`
			AgentID  string `json:"agent_id,omitempty" jsonschema:"agente que responde neste canal (opcional)"`
		}
		mcpsdk.AddTool(s, &mcpsdk.Tool{
			Name: "create_telegram_channel",
			Description: "Conecta um bot do Telegram como canal de atendimento. O usuário cria o bot no " +
				"@BotFather e fornece o token; o webhook é registrado automaticamente. Para WhatsApp e " +
				"Instagram, oriente o usuário a conectar pelo dashboard (fluxo com a Meta).",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in createTelegramInput) (*mcpsdk.CallToolResult, *channelSummary, error) {
			id, ok := IdentityFrom(ctx)
			if !ok {
				return nil, nil, errNoIdentity
			}
			cfg, _ := json.Marshal(map[string]string{"bot_token": in.BotToken})
			dto := usecases.CreateChannelDTO{
				AccountID:  id.AccountID,
				Type:       "telegram",
				Name:       in.Name,
				Config:     cfg,
				AppBaseURL: deps.AppBaseURL,
			}
			if in.AgentID != "" {
				dto.AgentID = &in.AgentID
			}
			channel, err := deps.CreateChannelUC.Execute(ctx, dto)
			if err != nil {
				return nil, nil, toolError(err)
			}
			out := toChannelSummary(channel)
			return nil, &out, nil
		})
	}

	type updateChannelInput struct {
		ChannelID string  `json:"channel_id"`
		Enabled   *bool   `json:"enabled,omitempty" jsonschema:"ativar/desativar o canal (opcional)"`
		AgentID   *string `json:"agent_id,omitempty" jsonschema:"agente responsável; string vazia remove o agente (opcional)"`
		Name      *string `json:"name,omitempty" jsonschema:"novo nome (opcional)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_channel",
		Description: "Atualiza um canal: ativa/desativa, troca o agente responsável ou renomeia. Campos omitidos não mudam.",
		Annotations: &mcpsdk.ToolAnnotations{IdempotentHint: true},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in updateChannelInput) (*mcpsdk.CallToolResult, *channelSummary, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		var channel *domain.Channel
		var err error
		if in.AgentID != nil || in.Name != nil {
			// UpdateChannelDTO semantics: nil AgentID = keep; non-nil
			// empty string = unassign; non-nil value = assign.
			dto := usecases.UpdateChannelDTO{
				AccountID: id.AccountID,
				ChannelID: in.ChannelID,
				Name:      in.Name,
				AgentID:   in.AgentID,
			}
			channel, err = deps.UpdateChannelUC.Execute(ctx, dto)
			if err != nil {
				return nil, nil, toolError(err)
			}
		}
		if in.Enabled != nil {
			// ToggleChannelUseCase flips the flag; only call when the
			// current state differs from the desired one.
			channels, listErr := deps.ListChannelsUC.Execute(ctx, id.AccountID)
			if listErr != nil {
				return nil, nil, toolError(listErr)
			}
			for _, ch := range channels {
				if ch.ID == in.ChannelID && ch.IsActive != *in.Enabled {
					channel, err = deps.ToggleChannelUC.Execute(ctx, id.AccountID, in.ChannelID)
					if err != nil {
						return nil, nil, toolError(err)
					}
				}
			}
		}
		if channel == nil {
			channels, listErr := deps.ListChannelsUC.Execute(ctx, id.AccountID)
			if listErr != nil {
				return nil, nil, toolError(listErr)
			}
			for _, ch := range channels {
				if ch.ID == in.ChannelID {
					channel = ch
				}
			}
			if channel == nil {
				return nil, nil, toolError(domain.ErrChannelNotFound)
			}
		}
		out := toChannelSummary(channel)
		return nil, &out, nil
	})
}
