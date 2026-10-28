import { describe, it, expect, vi } from 'vitest';
import { createApiClient, WidgetApiError } from '../src/api';

function mockFetch(response: { status: number; body: unknown }) {
  return vi.fn().mockResolvedValue({
    ok: response.status < 400,
    status: response.status,
    json: async () => response.body,
  } as Response);
}

describe('createApiClient', () => {
  it('rejects invalid channel tokens (defense-in-depth vs path injection)', () => {
    expect(() => createApiClient({ baseUrl: 'https://api.example.com', channelToken: 'a/b' })).toThrow();
    expect(() => createApiClient({ baseUrl: 'https://api.example.com', channelToken: '' })).toThrow();
    expect(() => createApiClient({ baseUrl: 'https://api.example.com', channelToken: 'evil?foo=bar' })).toThrow();
  });

  it('accepts valid ULID-shaped tokens', () => {
    expect(() =>
      createApiClient({ baseUrl: 'https://api.example.com', channelToken: '01HXYZ123ABC' }),
    ).not.toThrow();
  });

  it('strips trailing slashes from baseUrl', async () => {
    const fetchImpl = mockFetch({ status: 200, body: { title: 't', welcome_message: '', primary_color: '' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com/v1/',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await client.bootstrap();
    expect(fetchImpl).toHaveBeenCalledWith(
      'https://api.example.com/v1/widget/tok/bootstrap',
      expect.any(Object),
    );
  });

  it('omits credentials to avoid leaking host-site cookies', async () => {
    const fetchImpl = mockFetch({ status: 200, body: { title: '', welcome_message: '', primary_color: '' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await client.bootstrap();
    const opts = fetchImpl.mock.calls[0][1];
    expect(opts.credentials).toBe('omit');
    expect(opts.mode).toBe('cors');
  });

  it('parses server error body into WidgetApiError', async () => {
    const fetchImpl = mockFetch({ status: 403, body: { code: 'ORIGIN_NOT_ALLOWED', message: 'nope' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await expect(client.bootstrap()).rejects.toMatchObject({
      status: 403,
      code: 'ORIGIN_NOT_ALLOWED',
      message: 'nope',
    });
    await expect(client.bootstrap()).rejects.toBeInstanceOf(WidgetApiError);
  });

  it('falls back gracefully when error body is not JSON', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
      json: async () => {
        throw new Error('not json');
      },
    } as unknown as Response);
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await expect(client.bootstrap()).rejects.toMatchObject({ status: 502, code: 'UNKNOWN' });
  });

  it('send posts JSON body with visitor_id + text', async () => {
    const fetchImpl = mockFetch({ status: 202, body: { conversation_id: 'c', message_id: 'm' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await client.send('v-1', 'Olá!');
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('https://api.example.com/widget/tok/messages');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(opts.body)).toEqual({ visitor_id: 'v-1', text: 'Olá!' });
  });

  it('send includes name when provided', async () => {
    const fetchImpl = mockFetch({ status: 202, body: { conversation_id: 'c', message_id: 'm' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await client.send('v-1', 'oi', 'Maria');
    const body = JSON.parse(fetchImpl.mock.calls[0][1].body);
    expect(body.name).toBe('Maria');
  });

  it('poll builds URL with visitor_id query param', async () => {
    const fetchImpl = mockFetch({ status: 200, body: { messages: [], now: '2026-01-01T00:00:00Z' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await client.poll('v-1');
    expect(fetchImpl.mock.calls[0][0]).toBe('https://api.example.com/widget/tok/messages?visitor_id=v-1');
  });

  it('poll includes since cursor when provided', async () => {
    const fetchImpl = mockFetch({ status: 200, body: { messages: [], now: '' } });
    const client = createApiClient({
      baseUrl: 'https://api.example.com',
      channelToken: 'tok',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await client.poll('v-1', '2026-04-14T00:00:00.000Z');
    const url = fetchImpl.mock.calls[0][0] as string;
    expect(url).toContain('visitor_id=v-1');
    expect(url).toContain('since=2026-04-14T00%3A00%3A00.000Z');
  });
});
