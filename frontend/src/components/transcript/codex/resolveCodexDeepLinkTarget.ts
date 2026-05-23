// CF-475: Resolve a Codex transcript deep-link target to a render-item index.
//
// The `?msg=` query-param value for a Codex session is an ISO 8601 timestamp
// (sourced from the targeted render item's `timestamp` field — see
// `codexRenderItem.ts`). The resolver returns the index in `items` of the
// latest item whose `timestamp <= target`. If `target` precedes every item,
// returns 0 ("closest row, never not-found").
//
// Returns null when `target` is empty / unparseable, when `items` is empty,
// or when `target` is a bare digit string (legacy lineId-style values from
// pre-CF-475 URLs would otherwise be interpreted as years AD by lenient
// `Date.parse` engines).
import type { CodexRenderItem } from '@/types/codexRenderItem';

export function resolveCodexDeepLinkTarget(
  items: CodexRenderItem[],
  target: string,
): number | null {
  if (!target) return null;
  if (items.length === 0) return null;
  if (/^\d+$/.test(target)) return null;

  const targetMs = Date.parse(target);
  if (Number.isNaN(targetMs)) return null;

  let latest: number | null = null;
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (!item) continue;
    const itemMs = Date.parse(item.timestamp);
    if (Number.isNaN(itemMs)) continue;
    if (itemMs <= targetMs) latest = i;
  }

  // Target precedes every item — clamp to the first row so the user always
  // lands somewhere meaningful.
  return latest ?? 0;
}
