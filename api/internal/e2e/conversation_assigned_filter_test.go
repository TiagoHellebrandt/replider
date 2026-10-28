package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-019 (review fixes): the `assigned` filter must be applied in SQL,
// BEFORE LIMIT/OFFSET. The old handler fetched one page (default 50)
// and filtered in memory — an attendant whose conversations ranked
// past the page limit saw an empty "Minhas" list.
//
// Seeds 80 open conversations: the 50 MOST RECENT are unassigned, the
// 30 assigned to the attendant are older (ranked 51-80 by the default
// last_customer_message_at DESC sort). With post-pagination filtering
// this returns 0 rows; with the SQL filter it returns all 30.
func TestE2E_AssignedFilterAppliedBeforePagination(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	ctx := context.Background()
	var accountID, userID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT id, account_id FROM users WHERE email = $1", "admin@test.com").
		Scan(&userID, &accountID))

	w := doRequest("POST", "/v1/channels", map[string]interface{}{
		"type":   "telegram",
		"name":   "TG",
		"config": map[string]string{"bot_token": "fake_token"},
	}, token)
	require.Equal(t, 201, w.Code, "channel create failed: %s", w.Body.String())
	var channel map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &channel))
	channelID := channel["id"].(string)

	now := time.Now()
	seedConv := func(i int, assignedTo *string, lastMsgAt time.Time) {
		_, err := testPool.Exec(ctx, `
			INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_name, status, assigned_to, last_customer_message_at)
			VALUES ($1, $2, $3, $4, $5, 'open', $6, $7)
		`, ulid.Make().String(), accountID, channelID,
			fmt.Sprintf("contact-%03d", i), fmt.Sprintf("Contact %03d", i),
			assignedTo, lastMsgAt)
		require.NoError(t, err)
	}

	// 50 unassigned, most recent activity → they fill the whole first page.
	for i := 0; i < 50; i++ {
		seedConv(i, nil, now.Add(-time.Duration(i)*time.Minute))
	}
	// 30 assigned to the attendant, older → ranked 51-80.
	for i := 50; i < 80; i++ {
		seedConv(i, &userID, now.Add(-48*time.Hour).Add(-time.Duration(i)*time.Minute))
	}

	// "Minhas": all 30 assigned conversations must come back even though
	// every one of them ranks past the default page of 50.
	w = doRequest("GET", "/v1/conversations?assigned="+userID, nil, token)
	require.Equal(t, 200, w.Code, "list failed: %s", w.Body.String())
	mine := parseJSONArray(t, w)
	assert.Len(t, mine, 30, "assigned filter must run in SQL before LIMIT/OFFSET")
	for _, raw := range mine {
		conv := raw.(map[string]interface{})
		assert.Equal(t, userID, conv["assigned_to"], "row not assigned to the attendant leaked into the filter")
	}

	// Filtered pagination: limit/offset walk the FILTERED set.
	w = doRequest("GET", "/v1/conversations?assigned="+userID+"&limit=20", nil, token)
	require.Equal(t, 200, w.Code)
	assert.Len(t, parseJSONArray(t, w), 20)

	w = doRequest("GET", "/v1/conversations?assigned="+userID+"&limit=20&offset=20", nil, token)
	require.Equal(t, 200, w.Code)
	assert.Len(t, parseJSONArray(t, w), 10)

	// No filter: default page still returns the 50 most recent (unassigned).
	w = doRequest("GET", "/v1/conversations", nil, token)
	require.Equal(t, 200, w.Code)
	all := parseJSONArray(t, w)
	assert.Len(t, all, 50)
	for _, raw := range all {
		conv := raw.(map[string]interface{})
		assert.Nil(t, conv["assigned_to"], "unfiltered first page should be the unassigned recent rows")
	}

	// The filter composes with the non-default sorts too (each sort
	// variant is its own SQL query — all three must carry the WHERE).
	for _, sort := range []string{"first_in_conv", "oldest"} {
		w = doRequest("GET", "/v1/conversations?assigned="+userID+"&sort="+sort, nil, token)
		require.Equal(t, 200, w.Code, "sort=%s failed: %s", sort, w.Body.String())
		assert.Len(t, parseJSONArray(t, w), 30, "sort=%s must apply the assigned filter in SQL", sort)
	}

	// US-020: the "Não atribuídas" bucket — assigned=none returns only
	// rows with no assignee, paginated in SQL (the pre-US-020 whitelist
	// rejected the sentinel with 400).
	w = doRequest("GET", "/v1/conversations?assigned=none&limit=200", nil, token)
	require.Equal(t, 200, w.Code, "assigned=none failed: %s", w.Body.String())
	unassigned := parseJSONArray(t, w)
	assert.Len(t, unassigned, 50, "assigned=none must return exactly the unassigned rows")
	for _, raw := range unassigned {
		conv := raw.(map[string]interface{})
		assert.Nil(t, conv["assigned_to"], "assigned row leaked into the unassigned bucket")
	}

	for _, sort := range []string{"first_in_conv", "oldest"} {
		w = doRequest("GET", "/v1/conversations?assigned=none&limit=200&sort="+sort, nil, token)
		require.Equal(t, 200, w.Code, "assigned=none sort=%s failed: %s", sort, w.Body.String())
		assert.Len(t, parseJSONArray(t, w), 50, "sort=%s must apply the unassigned filter in SQL", sort)
	}
}
