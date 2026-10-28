// Shared helpers for the Replider k6 load suite (US-195).
//
// Every scenario imports BASE_URL + auth helpers from here so the target
// environment and credentials are configured in ONE place via env vars:
//
//   BASE_URL    API root, defaults to local dev (e.g. https://api.replider.com/v1)
//   AUTH_TOKEN  pre-minted JWT; skips the login round-trip when provided
//   AUTH_EMAIL  / AUTH_PASSWORD  used by login() when AUTH_TOKEN is absent
//   CHANNEL_TOKEN  webhook_token (ULID) of a Telegram channel for inbound load
//   PREVIEW_AGENT_ID  agent UUID used by the AI-reply scenario
//
// k6 has no process.env — only the __ENV global, populated from -e/--env or
// the shell environment at `k6 run` time.

import http from 'k6/http';
import { check } from 'k6';

export const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8080/v1').replace(/\/$/, '');

export const AUTH_EMAIL = __ENV.AUTH_EMAIL || 'loadtest@replider.com';
export const AUTH_PASSWORD = __ENV.AUTH_PASSWORD || 'loadtest-password';

export const CHANNEL_TOKEN = __ENV.CHANNEL_TOKEN || '';
export const PREVIEW_AGENT_ID = __ENV.PREVIEW_AGENT_ID || '';

export const JSON_HEADERS = { 'Content-Type': 'application/json' };

// login exchanges email/password for a JWT. Prefer passing AUTH_TOKEN
// directly in CI so the suite doesn't hammer the auth endpoint (which is
// itself rate-limited at 10/min, US-053) just to bootstrap.
export function login() {
  if (__ENV.AUTH_TOKEN) {
    return __ENV.AUTH_TOKEN;
  }
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: AUTH_EMAIL, password: AUTH_PASSWORD }),
    { headers: JSON_HEADERS, tags: { name: 'auth/login' } },
  );
  check(res, { 'login 200': (r) => r.status === 200 });
  const token = res.json('token');
  if (!token) {
    throw new Error(`login() got no token (status ${res.status}); set AUTH_TOKEN or valid AUTH_EMAIL/AUTH_PASSWORD`);
  }
  return token;
}

export function authHeaders(token) {
  return Object.assign({}, JSON_HEADERS, { Authorization: `Bearer ${token}` });
}
