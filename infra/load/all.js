// Combined Replider load suite (US-195).
//
// Runs all three scenarios — inbound webhook, conversation list, AI reply —
// in a single k6 run with per-scenario thresholds and one shared login.
// CI invokes this weekly; the per-scenario thresholds ARE the tracked
// baseline (a perf regression fails the run), and handleSummary writes a
// machine-readable summary.json that CI uploads as an artifact for trend
// tracking over time.
//
//   k6 run -e BASE_URL=... -e AUTH_TOKEN=... \
//          -e CHANNEL_TOKEN=... -e PREVIEW_AGENT_ID=... infra/load/all.js
//
// SMOKE=1 collapses every scenario to a tiny 1-VU / 10s shape so CI can
// validate the scripts execute end-to-end against an ephemeral API without
// generating real load (or spending real LLM tokens).

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, login, authHeaders, CHANNEL_TOKEN, PREVIEW_AGENT_ID, JSON_HEADERS } from './lib/common.js';

const SMOKE = __ENV.SMOKE === '1';

function ramp(peak) {
  if (SMOKE) {
    return { executor: 'constant-vus', vus: 1, duration: '10s' };
  }
  return {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: peak },
      { duration: '1m', target: peak },
      { duration: '15s', target: 0 },
    ],
  };
}

const conversationListExecutor = SMOKE
  ? { executor: 'constant-vus', vus: 1, duration: '10s' }
  : {
      executor: 'constant-arrival-rate',
      rate: 30,
      timeUnit: '1s',
      duration: '1m45s',
      preAllocatedVUs: 20,
      maxVUs: 50,
    };

export const options = {
  scenarios: {
    inbound_webhook: Object.assign({}, ramp(20), { exec: 'inboundWebhook', tags: { name: 'inbound-webhook' } }),
    conversation_list: Object.assign({}, conversationListExecutor, {
      exec: 'conversationList',
      tags: { name: 'conversation-list' },
    }),
    ai_reply: Object.assign({}, ramp(5), { exec: 'aiReply', tags: { name: 'ai-reply' } }),
  },
  thresholds: {
    'http_req_duration{name:inbound-webhook}': ['p(95)<400', 'p(99)<800'],
    'http_req_failed{name:inbound-webhook}': ['rate<0.01'],
    'http_req_duration{name:conversation-list}': ['p(95)<500', 'p(99)<1000'],
    'http_req_failed{name:conversation-list}': ['rate<0.01'],
    'http_req_duration{name:ai-reply}': ['p(95)<8000', 'p(99)<15000'],
    'http_req_failed{name:ai-reply}': ['rate<0.05'],
  },
};

export function setup() {
  // Auth is only needed by the read/AI scenarios. In SMOKE mode against a
  // freshly-booted API there may be no user; tolerate a missing token so the
  // unauthenticated webhook path is still exercised.
  let token = '';
  try {
    token = login();
  } catch (e) {
    if (!SMOKE) throw e;
  }
  return { token };
}

const STATUSES = ['', 'open', 'pending', 'resolved'];
const PROMPTS = [
  'Qual o horário de funcionamento?',
  'Vocês fazem entrega?',
  'Quanto custa o plano mais barato?',
];

export function inboundWebhook() {
  if (!CHANNEL_TOKEN) {
    const res = http.post(`${BASE_URL}/webhooks/load-test-no-channel`, '{}', {
      headers: JSON_HEADERS,
      tags: { name: 'inbound-webhook' },
    });
    check(res, { 'routed': (r) => r.status === 404 || r.status === 200 });
    return;
  }
  const chatId = 100000 + (__VU % 5000);
  const body = JSON.stringify({
    update_id: __VU * 1000000 + __ITER,
    message: {
      message_id: __ITER,
      from: { id: chatId, first_name: 'Load', username: `load_${chatId}` },
      chat: { id: chatId, type: 'private' },
      date: 1700000000,
      text: 'Olá!',
    },
  });
  const res = http.post(`${BASE_URL}/webhooks/${CHANNEL_TOKEN}`, body, {
    headers: Object.assign({}, JSON_HEADERS, { 'X-Telegram-Bot-Api-Secret-Token': CHANNEL_TOKEN }),
    tags: { name: 'inbound-webhook' },
  });
  check(res, { 'webhook 200': (r) => r.status === 200 });
}

export function conversationList(data) {
  if (!data.token) return;
  const limit = 50;
  const offset = (__ITER % 4) * limit;
  const status = STATUSES[__ITER % STATUSES.length];
  let url = `${BASE_URL}/conversations?limit=${limit}&offset=${offset}`;
  if (status) url += `&status=${status}`;
  const res = http.get(url, { headers: authHeaders(data.token), tags: { name: 'conversation-list' } });
  check(res, { 'list 200': (r) => r.status === 200 });
}

export function aiReply(data) {
  if (!data.token || !PREVIEW_AGENT_ID) return;
  const res = http.post(
    `${BASE_URL}/agents/preview`,
    JSON.stringify({ agent_id: PREVIEW_AGENT_ID, message: PROMPTS[__ITER % PROMPTS.length] }),
    { headers: authHeaders(data.token), tags: { name: 'ai-reply' }, timeout: '30s' },
  );
  check(res, { 'preview 200': (r) => r.status === 200 });
}

// Emit a JSON baseline artifact alongside the default stdout summary.
export function handleSummary(data) {
  return {
    stdout: textSummary(data),
    'summary.json': JSON.stringify(data, null, 2),
  };
}

// Minimal text summary (avoids importing the remote k6 summary module so the
// suite stays hermetic / offline-runnable).
function textSummary(data) {
  const lines = ['', '=== Replider load suite summary ==='];
  const m = data.metrics || {};
  for (const key of ['http_reqs', 'http_req_failed', 'http_req_duration', 'checks']) {
    const v = m[key] && m[key].values;
    if (!v) continue;
    if (key === 'http_req_duration') {
      lines.push(`http_req_duration: avg=${fmt(v.avg)}ms p95=${fmt(v['p(95)'])}ms p99=${fmt(v['p(99)'])}ms`);
    } else if (key === 'http_req_failed' || key === 'checks') {
      lines.push(`${key}: rate=${fmt((v.rate || 0) * 100)}%`);
    } else {
      lines.push(`${key}: count=${v.count}`);
    }
  }
  lines.push('');
  return lines.join('\n');
}

function fmt(n) {
  return n == null ? 'n/a' : Math.round(n * 100) / 100;
}
