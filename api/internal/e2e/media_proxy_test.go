package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The image a customer sent, reaching the screen.
//
// It did not, and the reason was invisible from the server: the transcript
// handed back a GCS V4 signed URL, which is correct, secure, and unusable by
// the dashboard — Flutter Web decodes an image by FETCHING it, and a GCS bucket
// answers with no CORS headers. The same URL opened fine in a browser tab,
// which is what made it look like the message had no image at all.
//
// Media now streams from the API, on an origin whose CORS policy the product
// owns. These pin the two halves: the transcript points at the proxy, and the
// proxy refuses anything it did not mint.

// seedMediaMessage writes a message carrying a GCS OBJECT PATH — the shape the
// inbound pipeline actually persists after re-hosting, not a URL.
func seedMediaMessage(t *testing.T, accountID, channelID, convID, objectPath string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, media_url, direction, created_at)
		 VALUES ($1, $2, $3, '5565988880001', 'image', '', $4, 'inbound', NOW())`,
		id, channelID, convID, objectPath)
	require.NoError(t, err)
	return id
}

func seedMediaConversation(t *testing.T, accountID string) (channelID, convID string) {
	t.Helper()
	channelID = ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO channels (id, account_id, name, provider, credentials, is_active, webhook_token, created_at, updated_at)
		 VALUES ($1, $2, 'WhatsApp', 'whatsapp', '{}', true, $3, NOW(), NOW())`,
		channelID, accountID, ulid.Make().String())
	require.NoError(t, err)

	convID = ulid.Make().String()
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, '5565988880001', 'open', NOW(), NOW())`,
		convID, accountID, channelID)
	require.NoError(t, err)
	return channelID, convID
}

func TestMediaProxy_TheTranscriptPointsAtTheAPINotAtGoogle(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	channelID, convID := seedMediaConversation(t, accountID)
	objectPath := "accounts/" + accountID + "/inbound_media/" + ulid.Make().String() + ".jpg"
	seedMediaMessage(t, accountID, channelID, convID, objectPath)

	rec := doRequest("GET", "/v1/conversations/"+convID+"/messages", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var messages []struct {
		Type     string  `json:"type"`
		MediaURL *string `json:"media_url"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &messages))
	require.Len(t, messages, 1)
	require.NotNil(t, messages[0].MediaURL)

	got := *messages[0].MediaURL
	assert.Contains(t, got, "/v1/media/file",
		"the browser has to be able to fetch this — a bucket URL cannot be fetched cross-origin")
	assert.NotContains(t, got, "storage.googleapis.com")
	// The raw object path must not be what reaches the client either: that was
	// the other failure shape, an <img> pointed at "accounts/…/x.jpg".
	assert.NotEqual(t, objectPath, got)

	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, accountID, u.Query().Get("a"))
	assert.Equal(t, objectPath, u.Query().Get("p"))
	assert.NotEmpty(t, u.Query().Get("e"))
	assert.NotEmpty(t, u.Query().Get("s"))
}

// The proxy carries no session — an <img> cannot send an Authorization header —
// so the signature has to be the whole of the authorisation.
func TestMediaProxy_SignatureIsTheAuth(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	channelID, convID := seedMediaConversation(t, accountID)
	objectPath := "accounts/" + accountID + "/inbound_media/" + ulid.Make().String() + ".jpg"
	seedMediaMessage(t, accountID, channelID, convID, objectPath)

	rec := doRequest("GET", "/v1/conversations/"+convID+"/messages", nil, token)
	require.Equal(t, http.StatusOK, rec.Code)
	var messages []struct {
		MediaURL *string `json:"media_url"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &messages))
	require.Len(t, messages, 1)
	signed, err := url.Parse(*messages[0].MediaURL)
	require.NoError(t, err)
	q := signed.Query()

	// A minted URL, with NO bearer token: it must get past authorisation.
	// GCS is unconfigured in e2e, so it stops at the storage call — 503, which
	// is precisely "the signature was accepted and the bucket was not there".
	ok := doRequest("GET", "/v1/media/file?"+q.Encode(), nil, "")
	assert.Equal(t, http.StatusServiceUnavailable, ok.Code,
		"a valid signature must reach the storage call; got %s", ok.Body.String())

	// Same URL, one byte of the signature changed.
	tampered := cloneQuery(q)
	tampered.Set("s", flipLastHexDigit(q.Get("s")))
	assert.Equal(t, http.StatusForbidden,
		doRequest("GET", "/v1/media/file?"+tampered.Encode(), nil, "").Code)

	// Same signature, pointed at another account's object.
	other := cloneQuery(q)
	other.Set("p", "accounts/"+ulid.Make().String()+"/inbound_media/x.jpg")
	assert.Equal(t, http.StatusBadRequest,
		doRequest("GET", "/v1/media/file?"+other.Encode(), nil, "").Code,
		"a path outside the signed account is refused on shape, before the MAC")

	// Traversal out of the account's tree.
	traversal := cloneQuery(q)
	traversal.Set("p", "accounts/"+accountID+"/../"+ulid.Make().String()+"/x.jpg")
	assert.Equal(t, http.StatusBadRequest,
		doRequest("GET", "/v1/media/file?"+traversal.Encode(), nil, "").Code)

	// Nothing at all.
	assert.Equal(t, http.StatusBadRequest,
		doRequest("GET", "/v1/media/file", nil, "").Code)
}

// An external CDN URL is not ours to proxy and must survive untouched —
// Instagram media and any pre-rehost row still live there.
func TestMediaProxy_AnExternalURLIsLeftAlone(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	channelID, convID := seedMediaConversation(t, accountID)
	const external = "https://lookaside.fbsbx.com/whatsapp/media/abc123"
	seedMediaMessage(t, accountID, channelID, convID, external)

	rec := doRequest("GET", "/v1/conversations/"+convID+"/messages", nil, token)
	require.Equal(t, http.StatusOK, rec.Code)

	var messages []struct {
		MediaURL *string `json:"media_url"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &messages))
	require.Len(t, messages, 1)
	require.NotNil(t, messages[0].MediaURL)
	assert.Equal(t, external, *messages[0].MediaURL)
}

func cloneQuery(in url.Values) url.Values {
	out := url.Values{}
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// flipLastHexDigit changes exactly one character, so the test proves the MAC is
// checked rather than that a malformed string was rejected.
func flipLastHexDigit(sig string) string {
	if sig == "" {
		return "0"
	}
	last := sig[len(sig)-1:]
	repl := "a"
	if strings.EqualFold(last, "a") {
		repl = "b"
	}
	return sig[:len(sig)-1] + repl
}
