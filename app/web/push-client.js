// Thin browser shim for Replider Web Push. Dart calls into these helpers
// via dart:js_util so the Flutter side stays free of Push API ceremony
// (URL-safe base64 decoding, SW registration, PushSubscription.toJSON).
(function () {
  const SW_URL = 'push-sw.js';

  function urlB64ToUint8(b64) {
    const padding = '='.repeat((4 - (b64.length % 4)) % 4);
    const base64 = (b64 + padding).replace(/-/g, '+').replace(/_/g, '/');
    const raw = atob(base64);
    const out = new Uint8Array(raw.length);
    for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
    return out;
  }

  function serialize(sub) {
    if (!sub) return null;
    const json = sub.toJSON();
    return {
      endpoint: json.endpoint,
      expirationTime: json.expirationTime,
      p256dh: (json.keys && json.keys.p256dh) || null,
      auth: (json.keys && json.keys.auth) || null,
    };
  }

  async function registerSw() {
    if (!('serviceWorker' in navigator)) return null;
    // pushManager.subscribe() throws AbortError ("no active Service
    // Worker") when called before the SW reaches `activated` —
    // register() resolves as soon as install *begins*, not when the
    // worker is active. Wait on serviceWorker.ready so callers always
    // get a registration with `.active` populated.
    await navigator.serviceWorker.register(SW_URL);
    return await navigator.serviceWorker.ready;
  }

  async function currentRegistration() {
    if (!('serviceWorker' in navigator)) return null;
    // Intentionally no fallback to serviceWorker.ready — that promise
    // never resolves when the push SW has never been registered, which
    // would hang the Settings toggle forever in the "checking" state.
    return await navigator.serviceWorker.getRegistration(SW_URL);
  }

  window.repliderPush = {
    supported: function () {
      return 'serviceWorker' in navigator
        && 'PushManager' in window
        && 'Notification' in window;
    },
    permission: function () {
      if (!('Notification' in window)) return 'denied';
      return Notification.permission;
    },
    requestPermission: async function () {
      if (!('Notification' in window)) return 'denied';
      return await Notification.requestPermission();
    },
    subscribe: async function (vapidPublicKey) {
      const reg = await registerSw();
      if (!reg) throw new Error('service worker unavailable');
      // Stash the VAPID key in a SW-readable Cache so the
      // pushsubscriptionchange handler can re-subscribe without a live
      // client to ferry the key in. Without this, browser-initiated
      // rotations silently kill push delivery until the user manually
      // re-toggles permission.
      try {
        const cache = await caches.open('replider-push-meta');
        await cache.put(
          'vapid-public-key',
          new Response(vapidPublicKey, { headers: { 'Content-Type': 'text/plain' } }),
        );
      } catch (_) { /* best effort */ }
      let sub = await reg.pushManager.getSubscription();
      if (!sub) {
        sub = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlB64ToUint8(vapidPublicKey),
        });
      }
      return serialize(sub);
    },
    getSubscription: async function () {
      const reg = await currentRegistration();
      if (!reg) return null;
      const sub = await reg.pushManager.getSubscription();
      return serialize(sub);
    },
    unsubscribe: async function () {
      const reg = await currentRegistration();
      if (!reg) return null;
      const sub = await reg.pushManager.getSubscription();
      if (!sub) return null;
      const endpoint = sub.endpoint;
      await sub.unsubscribe();
      return endpoint;
    },
    // Exposed so Dart can wire an in-app router hook to notification clicks
    // without re-implementing the postMessage contract in every page.
    onClick: function (handler) {
      if (!('serviceWorker' in navigator)) return function () {};
      const listener = function (e) {
        const d = e && e.data;
        if (!d || d.type !== 'replider_push_click') return;
        try { handler(d.target || '/'); } catch (_) { /* swallow */ }
      };
      navigator.serviceWorker.addEventListener('message', listener);
      return function () {
        navigator.serviceWorker.removeEventListener('message', listener);
      };
    },
    // Subscription-rotation events fired by push-sw.js. The SW handles
    // pushsubscriptionchange and either:
    //   - successfully re-subscribed → posts {subscription} so Dart can
    //     POST it to /v1/push/subscriptions and replace the dead row;
    //   - couldn't re-subscribe (no cached VAPID, permission revoked) →
    //     pings with the "needs_resubscribe" signal so Dart can flip
    //     the toggle into a degraded state and prompt the user to
    //     re-enable from Settings.
    onSubscriptionChange: function (handler) {
      if (!('serviceWorker' in navigator)) return function () {};
      const listener = function (e) {
        const d = e && e.data;
        if (!d) return;
        if (d.type === 'replider_push_resubscribed') {
          try { handler({ resubscribed: true, subscription: d.subscription }); } catch (_) {}
        } else if (d.type === 'replider_push_needs_resubscribe') {
          try { handler({ resubscribed: false }); } catch (_) {}
        }
      };
      navigator.serviceWorker.addEventListener('message', listener);
      return function () {
        navigator.serviceWorker.removeEventListener('message', listener);
      };
    },
  };

  // ── PWA install helpers ────────────────────────────────────────────
  // Surface the Android `beforeinstallprompt` deferred event so Dart can
  // trigger the native install dialog at the right moment (e.g. user
  // tapped "Instalar app" in our own banner). On iOS Safari the event
  // never fires — Apple intentionally requires manual Add-to-Home-Screen
  // — so the helpers also expose enough OS / display-mode metadata for
  // Dart to render iOS-specific instructions.
  function isStandalone() {
    if (typeof window === 'undefined') return false;
    if (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches) return true;
    // iOS reports installed PWAs via navigator.standalone (legacy but
    // still required as of iOS 18 — display-mode media query reports
    // "browser" inside the installed PWA shell on some Safari builds).
    if (typeof navigator !== 'undefined' && navigator.standalone === true) return true;
    return false;
  }

  function detectOS() {
    const ua = (typeof navigator !== 'undefined' && navigator.userAgent) || '';
    if (/iPad|iPhone|iPod/.test(ua) && !window.MSStream) return 'ios';
    if (/Android/i.test(ua)) return 'android';
    return 'other';
  }

  function isMobile() {
    const os = detectOS();
    return os === 'ios' || os === 'android';
  }

  // iOS 16.4+ is the first Safari that supports Web Push. The browser
  // exposes Notification + PushManager only inside an installed PWA
  // (display-mode standalone). We surface a couple of hints so the
  // Dart-side push toggle can render "instale o app primeiro" copy
  // instead of a generic "no permission" error.
  function iosPushAvailable() {
    if (detectOS() !== 'ios') return true; // not iOS → not our problem
    if (!isStandalone()) return false; // must be installed
    return ('Notification' in window) && ('PushManager' in window);
  }

  window.repliderPwa = {
    isStandalone: isStandalone,
    os: detectOS,
    isMobile: isMobile,
    iosPushAvailable: iosPushAvailable,
    canPromptInstall: function () {
      return !!(window.__replider_pwa && window.__replider_pwa.deferredPrompt);
    },
    // Triggers the native Android install dialog. Resolves with the
    // user's choice ('accepted' / 'dismissed') or null when there's no
    // deferred prompt available (Safari, already-installed, prompt
    // already consumed, etc.).
    promptInstall: async function () {
      const ev = window.__replider_pwa && window.__replider_pwa.deferredPrompt;
      if (!ev) return null;
      ev.prompt();
      try {
        const choice = await ev.userChoice;
        // The deferred prompt is single-use — clear it so we don't
        // accidentally call .prompt() twice on the same event.
        window.__replider_pwa.deferredPrompt = null;
        return (choice && choice.outcome) || null;
      } catch (e) {
        window.__replider_pwa.deferredPrompt = null;
        return null;
      }
    },
  };
})();
