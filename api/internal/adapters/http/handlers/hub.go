package handlers

import (
	"log/slog"
	"sync"
)

// BroadcastPublisher fans a local broadcast out to the OTHER API pods so a
// `new_message`/`conversation_updated` event produced on the pod that handled
// an inbound webhook still reaches a dashboard client connected to a different
// pod. It is satisfied by the Redis pub/sub adapter (US-196). When nil the hub
// stays single-pod and only delivers to its own locally-connected clients.
type BroadcastPublisher interface {
	PublishBroadcast(accountID string, payload []byte) error
}

// Hub maintains the set of active clients
type Hub struct {
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex

	// publisher, when set, mirrors every BroadcastToAccount onto a
	// cross-pod transport (Redis pub/sub). Guarded by mu so wiring at
	// startup races safely with the broadcast path.
	publisher BroadcastPublisher
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// SetPublisher attaches a cross-pod broadcast transport. Call once at startup
// before traffic. A nil publisher leaves the hub single-pod.
func (h *Hub) SetPublisher(p BroadcastPublisher) {
	h.mu.Lock()
	h.publisher = p
	h.mu.Unlock()
}

// Run starts the processing loop for the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()
		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastToAccount sends a message to all connected clients for a specific
// account AND mirrors it to the other API pods via the optional publisher, so
// real-time updates (new messages, status changes) reach the Flutter app no
// matter which pod the producing request and the dashboard socket landed on.
//
// This is the entry point for event PRODUCERS (webhook handlers, takeover use
// case). Messages arriving FROM another pod are delivered via DeliverLocal so
// they are not re-published (which would loop forever).
func (h *Hub) BroadcastToAccount(accountID string, payload []byte) {
	h.deliverLocal(accountID, payload)

	// Mirror to peer pods. Read the publisher under the same lock that
	// protects SetPublisher. A failed publish degrades to single-pod
	// delivery (already done above) rather than dropping the event.
	h.mu.RLock()
	pub := h.publisher
	h.mu.RUnlock()
	if pub != nil {
		if err := pub.PublishBroadcast(accountID, payload); err != nil {
			slog.Warn("ws cross-pod publish failed — event delivered to local clients only",
				"account_id", accountID, "error", err)
		}
	}
}

// DeliverLocal delivers a payload to this pod's locally-connected clients
// without re-publishing to peers. It is the sink for broadcasts received from
// another pod over the cross-pod transport (US-196).
//
// When a client's Send buffer (256 slots) is full we evict it instead
// of silently dropping the payload. A buffer that full means the
// client thread can't keep up with traffic, and silently dropping a
// `new_message` event leaves the dashboard stuck on stale state until
// the agent manually refreshes — exactly the "where did my message
// go?" support tickets we want to avoid. Closing the connection forces
// the Flutter client through its reconnect + initial-load path, which
// returns the missed events as a fresh REST page.
//
// We queue Unregister non-blockingly so a saturated Unregister channel
// (Run loop temporarily busy on Register) doesn't block the broadcast
// for other healthy clients on the same account.
func (h *Hub) DeliverLocal(accountID string, payload []byte) {
	h.deliverLocal(accountID, payload)
}

func (h *Hub) deliverLocal(accountID string, payload []byte) {
	var slow []*Client
	h.mu.RLock()
	for client := range h.Clients {
		if client.AccountID == accountID {
			select {
			case client.Send <- payload:
			default:
				slow = append(slow, client)
			}
		}
	}
	h.mu.RUnlock()
	for _, client := range slow {
		slog.Warn("ws client buffer full — evicting to force reconnect",
			"account_id", client.AccountID, "user_id", client.UserID)
		select {
		case h.Unregister <- client:
		default:
			// Unregister loop is momentarily busy. The next broadcast
			// or the client's own read-loop EOF will retry; we don't
			// hold up the broadcast for healthy clients.
		}
	}
}
