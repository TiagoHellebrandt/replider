package handlers

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:       hub,
		Send:      make(chan []byte, 256),
		AccountID: "acc-1",
		UserID:    "user-1",
	}

	hub.Register <- client

	// Give the goroutine time to process
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.Clients[client]
	hub.mu.RUnlock()

	assert.True(t, exists, "client should be registered in hub")
}

func TestHub_UnregisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:       hub,
		Send:      make(chan []byte, 256),
		AccountID: "acc-1",
		UserID:    "user-1",
	}

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.Clients[client]
	hub.mu.RUnlock()

	assert.False(t, exists, "client should be removed from hub")
}

func TestHub_UnregisterClient_ClosesSendChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:       hub,
		Send:      make(chan []byte, 256),
		AccountID: "acc-1",
		UserID:    "user-1",
	}

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	// Verify Send channel is closed
	_, ok := <-client.Send
	assert.False(t, ok, "Send channel should be closed after unregister")
}

func TestHub_UnregisterClient_NotRegistered(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:       hub,
		Send:      make(chan []byte, 256),
		AccountID: "acc-1",
		UserID:    "user-1",
	}

	// Unregister a client that was never registered — should not panic
	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	assert.Len(t, hub.Clients, 0)
	hub.mu.RUnlock()
}

func TestHub_BroadcastToAccount_SendsToCorrectClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-1", UserID: "user-1"}
	client2 := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-1", UserID: "user-2"}
	client3 := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-2", UserID: "user-3"}

	hub.Register <- client1
	hub.Register <- client2
	hub.Register <- client3
	time.Sleep(10 * time.Millisecond)

	payload := []byte(`{"event":"new_message","conversation_id":"conv-1"}`)
	hub.BroadcastToAccount("acc-1", payload)

	// client1 and client2 should receive the message
	select {
	case msg := <-client1.Send:
		assert.Equal(t, payload, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client1 should have received broadcast")
	}

	select {
	case msg := <-client2.Send:
		assert.Equal(t, payload, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client2 should have received broadcast")
	}

	// client3 (different account) should NOT receive the message
	select {
	case <-client3.Send:
		t.Fatal("client3 should not have received broadcast for acc-1")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_BroadcastToAccount_SkipsFullBuffer(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a client with buffer of 1
	client := &Client{Hub: hub, Send: make(chan []byte, 1), AccountID: "acc-1", UserID: "user-1"}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	client.Send <- []byte("first")

	// Broadcast should not block even though buffer is full
	done := make(chan struct{})
	go func() {
		hub.BroadcastToAccount("acc-1", []byte("second"))
		close(done)
	}()

	select {
	case <-done:
		// expected — BroadcastToAccount should not block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("BroadcastToAccount should not block when client buffer is full")
	}
}

func TestHub_BroadcastToAccount_NoClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Should not panic when broadcasting to account with no clients
	hub.BroadcastToAccount("acc-nonexistent", []byte(`{"event":"test"}`))
}

// fakePublisher records cross-pod publishes and can simulate transport errors.
type fakePublisher struct {
	mu    sync.Mutex
	calls []struct {
		accountID string
		payload   []byte
	}
	err error
}

func (p *fakePublisher) PublishBroadcast(accountID string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, struct {
		accountID string
		payload   []byte
	}{accountID, payload})
	return p.err
}

func (p *fakePublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// US-196: BroadcastToAccount must mirror to the cross-pod publisher AND deliver
// to local clients.
func TestHub_BroadcastToAccount_MirrorsToPublisher(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	pub := &fakePublisher{}
	hub.SetPublisher(pub)

	client := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-1", UserID: "user-1"}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	payload := []byte(`{"event":"new_message"}`)
	hub.BroadcastToAccount("acc-1", payload)

	// Local delivery still happens.
	select {
	case msg := <-client.Send:
		assert.Equal(t, payload, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("local client should still receive broadcast")
	}

	// And it was mirrored to peers exactly once.
	assert.Equal(t, 1, pub.count())
	assert.Equal(t, "acc-1", pub.calls[0].accountID)
	assert.Equal(t, payload, pub.calls[0].payload)
}

// US-196: a publish failure must not stop local delivery (degrade to single-pod).
func TestHub_BroadcastToAccount_PublishErrorStillDeliversLocal(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	hub.SetPublisher(&fakePublisher{err: assert.AnError})

	client := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-1", UserID: "user-1"}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	payload := []byte(`{"event":"x"}`)
	hub.BroadcastToAccount("acc-1", payload)

	select {
	case msg := <-client.Send:
		assert.Equal(t, payload, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("local delivery must survive a cross-pod publish error")
	}
}

// US-196: DeliverLocal is the sink for peer broadcasts — it must reach local
// clients but NOT re-publish (which would loop forever across pods).
func TestHub_DeliverLocal_DoesNotRepublish(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	pub := &fakePublisher{}
	hub.SetPublisher(pub)

	client := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-1", UserID: "user-1"}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	payload := []byte(`{"event":"from-peer"}`)
	hub.DeliverLocal("acc-1", payload)

	select {
	case msg := <-client.Send:
		assert.Equal(t, payload, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DeliverLocal should reach local clients")
	}
	assert.Equal(t, 0, pub.count(), "DeliverLocal must not re-publish to peers")
}

func TestHub_MultipleAccountsBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-1", UserID: "user-1"}
	client2 := &Client{Hub: hub, Send: make(chan []byte, 256), AccountID: "acc-2", UserID: "user-2"}

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(10 * time.Millisecond)

	payload1 := []byte(`{"event":"msg","for":"acc-1"}`)
	payload2 := []byte(`{"event":"msg","for":"acc-2"}`)

	hub.BroadcastToAccount("acc-1", payload1)
	hub.BroadcastToAccount("acc-2", payload2)

	msg1 := <-client1.Send
	msg2 := <-client2.Send

	assert.Equal(t, payload1, msg1)
	assert.Equal(t, payload2, msg2)
}
