// API client for the Replider webchat widget.
//
// The widget talks to the public /widget/:token/ endpoints exposed by the
// Replider API. All requests are unauthenticated — identity is:
//   - the channel_token in the URL path (identifies the tenant)
//   - the visitor_id chosen by the widget and persisted in localStorage
//
// Security notes:
// - The visitor_id is client-generated and opaque to the server, so the widget
//   MUST NEVER put anything sensitive in it (it's effectively a cookie).
// - CORS is enforced by the server, not by the widget — the widget only
//   surfaces errors to the UI.

export interface BootstrapConfig {
  title: string;
  welcome_message: string;
  primary_color: string;
  // When true (free/trial accounts), the widget renders a discreet
  // "Powered by Replider" badge. Paid plans turn it off server-side.
  // Optional for backwards-compat with older API responses.
  show_branding?: boolean;
}

export interface Message {
  id: string;
  direction: 'inbound' | 'outbound';
  text: string;
  created_at: string; // RFC 3339
}

export interface PollResponse {
  messages: Message[];
  now: string;
}

export interface SendResponse {
  conversation_id: string;
  message_id: string;
}

export interface ApiError {
  code: string;
  message: string;
}

export class WidgetApiError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export interface ApiClientOptions {
  baseUrl: string; // e.g. https://api.replider.com/v1
  channelToken: string;
  fetchImpl?: typeof fetch; // injectable for tests
}

/**
 * Returns a fully-formed widget API client bound to a single channel token.
 * We bind the token in closure so no part of the widget can accidentally
 * re-use a different token.
 */
export function createApiClient(opts: ApiClientOptions) {
  const { baseUrl, channelToken } = opts;
  const fetchImpl = opts.fetchImpl ?? fetch;

  if (!/^[A-Za-z0-9_-]+$/.test(channelToken)) {
    // ULID tokens are [0-9A-Z] but we allow a bit wider for testability.
    // Refuse anything that could path-inject.
    throw new Error('invalid channel token');
  }

  const endpoint = (path: string) =>
    `${baseUrl.replace(/\/+$/, '')}/widget/${channelToken}${path}`;

  async function request<T>(
    method: 'GET' | 'POST',
    path: string,
    body?: unknown,
  ): Promise<T> {
    const res = await fetchImpl(endpoint(path), {
      method,
      // Credentials deliberately excluded: the widget is unauthenticated
      // and does not carry session cookies of the hosting site.
      credentials: 'omit',
      mode: 'cors',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      let code = 'UNKNOWN';
      let message = `HTTP ${res.status}`;
      try {
        const data = (await res.json()) as ApiError;
        if (data && typeof data.code === 'string') code = data.code;
        if (data && typeof data.message === 'string') message = data.message;
      } catch {
        // body wasn't JSON — keep defaults
      }
      throw new WidgetApiError(res.status, code, message);
    }

    // 204 No Content
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  return {
    bootstrap(): Promise<BootstrapConfig> {
      return request<BootstrapConfig>('GET', '/bootstrap');
    },

    send(visitorId: string, text: string, name?: string): Promise<SendResponse> {
      return request<SendResponse>('POST', '/messages', {
        visitor_id: visitorId,
        text,
        ...(name ? { name } : {}),
      });
    },

    poll(visitorId: string, since?: string): Promise<PollResponse> {
      const qs = new URLSearchParams({ visitor_id: visitorId });
      if (since) qs.set('since', since);
      return request<PollResponse>('GET', `/messages?${qs.toString()}`);
    },

    // Submits a 1-5 satisfaction rating for the visitor's own conversation.
    // Server validates the visitor_id owns a conversation on this channel,
    // so spoofing another visitor's rating isn't possible even though the
    // endpoint is unauthenticated.
    recordCsat(visitorId: string, rating: number, feedback?: string): Promise<void> {
      return request<void>('POST', '/csat', {
        visitor_id: visitorId,
        rating,
        ...(feedback ? { feedback } : {}),
      });
    },
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;
