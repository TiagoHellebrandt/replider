# WebSocket Scaling — Decision (US-196)

## Problem

The dashboard real-time layer is a single in-process `Hub`
(`api/internal/adapters/http/handlers/hub.go`). It holds the set of connected
clients in a Go map and `BroadcastToAccount` walks that map to push
`new_message` / `conversation_updated` / `usage_alert` events to the Flutter
app.

This works only while the API runs as **one pod**. With horizontal scale (N API
replicas behind the load balancer) the broadcast breaks in a non-obvious way:

- A dashboard agent's WebSocket lands on **pod A**.
- A WhatsApp/Telegram inbound webhook for the same account is load-balanced to
  **pod B**, which runs the AI reply and calls `BroadcastToAccount`.
- Pod B's hub has **no client** for that account — pod A does. The event is
  dropped. The agent's dashboard stays stale until they manually refresh.

So the unit of the problem is **cross-pod fanout of a server-produced event**,
not merely keeping one client pinned to one pod.

## Options considered

### Option 1 — Sticky sessions only (LB affinity)

Pin each client to a pod (nginx `ip_hash`, or cookie/`Upgrade`-aware affinity).

- ✅ Trivial to configure; no app code.
- ❌ **Does not solve the actual bug.** Affinity controls where a *client's
  connection* terminates. It does nothing for an event *produced* by a
  different pod (the webhook handler). Pod B still can't reach pod A's client.
- ❌ Uneven balancing (`ip_hash` buckets whole NATs/offices onto one pod);
  a pod restart drops every socket it owned with no failover.

Sticky alone is **rejected** as the primary mechanism — it addresses connection
placement, not broadcast delivery.

### Option 2 — Redis pub/sub fanout (chosen)

Every pod publishes each broadcast to one shared Redis channel and subscribes to
it; each pod delivers received events to *its own* local clients.

- ✅ Solves the real problem: an event produced on any pod reaches a client on
  any pod.
- ✅ Redis is **already a dependency** (`redis/go-redis/v9`, used for the cache
  and token blacklist; `REDIS_URL` already configured). No new infrastructure.
- ✅ Degrades cleanly: if Redis is unreachable the hub falls back to single-pod
  local delivery (same contract as the cache), so dev and single-replica prod
  are unaffected and zero-config.
- ✅ Stateless pods — any pod can restart; clients reconnect through the LB and
  resume via the REST initial-load path.
- ⚠️ At-most-once, fire-and-forget (no replay). Acceptable: the client's
  reconnect + initial REST page is the durability backstop, exactly as it
  already is for the buffer-full eviction path.

### Option 3 — Dedicated broker (NATS/Kafka) / external WS gateway

Over-engineered for the current scale and adds an operational component we don't
otherwise need. Revisit only if we need broadcast replay, ordering guarantees,
or millions of concurrent sockets.

## Decision

**Redis pub/sub for cross-pod fanout, with sticky sessions as a complementary
LB optimization (not the mechanism).**

- Redis pub/sub is the correctness mechanism (Option 2).
- Sticky affinity is *optional* and only an efficiency tweak (fewer reconnect
  storms on the same pod); correctness no longer depends on it.

## Implementation

| Piece | Location |
|-------|----------|
| Cross-pod transport | `api/internal/adapters/realtime/fanout.go` (`RedisFanout`) |
| Hub seam | `handlers.Hub`: `BroadcastPublisher` port + `SetPublisher` + `DeliverLocal` |
| Wiring | `cmd/api/main.go` (after `NewHub()`) |

Flow:

1. A producer calls `hub.BroadcastToAccount(accountID, payload)` (unchanged API —
   every existing call site keeps working).
2. The hub delivers to **local** clients (`deliverLocal`) **and** calls
   `publisher.PublishBroadcast` to mirror the event onto the Redis channel
   `replider:ws:broadcast`. Publish carries a per-process `Origin` ULID.
3. Every pod's `RedisFanout.Run` subscribes to that channel. On receipt it
   **skips messages whose `Origin` is its own** (already delivered locally) and
   otherwise calls `hub.DeliverLocal` — which delivers without re-publishing, so
   there is no fanout loop.

Fallback: `NewRedisFanout` returns an error when Redis is unreachable; `main.go`
logs it and never calls `SetPublisher`, leaving the hub single-pod. A transient
publish error after wiring degrades to local-only delivery for that one event
(logged), never a panic or a dropped local notification.

### Sticky-session config (when scaling out)

When running N replicas, enable WebSocket-aware upstream affinity at nginx
(`api/nginx` / `nginx/conf.d/20-api.conf`). Example:

```nginx
upstream replider_api {
    ip_hash;                 # or a cookie/consistent-hash module
    server api1:8080;
    server api2:8080;
    keepalive 32;
}
```

This is optional with Redis fanout in place; it only reduces reconnect churn.

## Load-test verification (AC: "Load test verifies")

`infra/load/scenarios/ws-fanout.js` (also in the combined `all.js`) opens many
dashboard WebSocket clients, drives inbound webhooks that trigger broadcasts,
and asserts the sockets receive the resulting `new_message` events within a
latency threshold.

- **Single pod:** verifies the broadcast path end-to-end and the latency
  baseline.
- **Decisive multi-pod check:** point `BASE_URL` at a ≥2-replica staging deploy
  with Redis configured. Because the LB spreads the WS connects and the webhook
  POSTs across pods, a non-zero receive rate proves cross-pod fanout. Run the
  same script against the **same deploy with Redis disabled** and observe the
  receive rate collapse for events that crossed a pod boundary — that delta is
  the regression gate the story asks for.

```bash
# single pod / smoke
k6 run -e BASE_URL=http://localhost:8080/v1 -e AUTH_TOKEN=... -e CHANNEL_TOKEN=... \
       infra/load/scenarios/ws-fanout.js

# multi-pod staging (Redis on) — the real verification
k6 run -e BASE_URL=https://api.staging.replider.com/v1 -e AUTH_TOKEN=... -e CHANNEL_TOKEN=... \
       infra/load/scenarios/ws-fanout.js
```

## Operational notes

- Redis is now load-bearing for real-time *across pods* (still optional for a
  single pod). Monitor it; an outage degrades multi-pod dashboards to
  "refresh to see new messages", not an error.
- The channel is a single global `replider:ws:broadcast`; per-account routing is
  done in-process by `DeliverLocal` (cheap relative to the network hop and
  avoids subscription churn as accounts come and go).
- Payloads are the same JSON the client already receives; no PII beyond what the
  socket already carries is added to the envelope (only an opaque pod ULID).
