import { beforeEach, describe, it, expect } from 'vitest';
import { createStorage } from '../src/storage';

describe('createStorage', () => {
  beforeEach(() => {
    globalThis.localStorage?.clear();
  });

  it('generates a stable visitor id on first call', () => {
    const s = createStorage('tok-1');
    const id1 = s.getVisitorId();
    const id2 = s.getVisitorId();
    expect(id1).toBe(id2);
    expect(id1.length).toBeGreaterThan(5);
  });

  it('scopes ids per channel token', () => {
    const a = createStorage('tok-A').getVisitorId();
    const b = createStorage('tok-B').getVisitorId();
    expect(a).not.toBe(b);
  });

  it('persists cursor across instances with same token', () => {
    const s1 = createStorage('tok-2');
    s1.setCursor('2026-04-14T00:00:00Z');
    const s2 = createStorage('tok-2');
    expect(s2.getCursor()).toBe('2026-04-14T00:00:00Z');
  });

  it('reset wipes visitor id and cursor', () => {
    const s = createStorage('tok-3');
    const before = s.getVisitorId();
    s.setCursor('2026-01-01T00:00:00Z');
    s.reset();
    expect(s.getCursor()).toBeUndefined();
    const after = s.getVisitorId();
    expect(after).not.toBe(before);
  });

  it('visitor id format is safe for the server regex', () => {
    // Server allows ^[A-Za-z0-9._-]{1,128}$
    const allowed = /^[A-Za-z0-9._-]{1,128}$/;
    const s = createStorage('tok-fmt');
    expect(allowed.test(s.getVisitorId())).toBe(true);
  });
});
