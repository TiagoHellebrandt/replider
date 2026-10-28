package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// Inbound debounce queue against a real database (§6.15, K1).
//
// The queue's guarantees are all in SQL, so this is where they have to be proven:
// a burst COLLAPSES into one pending run, a customer who keeps typing cannot
// defer the answer past the ceiling, and two pods claim disjoint sets. Getting
// any of these wrong means either a duplicate reply or a swallowed question.

func seedDebounceConversation(t *testing.T, accountID string) (conversationID, channelID, agentID string) {
	t.Helper()
	ctx := context.Background()

	agentID = ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO agents (id, account_id, name, system_prompt, created_at, updated_at)
		 VALUES ($1, $2, 'SDR', 'você é um SDR', NOW(), NOW())`,
		agentID, accountID)
	require.NoError(t, err)

	channelID = ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, agent_id, name, provider, webhook_token,
		                       credentials, created_at, updated_at)
		 VALUES ($1, $2, $3, 'WhatsApp', 'whatsapp', $4, '{}'::jsonb, NOW(), NOW())`,
		channelID, accountID, agentID, ulid.Make().String())
	require.NoError(t, err)

	conversationID = ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, status,
		                           created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'open', NOW(), NOW())`,
		conversationID, accountID, channelID, "+5565999990000")
	require.NoError(t, err)
	return conversationID, channelID, agentID
}

func pendingFor(t *testing.T, convID, accountID, channelID, agentID string, respondAt, firstQueued time.Time, content string) *domain.PendingInbound {
	t.Helper()
	return &domain.PendingInbound{
		ConversationID: convID,
		AccountID:      accountID,
		ChannelID:      channelID,
		AgentID:        agentID,
		RespondAt:      respondAt,
		FirstQueuedAt:  firstQueued,
		Content:        content,
	}
}

func TestInboundDebounce_ABurstCollapsesIntoOneRun(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	now := time.Now()
	// "oi" / "tudo bem?" / "qual o preço?" over eight seconds.
	for i, text := range []string{"oi", "tudo bem?", "vocês entregam em Cuiabá?"} {
		at := now.Add(time.Duration(i*4) * time.Second)
		_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
			at.Add(40*time.Second), at, text))
		require.NoError(t, err)
	}

	pending, err := repo.Get(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, pending)

	// One row, three messages. This is the whole feature: three paid replies
	// become one.
	assert.Equal(t, 3, pending.MessageCount)
	// And the trigger is the LAST thing they said — the question to answer.
	assert.Equal(t, "vocês entregam em Cuiabá?", pending.Content)

	var rows int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM inbound_debounce WHERE conversation_id = $1`,
		convID).Scan(&rows))
	assert.Equal(t, 1, rows)
}

func TestInboundDebounce_RespondAtOnlyMovesForward(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	now := time.Now()
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		now.Add(60*time.Second), now, "pergunta longa"))
	require.NoError(t, err)

	// A late-arriving webhook for an OLDER message, asking for a sooner reply.
	out, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		now.Add(10*time.Second), now, "oi"))
	require.NoError(t, err)
	require.NotNil(t, out)

	// It must not pull the answer earlier than the newest message justifies —
	// otherwise a reordered webhook answers before the customer finished.
	assert.WithinDuration(t, now.Add(60*time.Second), out.RespondAt, 2*time.Second)
}

func TestInboundDebounce_ACustomerWhoKeepsTypingStillGetsAnswered(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	first := time.Now()
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		first.Add(40*time.Second), first, "primeira"))
	require.NoError(t, err)

	// Twenty more messages, each pushing the wait out by another 40s.
	for i := 1; i <= 20; i++ {
		at := first.Add(time.Duration(i*30) * time.Second)
		_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
			at.Add(40*time.Second), at, "mais uma"))
		require.NoError(t, err)
	}

	pending, err := repo.Get(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, pending)

	// At some point "waiting for them to finish" becomes "ignoring them". The
	// ceiling is measured from the FIRST message, and the clamp is in the upsert
	// so two concurrent inbounds cannot race past it.
	ceiling := first.Add(domain.DebounceMaxTotalWait)
	assert.False(t, pending.RespondAt.After(ceiling.Add(time.Second)),
		"respond_at %v pushed past the %v ceiling", pending.RespondAt, domain.DebounceMaxTotalWait)
	assert.Equal(t, 21, pending.MessageCount)
}

func TestInboundDebounce_ClaimIsExclusiveSoTwoPodsCannotBothAnswer(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	past := time.Now().Add(-time.Minute)
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		past, past, "já venceu"))
	require.NoError(t, err)

	firstPod, err := repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, firstPod, 1)

	// The second pod's pass finds nothing: the row is claimed. Without this a
	// burst gets two replies, which is the failure the whole table exists to
	// prevent.
	secondPod, err := repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)
	assert.Empty(t, secondPod)
}

func TestInboundDebounce_AMessageArrivingMidRunIsNotFoldedIntoIt(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	past := time.Now().Add(-time.Minute)
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		past, past, "pergunta"))
	require.NoError(t, err)
	claimed, err := repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// A new message while the reply is being generated. Extending the claimed
	// row would fold a question into an answer already written without it.
	now := time.Now()
	out, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		now.Add(40*time.Second), now, "e mais uma coisa"))
	require.NoError(t, err)

	// The claimed row is returned unchanged — the caller learns a run is in
	// flight rather than silently getting a second one.
	require.NotNil(t, out)
	assert.NotNil(t, out.ClaimedAt)
	assert.Equal(t, 1, out.MessageCount)
	assert.Equal(t, "pergunta", out.Content)
}

func TestInboundDebounce_AnswerNowIsAccountScoped(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, mine)
	repo := postgres.NewInboundDebounceRepository(testPool)

	future := time.Now().Add(2 * time.Minute)
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, mine, channelID, agentID,
		future, time.Now(), "pergunta"))
	require.NoError(t, err)

	// Another tenant naming my conversation id matches zero rows: the scope is in
	// the UPDATE, not in a caller's memory.
	pulled, err := repo.AnswerNow(ctx, theirs, convID, time.Now())
	require.NoError(t, err)
	assert.False(t, pulled)

	pulled, err = repo.AnswerNow(ctx, mine, convID, time.Now())
	require.NoError(t, err)
	assert.True(t, pulled)

	// And it is now due.
	due, err := repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)
	assert.Len(t, due, 1)
}

func TestInboundDebounce_AnswerNowRefusesToHurryARunInFlight(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	past := time.Now().Add(-time.Minute)
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		past, past, "pergunta"))
	require.NoError(t, err)
	_, err = repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)

	// There is nothing to hurry — the reply is being written. Clearing the claim
	// would let a second worker answer the same burst.
	pulled, err := repo.AnswerNow(ctx, accountID, convID, time.Now())
	require.NoError(t, err)
	assert.False(t, pulled)
}

func TestInboundDebounce_ADeadWorkersClaimIsRetried(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	past := time.Now().Add(-time.Minute)
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		past, past, "pergunta"))
	require.NoError(t, err)
	_, err = repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)

	// The pod died between claiming and replying. A permanently unanswered
	// message is the worst failure this feature can produce: the customer's
	// question was accepted and then dropped.
	released, err := repo.ReleaseStaleClaims(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(1), released)

	due, err := repo.ClaimDue(ctx, time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, due, 1, "a stale claim must become due again")
}

func TestInboundDebounce_DeletingTheConversationTakesTheQueueRow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	convID, channelID, agentID := seedDebounceConversation(t, accountID)
	repo := postgres.NewInboundDebounceRepository(testPool)

	now := time.Now()
	_, err := repo.Enqueue(ctx, pendingFor(t, convID, accountID, channelID, agentID,
		now.Add(time.Minute), now, "pergunta"))
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, convID)
	require.NoError(t, err)

	// A pending reply to a conversation that no longer exists is a run that can
	// only fail — the CASCADE is what keeps the worker's queue honest.
	pending, err := repo.Get(ctx, convID)
	require.NoError(t, err)
	assert.Nil(t, pending)
}
