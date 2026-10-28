package handlers

import (
	"sync"
	"time"
)

// metaResponseCache is an in-process TTL cache for read-only Meta Graph
// proxy endpoints (whatsapp profile, phone-numbers, webhook-status).
// These calls hit Meta synchronously and cost 400-850ms each; the
// dashboard fires all three in parallel on every Settings page render
// and again on tab focus, which dominates account latency.
//
// Per-channel scoped (key = "<endpoint>:<channel_id>") so a write to
// one channel never serves a stale read for another. Writes call
// metaCacheInvalidate so the next GET re-fetches from Meta.
//
// In-memory is intentional: the platform is single-instance today
// (one API container behind nginx) so per-process state is consistent.
// Move to Redis if/when we run >1 replica.
type metaResponseCache struct {
	mu      sync.RWMutex
	entries map[string]metaCacheEntry
}

type metaCacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

const metaCacheTTL = 30 * time.Second

var metaCache = &metaResponseCache{entries: make(map[string]metaCacheEntry)}

func metaCacheGet(key string) (interface{}, bool) {
	metaCache.mu.RLock()
	e, ok := metaCache.entries[key]
	metaCache.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

func metaCacheSet(key string, value interface{}) {
	metaCache.mu.Lock()
	metaCache.entries[key] = metaCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(metaCacheTTL),
	}
	metaCache.mu.Unlock()
}

// metaCacheInvalidate drops every entry tied to a channel. Called from
// every write handler that mutates Meta-side state (register, verify,
// deregister, subscribe, profile update) so the next read reflects
// the new truth instead of returning the previous 30s window.
func metaCacheInvalidate(channelID string) {
	suffix := ":" + channelID
	metaCache.mu.Lock()
	for k := range metaCache.entries {
		if len(k) > len(suffix) && k[len(k)-len(suffix):] == suffix {
			delete(metaCache.entries, k)
		}
	}
	metaCache.mu.Unlock()
}

// metaCacheResetForTesting clears all entries. Tests reuse channel IDs
// (ch-1) across cases, so without resetting between tests a cached
// success from one test pollutes the next test that expects a fresh
// upstream call.
func metaCacheResetForTesting() {
	metaCache.mu.Lock()
	metaCache.entries = make(map[string]metaCacheEntry)
	metaCache.mu.Unlock()
}
