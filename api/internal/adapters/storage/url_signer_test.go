package storage

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The signature is the only thing standing between a media URL and anybody who
// wants to read another account's attachments, so these pin the ways it must
// refuse rather than the way it succeeds.

func signer() *URLSigner {
	return &URLSigner{APIBaseURL: "https://api.replider.com", Secret: []byte("s3cr3t")}
}

func parseSigned(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "/v1/media/file", u.Path)
	return u.Query()
}

func TestSignObjectURL_RoundTrips(t *testing.T) {
	s := signer()
	raw := s.SignObjectURL("acc-1", "accounts/acc-1/inbound_media/01H.jpg", time.Hour)

	q := parseSigned(t, raw)
	assert.True(t, s.Verify(q.Get("a"), q.Get("p"), q.Get("e"), q.Get("s")))
	// The object path travels in the clear — it is not a secret, and hiding it
	// would only make the proxy's logs useless.
	assert.Equal(t, "accounts/acc-1/inbound_media/01H.jpg", q.Get("p"))
}

// The reason the account id is INSIDE the signed message rather than merely
// travelling beside it.
func TestVerify_RefusesTheSameObjectUnderAnotherAccount(t *testing.T) {
	s := signer()
	q := parseSigned(t, s.SignObjectURL("acc-1", "accounts/acc-1/inbound_media/01H.jpg", time.Hour))

	assert.False(t, s.Verify("acc-2", q.Get("p"), q.Get("e"), q.Get("s")),
		"replaying a signature under a different account must not verify")
}

func TestVerify_RefusesATamperedPath(t *testing.T) {
	s := signer()
	q := parseSigned(t, s.SignObjectURL("acc-1", "accounts/acc-1/inbound_media/01H.jpg", time.Hour))

	assert.False(t, s.Verify("acc-1", "accounts/acc-1/inbound_media/OTHER.jpg",
		q.Get("e"), q.Get("s")))
}

// Pushing the expiry out is the cheapest attack on a scheme like this, and it
// fails because exp is signed too.
func TestVerify_RefusesAnExtendedExpiry(t *testing.T) {
	s := signer()
	q := parseSigned(t, s.SignObjectURL("acc-1", "accounts/acc-1/x.jpg", time.Hour))

	// A far-future exp, still carrying the ORIGINAL signature: the attacker
	// controls the query string but not the MAC over it.
	later := strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10)
	assert.False(t, s.Verify("acc-1", q.Get("p"), later, q.Get("s")))
	// And the same URL with its own exp untouched still works, so the
	// assertion above failed for the tampering and not for something else.
	assert.True(t, s.Verify("acc-1", q.Get("p"), q.Get("e"), q.Get("s")))
}

func TestVerify_RefusesAnExpiredURL(t *testing.T) {
	s := signer()
	// Signed already-expired: the TTL is negative, so exp is in the past.
	q := parseSigned(t, s.SignObjectURL("acc-1", "accounts/acc-1/x.jpg", -time.Minute))

	assert.False(t, s.Verify("acc-1", q.Get("p"), q.Get("e"), q.Get("s")),
		"an expired signature is still a valid signature — the clock is the other half")
}

func TestVerify_RefusesGarbage(t *testing.T) {
	s := signer()
	assert.False(t, s.Verify("acc-1", "accounts/acc-1/x.jpg", "not-a-number", "deadbeef"))
	assert.False(t, s.Verify("", "", "", ""))
}

// A different secret must not verify — the guard that makes rotating the JWT
// secret invalidate every outstanding media URL.
func TestVerify_RefusesAnotherSecret(t *testing.T) {
	q := parseSigned(t, signer().SignObjectURL("acc-1", "accounts/acc-1/x.jpg", time.Hour))
	other := &URLSigner{APIBaseURL: "https://api.replider.com", Secret: []byte("different")}

	assert.False(t, other.Verify("acc-1", q.Get("p"), q.Get("e"), q.Get("s")))
}

func TestObjectBelongsToAccount(t *testing.T) {
	// The shapes every writer actually produces.
	assert.True(t, ObjectBelongsToAccount("acc-1", "accounts/acc-1/inbound_media/01H.jpg"))
	assert.True(t, ObjectBelongsToAccount("acc-1", "accounts/acc-1/attachments/msg-1/att-1"))
	assert.True(t, ObjectBelongsToAccount("acc-1", "accounts/acc-1/1720000000/foto.png"))

	// Another tenant's tree.
	assert.False(t, ObjectBelongsToAccount("acc-1", "accounts/acc-2/inbound_media/01H.jpg"))
	// A prefix that merely STARTS with the id — "acc-1" must not open "acc-10".
	assert.False(t, ObjectBelongsToAccount("acc-1", "accounts/acc-10/inbound_media/01H.jpg"))
	// Traversal, absolute, and outside the media tree entirely.
	assert.False(t, ObjectBelongsToAccount("acc-1", "accounts/acc-1/../acc-2/x.jpg"))
	assert.False(t, ObjectBelongsToAccount("acc-1", "/accounts/acc-1/x.jpg"))
	assert.False(t, ObjectBelongsToAccount("acc-1", "contracts/acc-1/x.pdf"))
	assert.False(t, ObjectBelongsToAccount("", "accounts//x.jpg"))
	assert.False(t, ObjectBelongsToAccount("acc-1", ""))
}
