package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
)

// Canal-interno US-004: the unread-email-fallback due-query is the
// idempotency guard (one email per unread window, restart-safe), so its
// window logic is pinned against a real Postgres here.

// supportFixture seeds: a platform account with an internal channel, a
// customer account with a registered user, and one internal thread owned
// by that user carrying an outbound (platform-side) message at lastOutbound.
type supportFixture struct {
	convID     string
	userID     string
	customerID string
	channelID  string
}

func seedSupportThread(t *testing.T, lastOutbound time.Time) supportFixture {
	t.Helper()
	ctx := context.Background()

	registerUser(t, "Plataforma", "sf-platform@test.com", "pass1234")
	registerUser(t, "Cliente", "sf-customer@test.com", "pass1234")

	var platformID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT account_id FROM users WHERE email = $1", "sf-platform@test.com").Scan(&platformID))
	var customerID, userID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT account_id, id FROM users WHERE email = $1", "sf-customer@test.com").Scan(&customerID, &userID))

	channelID := ulid.Make().String()
	_, err := testPool.Exec(ctx, `
		INSERT INTO channels (id, account_id, provider, name, credentials, webhook_token)
		VALUES ($1, $2, 'internal', 'Suporte Replider', '{}', $3)`,
		channelID, platformID, ulid.Make().String())
	require.NoError(t, err)

	convID := ulid.Make().String()
	_, err = testPool.Exec(ctx, `
		INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_name, status, customer_account_id, created_at)
		VALUES ($1, $2, $3, $4, 'Cliente', 'open', $5, $6)`,
		convID, platformID, channelID, userID, customerID, lastOutbound.Add(-time.Hour))
	require.NoError(t, err)

	// Customer inbound then the platform-side reply at lastOutbound.
	_, err = testPool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
		VALUES ($1, $2, $3, $4, 'text', 'Preciso de ajuda', 'inbound', $5)`,
		ulid.Make().String(), channelID, convID, userID, lastOutbound.Add(-time.Hour))
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
		VALUES ($1, $2, $3, 'bot', 'text', 'Posso ajudar!', 'outbound', $4)`,
		ulid.Make().String(), channelID, convID, lastOutbound)
	require.NoError(t, err)

	return supportFixture{convID: convID, userID: userID, customerID: customerID, channelID: channelID}
}

func dueConvIDs(t *testing.T, repo *postgres.SupportNotifyRepository, cutoff time.Time) []string {
	t.Helper()
	rows, err := repo.ListDueEmailFallbacks(context.Background(), cutoff, 50)
	require.NoError(t, err)
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ConversationID)
	}
	return ids
}

func TestE2E_SupportFallback_WindowLogic(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewSupportNotifyRepository(testPool)
	fx := seedSupportThread(t, time.Now().Add(-5*time.Hour))
	cutoff := time.Now().Add(-4 * time.Hour)

	// 1. Unread outbound past the window → due, with the user's email +
	// customer account currency for locale resolution.
	rows, err := repo.ListDueEmailFallbacks(context.Background(), cutoff, 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, fx.convID, rows[0].ConversationID)
	assert.Equal(t, fx.userID, rows[0].UserID)
	assert.Equal(t, "sf-customer@test.com", rows[0].UserEmail)
	assert.Equal(t, fx.customerID, rows[0].CustomerAccountID)
	assert.NotEmpty(t, rows[0].Currency)
	assert.WithinDuration(t, time.Now().Add(-5*time.Hour), rows[0].LastOutboundAt, time.Minute)

	// 2. Email sent → no longer due (idempotent under worker restart).
	require.NoError(t, repo.MarkFallbackEmailSent(context.Background(), fx.convID))
	assert.Empty(t, dueConvIDs(t, repo, cutoff))

	// 3. A SECOND unread message inside the same unread window must NOT
	// re-spam: still not due even though it's older than the cutoff.
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
		VALUES ($1, $2, $3, 'bot', 'text', 'Follow-up', 'outbound', $4)`,
		ulid.Make().String(), fx.channelID, fx.convID, time.Now().Add(-Hours(4.5)))
	require.NoError(t, err)
	assert.Empty(t, dueConvIDs(t, repo, cutoff), "subsequent unread message inside the same window must not re-email")

	// 4. The owner reads the thread → the pending window closes.
	require.NoError(t, repo.MarkThreadRead(context.Background(), fx.convID))
	assert.Empty(t, dueConvIDs(t, repo, cutoff))

	// 5. Re-arm: a NEW platform reply after the read, once past the
	// window, becomes due again (one email per unread session).
	_, err = testPool.Exec(context.Background(), `
		UPDATE conversations SET support_last_read_at = $2, support_fallback_email_at = $3 WHERE id = $1`,
		fx.convID, time.Now().Add(-6*time.Hour), time.Now().Add(-7*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, []string{fx.convID}, dueConvIDs(t, repo, cutoff),
		"a reply sent after the last read re-arms the fallback")
}

func TestE2E_SupportFallback_IgnoresRecentAndNonInternal(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewSupportNotifyRepository(testPool)
	ctx := context.Background()

	// Outbound only 1h old → inside the window, not due yet.
	fx := seedSupportThread(t, time.Now().Add(-time.Hour))
	cutoff := time.Now().Add(-4 * time.Hour)
	assert.Empty(t, dueConvIDs(t, repo, cutoff))

	// A NON-internal conversation with an ancient unread outbound never
	// qualifies — the fallback is an internal-CS-channel feature.
	tgChannel := ulid.Make().String()
	_, err := testPool.Exec(ctx, `
		INSERT INTO channels (id, account_id, provider, name, credentials, webhook_token)
		SELECT $1, account_id, 'telegram', 'TG', '{}', $2 FROM users WHERE id = $3`,
		tgChannel, ulid.Make().String(), fx.userID)
	require.NoError(t, err)
	tgConv := ulid.Make().String()
	_, err = testPool.Exec(ctx, `
		INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_name, status, created_at)
		SELECT $1, account_id, $2, $3::varchar, 'Cliente', 'open', NOW() - INTERVAL '1 day' FROM users WHERE id = $4`,
		tgConv, tgChannel, fx.userID, fx.userID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at)
		VALUES ($1, $2, $3, 'bot', 'text', 'oi', 'outbound', NOW() - INTERVAL '1 day')`,
		ulid.Make().String(), tgChannel, tgConv)
	require.NoError(t, err)

	assert.Empty(t, dueConvIDs(t, repo, cutoff))
}

// Hours converts a fractional hour count to a Duration (test readability).
func Hours(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
