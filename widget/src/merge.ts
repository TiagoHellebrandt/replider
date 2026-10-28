import type { Message } from './api';

/**
 * Merges two arrays of messages by id, preferring the `incoming` version on
 * id collisions. Returned list is sorted by `created_at` ascending.
 *
 * Used by the polling loop: each poll may redeliver messages we already
 * rendered (the server uses a cursor which can include the boundary
 * message). We also reconcile optimistic local messages with the canonical
 * server version that arrives with the final id on the next poll.
 */
export function mergeMessages(existing: Message[], incoming: Message[]): Message[] {
  const byId = new Map<string, Message>();
  for (const m of existing) byId.set(m.id, m);
  for (const m of incoming) byId.set(m.id, m);
  return Array.from(byId.values()).sort((a, b) => a.created_at.localeCompare(b.created_at));
}
