import { render, waitFor, cleanup } from '@testing-library/preact';
import { afterEach, describe, expect, it } from 'vitest';
import { Widget } from '../src/WidgetApp';
import type { ApiClient, BootstrapConfig } from '../src/api';
import type { WidgetStorage } from '../src/storage';

// Minimal in-memory storage stub.
function fakeStorage(): WidgetStorage {
  const mem = new Map<string, string>();
  return {
    getVisitorId: () => 'v-test',
    getCursor: () => mem.get('cursor'),
    setCursor: (ts) => void mem.set('cursor', ts),
    getCsatDone: () => mem.get('csat') === '1',
    setCsatDone: () => void mem.set('csat', '1'),
    reset: () => mem.clear(),
  };
}

// API stub returning a fixed bootstrap; poll never resolves new messages.
function fakeApi(bootstrap: BootstrapConfig): ApiClient {
  return {
    bootstrap: () => Promise.resolve(bootstrap),
    send: () => Promise.resolve({ conversation_id: 'c', message_id: 'm' }),
    poll: () => Promise.resolve({ messages: [], now: '2026-01-01T00:00:00Z' }),
    recordCsat: () => Promise.resolve(),
  };
}

const baseCfg: BootstrapConfig = {
  title: 'Suporte',
  welcome_message: 'Oi',
  primary_color: '#25D366',
};

afterEach(() => cleanup());

describe('Powered by Replider badge (US-172)', () => {
  it('renders the badge when show_branding is true and the panel is open', async () => {
    const { getByTestId, queryByTestId } = render(
      <Widget api={fakeApi({ ...baseCfg, show_branding: true })} storage={fakeStorage()} />,
    );
    // Open the panel.
    (getByTestId('rw-bubble') as HTMLButtonElement).click();
    await waitFor(() => {
      const badge = queryByTestId('rw-branding') as HTMLAnchorElement | null;
      expect(badge).not.toBeNull();
      expect(badge!.getAttribute('href')).toBe('https://replider.com?utm_source=embed');
      expect(badge!.getAttribute('rel')).toContain('noopener');
    });
  });

  it('hides the badge when show_branding is false (paid plan)', async () => {
    const { getByTestId, queryByTestId } = render(
      <Widget api={fakeApi({ ...baseCfg, show_branding: false })} storage={fakeStorage()} />,
    );
    (getByTestId('rw-bubble') as HTMLButtonElement).click();
    // Wait for bootstrap to settle, then assert the badge stays absent.
    await waitFor(() => expect(getByTestId('rw-panel')).toBeTruthy());
    expect(queryByTestId('rw-branding')).toBeNull();
  });
});
