import { useCallback, useEffect, useState, type RefObject } from 'react';
import type { Virtualizer } from '@tanstack/react-virtual';

/**
 * cca0: track the topmost visible row of type `rowType`, so a pane's
 * TimelineBar position indicator has something to point at when the user
 * hasn't hovered a row. Scans the virtualizer's visible rows in order, skips
 * other row types (divider/separator rows), and reports the first match's
 * `index` (whatever index axis the pane's builder tags rows with). Re-scans
 * on every scroll of `scrollRef` (passive listener) and once on attach.
 * Starts at 0, and keeps the last reported value while no matching row is
 * visible.
 *
 * `rowType` is a string discriminator rather than a predicate callback so an
 * inline function can't re-subscribe the listener on every render.
 */
export function useFirstVisibleIndex(
  scrollRef: RefObject<HTMLElement | null>,
  virtualizer: Pick<Virtualizer<HTMLDivElement, Element>, 'getVirtualItems'>,
  virtualItems: ReadonlyArray<{ type: string; index?: number }>,
  rowType: string,
): number {
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);

  const updateFirstVisible = useCallback(() => {
    for (const vItem of virtualizer.getVirtualItems()) {
      const row = virtualItems[vItem.index];
      if (row?.type === rowType && row.index !== undefined) {
        setFirstVisibleIndex(row.index);
        return;
      }
    }
  }, [virtualizer, virtualItems, rowType]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener('scroll', updateFirstVisible, { passive: true });
    updateFirstVisible();
    return () => el.removeEventListener('scroll', updateFirstVisible);
  }, [scrollRef, updateFirstVisible]);

  return firstVisibleIndex;
}
