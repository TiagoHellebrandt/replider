# Dashboard data-fetching: GraphQL vs BFF (US-197)

**Status:** Accepted — BFF aggregate endpoint shipped as a prototype.
**Date:** 2026-06
**Scope:** Reduce the HTTP round-trips the Flutter dashboard makes on cold start.

## Problem

When the dashboard mounts (`app/lib/features/dashboard/presentation/screens/dashboard_screen.dart`)
it fans out a handful of independent reads before it can paint the first screen:

| # | REST call | Purpose |
|---|-----------|---------|
| 1 | `GET /v1/me` | user + account identity (+ inlined onboarding progress) |
| 2 | `GET /v1/me/plan-limits` | effective per-account caps for quota widgets |
| 3 | `GET /v1/usage` | token / conversation counters + usage % |
| 4 | `GET /v1/agents` | agents list |
| 5 | `GET /v1/channels` | channels list |
| 6 | `GET /v1/conversations?status=open` | the inbox preview |

Each call pays the full per-request tax: a TLS frame, the auth-middleware
chain (JWT verify → account/role claims → per-account+IP rate-limit bucket),
request-ID + structured-logging middleware, and a JSON marshal/unmarshal pair.
Six calls = six of everything, plus six chances for the mobile radio to stall.

## Options considered

### A. GraphQL gateway

Add a GraphQL server (e.g. `gqlgen`) in front of the use-case layer; the client
sends one query selecting exactly the fields it needs.

- **Pros:** client-driven field selection; one transport for everything; strong
  schema tooling.
- **Cons:**
  - **A second API surface** to secure, version, rate-limit and document — on top
    of the REST surface that the marketing site, webhooks and the public OpenAPI
    spec (US-194) already depend on. We are not replacing REST, so this is *additive*
    complexity, not a swap.
  - **Authorization gets harder, not easier.** Every tenant-isolation guarantee
    audited in `docs/security-tenant-audit.md` is enforced per use-case/handler.
    A GraphQL resolver graph re-opens that surface (field-level auth, query depth/
    complexity limits, N+1 batching via dataloaders) — exactly the class of bug the
    P0 security epic just closed.
  - **No introspection/playground** can be exposed publicly without leaking the
    schema; another thing to lock down at the proxy.
  - The dashboard's needs are **known and stable** — there is no long tail of
    bespoke field selections that would justify a query language.
- **Cost/benefit:** large, permanent platform investment for a bounded, well-known
  fan-out problem.

### B. BFF aggregate endpoint  ✅ chosen

Add one REST endpoint, `GET /v1/me/dashboard`, that composes the existing reads
server-side and returns them in a single payload.

- **Pros:**
  - **Zero new infrastructure or transport.** Same Gin router, same auth chain,
    same JWT, same rate limiter, same OpenAPI generator. It is one more annotated
    handler.
  - **No new business logic and no drift risk.** The aggregator
    (`usecases.GetDashboardBootstrapUseCase`) is a thin orchestration layer that
    calls the *same* use cases the standalone endpoints call
    (`GetMeUseCase`, `GetUsageUseCase`, `ListAgentsUseCase`, `ListChannelsUseCase`,
    `ConversationRepository.ListEnriched`). Tenant isolation is inherited unchanged.
  - **Server-side concurrency.** The secondary sections run on their own
    goroutines (each pulls its own pgx connection from the pool), so wall-clock
    latency tracks the slowest single query — matching what the client got from
    issuing the REST calls in parallel, minus five round-trips of network + auth tax.
  - **Graceful degradation.** Identity (`me`) is the only hard dependency; every
    other section is best-effort and, on failure, is omitted and named in a
    `warnings` array instead of blanking the whole dashboard. A single slow
    subsystem can no longer take the dashboard down.
- **Cons:** the endpoint is dashboard-shaped (it knows what *this* client needs).
  That coupling is the explicit point of a BFF and is acceptable for a
  first-party client.

## Decision

Ship **option B**. The dashboard bootstrap is a fixed, first-party fan-out — the
textbook case for a Backend-for-Frontend aggregate, not a general-purpose query
language. GraphQL's flexibility buys us nothing here and costs a second
security/ops surface.

## Prototype

- **Use case:** `api/internal/core/usecases/get_dashboard_bootstrap.go`
  (`GetDashboardBootstrapUseCase`) + `get_dashboard_bootstrap_test.go`.
- **Endpoint:** `GET /v1/me/dashboard` → `MeHandler.HandleGetDashboard`
  (wired via `meHandler.SetDashboardBootstrapUC(...)` in `cmd/api/main.go`).
- **Response:** `DashboardBootstrapResponse` — `me`, `plan_limits`, `usage`,
  `agents`, `channels`, `recent_conversations`, plus an optional `warnings` array.
  Each section is byte-for-byte the shape its standalone endpoint returns, so the
  Flutter client can route a section straight into the model it already uses.

## Benchmark — requests vs REST

The acceptance criterion is **< 50 % of the REST request count**.

| | Round-trips on dashboard cold start |
|--------------------|:--:|
| REST (status quo)  | **6** (`/me`, `/me/plan-limits`, `/usage`, `/agents`, `/channels`, `/conversations`) |
| BFF (`/me/dashboard`) | **1** |
| **Reduction**      | **1 ÷ 6 = 0.167 → 83 % fewer requests** |

83 % well clears the 50 % bar. Each eliminated request also removes one auth-middleware
pass, one rate-limiter token, one request-ID/log line and one TLS frame. The single
remaining request does the same DB work, but concurrently and behind one auth pass.

### How to reproduce the count

- Server-side: hit the app with the dashboard open and count `request_id`-tagged
  access-log lines (US-069) in the cold-start window — six distinct routes before,
  one after.
- Client-side: the Dio logging interceptor logs one line per request; the dashboard
  controller fan-out drops from six to one.
- Load test: the k6 suite (`infra/load/`, US-195) drives the bootstrap path — a
  `dashboard-bootstrap.js` scenario can assert one 200 carries all six sections.

## Migration / rollout

The standalone endpoints stay in place (other screens and the marketing site use
them). The dashboard controller switches its cold-start fan-out to the single
`/me/dashboard` call; everything after first paint (paging the inbox, polling usage)
keeps using the granular endpoints. If the aggregator is ever unwired the route
returns `503 BFF_UNAVAILABLE`, so the client can fall back to the REST fan-out.

## Future work (not in this prototype)

- Switch the Flutter dashboard controller to consume `/me/dashboard` and delete the
  redundant cold-start providers.
- Optional `ETag`/`If-None-Match` on the aggregate for a cheap 304 on revisit.
- A `?sections=` selector if a second consumer ever wants a subset (the small step
  toward GraphQL-style selection, without the gateway).
