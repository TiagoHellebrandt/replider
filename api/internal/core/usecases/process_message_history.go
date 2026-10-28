package usecases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// summaryRefreshInterval throttles how often the rolling conversation
// summary is regenerated. Once a conversation grows past the prompt
// window every turn would otherwise trigger a summarization LLM call;
// capping it to one refresh per interval keeps that cost bounded while
// still folding in newly aged-out turns within a few minutes.
const summaryRefreshInterval = 10 * time.Minute

// summaryMaxOlderMessages bounds how many aged-out messages are fed into a
// single summarization call so the summarizer prompt itself can't grow
// unbounded.
const summaryMaxOlderMessages = 50

// buildBoundedHistory turns the most-recent messages of a conversation
// (as returned newest-first by ListByConversationID) into a chronological
// ChatMessage slice for the AI prompt, bounded to maxHistory turns
// (US-024). When a rolling summary of older turns exists it is injected as
// a single leading context line so long conversations keep context without
// replaying the entire (unbounded) history.
//
// The result length is at most: 1 (summary, if present) + maxHistory
// (recent turns) + 1 (the current message) — the prompt is provably
// bounded no matter how long the conversation runs.
func buildBoundedHistory(prevMsgs []*domain.Message, currentMessage string, summary *string, maxHistory int) []domain.ChatMessage {
	if maxHistory <= 0 {
		maxHistory = domain.DefaultMaxHistoryMessages
	}

	history := make([]domain.ChatMessage, 0, maxHistory+2)

	// Inject the rolling summary first so the model reads it as background
	// before the recent turns. Uses the same "System: ..." convention the
	// rest of this pipeline uses for injected, non-customer context.
	if summary != nil && strings.TrimSpace(*summary) != "" {
		history = append(history, domain.ChatMessage{
			Role:    "user",
			Content: "System: Resumo da conversa anterior: " + strings.TrimSpace(*summary),
		})
	}

	// Defensive cap: even if the caller passed more than maxHistory rows,
	// keep only the most recent maxHistory (prevMsgs is newest-first, so
	// the most recent are at the front).
	recent := prevMsgs
	if len(recent) > maxHistory {
		recent = recent[:maxHistory]
	}

	// Reverse into chronological order, skipping empty (media-only) rows.
	for i := len(recent) - 1; i >= 0; i-- {
		msg := recent[i]
		content := ""
		if msg.Content != nil {
			content = *msg.Content
		}
		if content == "" {
			continue
		}
		role := "user"
		if msg.Direction == "outbound" {
			role = "model"
		}
		history = append(history, domain.ChatMessage{Role: role, Content: content})
	}

	// Always append the current message last.
	history = append(history, domain.ChatMessage{Role: "user", Content: currentMessage})
	return history
}

// refreshSummaryAsync regenerates the rolling summary of turns that have
// aged out of the prompt window and persists it on the conversation
// (US-024). It is launched fire-and-forget and runs on a detached context
// (US-033 justified-Background: it must outlive the originating request,
// which returns a stream the caller drains afterwards). Best-effort: any
// failure is logged and dropped so it can never break or delay a reply.
func (uc *ProcessMessageUseCase) refreshSummaryAsync(ctx context.Context, conv *domain.Conversation, plan *domain.Plan, aiPrv domain.AIProvider, maxHistory int) {
	if conv == nil || aiPrv == nil {
		return
	}
	// Throttle: skip if we summarized recently. Bounds summarization cost
	// to ~one LLM call per conversation per summaryRefreshInterval.
	if conv.SummaryUpdatedAt != nil && time.Since(*conv.SummaryUpdatedAt) < summaryRefreshInterval {
		return
	}

	// Caller detaches the request ctx with context.WithoutCancel so the
	// fire-and-forget summary survives the response returning (US-024); we
	// still bound it with our own timeout.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Pull the batch of messages just beyond the prompt window — these are
	// the aged-out turns the live history no longer carries.
	older, err := uc.msgRepo.ListByConversationID(ctx, conv.ID, summaryMaxOlderMessages, int32(maxHistory)) //nolint:gosec // G115: maxHistory is a small plan-bounded int
	if err != nil || len(older) == 0 {
		return
	}

	// Render the older turns (oldest-first) into a compact transcript.
	var sb strings.Builder
	for i := len(older) - 1; i >= 0; i-- {
		msg := older[i]
		if msg.Content == nil || *msg.Content == "" {
			continue
		}
		speaker := "Cliente"
		if msg.Direction == "outbound" {
			speaker = "Atendente"
		}
		sb.WriteString(speaker)
		sb.WriteString(": ")
		sb.WriteString(*msg.Content)
		sb.WriteString("\n")
	}
	transcript := strings.TrimSpace(sb.String())
	if transcript == "" {
		return
	}

	systemPrompt := "Você resume conversas de atendimento. Produza um resumo conciso (no máximo 5 frases) " +
		"em português dos fatos importantes da conversa: pedido do cliente, dados fornecidos (nome, " +
		"produto, datas), decisões e pendências. Não invente informações."
	// Merge with any existing summary so context accumulates across slides.
	userContent := transcript
	if conv.Summary != nil && strings.TrimSpace(*conv.Summary) != "" {
		userContent = "Resumo até agora:\n" + strings.TrimSpace(*conv.Summary) +
			"\n\nNovas mensagens a incorporar:\n" + transcript
	}

	resp, err := aiPrv.GenerateResponse(ctx, systemPrompt, []domain.ChatMessage{{Role: "user", Content: userContent}}, nil)
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		slog.Warn("US-024: conversation summary refresh failed",
			"conversation_id", conv.ID, "error", err)
		return
	}

	if err := uc.convRepo.UpdateSummary(ctx, conv.ID, conv.AccountID, strings.TrimSpace(resp.Content)); err != nil {
		slog.Warn("US-024: persisting conversation summary failed",
			"conversation_id", conv.ID, "error", err)
	}
}
