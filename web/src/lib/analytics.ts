// Thin client-side wrapper around the consent-gated tracking pixels wired up
// in Layout.astro. The actual fbq/gtag loaders only run AFTER the visitor
// accepts cookies, so `window.trackEvent` may be undefined (consent declined
// or pixels not configured) — in which case every call is a safe no-op.
//
// Standard event names used across the site:
//   - PageView         (auto-fired by the loaders)
//   - ViewContent      (auto-fired by the loaders)
//   - Lead             (newsletter subscribe success)
//   - InitiateCheckout (Register / Start CTA click)
declare global {
  interface Window {
    trackEvent?: (name: string, params?: Record<string, unknown>) => void;
  }
}

export function trackEvent(
  name: string,
  params?: Record<string, unknown>,
): void {
  if (
    typeof window !== "undefined" &&
    typeof window.trackEvent === "function"
  ) {
    window.trackEvent(name, params);
  }
}
