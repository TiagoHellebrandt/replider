package services

import (
	"context"
	"sync"
)

// ConversationSerializer serializes work per conversation so two inbound
// messages for the SAME conversation never run their AI pipeline
// concurrently (US-046). All three webhook handlers dispatch ProcessAI on
// detached goroutines, so when a customer fires 5 messages in a row the
// replies could otherwise interleave and arrive out of order, confusing
// the customer. Acquiring the per-conversation lock at the top of ProcessAI
// forces them through one at a time, in arrival order.
//
// Implementation: a buffered(1) channel per conversation acts as a mutex
// whose Lock respects context cancellation (a takeover/disconnect must not
// leave a goroutine parked forever waiting for the lock). A refcount evicts
// the channel from the map once nobody holds or waits on it, so the map
// can't grow unbounded across millions of conversations.
//
// A nil receiver is a no-op so wiring stays optional. Lock returns false
// only when ctx is cancelled before the lock was acquired — the caller must
// then skip its critical section and NOT call the returned release func.
type ConversationSerializer struct {
	mu    sync.Mutex
	locks map[string]*convLock
}

type convLock struct {
	ch       chan struct{} // buffered(1); holding a token == holding the lock
	refcount int           // holders + waiters; entry is evicted at 0
}

func NewConversationSerializer() *ConversationSerializer {
	return &ConversationSerializer{locks: make(map[string]*convLock)}
}

// Lock blocks until it holds the lock for conversationID or ctx is done.
// On success it returns a release func (call it once, via defer) and true.
// On ctx cancellation it returns a no-op func and false — the caller must
// abort without releasing.
func (s *ConversationSerializer) Lock(ctx context.Context, conversationID string) (func(), bool) {
	if s == nil || conversationID == "" {
		return func() {}, true
	}

	s.mu.Lock()
	l, ok := s.locks[conversationID]
	if !ok {
		l = &convLock{ch: make(chan struct{}, 1)}
		s.locks[conversationID] = l
	}
	l.refcount++
	s.mu.Unlock()

	select {
	case l.ch <- struct{}{}:
		// Acquired. Release drains the token and drops our refcount,
		// evicting the entry when this was the last user.
		return func() {
			<-l.ch
			s.release(conversationID)
		}, true
	case <-ctx.Done():
		// Gave up waiting — drop the refcount we took on entry so the
		// entry can still be evicted once the real holder releases.
		s.release(conversationID)
		return func() {}, false
	}
}

// release decrements the refcount and deletes the lock entry when no
// goroutine still holds or waits on it.
func (s *ConversationSerializer) release(conversationID string) {
	s.mu.Lock()
	if l, ok := s.locks[conversationID]; ok {
		l.refcount--
		if l.refcount <= 0 {
			delete(s.locks, conversationID)
		}
	}
	s.mu.Unlock()
}
