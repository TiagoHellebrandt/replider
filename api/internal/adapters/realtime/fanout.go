// Package realtime carries dashboard WebSocket broadcasts across API pods.
//
// The WebSocket Hub (internal/adapters/http/handlers) keeps its client set in
// process memory, so a `new_message` event produced on the pod that handled an
// inbound webhook never reaches a dashboard socket pinned to a different pod.
// RedisFanout closes that gap (US-196): every BroadcastToAccount is also
// published to a shared Redis channel that all pods subscribe to, and each pod
// delivers received events to its own local clients.
//
// Decision rationale (sticky LB vs Redis pub/sub) lives in docs/ws-scaling.md.
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
)

// broadcastChannel is the single Redis pub/sub channel every pod uses. One
// channel keeps fanout simple; the per-account routing happens in-process
// (DeliverLocal filters by AccountID), which is cheap relative to the network
// hop and avoids a subscription churn problem as accounts come and go.
const broadcastChannel = "replider:ws:broadcast"

// LocalDeliverer is the slice of the Hub that RedisFanout drives: deliver a
// payload to this pod's locally-connected clients WITHOUT re-publishing.
// *handlers.Hub satisfies it via DeliverLocal.
type LocalDeliverer interface {
	DeliverLocal(accountID string, payload []byte)
}

// envelope is the wire format on the Redis channel. Origin lets a pod skip the
// message it published itself (it already delivered locally), so clients on the
// producing pod aren't double-notified.
type envelope struct {
	Origin    string `json:"o"`
	AccountID string `json:"a"`
	Payload   []byte `json:"p"` // json marshals []byte as base64
}

// RedisFanout publishes local broadcasts to peer pods and delivers peer
// broadcasts to local clients. It implements handlers.BroadcastPublisher.
type RedisFanout struct {
	client   *redis.Client
	channel  string
	instance string // unique per process; identifies our own messages
	local    LocalDeliverer
}

// NewRedisFanout connects to Redis and returns a ready fanout. It returns an
// error (not a no-op) when Redis is unreachable so the caller can log and fall
// back to single-pod operation — mirroring the cache.NewRedisCache contract.
func NewRedisFanout(redisURL string, local LocalDeliverer) (*RedisFanout, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("realtime: invalid redis URL: %w", err)
	}
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("realtime: failed to connect to redis: %w", err)
	}

	return &RedisFanout{
		client:   client,
		channel:  broadcastChannel,
		instance: ulid.Make().String(),
		local:    local,
	}, nil
}

// PublishBroadcast mirrors a local broadcast to peer pods. Best-effort: a
// publish error is returned to the caller (the Hub), which has already
// delivered to local clients.
func (f *RedisFanout) PublishBroadcast(accountID string, payload []byte) error {
	data, err := json.Marshal(envelope{Origin: f.instance, AccountID: accountID, Payload: payload})
	if err != nil {
		return fmt.Errorf("realtime: marshal envelope: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := f.client.Publish(ctx, f.channel, data).Err(); err != nil {
		return fmt.Errorf("realtime: publish: %w", err)
	}
	return nil
}

// Run subscribes to the broadcast channel and delivers peer events to local
// clients until ctx is cancelled. Intended to be called in its own goroutine.
func (f *RedisFanout) Run(ctx context.Context) {
	sub := f.client.Subscribe(ctx, f.channel)
	defer func() { _ = sub.Close() }()

	ch := sub.Channel()
	slog.Info("ws Redis fanout subscribed", "channel", f.channel, "instance", f.instance)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var env envelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				slog.Warn("ws fanout: dropping malformed broadcast envelope", "error", err)
				continue
			}
			if env.Origin == f.instance {
				continue // our own publish; already delivered locally
			}
			f.local.DeliverLocal(env.AccountID, env.Payload)
		}
	}
}

// Close releases the underlying Redis connection.
func (f *RedisFanout) Close() error {
	return f.client.Close()
}
