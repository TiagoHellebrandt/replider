package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// LearnPersonalityUseCase analyzes human agent messages to generate a personality prompt.
// This makes the AI respond like the human agent, not like a generic bot.
type LearnPersonalityUseCase struct {
	agentRepo   domain.AgentRepository
	convRepo    domain.ConversationRepository
	msgRepo     domain.MessageRepository
	channelRepo domain.ChannelRepository
	aiFactory   domain.AIFactory
	// accountRepo/planRepo resolve the agent's account so the analysis runs
	// on the ACCOUNT's key (BYOK), not the platform's. Optional; without them
	// the call falls through to the factory's nil-account path.
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	// meter lands the analysis in the cost ledger (operation
	// "learn_personality"). Optional.
	meter *AIMeter
	spend *spendGate
}

// SetAccountResolver wires the BYOK key resolution (account + plan).
func (uc *LearnPersonalityUseCase) SetAccountResolver(accounts domain.AccountRepository, plans domain.PlanRepository) {
	uc.accountRepo, uc.planRepo = accounts, plans
}

// SetAIMeter wires transparent token metering + the budget gate.
func (uc *LearnPersonalityUseCase) SetAIMeter(m *AIMeter) {
	uc.meter = m
	uc.spend = budgetGateFromMeter(uc.accountRepo, m)
}

func NewLearnPersonalityUseCase(
	agentRepo domain.AgentRepository,
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	channelRepo domain.ChannelRepository,
	aiFactory domain.AIFactory,
) *LearnPersonalityUseCase {
	return &LearnPersonalityUseCase{
		agentRepo:   agentRepo,
		convRepo:    convRepo,
		msgRepo:     msgRepo,
		channelRepo: channelRepo,
		aiFactory:   aiFactory,
	}
}

// Execute collects human messages from all conversations of the agent,
// sends them to the AI to generate a personality profile, and saves it.
func (uc *LearnPersonalityUseCase) Execute(ctx context.Context, agentID, accountID string) (*domain.Agent, error) {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return nil, domain.ErrAgentNotFound
	}
	if agent.AccountID != accountID {
		return nil, domain.ErrUnauthorized
	}

	// Collect human outbound messages from conversations linked to this agent's channels
	channels, err := uc.channelRepo.ListByAgentID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}

	var humanMessages []string

	for _, ch := range channels {
		conversations, err := uc.convRepo.ListByAccountID(ctx, accountID, "")
		if err != nil {
			continue
		}

		for _, conv := range conversations {
			if conv.ChannelID != ch.ID {
				continue
			}
			messages, err := uc.msgRepo.ListByConversationID(ctx, conv.ID, 100, 0)
			if err != nil {
				continue
			}
			for _, msg := range messages {
				// Only collect human outbound messages (not bot messages)
				if msg.Direction == "outbound" && msg.From != "bot" && msg.Content != nil && *msg.Content != "" {
					humanMessages = append(humanMessages, *msg.Content)
				}
			}

			// Cap at 200 messages to avoid token explosion
			if len(humanMessages) >= 200 {
				break
			}
		}
		if len(humanMessages) >= 200 {
			break
		}
	}

	if len(humanMessages) < 5 {
		return nil, &domain.AppError{
			Code:    "INSUFFICIENT_DATA",
			Message: "Pelo menos 5 mensagens humanas são necessárias para aprender o estilo. Responda algumas conversas manualmente primeiro.",
		}
	}

	// Cap and join messages
	if len(humanMessages) > 200 {
		humanMessages = humanMessages[:200]
	}

	sampleText := strings.Join(humanMessages, "\n---\n")

	// Use AI to analyze and generate personality profile
	// Account-scoped provider: BYOK key when the account has one, a clear
	// BYOK_KEY_MISSING when it does not (never the platform key by default).
	var account *domain.Account
	plan := domain.TrialPlan()
	if uc.accountRepo != nil {
		if a, aErr := uc.accountRepo.GetByID(ctx, agent.AccountID); aErr == nil && a != nil {
			account = a
			if a.PlanID != nil && *a.PlanID != "" && uc.planRepo != nil {
				if p, pErr := uc.planRepo.GetByID(ctx, *a.PlanID); pErr == nil && p != nil {
					plan = p
				}
			}
		}
	}
	if account != nil {
		if err := uc.spend.ensureAnyFor(ctx, account); err != nil {
			return nil, err
		}
	}
	aiPrv, err := uc.aiFactory.GetProviderForAccount(ctx, agent.Provider, account, plan)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		return nil, domain.ErrBYOKKeyMissing
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get AI provider: %w", err)
	}

	analysisPrompt := `Analise as mensagens abaixo, que são de um atendente humano real respondendo clientes.
Gere um guia de personalidade e estilo de comunicação para uma IA imitar EXATAMENTE como essa pessoa escreve.

Inclua:
1. Tom de voz (formal/informal/amigável/direto)
2. Nível de formalidade (tu/você/senhor)
3. Comprimento típico das respostas (curtas/médias/longas)
4. Expressões e palavras frequentes que a pessoa usa
5. Como a pessoa cumprimenta e se despede
6. Se usa emojis, gírias, abreviações
7. Como lida com reclamações ou problemas
8. Padrões de pontuação e formatação

Responda APENAS com o guia de personalidade, sem explicações extras. O texto será usado como instrução direta para a IA.

MENSAGENS DO ATENDENTE:
` + sampleText

	history := []domain.ChatMessage{
		{Role: "user", Content: analysisPrompt},
	}

	resp, err := aiPrv.GenerateResponse(ctx, "Você é um especialista em análise de comunicação. Gere guias de personalidade precisos.", history, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze personality: %w", err)
	}
	uc.meter.Record(ctx, agent.AccountID, "learn_personality", plan.AIProvider, plan.AIModel, resp.Usage)

	personality := strings.TrimSpace(resp.Content)
	if personality == "" {
		return nil, &domain.AppError{Code: "ANALYSIS_FAILED", Message: "Não foi possível gerar o perfil de personalidade"}
	}

	slog.Info("personality learned", "agent_id", agentID, "human_messages_analyzed", len(humanMessages), "personality_length", len(personality))

	// Save personality prompt on agent
	agent.PersonalityPrompt = &personality
	agent.LearnFromHumans = true
	if err := uc.agentRepo.Update(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to save personality: %w", err)
	}

	return agent, nil
}
