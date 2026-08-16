import type { Command } from '@keyhole-koro/paper-in-paper';

/**
 * The worker already splits commands into these two fields, and this checks the
 * result again on arrival.
 *
 * Not redundant: `commands` is dispatched without the reader seeing it first,
 * so the field that decides "apply silently" is validated on both sides. A bug
 * on either one alone cannot get a structural change auto-applied.
 *
 * Kept out of the hook module so it can be tested without pulling in Firebase.
 */
export const AUTO_COMMAND_TYPES: ReadonlySet<string> = new Set([
  'OPEN_NODE',
  'CLOSE_NODE',
  'FOCUS_NODE',
  'PIN_NODE',
  'UNPIN_NODE',
]);

export const PROPOSAL_COMMAND_TYPES: ReadonlySet<string> = new Set([
  'CREATE_CHILD_NODE',
  'PATCH_NODE',
  'MOVE_NODE',
  'MERGE_PAPERS',
  'UPSERT_PAPERS',
]);

/**
 * Parses a command list, keeping only the types permitted for that field.
 *
 * DELETE_NODE and the internal __SYNC_* commands are in neither set, so they
 * are dropped wherever they appear.
 */
export function parseCommands(raw: string | undefined, permitted: ReadonlySet<string>): Command[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (c): c is Command =>
        typeof c === 'object' &&
        c !== null &&
        'type' in c &&
        typeof (c as { type: unknown }).type === 'string' &&
        permitted.has((c as { type: string }).type),
    );
  } catch {
    return [];
  }
}
