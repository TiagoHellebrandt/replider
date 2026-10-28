// Widget-local state persisted in the customer's browser.
//
// We persist only a random visitor id and a polling cursor (last-seen
// timestamp) — never message content or contact information. This avoids
// leaking chat history if the hosting site is compromised via XSS.

const KEY_PREFIX = 'replider_widget_';

function key(channelToken: string, suffix: string): string {
  return `${KEY_PREFIX}${channelToken}_${suffix}`;
}

function safeRandomId(): string {
  // Prefer crypto.randomUUID when available (all modern browsers).
  // Fallback generates 16 hex chars from Math.random — acceptable because
  // visitor_id is not a security boundary (the server authenticates by
  // channel_token + origin, not by visitor_id).
  const cryptoImpl = typeof globalThis.crypto !== 'undefined' ? globalThis.crypto : undefined;
  if (cryptoImpl?.randomUUID) return `v-${cryptoImpl.randomUUID()}`;
  const part = () => Math.floor(Math.random() * 0xffffffff).toString(16).padStart(8, '0');
  return `v-${part()}${part()}`;
}

export interface WidgetStorage {
  getVisitorId(): string;
  getCursor(): string | undefined;
  setCursor(ts: string): void;
  // CSAT already submitted for this visitor. Persisted so the prompt
  // doesn't reappear on every page reload after the visitor rated once.
  getCsatDone(): boolean;
  setCsatDone(): void;
  reset(): void;
}

/**
 * LocalStorage-backed widget storage. On environments where localStorage
 * is unavailable (private browsing, very old browsers), a per-page in-memory
 * fallback is used — visitor keeps the same id during the session but a
 * reload starts a fresh conversation.
 */
export function createStorage(channelToken: string): WidgetStorage {
  const ls = safeLocalStorage();
  const memory = new Map<string, string>();

  const get = (k: string): string | undefined => {
    const ns = key(channelToken, k);
    return ls ? ls.getItem(ns) ?? undefined : memory.get(ns);
  };
  const set = (k: string, v: string): void => {
    const ns = key(channelToken, k);
    if (ls) ls.setItem(ns, v);
    else memory.set(ns, v);
  };
  const remove = (k: string): void => {
    const ns = key(channelToken, k);
    if (ls) ls.removeItem(ns);
    else memory.delete(ns);
  };

  return {
    getVisitorId(): string {
      let id = get('visitor');
      if (!id) {
        id = safeRandomId();
        set('visitor', id);
      }
      return id;
    },
    getCursor(): string | undefined {
      return get('cursor');
    },
    setCursor(ts: string): void {
      set('cursor', ts);
    },
    getCsatDone(): boolean {
      return get('csat_done') === '1';
    },
    setCsatDone(): void {
      set('csat_done', '1');
    },
    reset(): void {
      remove('visitor');
      remove('cursor');
      remove('csat_done');
    },
  };
}

function safeLocalStorage(): Storage | undefined {
  try {
    const ls = globalThis.localStorage;
    // Some browsers throw on getItem in private mode.
    const probe = '__replider_probe__';
    ls.setItem(probe, '1');
    ls.removeItem(probe);
    return ls;
  } catch {
    return undefined;
  }
}
