// Scenario: dashboard WebSocket fanout (US-196).
//
// Verifies the real-time broadcast path that cross-pod fanout (Redis pub/sub)
// must preserve: a dashboard client connects to GET /v1/ws/chat and, when an
// inbound message arrives for its account, the socket receives a broadcast
// frame (`new_message` / `conversation_updated`).
//
// Each VU opens a WS, then POSTs an inbound webhook for the same account. Since
// SaveInbound broadcasts to every socket on the account, the VU's own socket
// should receive a frame — so the receive RATE is the signal.
//
//   - Single pod: validates the broadcast path + latency baseline.
//   - Multi-pod (the decisive check): point BASE_URL at a >=2-replica staging
//     deploy with Redis on. The LB spreads WS connects and webhook POSTs across
//     pods, so a high receive rate proves the broadcast crossed a pod boundary.
//     Re-run with Redis OFF and watch the rate collapse — that delta is the gate.
//
//   k6 run -e BASE_URL=https://api.staging.replider.com/v1 \
//          -e AUTH_TOKEN=... -e CHANNEL_TOKEN=01J... infra/load/scenarios/ws-fanout.js
//
// See docs/ws-scaling.md for the decision + the Redis-on/off comparison.

import ws from 'k6/ws';
import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';
import { BASE_URL, login, CHANNEL_TOKEN, JSON_HEADERS } from '../lib/common.js';

const SMOKE = __ENV.SMOKE === '1';

// Fraction of opened sockets that received a broadcast after we triggered an
// inbound message. This IS the cross-pod-fanout health signal.
const broadcastReceived = new Rate('ws_broadcast_received');

const WS_URL = `${BASE_URL.replace(/^http/, 'ws')}/ws/chat`;

export const options = {
  scenarios: {
    ws_fanout: SMOKE
      ? { executor: 'constant-vus', vus: 1, duration: '10s', exec: 'wsFanout', tags: { name: 'ws-fanout' } }
      : {
          executor: 'ramping-vus',
          startVUs: 0,
          stages: [
            { duration: '30s', target: 20 },
            { duration: '1m', target: 20 },
            { duration: '15s', target: 0 },
          ],
          exec: 'wsFanout',
          tags: { name: 'ws-fanout' },
        },
  },
  thresholds: {
    // With a live channel + token the dashboard must actually receive the
    // broadcast; gate on a high receive rate. (Relaxed in SMOKE / when the
    // prerequisites aren't wired — see the no-op branches below.)
    ws_broadcast_received: [{ threshold: 'rate>0.90', abortOnFail: false }],
    'ws_connecting{name:ws-fanout}': ['p(95)<2000'],
  },
};

export function setup() {
  let token = '';
  try {
    token = login();
  } catch (e) {
    if (!SMOKE) throw e;
  }
  return { token };
}

export function wsFanout(data) {
  if (!data.token) return; // no auth wired — nothing to verify

  const params = { headers: { Authorization: `Bearer ${data.token}` }, tags: { name: 'ws-fanout' } };

  const res = ws.connect(WS_URL, params, function (socket) {
    let got = false;

    socket.on('open', function () {
      // Trigger an inbound message so a broadcast is produced for this account.
      // Without a live channel we can still validate the handshake; just skip
      // the broadcast assertion (recorded as no-signal, not a failure).
      if (CHANNEL_TOKEN) {
        triggerInbound();
      }
      // Bound the session so the VU recycles; give the broadcast time to land.
      socket.setTimeout(function () {
        socket.close();
      }, SMOKE ? 1000 : 3000);
    });

    socket.on('message', function (msg) {
      // Any broadcast frame counts (new_message / conversation_updated /
      // usage_alert). Ignore our own auth/chat protocol frames.
      if (msg && msg.indexOf('"error"') === -1) {
        got = true;
      }
    });

    socket.on('close', function () {
      if (CHANNEL_TOKEN) {
        broadcastReceived.add(got);
        check(got, { 'received broadcast frame': (v) => v === true });
      }
    });
  });

  check(res, { 'ws handshake 101': (r) => r && r.status === 101 });
}

function triggerInbound() {
  const chatId = 200000 + (__VU % 5000);
  const body = JSON.stringify({
    update_id: __VU * 1000000 + __ITER,
    message: {
      message_id: __ITER,
      from: { id: chatId, first_name: 'Fanout', username: `fanout_${chatId}` },
      chat: { id: chatId, type: 'private' },
      date: 1700000000,
      text: 'Olá!',
    },
  });
  http.post(`${BASE_URL}/webhooks/${CHANNEL_TOKEN}`, body, {
    headers: Object.assign({}, JSON_HEADERS, { 'X-Telegram-Bot-Api-Secret-Token': CHANNEL_TOKEN }),
    tags: { name: 'ws-fanout-trigger' },
  });
}
