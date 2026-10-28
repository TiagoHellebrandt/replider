package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ──────────────────────────── EnsureProductKnowledgeUseCase ────────────────────────────
//
// Canal interno de CS (US-010): the platform agent that answers customer
// support threads must know the Replider PRODUCT — onboarding, channels,
// billing, plans — not just its generic persona. This use case seeds a
// knowledge source with an honest product FAQ onto the agent assigned to
// the platform's internal channel, at boot, idempotently.
//
// Idempotency is per-CHUNK by content: the source is found by name and
// only FAQ entries whose exact content isn't already ingested get
// embedded + created. That makes the seed self-healing (a boot that died
// mid-seed resumes where it stopped) and additive (editing/adding an FAQ
// entry ships on the next deploy; superseded chunks are pruned manually
// via the normal knowledge UI, which fully manages this source).

// ProductFAQSourceName identifies the seeded source. Renaming it would
// re-seed a duplicate source on the next boot — don't.
const ProductFAQSourceName = "Replider — FAQ do Produto"

// productFAQChunks is the honest product FAQ, sourced from the repo's
// own docs (onboarding flow, channel set, trial limits, RAG, takeover,
// calendars). Deliberately NO specific plan prices — they change by
// currency/tier and would rot here; the agent points at the dashboard's
// billing page instead.
var productFAQChunks = []string{
	"O que é o Replider: uma plataforma SaaS de atendimento omnichannel com IA. Você cria um agente de IA, conecta canais de mensagem (WhatsApp, Telegram, Instagram, SMS) e o agente responde os clientes automaticamente usando uma base de conhecimento própria (RAG). Atendentes humanos monitoram tudo em um dashboard em tempo real e podem assumir qualquer conversa a qualquer momento.",

	"Onboarding / primeiros passos: o cadastro cria sua conta com um período de teste grátis. O fluxo guiado tem 3 passos: (1) dados da empresa, (2) criar o agente de IA — o assistente de prompt monta a persona a partir de perguntas simples, ou use um template no modo avançado, (3) conectar um canal. Depois disso, um checklist no dashboard acompanha os marcos restantes: adicionar base de conhecimento, testar o agente no preview e receber a primeira mensagem real.",

	"Canais suportados: WhatsApp (Meta Cloud API, com cadastro embutido da Meta), Telegram (token de bot, registro automático do webhook), Instagram Direct (DMs) e SMS (via sua própria conta Twilio). Um canal pertence à conta e o agente é opcional: canal sem agente funciona em modo 100% humano (só atendentes respondem). Excluir um agente não exclui os canais dele.",

	"Base de conhecimento (RAG): cada agente tem fontes de conhecimento próprias. Você ingere texto direto, URLs (o conteúdo é extraído automaticamente) ou PDFs com texto selecionável (até 10 MB; PDFs escaneados sem camada de texto não são suportados ainda). O agente busca os trechos mais relevantes por similaridade semântica e responde com base neles.",

	"Plano de teste e limites: o teste grátis inclui 1 agente, 1 canal, 50 conversas por mês e 5.000 tokens de IA por mês. Ao atingir 80% e 100% do limite de tokens você recebe avisos por email. O consumo atual fica visível na tela de uso do dashboard. Para aumentar os limites, faça upgrade para um plano pago.",

	"Planos pagos e cobrança: a assinatura é mensal via Stripe (cartão de crédito), contratada direto no dashboard em Configurações → Faturamento — lá estão os valores atuais de cada plano. Cada plano define os limites de agentes, canais, conversas e tokens de IA. Você pode cancelar a qualquer momento pelo portal de cobrança; o plano segue ativo até o fim do período já pago.",

	"Atendimento humano e takeover: qualquer atendente pode assumir uma conversa — isso pausa o bot naquela conversa e as respostas passam a ser humanas. Dá para pausar/retomar o bot por conversa, atribuir conversas a atendentes específicos e ativar rodízio automático de atribuição para a equipe. Papéis de usuário: admin (gerencia tudo) e atendente (atende conversas).",

	"Agendamento: agentes podem marcar compromissos. Configure um calendário por agente com dias disponíveis, janelas de horário e duração do slot; a IA agenda sozinha durante a conversa e, com a integração opcional de Google Calendar, o evento é criado na agenda real.",

	"Suporte Replider: este canal é o jeito oficial de falar com seu gerente de conta — direto do dashboard, sem WhatsApp. Sua mensagem chega para a equipe Replider, a IA responde primeiro com base neste FAQ e um humano assume quando necessário. Você recebe notificação push (se ativada) e, se uma resposta ficar horas sem leitura, um aviso por email.",
}

// EnsureProductKnowledgeUseCase seeds the internal-channel agent's
// product FAQ. Runs at boot (main.go), detached — embedding is a
// network call and must not block startup.
type EnsureProductKnowledgeUseCase struct {
	channelRepo   domain.ChannelRepository
	knowledgeRepo domain.KnowledgeRepository
	aiProvider    domain.AIProvider
}

func NewEnsureProductKnowledgeUseCase(
	channelRepo domain.ChannelRepository,
	knowledgeRepo domain.KnowledgeRepository,
	aiProvider domain.AIProvider,
) *EnsureProductKnowledgeUseCase {
	return &EnsureProductKnowledgeUseCase{
		channelRepo:   channelRepo,
		knowledgeRepo: knowledgeRepo,
		aiProvider:    aiProvider,
	}
}

// Execute seeds any missing FAQ chunks and returns how many were
// created this run. Skips (0, nil) when the feature is off, the
// internal channel doesn't exist yet, or no agent is assigned to it —
// the seed re-runs on every boot, so it self-arms once the platform
// assigns its agent. An embedding/persist error aborts the remaining
// chunks (returned alongside the partial count); the content-based
// idempotency resumes the seed on the next boot.
func (uc *EnsureProductKnowledgeUseCase) Execute(ctx context.Context, platformAccountID string) (int, error) {
	if platformAccountID == "" {
		return 0, nil
	}

	channels, err := uc.channelRepo.ListByAccountID(ctx, platformAccountID)
	if err != nil {
		return 0, err
	}
	var agentID string
	for _, ch := range channels {
		if ch.Type == domain.ChannelTypeInternal && ch.AgentID != nil && *ch.AgentID != "" {
			agentID = *ch.AgentID
			break
		}
	}
	if agentID == "" {
		slog.Info("product knowledge seed skipped: internal channel has no agent assigned")
		return 0, nil
	}

	source, err := uc.findOrCreateSource(ctx, agentID)
	if err != nil {
		return 0, err
	}

	existing, err := uc.knowledgeRepo.ListChunksBySource(ctx, source.ID)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]bool, len(existing))
	for _, c := range existing {
		seen[c.Content] = true
	}

	sourceName := ProductFAQSourceName
	created := 0
	for _, content := range productFAQChunks {
		if seen[content] {
			continue
		}
		embedding, err := uc.aiProvider.GetEmbedding(ctx, content)
		if err != nil {
			return created, err
		}
		chunk := &domain.KnowledgeChunk{
			ID:         ulid.Make().String(),
			SourceID:   source.ID,
			Content:    content,
			Embedding:  embedding,
			SourceName: &sourceName,
			CreatedAt:  time.Now(),
		}
		if err := uc.knowledgeRepo.CreateChunk(ctx, chunk); err != nil {
			return created, err
		}
		created++
	}
	if created > 0 {
		slog.Info("product knowledge seeded for internal-channel agent",
			"agent_id", agentID, "chunks_created", created)
	}
	return created, nil
}

func (uc *EnsureProductKnowledgeUseCase) findOrCreateSource(ctx context.Context, agentID string) (*domain.KnowledgeSource, error) {
	sources, err := uc.knowledgeRepo.ListSources(ctx, agentID)
	if err != nil {
		return nil, err
	}
	for _, s := range sources {
		if s.Name == ProductFAQSourceName {
			return s, nil
		}
	}
	desc := "FAQ oficial do produto, seed automático do canal interno de CS (US-010)"
	source := &domain.KnowledgeSource{
		ID:          ulid.Make().String(),
		AgentID:     agentID,
		Name:        ProductFAQSourceName,
		Description: &desc,
		CreatedAt:   time.Now(),
	}
	if err := uc.knowledgeRepo.CreateSource(ctx, source); err != nil {
		return nil, err
	}
	return source, nil
}
