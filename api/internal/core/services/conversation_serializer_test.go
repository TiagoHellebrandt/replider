package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConversationSerializer_SerializesSameConversation proves that work
// for one conversation never overlaps: 5 goroutines racing on the same
// conversation each run their critical section to completion before the
// next starts. This is the US-046 guarantee that 5 rapid inbounds can't
// interleave their AI replies.
func TestConversationSerializer_SerializesSameConversation(t *testing.T) {
	s := NewConversationSerializer()
	const convID = "conv_1"
	const n = 5

	var inside int32 // count of goroutines currently in the critical section
	var maxConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, ok := s.Lock(context.Background(), convID)
			if !ok {
				t.Error("expected lock acquisition to succeed")
				return
			}
			defer release()

			cur := atomic.AddInt32(&inside, 1)
			for {
				prev := atomic.LoadInt32(&maxConcurrent)
				if cur <= prev || atomic.CompareAndSwapInt32(&maxConcurrent, prev, cur) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond) // hold the lock to expose overlap
			atomic.AddInt32(&inside, -1)
		}()
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("expected serialized access (maxConcurrent=1), got %d", maxConcurrent)
	}
}

// TestConversationSerializer_DifferentConversationsRunConcurrently proves
// that distinct conversations are NOT blocked by each other — the lock is
// per-key, so one slow conversation can't stall every other customer.
func TestConversationSerializer_DifferentConversationsRunConcurrently(t *testing.T) {
	s := NewConversationSerializer()

	releaseA, ok := s.Lock(context.Background(), "conv_A")
	if !ok {
		t.Fatal("conv_A lock should succeed")
	}
	defer releaseA()

	done := make(chan struct{})
	go func() {
		releaseB, ok := s.Lock(context.Background(), "conv_B")
		if ok {
			releaseB()
		}
		close(done)
	}()

	select {
	case <-done:
		// conv_B acquired while conv_A is still held — good.
	case <-time.After(time.Second):
		t.Fatal("conv_B blocked by conv_A: lock is not per-conversation")
	}
}

// TestConversationSerializer_CtxCancelWhileWaiting proves a goroutine
// waiting on a held lock gives up when its context is cancelled instead of
// parking forever (a takeover/disconnect must release the goroutine).
func TestConversationSerializer_CtxCancelWhileWaiting(t *testing.T) {
	s := NewConversationSerializer()
	const convID = "conv_x"

	release, ok := s.Lock(context.Background(), convID)
	if !ok {
		t.Fatal("first lock should succeed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan bool, 1)
	go func() {
		_, ok := s.Lock(ctx, convID)
		result <- ok
	}()

	cancel() // unblock the waiter

	select {
	case ok := <-result:
		if ok {
			t.Fatal("expected lock to fail after ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not return after ctx cancel")
	}

	release()

	// After the holder releases and the cancelled waiter dropped its
	// refcount, the entry must be evicted (no leak).
	s.mu.Lock()
	_, exists := s.locks[convID]
	s.mu.Unlock()
	if exists {
		t.Fatal("lock entry leaked after all users released")
	}
}

// TestConversationSerializer_NilReceiver confirms wiring stays optional —
// a nil serializer is a no-op that always "acquires".
func TestConversationSerializer_NilReceiver(t *testing.T) {
	var s *ConversationSerializer
	release, ok := s.Lock(context.Background(), "conv")
	if !ok {
		t.Fatal("nil serializer should always succeed")
	}
	release() // must not panic
}
