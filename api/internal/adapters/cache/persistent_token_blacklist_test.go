package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRevokedStore is an in-memory domain.RevokedTokenStore for tests —
// no database required. It records the jti keys it was asked about so we
// can assert the durable path was (or was not) consulted.
type fakeRevokedStore struct {
	entries    map[string]time.Time // jti -> expiresAt
	addErr     error
	existErr   error
	existCalls int
}

func newFakeRevokedStore() *fakeRevokedStore {
	return &fakeRevokedStore{entries: map[string]time.Time{}}
}

func (f *fakeRevokedStore) Add(_ context.Context, jti string, expiresAt time.Time) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.entries[jti] = expiresAt
	return nil
}

func (f *fakeRevokedStore) Exists(_ context.Context, jti string) (bool, error) {
	f.existCalls++
	if f.existErr != nil {
		return false, f.existErr
	}
	exp, ok := f.entries[jti]
	if !ok {
		return false, nil
	}
	return exp.After(time.Now()), nil
}

func (f *fakeRevokedStore) PruneExpired(_ context.Context, now time.Time) (int64, error) {
	var n int64
	for k, exp := range f.entries {
		if !exp.After(now) {
			delete(f.entries, k)
			n++
		}
	}
	return n, nil
}

// With no Redis cache wired (the scenario this story fixes — Redis down or
// not configured), a revoked token must still be rejected via the durable
// store, and a never-revoked token must pass.
func TestPersistentBlacklist_DurableOnly_RevokedRejected(t *testing.T) {
	store := newFakeRevokedStore()
	bl := NewPersistentTokenBlacklist(store, nil)
	ctx := context.Background()

	const token = "header.payload.signature"

	if bl.IsRevoked(ctx, token) {
		t.Fatal("token should not be revoked before logout")
	}

	if err := bl.Revoke(ctx, token, time.Hour); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if !bl.IsRevoked(ctx, token) {
		t.Fatal("revoked token must be rejected via the durable store")
	}

	// A different token is unaffected.
	if bl.IsRevoked(ctx, "other.token.value") {
		t.Fatal("unrelated token must not be revoked")
	}

	// The raw bearer token is never stored — only its hash.
	if _, leaked := store.entries[token]; leaked {
		t.Fatal("raw token must not be persisted; expected a hashed jti")
	}
}

// An entry whose expiry has passed must not cause a rejection (mirrors a
// cache TTL expiry).
func TestPersistentBlacklist_ExpiredEntryNotRejected(t *testing.T) {
	store := newFakeRevokedStore()
	bl := NewPersistentTokenBlacklist(store, nil)
	ctx := context.Background()

	const token = "tok"
	// Revoke with a negative TTL so the durable entry is already expired.
	if err := bl.Revoke(ctx, token, -time.Minute); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if bl.IsRevoked(ctx, token) {
		t.Fatal("an aged-out durable entry must not reject the token")
	}
}

// If the durable write fails, Revoke must surface the error (the token may
// still be usable, the caller must know).
func TestPersistentBlacklist_RevokeDurableErrorPropagates(t *testing.T) {
	store := newFakeRevokedStore()
	store.addErr = errors.New("db down")
	bl := NewPersistentTokenBlacklist(store, nil)

	if err := bl.Revoke(context.Background(), "tok", time.Hour); err == nil {
		t.Fatal("expected Revoke to propagate the durable store error")
	}
}

// A durable read error fails open (don't lock everyone out on a DB hiccup).
func TestPersistentBlacklist_DurableReadErrorFailsOpen(t *testing.T) {
	store := newFakeRevokedStore()
	store.existErr = errors.New("db timeout")
	bl := NewPersistentTokenBlacklist(store, nil)

	if bl.IsRevoked(context.Background(), "tok") {
		t.Fatal("a durable read error must fail open (allow the request)")
	}
}

// Hashing must be deterministic and distinct per input so the jti key is
// stable across Revoke/IsRevoked and never collides trivially.
func TestRevocationKey_StableAndDistinct(t *testing.T) {
	a1 := revocationKey("token-a")
	a2 := revocationKey("token-a")
	b := revocationKey("token-b")

	if a1 != a2 {
		t.Fatal("revocationKey must be deterministic")
	}
	if a1 == b {
		t.Fatal("distinct tokens must hash to distinct keys")
	}
	if a1 == "token-a" {
		t.Fatal("revocationKey must hash, not echo the raw token")
	}
}
