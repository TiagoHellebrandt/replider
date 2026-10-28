package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// countingDebounceRepo records Enqueue calls so the replay-loop regression is
// observable: RunPending must NOT re-enqueue the row it just claimed.
type countingDebounceRepo struct {
	enqueued int
	deleted  int
}

func (r *countingDebounceRepo) Enqueue(ctx context.Context, p *domain.PendingInbound) (*domain.PendingInbound, error) {
	r.enqueued++
	// Mirror the real repo: a fresh insert after a delete returns the row.
	return p, nil
}
func (r *countingDebounceRepo) Get(ctx context.Context, conversationID string) (*domain.PendingInbound, error) {
	return nil, nil
}
func (r *countingDebounceRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]*domain.PendingInbound, error) {
	return nil, nil
}
func (r *countingDebounceRepo) Release(ctx context.Context, conversationID string) error { return nil }
func (r *countingDebounceRepo) Delete(ctx context.Context, conversationID string) error {
	r.deleted++
	return nil
}
func (r *countingDebounceRepo) AnswerNow(ctx context.Context, accountID, conversationID string, now time.Time) (bool, error) {
	return false, nil
}
func (r *countingDebounceRepo) ReleaseStaleClaims(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}

// The reported CRITICAL: the debounce worker replayed a non-buy-intent burst
// through ProcessAI, whose first step (deferInbound) re-enqueued the very row
// the worker had just deleted — looping forever, so the customer was never
// answered. RunPending must go straight into the shared pipeline
// (processAIAfterDefer) and never re-enqueue.
func TestRunPending_DoesNotReEnqueueTheClaimedRun(t *testing.T) {
	ctx := context.Background()

	debounce := &countingDebounceRepo{}
	convRepo := &MockConversationRepo{}
	agentRepo := &MockAgentRepo{}

	// The conversation is live and not taken over.
	convRepo.On("GetByID", mock.Anything, "conv-1", "acc-1").
		Return(&domain.Conversation{ID: "conv-1", AccountID: "acc-1"}, nil)
	// Bail the pipeline right after the defer decision — a missing agent is the
	// cheapest early return in processAIAfterDefer. What matters is only that
	// Enqueue is never called on this path.
	agentRepo.On("GetByID", mock.Anything, "agent-1").
		Return(nil, errors.New("stop here"))

	uc := NewInboundWebhookUseCase(new(MockChannelRepo), convRepo, new(MockMessageRepo), nil, agentRepo)
	uc.SetInboundDebounce(debounce)

	uc.RunPending(ctx, &domain.PendingInbound{
		ConversationID: "conv-1",
		AccountID:      "acc-1",
		ChannelID:      "chan-1",
		AgentID:        "agent-1",
		Content:        "oi, vocês atendem em Cuiabá?", // no buy-intent phrase
	})

	assert.Equal(t, 1, debounce.deleted, "the claimed row is deleted once")
	assert.Equal(t, 0, debounce.enqueued, "RunPending must NOT re-enqueue — that was the infinite-loop bug")
}
