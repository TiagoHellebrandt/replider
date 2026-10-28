// Scenario: inbound webhook (US-195).
//
// Models the hottest, unauthenticated write path: a messaging provider
// POSTing an inbound message to POST /webhooks/:channel_token. We use the
// Telegram update shape because that route parses the body
// (telegram.ParseUpdate) — WhatsApp uses a separate global endpoint.
//
// Note: a Telegram channel rejects bodies whose
// X-Telegram-Bot-Api-Secret-Token header != the channel's webhook_token
// (constant-time compare). So CHANNEL_TOKEN must be a real Telegram channel
// token and we echo it back as the secret header. Without it the request
// 403s — still a valid load signal for the routing/auth layer, but to
// exercise SaveInbound + AI dispatch set a live CHANNEL_TOKEN.
//
//   k6 run -e BASE_URL=https://api.staging.replider.com/v1 \
//          -e CHANNEL_TOKEN=01J... infra/load/scenarios/inbound-webhook.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, CHANNEL_TOKEN, JSON_HEADERS } from '../lib/common.js';

export const options = {
  scenarios: {
    inbound_webhook: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 20 },
        { duration: '1m', target: 20 },
        { duration: '15s', target: 0 },
      ],
    },
  },
  // Baseline gate: webhook ack must stay fast (Telegram flips the hook to
  // "fora" on 5xx, so any server error is a hard failure here).
  thresholds: {
    'http_req_duration{name:inbound-webhook}': ['p(95)<400', 'p(99)<800'],
    'http_req_failed{name:inbound-webhook}': ['rate<0.01'],
  },
};

function telegramUpdate(vu, iter) {
  const chatId = 100000 + (vu % 5000); // a handful of recurring "contacts"
  return JSON.stringify({
    update_id: vu * 1000000 + iter,
    message: {
      message_id: iter,
      from: { id: chatId, first_name: 'Load', username: `load_${chatId}` },
      chat: { id: chatId, type: 'private' },
      date: 1700000000,
      text: 'Olá, qual o horário de funcionamento?',
    },
  });
}

export default function () {
  if (!CHANNEL_TOKEN) {
    // No live channel configured — still exercise routing/404 path.
    const res = http.post(`${BASE_URL}/webhooks/load-test-no-channel`, '{}', {
      headers: JSON_HEADERS,
      tags: { name: 'inbound-webhook' },
    });
    check(res, { 'routed (404/200)': (r) => r.status === 404 || r.status === 200 });
    return;
  }

  const res = http.post(
    `${BASE_URL}/webhooks/${CHANNEL_TOKEN}`,
    telegramUpdate(__VU, __ITER),
    {
      headers: Object.assign({}, JSON_HEADERS, { 'X-Telegram-Bot-Api-Secret-Token': CHANNEL_TOKEN }),
      tags: { name: 'inbound-webhook' },
    },
  );
  // Provider-facing endpoint always 200s (even on internal recover) to keep
  // the webhook healthy; treat anything else as a failure.
  check(res, { 'webhook 200': (r) => r.status === 200 });
}
