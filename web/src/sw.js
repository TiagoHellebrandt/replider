// US-167: Service worker for repeat-visit performance.
//
// This file is the *source* for the worker. At build time the Astro
// integration in `integrations/service-worker.mjs` bundles it with esbuild and
// uses workbox-build's `injectManifest` to replace `self.__WB_MANIFEST` with a
// precache manifest of every hashed CSS/JS/font/image asset in `dist/`. The
// bundled, manifest-injected result is written to `dist/sw.js`.
//
// Caching strategy summary:
//   - HTML navigations  -> NetworkFirst (fresh content, fast offline repeat)
//   - CSS / JS          -> StaleWhileRevalidate (instant, updates in bg)
//   - Fonts             -> CacheFirst (immutable, long TTL)
//   - Images            -> CacheFirst with size/age cap
//   - Offline fallback  -> precached /offline.html for failed navigations
/* global self */
import { precacheAndRoute, matchPrecache } from "workbox-precaching";
import { registerRoute, setCatchHandler } from "workbox-routing";
import {
  NetworkFirst,
  CacheFirst,
  StaleWhileRevalidate,
} from "workbox-strategies";
import { ExpirationPlugin } from "workbox-expiration";
import { CacheableResponsePlugin } from "workbox-cacheable-response";
import { clientsClaim } from "workbox-core";

// Take control of open clients as soon as the new worker activates so repeat
// visitors get the caching behaviour without a second reload.
self.skipWaiting();
clientsClaim();

// Precache hashed build assets (manifest injected at build time).
precacheAndRoute(self.__WB_MANIFEST || []);

// HTML: NetworkFirst so content stays fresh, but a cached copy keeps repeat
// visits fast and survives flaky connections. 3s timeout falls back to cache.
registerRoute(
  ({ request }) => request.mode === "navigate",
  new NetworkFirst({
    cacheName: "html-pages",
    networkTimeoutSeconds: 3,
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
      new ExpirationPlugin({ maxEntries: 50, maxAgeSeconds: 60 * 60 * 24 }),
    ],
  }),
);

// CSS / JS / web workers: StaleWhileRevalidate — serve instantly from cache,
// refresh in the background.
registerRoute(
  ({ request }) =>
    request.destination === "style" ||
    request.destination === "script" ||
    request.destination === "worker",
  new StaleWhileRevalidate({
    cacheName: "static-resources",
    plugins: [new CacheableResponsePlugin({ statuses: [0, 200] })],
  }),
);

// Google Fonts stylesheet: SWR (small, occasionally changes).
registerRoute(
  ({ url }) => url.origin === "https://fonts.googleapis.com",
  new StaleWhileRevalidate({ cacheName: "google-fonts-stylesheets" }),
);

// Fonts (Google gstatic files + any same-origin font): CacheFirst, long TTL —
// font files are content-hashed/immutable.
registerRoute(
  ({ url, request }) =>
    url.origin === "https://fonts.gstatic.com" ||
    request.destination === "font",
  new CacheFirst({
    cacheName: "fonts",
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
      new ExpirationPlugin({
        maxEntries: 30,
        maxAgeSeconds: 60 * 60 * 24 * 365,
      }),
    ],
  }),
);

// Images: CacheFirst with a bounded cache so disk usage stays sane.
registerRoute(
  ({ request }) => request.destination === "image",
  new CacheFirst({
    cacheName: "images",
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
      new ExpirationPlugin({
        maxEntries: 60,
        maxAgeSeconds: 60 * 60 * 24 * 30,
      }),
    ],
  }),
);

// Offline fallback: when a navigation fails and nothing is cached, serve the
// precached offline page instead of the browser's dino error.
setCatchHandler(async ({ request }) => {
  if (request.mode === "navigate") {
    const cached = await matchPrecache("/offline.html");
    if (cached) return cached;
  }
  return Response.error();
});
