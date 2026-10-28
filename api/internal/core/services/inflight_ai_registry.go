package services

import (
	"context"
	"sync"
)

// InflightAIRegistry tracks the cancel funcs of in-flight AI generations
// keyed by conversation ID so an out-of-band event — a human assuming the
// conversation (which pauses the bot) — can cancel a reply that is already
// mid-stream BEFORE it gets persisted and dispatched to the customer
// (US-044). Without this, the bot-pause check at the start of the pipeline
// loses the race against a stream that's already running and the customer
// gets a bot reply on a conversation a human just took over.
//
// Concurrency: keyed by conversation, storing the most-recently registered
// cancel. A monotonic seq guards the cleanup so an older generation's
// deferred unregister can't evict a newer one's entry for the same
// conversation. All methods are safe for concurrent use; a nil receiver is
// a no-op so wiring the registry stays optional.
type InflightAIRegistry struct {
	mu      sync.Mutex
	seq     uint64
	cancels map[string]inflightEntry
}

type inflightEntry struct {
	seq    uint64
	cancel context.CancelFunc
}

func NewInflightAIRegistry() *InflightAIRegistry {
	return &InflightAIRegistry{cancels: make(map[string]inflightEntry)}
}

// Register stores cancel under conversationID and returns a cleanup func
// that removes it — call it via defer once the generation finishes. The
// cleanup only deletes the entry if it's still the one this call
// registered, so a stale defer can't clobber a newer generation.
func (r *InflightAIRegistry) Register(conversationID string, cancel context.CancelFunc) func() {
	if r == nil || conversationID == "" {
		return func() {}
	}
	r.mu.Lock()
	r.seq++
	mySeq := r.seq
	r.cancels[conversationID] = inflightEntry{seq: mySeq, cancel: cancel}
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		if e, ok := r.cancels[conversationID]; ok && e.seq == mySeq {
			delete(r.cancels, conversationID)
		}
		r.mu.Unlock()
	}
}

// Cancel cancels the in-flight generation for conversationID, if any, and
// returns whether one was found. Removing the entry makes the call
// idempotent: a second Cancel (or the generation's own deferred cleanup)
// is a harmless no-op.
func (r *InflightAIRegistry) Cancel(conversationID string) bool {
	if r == nil || conversationID == "" {
		return false
	}
	r.mu.Lock()
	e, ok := r.cancels[conversationID]
	if ok {
		delete(r.cancels, conversationID)
	}
	r.mu.Unlock()
	if ok && e.cancel != nil {
		e.cancel()
		return true
	}
	return false
}
