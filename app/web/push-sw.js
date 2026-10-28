// Replider service worker — handles Web Push events AND satisfies the
// PWA install criteria so the app can be added to the iOS / Android
// home screen. Scoped at the app origin.
//
// IMPORTANT — the SW is intentionally LIGHT on caching:
//
//   * The Flutter build runs with `--pwa-strategy=none`, so we never
//     try to cache main.dart.js / canvaskit / fonts here. That's the
//     Flutter framework's territory and double-caching causes
//     stale-bundle bugs.
//   * The handful of static shell files (manifest, icons) are cached
//     so an offline launch from the home-screen icon doesn't show a
//     network-error page before Flutter mounts.
//   * Every other request is network-only — the SW intercepts the
//     `fetch` event ONLY to satisfy install criteria (Chromium and
//     Safari require a SW with a fetch handler before they expose the
//     "Install app" affordance) and proxies to network with a
//     no-cache passthrough.
//
// Push handlers live below the cache logic; each is independent.

const CACHE_NAME = 'replider-shell-v1';
const SHELL_ASSETS = [
  '/',
  '/manifest.json',
  '/favicon.png',
  '/icons/Icon-192.png',
  '/icons/Icon-512.png',
];

self.addEventListener('install', function (event) {
  // Activate this SW immediately so a freshly-logged-in user starts
  // receiving pushes without a page reload.
  self.skipWaiting();
  event.waitUntil(
    caches.open(CACHE_NAME).then(function (cache) {
      // addAll fails atomically on the first 404, which is too brittle
      // for a shell list (icons may not exist on every deploy). Fetch
      // each asset individually and tolerate failures — the app still
      // works, install criteria are met as long as the SW registers.
      return Promise.all(SHELL_ASSETS.map(function (url) {
        return fetch(url).then(function (resp) {
          if (resp && resp.ok) return cache.put(url, resp);
        }).catch(function () { /* swallow */ });
      }));
    })
  );
});

self.addEventListener('activate', function (event) {
  // Drop old shell caches on version bump so we don't hoard stale icons.
  event.waitUntil((async function () {
    const keys = await caches.keys();
    await Promise.all(keys.map(function (k) {
      if (k !== CACHE_NAME && k.indexOf('replider-shell-') === 0) {
        return caches.delete(k);
      }
    }));
    await self.clients.claim();
  })());
});

// Fetch handler. Required for the browser to consider the page
// installable. Network-first for shell assets so updates to manifest /
// icons propagate; falls back to cache only when offline.
self.addEventListener('fetch', function (event) {
  // Only handle GETs for our origin; everything else (API XHR, GCS
  // signed URLs, third-party scripts) goes straight to network.
  const req = event.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  // Don't intercept Flutter's own assets — let the browser cache them
  // via HTTP headers (the deploy uses long-lived hashed filenames).
  if (
    url.pathname.startsWith('/assets/') ||
    url.pathname.startsWith('/canvaskit/') ||
    url.pathname.endsWith('.js') ||
    url.pathname.endsWith('.wasm')
  ) {
    return;
  }
  event.respondWith((async function () {
    try {
      const fresh = await fetch(req);
      // Mirror successful shell responses into the cache so the next
      // offline launch has them.
      if (fresh && fresh.ok) {
        const clone = fresh.clone();
        caches.open(CACHE_NAME).then(function (c) { c.put(req, clone); }).catch(function () {});
      }
      return fresh;
    } catch (e) {
      const cached = await caches.match(req);
      if (cached) return cached;
      // Last resort: serve the cached root so the app shell mounts and
      // can render an offline UI. Better than the browser's default
      // "no internet" chrome page.
      const root = await caches.match('/');
      if (root) return root;
      throw e;
    }
  })());
});

// Localised copy table. The API never bakes language strings into the
// push payload (an account may have attendants in different locales);
// the SW resolves the right pair per device using navigator.language.
// Add a new locale by inserting a key here — no API change required.
const PUSH_I18N = {
  en: {
    fallback_title: 'Replider',
    new_message: 'New message',
    media_image: '📷 Image',
    media_audio: '🎙️ Audio',
    media_video: '🎥 Video',
    media_document: '📎 Document',
    test_title: 'Replider — test',
    test_body: 'Test notification delivered successfully.',
    handoff_body: '🟠 AI did not answer — human takeover required',
    reminder_body: '⏰ Customer waiting on your reply',
  },
  pt: {
    fallback_title: 'Replider',
    new_message: 'Nova mensagem',
    media_image: '📷 Imagem',
    media_audio: '🎙️ Áudio',
    media_video: '🎥 Vídeo',
    media_document: '📎 Documento',
    test_title: 'Replider — teste',
    test_body: 'Notificação de teste recebida com sucesso.',
    handoff_body: '🟠 IA não soube responder — atendimento humano necessário',
    reminder_body: '⏰ Cliente aguardando sua resposta',
  },
};

function pickLocale() {
  const raw = (self.navigator && self.navigator.language) || 'en';
  return raw.toLowerCase().startsWith('pt') ? 'pt' : 'en';
}

function resolveBody(data, t) {
  if (data.body) return data.body;
  switch (data.media_type) {
    case 'image': return t.media_image;
    case 'audio':
    case 'voice': return t.media_audio;
    case 'video': return t.media_video;
    case 'document': return t.media_document;
    default: return t.new_message;
  }
}

self.addEventListener('push', function (event) {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch (e) {
    data = { body: event.data ? event.data.text() : '' };
  }
  const t = PUSH_I18N[pickLocale()];
  let title = data.title || t.fallback_title;
  let body = resolveBody(data, t);
  // Test pings go out with Title/Body deliberately empty — render the
  // locale-aware test pair here so /push/test doesn't need to know
  // which language the admin's browser speaks.
  if (data.event === 'test') {
    title = t.test_title;
    body = t.test_body;
  }
  // Handoff and reminder events have server-supplied Body strings, but
  // resolve a localised fallback here for older API builds that may
  // ship empty Body. Keeps the OS notification informative even before
  // the API rolls out matching copy.
  if (data.event === 'human_handoff_requested' && !data.body) {
    body = t.handoff_body;
  }
  if (data.event === 'reply_reminder' && !data.body) {
    body = t.reminder_body;
  }
  const options = {
    body: body,
    icon: data.icon || 'icons/Icon-192.png',
    badge: data.badge || 'icons/Icon-192.png',
    // Collapsing by conversation keeps a burst of customer messages to
    // a single OS notification instead of a flood.
    tag: data.tag || 'replider',
    renotify: true,
    data: {
      click_url: data.click_url || '/',
      conversation_id: data.conversation_id || null,
      account_id: data.account_id || null,
      event: data.event || null,
    },
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

// Browsers occasionally rotate the push subscription on their own
// (token refresh, push-service migration, etc.). When that happens the
// SW receives `pushsubscriptionchange` with the old + (sometimes) the
// new sub. Without a handler the rotated endpoint is silently lost and
// every subsequent send goes nowhere — the most common reason a user
// reports "I stopped getting notifications and I don't know why."
//
// Strategy: try to re-subscribe inside the SW with the cached VAPID
// key. If that succeeds, ping every open client so it can POST the
// fresh row to the API (the SW has no auth context to call it
// directly). If anything fails, ping clients with a "needs re-enable"
// signal so the next foreground tab walks the user through the toggle.
self.addEventListener('pushsubscriptionchange', function (event) {
  event.waitUntil((async function () {
    let fresh = null;
    try {
      const cached = await caches.open('replider-push-meta');
      const keyResp = await cached.match('vapid-public-key');
      const vapid = keyResp ? await keyResp.text() : '';
      if (vapid) {
        const raw = (function (b64) {
          const padding = '='.repeat((4 - (b64.length % 4)) % 4);
          const base64 = (b64 + padding).replace(/-/g, '+').replace(/_/g, '/');
          const bin = atob(base64);
          const out = new Uint8Array(bin.length);
          for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
          return out;
        })(vapid);
        fresh = await self.registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: raw,
        });
      }
    } catch (_) { /* fall through to client postMessage below */ }

    const clientsList = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    const payload = fresh
      ? { type: 'replider_push_resubscribed', subscription: fresh.toJSON() }
      : { type: 'replider_push_needs_resubscribe' };
    for (const c of clientsList) {
      try { c.postMessage(payload); } catch (_) { /* offline tab */ }
    }
  })());
});

self.addEventListener('notificationclick', function (event) {
  event.notification.close();
  const target = (event.notification.data && event.notification.data.click_url) || '/';
  event.waitUntil((async function () {
    const all = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    // Focus an already-open tab and tell Flutter to navigate via
    // postMessage — direct SPA routing avoids a full reload.
    for (const c of all) {
      try {
        await c.focus();
      } catch (_) { /* focus can fail mid-navigation; ignore */ }
      try {
        c.postMessage({ type: 'replider_push_click', target: target });
      } catch (_) { /* non-controlled tabs may refuse postMessage */ }
      return;
    }
    if (self.clients.openWindow) await self.clients.openWindow(target);
  })());
});
