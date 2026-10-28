// Scenario: AI reply (US-195).
//
// Models the most expensive path: POST /agents/preview, which runs the full
// AgentRunner pipeline (prompt assembly + pgvector RAG + LLM call) without
// persisting a conversation. This is the load test for AI latency/cost and
// the tool-call loop cap (US-023).
//
// Lower concurrency than the read paths on purpose: the endpoint is rate
// limited to 60/min per account (US-022 route cap) and each call costs real
// LLM tokens. Run this against a staging account, not production.
//
//   k6 run -e BASE_URL=... -e AUTH_TOKEN=eyJ... \
//          -e PREVIEW_AGENT_ID=<uuid> infra/load/scenarios/ai-reply.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, login, authHeaders, PREVIEW_AGENT_ID } from '../lib/common.js';

export const options = {
  scenarios: {
    ai_reply: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 5 },
        { duration: '40s', target: 5 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  // LLM latency is inherently high and provider-dependent; the baseline is
  // generous and mainly guards against runaway tool-loop / timeout regressions.
  thresholds: {
    'http_req_duration{name:ai-reply}': ['p(95)<8000', 'p(99)<15000'],
    'http_req_failed{name:ai-reply}': ['rate<0.05'],
  },
};

export function setup() {
  if (!PREVIEW_AGENT_ID) {
    throw new Error('PREVIEW_AGENT_ID is required for the ai-reply scenario');
  }
  return { token: login() };
}

const PROMPTS = [
  'Qual o horário de funcionamento?',
  'Vocês fazem entrega?',
  'Quanto custa o plano mais barato?',
  'Preciso falar com um atendente humano.',
  'Como faço para agendar um horário?',
];

export default function (data) {
  const res = http.post(
    `${BASE_URL}/agents/preview`,
    JSON.stringify({
      agent_id: PREVIEW_AGENT_ID,
      message: PROMPTS[__ITER % PROMPTS.length],
    }),
    { headers: authHeaders(data.token), tags: { name: 'ai-reply' }, timeout: '30s' },
  );
  check(res, { 'preview 200': (r) => r.status === 200 });
}
