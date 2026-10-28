package e2e

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// US-021 (review fixes): the round-robin cursor is persisted in
// auto_assign_rotation — it must survive a process restart (new use
// case instance) and never advance on a failed assignment. The old
// in-memory counters map reset to the lowest-ULID attendant on every
// deploy and skipped a turn when the conversation UPDATE failed.

type rotationFixture struct {
	accountID  string
	channelID  string
	attendants []string // sorted ascending (ULID order — the rotation order)
}

func seedRotationFixture(t *testing.T) rotationFixture {
	t.Helper()
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")

	ctx := context.Background()
	var accountID, adminID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT id, account_id FROM users WHERE email = $1", "admin@test.com").
		Scan(&adminID, &accountID))

	attendants := []string{adminID}
	for i := 0; i < 2; i++ {
		id := ulid.Make().String()
		_, err := testPool.Exec(ctx, `
			INSERT INTO users (id, account_id, name, email, password_hash, role, is_email_verified)
			VALUES ($1, $2, $3, $4, 'x', 'attendant', true)
		`, id, accountID, fmt.Sprintf("Attendant %d", i), fmt.Sprintf("attendant%d@test.com", i))
		require.NoError(t, err)
		attendants = append(attendants, id)
	}
	sort.Strings(attendants)

	channelID := ulid.Make().String()
	_, err := testPool.Exec(ctx, `
		INSERT INTO channels (id, account_id, provider, name, credentials, webhook_token)
		VALUES ($1, $2, 'telegram', 'TG', '{}', $3)
	`, channelID, accountID, channelID)
	require.NoError(t, err)

	return rotationFixture{accountID: accountID, channelID: channelID, attendants: attendants}
}

func (f rotationFixture) seedConversation(t *testing.T, i int) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_name, status)
		VALUES ($1, $2, $3, $4, $5, 'open')
	`, id, f.accountID, f.channelID, fmt.Sprintf("contact-%03d", i), fmt.Sprintf("Contact %03d", i))
	require.NoError(t, err)
	return id
}

func (f rotationFixture) newUseCase() *usecases.AutoAssignUseCase {
	uc := usecases.NewAutoAssignUseCase(
		postgres.NewConversationRepository(testPool),
		postgres.NewAutoAssignRotationRepository(testPool),
	)
	uc.SetSettingsRepo(postgres.NewAutoAssignSettingsRepository(testPool))
	uc.SetPlatformAccountID(f.accountID)
	return uc
}

func (f rotationFixture) assignedTo(t *testing.T, conversationID string) string {
	t.Helper()
	var assignee *string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT assigned_to FROM conversations WHERE id = $1", conversationID).Scan(&assignee))
	require.NotNil(t, assignee, "conversation %s should be assigned", conversationID)
	return *assignee
}

// A fresh use case instance over the same DB (= a restarted pod)
// continues the rotation where the previous one stopped. FAILS on the
// pre-US-021 in-memory counter (the "restarted" instance handed
// conv-3 back to attendants[0]).
func TestE2E_AutoAssignRotation_SurvivesRestart(t *testing.T) {
	f := seedRotationFixture(t)
	ctx := context.Background()

	uc1 := f.newUseCase()
	conv1, conv2 := f.seedConversation(t, 1), f.seedConversation(t, 2)

	a, err := uc1.Assign(ctx, conv1, f.accountID)
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, f.attendants[0], *a)

	b, err := uc1.Assign(ctx, conv2, f.accountID)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, f.attendants[1], *b)

	// "Deploy": brand-new use case, same database.
	uc2 := f.newUseCase()
	conv3 := f.seedConversation(t, 3)
	c, err := uc2.Assign(ctx, conv3, f.accountID)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, f.attendants[2], *c,
		"restart must continue the persisted rotation, not reset to the lowest ULID")

	// Wrap-around still works across the restart boundary.
	conv4 := f.seedConversation(t, 4)
	d, err := uc2.Assign(ctx, conv4, f.accountID)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, f.attendants[0], *d)

	// The DB rows agree with the returned assignees.
	assert.Equal(t, f.attendants[0], f.assignedTo(t, conv1))
	assert.Equal(t, f.attendants[1], f.assignedTo(t, conv2))
	assert.Equal(t, f.attendants[2], f.assignedTo(t, conv3))
}

// A failed assignment (missing conversation) must not advance the
// cursor: the attendant whose turn it was still gets the next real
// conversation. The claim+assign statement is atomic, so there is no
// state where the cursor moved but no conversation was assigned.
func TestE2E_AutoAssignRotation_ErrorDoesNotAdvanceCursor(t *testing.T) {
	f := seedRotationFixture(t)
	ctx := context.Background()
	uc := f.newUseCase()

	conv1 := f.seedConversation(t, 1)
	a, err := uc.Assign(ctx, conv1, f.accountID)
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, f.attendants[0], *a)

	// Nonexistent conversation → error, no cursor advance.
	_, err = uc.Assign(ctx, ulid.Make().String(), f.accountID)
	require.ErrorIs(t, err, domain.ErrConversationNotFound)

	var cursor string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT last_assigned_user_id FROM auto_assign_rotation WHERE account_id = $1",
		f.accountID).Scan(&cursor))
	assert.Equal(t, f.attendants[0], cursor, "failed assignment must not advance the persisted cursor")

	conv2 := f.seedConversation(t, 2)
	b, err := uc.Assign(ctx, conv2, f.accountID)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, f.attendants[1], *b, "the failed turn must not be skipped")
}

// A stale cursor (the last-assigned user left the rotation pool)
// degrades to "next in ULID order" instead of erroring or resetting.
func TestE2E_AutoAssignRotation_StaleCursorDegrades(t *testing.T) {
	f := seedRotationFixture(t)
	ctx := context.Background()
	uc := f.newUseCase()

	conv1 := f.seedConversation(t, 1)
	a, err := uc.Assign(ctx, conv1, f.accountID)
	require.NoError(t, err)
	assert.Equal(t, f.attendants[0], *a)

	// The cursor's user leaves the pool.
	_, err = testPool.Exec(ctx,
		"UPDATE users SET is_active = false WHERE id = $1", f.attendants[1])
	require.NoError(t, err)

	conv2 := f.seedConversation(t, 2)
	b, err := uc.Assign(ctx, conv2, f.accountID)
	require.NoError(t, err)
	assert.Equal(t, f.attendants[2], *b, "rotation must skip the removed user and keep ULID order")

	conv3 := f.seedConversation(t, 3)
	c, err := uc.Assign(ctx, conv3, f.accountID)
	require.NoError(t, err)
	assert.Equal(t, f.attendants[0], *c)
}
