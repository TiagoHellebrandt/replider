// Scenario: conversation list (US-195).
//
// Models the dashboard's hottest authenticated read path: an attendant
// paging GET /conversations. Exercises the account-scoped query + the
// pagination defaults (limit/offset, max 200). Sustained constant rate so
// p95 regressions on the list query (missing index, N+1) surface.
//
//   k6 run -e BASE_URL=... -e AUTH_TOKEN=eyJ... infra/load/scenarios/conversation-list.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, login, authHeaders } from '../lib/common.js';

export const options = {
  scenarios: {
    conversation_list: {
      executor: 'constant-arrival-rate',
      rate: 30, // 30 req/s
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 20,
      maxVUs: 50,
    },
  },
  thresholds: {
    'http_req_duration{name:conversation-list}': ['p(95)<500', 'p(99)<1000'],
    'http_req_failed{name:conversation-list}': ['rate<0.01'],
  },
};

export function setup() {
  return { token: login() };
}

const STATUSES = ['', 'open', 'pending', 'resolved'];

export default function (data) {
  const status = STATUSES[__ITER % STATUSES.length];
  const limit = 50;
  const offset = (__ITER % 4) * limit; // page through the first few pages
  let url = `${BASE_URL}/conversations?limit=${limit}&offset=${offset}`;
  if (status) {
    url += `&status=${status}`;
  }
  const res = http.get(url, {
    headers: authHeaders(data.token),
    tags: { name: 'conversation-list' },
  });
  check(res, { 'list 200': (r) => r.status === 200 });
}
