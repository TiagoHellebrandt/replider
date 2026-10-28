package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/usecases"
)

// US-002 (review fixes): the SLA first-response metric must measure the
// first HUMAN OUTBOUND message. Inbound rows store the contact's phone
// in sender (never the literal 'customer'), so the old filter
// sender NOT IN ('bot','customer') matched the customer's own inbound
// message and every conversation "responded" in ~0 minutes — the
// breach ladder was dead code.

// seedSLAConversation inserts an assigned conversation plus its message
// timeline directly via SQL (simulating the webhook + reply flow).
func seedSLAConversation(t *testing.T, accountID, channelID, assignedTo string, createdAt time.Time, humanReplyAt *time.Time) string {
	t.Helper()
	ctx := context.Background()
	convID := ulid.Make().String()
	// Unique phone per conversation — an account can only have one OPEN
	// conversation per contact (uniq_conversations_open_per_contact).
	contactPhone := "+55659" + convID[18:]
	_, err := testPool.Exec(ctx, `
		INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_name, status, assigned_to, created_at)
		VALUES ($1, $2, $3, $4, 'Cliente', 'open', $5, $6)
	`, convID, accountID, channelID, contactPhone, assignedTo, createdAt)
	require.NoError(t, err)

	// Customer inbound: sender is the contact's phone, NOT 'customer'.
	_, err = testPool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
		VALUES ($1, $2, $3, $4, 'text', 'Preciso de ajuda', 'inbound', $5)
	`, ulid.Make().String(), channelID, convID, contactPhone, createdAt)
	require.NoError(t, err)

	// Bot auto-reply one minute later — must not count as human response.
	_, err = testPool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
		VALUES ($1, $2, $3, 'bot', 'text', 'Sou o assistente', 'outbound', $4)
	`, ulid.Make().String(), channelID, convID, createdAt.Add(time.Minute))
	require.NoError(t, err)

	if humanReplyAt != nil {
		_, err = testPool.Exec(ctx, `
			INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
			VALUES ($1, $2, $3, $4, 'text', 'Oi, vou te ajudar', 'outbound', $5)
		`, ulid.Make().String(), channelID, convID, assignedTo, *humanReplyAt)
		require.NoError(t, err)
	}
	return convID
}

// setupSLAFixture registers an account with a telegram channel and
// returns (accountID, channelID, adminUserID).
func setupSLAFixture(t *testing.T, name, email string) (string, string, string) {
	t.Helper()
	registerUser(t, name, email, "pass1234")
	verifyEmail(t, email)
	token := login(t, email, "pass1234")

	w := doRequest("POST", "/v1/channels", map[string]interface{}{
		"type": "telegram", "name": "TG", "config": map[string]string{"bot_token": "fake"},
	}, token)
	require.Equal(t, 201, w.Code, "channel create failed: %s", w.Body.String())
	var channel map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &channel))
	channelID := channel["id"].(string)

	var accountID, userID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT account_id, id FROM users WHERE email = $1", email).Scan(&accountID, &userID))
	return accountID, channelID, userID
}

func TestE2E_SLAFirstHumanResponse_UsesOutboundDirection(t *testing.T) {
	cleanDB(t)
	accountID, channelID, attendantID := setupSLAFixture(t, "Plataforma", "sla-admin@test.com")

	// 2026-06-02 is a Tuesday; 14:00 UTC = 10:00 America/Cuiaba, well
	// inside the 8h-18h business window.
	createdAt := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)
	humanReplyAt := createdAt.Add(5 * time.Hour)
	convID := seedSLAConversation(t, accountID, channelID, attendantID, createdAt, &humanReplyAt)

	repo := postgres.NewCommissionRepository(testPool)
	convs, err := repo.ListSLAConversations(context.Background(), accountID,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, convs, 1)
	require.Equal(t, convID, convs[0].ConversationID)

	// The first human response is the attendant's outbound reply 5h in
	// — not the customer's own inbound message at t0 (old bug) and not
	// the bot's auto-reply.
	require.NotNil(t, convs[0].FirstHumanResponseAt)
	assert.WithinDuration(t, humanReplyAt, *convs[0].FirstHumanResponseAt, time.Second)
}

func TestE2E_SLAComputeCycle_BreachDetection(t *testing.T) {
	cleanDB(t)
	accountID, channelID, slowAttendant := setupSLAFixture(t, "Plataforma", "sla-slow@test.com")

	// Second attendant (separate registered user — the FK only needs a
	// users row) answering within the 4h default threshold.
	registerUser(t, "Rapida", "sla-fast@test.com", "pass1234")
	var fastAttendant string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "sla-fast@test.com").Scan(&fastAttendant))

	createdAt := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC) // Tue 10:00 Cuiaba

	// Slow: human reply 5h later = 300 business minutes > 240 -> breach.
	slowReply := createdAt.Add(5 * time.Hour)
	seedSLAConversation(t, accountID, channelID, slowAttendant, createdAt, &slowReply)

	// Fast: human reply 1h later = 60 business minutes -> within SLA.
	fastReply := createdAt.Add(time.Hour)
	seedSLAConversation(t, accountID, channelID, fastAttendant, createdAt, &fastReply)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))
	uc.SetPlatformAccountID(accountID)
	_, err := uc.ComputeCycle(context.Background(), "owner@test.com", "2026-06")
	require.NoError(t, err)

	// Old bug: first "human response" was the inbound at t0 -> 0
	// business minutes -> no conversation ever breached.
	slow, err := repo.GetSLAState(context.Background(), slowAttendant, "2026-06")
	require.NoError(t, err)
	require.NotNil(t, slow)
	assert.Equal(t, 1, slow.TotalConversations)
	assert.Equal(t, 1, slow.BreachedConversations)
	assert.True(t, slow.Breach, "5h human first response must breach the 4h default SLA")

	fast, err := repo.GetSLAState(context.Background(), fastAttendant, "2026-06")
	require.NoError(t, err)
	require.NotNil(t, fast)
	assert.Equal(t, 1, fast.TotalConversations)
	assert.Equal(t, 0, fast.BreachedConversations)
	assert.False(t, fast.Breach, "1h human first response must stay within the SLA")
}

func TestE2E_SellerConvStats_HumanReplyRequiresOutbound(t *testing.T) {
	cleanDB(t)
	accountID, channelID, attendantID := setupSLAFixture(t, "Carteira", "sla-carteira@test.com")

	// Link the account to a seller so it shows up in the carteira stats.
	sellerID := attendantID
	_, err := testPool.Exec(context.Background(),
		"UPDATE accounts SET seller_user_id = $1 WHERE id = $2", sellerID, accountID)
	require.NoError(t, err)

	createdAt := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)

	// Conversation 1: customer inbound + bot only — no human takeover.
	seedSLAConversation(t, accountID, channelID, attendantID, createdAt, nil)
	// Conversation 2: human outbound reply — a real takeover.
	conv2At := createdAt.Add(time.Hour)
	replyAt := conv2At.Add(30 * time.Minute)
	seedSLAConversation(t, accountID, channelID, attendantID, conv2At, &replyAt)

	repo := postgres.NewSellerSuggestionRepository(testPool)
	stats, err := repo.ListConversationStats(context.Background(), sellerID,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats[0].CurrentTotal)
	// Old bug: the customer's inbound (sender = phone) counted as a
	// "human reply", so this read 2.
	assert.Equal(t, 1, stats[0].CurrentHuman)
}

// canal-interno US-009: internal support threads are thread-per-USER,
// so one customer account can hold several — the SLA metric must
// aggregate them per customer account. 2 threads of the same account
// with 1 blown = the account counts ONE unit and ONE breach, never two.
// Pins the SQL round-trip of conversations.customer_account_id through
// ListSLAConversations into the compute.
func TestE2E_SLAInternalThreadsAggregatePerCustomerAccount(t *testing.T) {
	cleanDB(t)
	accountID, channelID, attendant := setupSLAFixture(t, "Plataforma", "sla-internal@test.com")

	// The customer account both threads belong to.
	registerUser(t, "Cliente", "sla-cust@test.com", "pass1234")
	var custAccountID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "sla-cust@test.com").Scan(&custAccountID))

	createdAt := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC) // Tue 10:00 Cuiaba
	slowReply := createdAt.Add(5 * time.Hour)                 // 300 business min > 240 -> blown
	fastReply := createdAt.Add(time.Hour)                     // within SLA
	c1 := seedSLAConversation(t, accountID, channelID, attendant, createdAt, &slowReply)
	c2 := seedSLAConversation(t, accountID, channelID, attendant, createdAt, &fastReply)
	for _, id := range []string{c1, c2} {
		_, err := testPool.Exec(context.Background(),
			"UPDATE conversations SET customer_account_id = $1 WHERE id = $2", custAccountID, id)
		require.NoError(t, err)
	}

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))
	uc.SetPlatformAccountID(accountID)
	_, err := uc.ComputeCycle(context.Background(), "owner@test.com", "2026-06")
	require.NoError(t, err)

	st, err := repo.GetSLAState(context.Background(), attendant, "2026-06")
	require.NoError(t, err)
	require.NotNil(t, st)
	assert.Equal(t, 1, st.TotalConversations, "two threads of one account must collapse into one SLA unit")
	assert.Equal(t, 1, st.BreachedConversations, "one blown thread = one account breach, never two")
	assert.True(t, st.Breach)
}
