package e2e

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
)

// "Mensagens por Remetente" on the analytics screen.
//
// The old query split by a literal sender value: the contact was
// `sender = 'customer'` and a human attendant was "anything that is neither
// 'bot' nor 'customer'". But an inbound row stores the CONTACT's own phone in
// `sender` and the string 'customer' is never written in production (see the
// `messages.sender` note in api/CLAUDE.md). So on a live account every customer
// message was filed under "Humano" and "Contato" showed 0 — on a dashboard
// whose entire content is customer conversations. A buyer auditing the screen
// called it impossible; it was.
//
// This needs a real Postgres because the bug lives entirely in the SQL: any
// mock returns whatever the test tells it to.
func TestMessageStats_SplitsByDirectionNotSenderLiteral(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")

	ctx := context.Background()
	var accountID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT account_id FROM users WHERE email = $1", "admin@test.com").Scan(&accountID))

	channelID := ulid.Make().String()
	_, err := testPool.Exec(ctx, `
		INSERT INTO channels (id, account_id, provider, name, credentials, webhook_token)
		VALUES ($1, $2, 'telegram', 'TG', '{}', $3)
	`, channelID, accountID, channelID)
	require.NoError(t, err)

	convID := ulid.Make().String()
	const contactPhone = "5511988887777"
	_, err = testPool.Exec(ctx, `
		INSERT INTO conversations (id, account_id, channel_id, contact_id, status)
		VALUES ($1, $2, $3, $4, 'open')
	`, convID, accountID, channelID, contactPhone)
	require.NoError(t, err)

	// Exactly how production writes them: the customer's phone as `sender` on
	// the inbound rows, 'bot' and a user ULID on the outbound ones.
	attendantID := ulid.Make().String()
	rows := []struct {
		sender    string
		direction string
	}{
		{contactPhone, "inbound"},
		{contactPhone, "inbound"},
		{contactPhone, "inbound"},
		{"bot", "outbound"},
		{"bot", "outbound"},
		{attendantID, "outbound"},
	}
	for _, r := range rows {
		_, err := testPool.Exec(ctx, `
			INSERT INTO messages (id, conversation_id, channel_id, sender, direction, type, content)
			VALUES ($1, $2, $3, $4, $5, 'text', 'oi')
		`, ulid.Make().String(), convID, channelID, r.sender, r.direction)
		require.NoError(t, err)
	}

	repo := postgres.NewAnalyticsRepository(testPool)
	stats, err := repo.GetMessageStats(ctx, accountID)
	require.NoError(t, err)

	assert.EqualValues(t, 6, stats.Total)
	// The three the customer sent are the customer's — not an attendant's.
	assert.EqualValues(t, 3, stats.Contact,
		"inbound messages are the contact's; counting sender='customer' made this 0")
	assert.EqualValues(t, 2, stats.Bot)
	assert.EqualValues(t, 1, stats.Human,
		"only a real outbound reply from a person counts as human")
	// The three buckets must account for every message, or the screen's
	// percentages describe something other than the total it prints beside them.
	assert.EqualValues(t, stats.Total, stats.Bot+stats.Human+stats.Contact)
}
