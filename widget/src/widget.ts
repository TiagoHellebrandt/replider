// Entry point for the embeddable widget.
//
// Users embed a single script tag like:
//
//   <script async src="https://cdn.replider.com/widget.js"
//           data-token="01H..." data-api="https://api.replider.com/v1"></script>
//
// We read configuration from the script's data-* attributes and mount the
// widget into a dedicated root element appended to <body>. We deliberately do
// NOT auto-mount on document.body itself so hosting sites can restyle without
// breaking our shadow tree.
//
// Exported programmatically as `window.ReplyderWidget.mount({token, apiUrl})`
// for single-page apps or hosts that prefer explicit control.

import { render, h } from 'preact';
import { Widget } from './WidgetApp';
import { createApiClient } from './api';
import { createStorage } from './storage';
import './styles.css';

export interface MountOptions {
  token: string;
  apiUrl: string; // e.g. https://api.replider.com/v1
  root?: HTMLElement; // default: new div appended to body
  pollIntervalMs?: number;
}

export function mount(opts: MountOptions): () => void {
  const api = createApiClient({ baseUrl: opts.apiUrl, channelToken: opts.token });
  const storage = createStorage(opts.token);

  const root = opts.root ?? createDefaultRoot();
  render(
    h(Widget, {
      api,
      storage,
      pollIntervalMs: opts.pollIntervalMs,
    }),
    root,
  );
  return () => {
    render(null, root);
    if (!opts.root) root.remove();
  };
}

function createDefaultRoot(): HTMLElement {
  const el = document.createElement('div');
  el.id = 'replider-widget-root';
  document.body.appendChild(el);
  return el;
}

// Auto-mount when loaded via <script src=... data-token=... data-api=...>
function autoMountFromScript(): void {
  const current = (document.currentScript ?? lastReplyderScript()) as HTMLScriptElement | null;
  if (!current) return;
  const token = current.dataset.token;
  const apiUrl = current.dataset.api;
  if (!token || !apiUrl) return; // nothing to do — caller will use mount() manually

  // Auto-inject the stylesheet sibling so the host site only needs ONE
  // <script> tag. We derive the CSS URL from the script's own URL so it
  // works regardless of the CDN host (cdn.replider.com in prod, localhost
  // in dev, customer-hosted mirror, etc).
  ensureStylesheet(current.src);

  try {
    mount({ token, apiUrl });
  } catch (err) {
    // Fail silently — a broken widget must not crash the host page.
    // eslint-disable-next-line no-console
    console.warn('[replider] failed to auto-mount:', err);
  }
}

const STYLESHEET_MARKER = 'data-replider-widget-css';

function ensureStylesheet(scriptSrc: string): void {
  if (document.querySelector(`link[${STYLESHEET_MARKER}]`)) return;
  // Replace ".js" → ".css" preserving query string / hash for cache busting.
  const cssHref = scriptSrc.replace(/widget\.js(\?.*|#.*)?$/, 'widget.css$1');
  if (cssHref === scriptSrc) return; // pattern didn't match — bail rather than ship a wrong URL
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = cssHref;
  link.setAttribute(STYLESHEET_MARKER, '');
  document.head.appendChild(link);
}

function lastReplyderScript(): HTMLScriptElement | null {
  const scripts = document.getElementsByTagName('script');
  for (let i = scripts.length - 1; i >= 0; i--) {
    const s = scripts[i];
    if (s.dataset && s.dataset.token && s.dataset.api) return s;
  }
  return null;
}

// The IIFE bundle executes this at load time.
if (typeof document !== 'undefined' && document.readyState !== 'loading') {
  autoMountFromScript();
} else if (typeof document !== 'undefined') {
  document.addEventListener('DOMContentLoaded', autoMountFromScript);
}

// Expose programmatic API for SPA hosts.
// (Vite's IIFE build assigns this file's exports to window.ReplyderWidget.)
