import { describe, it, expect } from 'vitest';
import { mergeMessages } from '../src/merge';
import type { Message } from '../src/api';

const m = (id: string, t: string, direction: 'inbound' | 'outbound' = 'inbound'): Message => ({
  id,
  direction,
  text: 'x',
  created_at: t,
});

describe('mergeMessages', () => {
  it('dedupes by id, newer wins on overlap', () => {
    const prev = [m('1', '2026-04-14T00:00:00Z'), m('2', '2026-04-14T00:00:01Z')];
    const incoming = [m('2', '2026-04-14T00:00:01Z'), m('3', '2026-04-14T00:00:02Z')];
    const merged = mergeMessages(prev, incoming);
    expect(merged.map((x) => x.id)).toEqual(['1', '2', '3']);
  });

  it('sorts by created_at', () => {
    const merged = mergeMessages([], [
      m('b', '2026-04-14T00:00:02Z'),
      m('a', '2026-04-14T00:00:01Z'),
    ]);
    expect(merged.map((x) => x.id)).toEqual(['a', 'b']);
  });

  it('handles empty inputs', () => {
    expect(mergeMessages([], [])).toEqual([]);
    expect(mergeMessages([m('1', '2026-04-14T00:00:00Z')], [])).toHaveLength(1);
    expect(mergeMessages([], [m('1', '2026-04-14T00:00:00Z')])).toHaveLength(1);
  });
});
