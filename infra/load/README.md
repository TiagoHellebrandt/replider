# Load tests (k6) — US-195

[k6](https://k6.io) load suite that catches performance regressions on the
three hottest API paths:

| Scenario | File | Path | Notes |
|----------|------|------|-------|
| Inbound webhook | `scenarios/inbound-webhook.js` | `POST /webhooks/:channel_token` | Unauthenticated write path (Telegram shape) |
| Conversation list | `scenarios/conversation-list.js` | `GET /conversations` | Authenticated dashboard read path |
| AI reply | `scenarios/ai-reply.js` | `POST /agents/preview` | Full AgentRunner (RAG + LLM); costs real tokens |

`all.js` runs all three together with per-scenario thresholds and a shared
login — this is what CI runs.

## Running locally

Install k6 (`brew install k6`, or see [k6 install docs](https://grafana.com/docs/k6/latest/set-up/install-k6/)), then:

```bash
# Full suite against staging
k6 run \
  -e BASE_URL=https://api.staging.replider.com/v1 \
  -e AUTH_TOKEN=eyJ... \
  -e CHANNEL_TOKEN=01J...           # a real Telegram channel webhook_token \
  -e PREVIEW_AGENT_ID=<agent-uuid> \
  infra/load/all.js

# A single scenario
k6 run -e BASE_URL=... -e AUTH_TOKEN=... infra/load/scenarios/conversation-list.js

# Smoke (1 VU / 10s per scenario) — validates the scripts execute
SMOKE=1 k6 run -e BASE_URL=http://localhost:8080/v1 infra/load/all.js
```

## Configuration (env vars, passed via `-e` or the shell)

| Var | Default | Used by |
|-----|---------|---------|
| `BASE_URL` | `http://localhost:8080/v1` | all |
| `AUTH_TOKEN` | — | conversation-list, ai-reply (skips login) |
| `AUTH_EMAIL` / `AUTH_PASSWORD` | `loadtest@replider.com` / `loadtest-password` | `login()` fallback when no `AUTH_TOKEN` |
| `CHANNEL_TOKEN` | — | inbound-webhook (live Telegram channel) |
| `PREVIEW_AGENT_ID` | — | ai-reply (required) |
| `SMOKE` | — | `1` collapses every scenario to a 1-VU / 10s shape |

Prefer `AUTH_TOKEN` in CI: the login endpoint is itself rate-limited
(10/min, US-053) and shouldn't be hammered just to bootstrap.

## Baseline tracking

The per-scenario `thresholds` in each script **are** the tracked baseline —
a regression past them fails the run (and the CI job). Current targets:

| Scenario | p95 | p99 | error rate |
|----------|-----|-----|-----------|
| inbound-webhook | 400ms | 800ms | <1% |
| conversation-list | 500ms | 1000ms | <1% |
| ai-reply | 8000ms | 15000ms | <5% |

`all.js`'s `handleSummary` also writes `summary.json` (full metric set),
which the weekly CI job uploads as an artifact so trends can be compared
run-over-run. Tune the thresholds here when the infra baseline legitimately
shifts.

## CI

`.github/workflows/load-test.yml` runs the suite **weekly** (Mon 06:00
America/Sao_Paulo) and on demand (`workflow_dispatch`). It always validates
the scripts (`k6 inspect`); when a `LOAD_TEST_BASE_URL` is configured it runs
the full suite against that environment and uploads `summary.json`.
