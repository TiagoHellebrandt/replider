package domainage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeRDAP(t *testing.T, status int, body string) (*RDAPLookup, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	l := New(true, 2*time.Second).(*RDAPLookup)
	l.baseURL = srv.URL + "/domain/"
	return l, &calls
}

func TestRegisteredAt_ReadsTheRegistrationEvent(t *testing.T) {
	l, _ := fakeRDAP(t, http.StatusOK, `{"events":[
	  {"eventAction":"last changed","eventDate":"2026-01-02T00:00:00Z"},
	  {"eventAction":"registration","eventDate":"2019-03-14T10:00:00Z"}
	]}`)

	at, err := l.RegisteredAt(context.Background(), "medline.com.br")

	require.NoError(t, err)
	require.NotNil(t, at)
	assert.Equal(t, 2019, at.Year())
}

// A redacted date is not a young domain. It must come back as "no answer",
// never as something the risk layer can flag.
func TestRegisteredAt_RedactedDateIsNoAnswer(t *testing.T) {
	l, _ := fakeRDAP(t, http.StatusOK, `{"events":[{"eventAction":"last changed","eventDate":"2026-01-02T00:00:00Z"}]}`)

	at, err := l.RegisteredAt(context.Background(), "privado.com.br")

	require.NoError(t, err)
	assert.Nil(t, at)
}

// A TLD the service cannot route is not an error worth surfacing.
func TestRegisteredAt_NotFoundIsNotAnError(t *testing.T) {
	l, _ := fakeRDAP(t, http.StatusNotFound, `{}`)

	at, err := l.RegisteredAt(context.Background(), "algo.exotico")

	require.NoError(t, err)
	assert.Nil(t, at)
}

func TestRegisteredAt_ServerErrorIsAnError(t *testing.T) {
	l, _ := fakeRDAP(t, http.StatusInternalServerError, ``)

	_, err := l.RegisteredAt(context.Background(), "medline.com.br")

	assert.Error(t, err, "a failed lookup must not be mistaken for 'no answer'")
}

// Registries rate-limit hard and prospecting hits the same domains over and
// over. A known date never changes, so it is cached for the process.
func TestRegisteredAt_CachesAKnownDate(t *testing.T) {
	l, calls := fakeRDAP(t, http.StatusOK,
		`{"events":[{"eventAction":"registration","eventDate":"2019-03-14T10:00:00Z"}]}`)

	for i := 0; i < 3; i++ {
		_, err := l.RegisteredAt(context.Background(), "medline.com.br")
		require.NoError(t, err)
	}

	assert.Equal(t, 1, *calls)
}

// Anything that could turn the URL into a different request is refused
// before it is built.
func TestRegisteredAt_RefusesDomainsThatAreNotDomains(t *testing.T) {
	l, calls := fakeRDAP(t, http.StatusOK, `{}`)

	for _, bad := range []string{"", "   ", "evil.com/path", "a b.com", "x@y.com", "a?b.com"} {
		at, err := l.RegisteredAt(context.Background(), bad)
		require.NoError(t, err, "input %q", bad)
		assert.Nil(t, at)
	}
	assert.Zero(t, *calls, "none of those may reach the network")
}

// Disabled means nil, not a typed nil that satisfies the interface and then
// fails on every call.
func TestNew_DisabledReturnsNilInterface(t *testing.T) {
	assert.Nil(t, New(false, time.Second))
}
