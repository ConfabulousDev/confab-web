// jgk8: nested-subagent path row under the thread strip (D9 review: trim + inline).
//
//   ↳ ◔ Judge arrays r1 (43) ▾ › ◔ Re-run judge (1) ▾ › Launched here (n) ▾   ⧉
//
// Shown only when a nested agent (depth ≥ 2) is open. It starts at depth 2:
// Main and the depth-1 agent are already in the strip above (the depth-1 chip
// shows "in path"). Each segment is a dropdown of that level's siblings (the
// children of the level above) with the current one checked; picking an
// ancestor's current item lands on the launch row of the next agent down. A
// trailing "Launched here (n)" lists the open agent's children, then a
// copy-link button. With unknown ancestry (a cold deep link, D8) it shows only
// "↳ Launched here (n) ▾ ⧉", and only when children are known. A depth-1 agent
// never gets this row; its children sit in the strip's own Launched here.

import { useCopyToClipboard } from '@/hooks';
import { CheckIcon, LinkIcon, NestedArrowIcon } from '@/components/icons';
import type { TranscriptThreadRef } from '@/providers/types';
import { cx } from '@/utils/utils';
import ThreadListDropdown from './ThreadListDropdown';
import ThreadStatusGlyph from './ThreadStatusGlyph';
import { threadDeepLink, threadStatus } from './threadDetails';
import styles from './ThreadBreadcrumb.module.css';

const COPIED_MS = 800;

interface ThreadBreadcrumbProps {
  /** The open thread. */
  active: TranscriptThreadRef;
  /** Depth-1 first through `active`, when its ancestry is known; undefined otherwise. */
  path: readonly TranscriptThreadRef[] | undefined;
  /** Children known for a thread this visit. */
  childrenOf: (threadId: string) => readonly TranscriptThreadRef[];
  /** For the copied deep link. */
  sessionId: string;
  /** Switch threads; `targetId` is the row to land on. */
  onSelect: (threadId: string | null, targetId?: string) => void;
}

export default function ThreadBreadcrumb({ active, path, childrenOf, sessionId, onSelect }: ThreadBreadcrumbProps) {
  const { copy, copied } = useCopyToClipboard({ messageDuration: COPIED_MS });
  const launched = childrenOf(active.id);
  const nested = path !== undefined && path.length >= 2;
  if (!nested && (path !== undefined || launched.length === 0)) return null;

  const childCountOf = (threadId: string) => childrenOf(threadId).length;
  const last = (path?.length ?? 0) - 1;

  return (
    <nav aria-label="Subagent path" className={styles.bar}>
      <span className={styles.nestedMark} aria-hidden="true">
        {NestedArrowIcon}
      </span>
      <ol className={styles.list}>
        {nested &&
          path.slice(1).map((segment, i) => {
            const level = i + 1;
            const parent = path[level - 1]!;
            const known = childrenOf(parent.id);
            const siblings = known.some((t) => t.id === segment.id) ? known : [segment];
            const isCurrent = level === last;
            return (
              <li key={segment.id} className={styles.item} aria-current={isCurrent ? 'location' : undefined}>
                <ThreadListDropdown
                  threads={siblings}
                  activeThreadId={segment.id}
                  label={segment.label}
                  listLabel={`Launched by ${parent.label}`}
                  buttonContent={
                    <>
                      <ThreadStatusGlyph status={threadStatus(segment)} />
                      <span className={cx(styles.segmentLabel, isCurrent && styles.current)}>{segment.label}</span>
                      <span className={styles.count}>({siblings.length})</span>
                    </>
                  }
                  childCountOf={childCountOf}
                  align="start"
                  triggerClassName={styles.dropdownTrigger}
                  onSelect={(id) => {
                    if (id === null) return;
                    if (id !== segment.id) onSelect(id);
                    else if (!isCurrent) onSelect(id, path[level + 1]?.launchTargetId);
                  }}
                />
              </li>
            );
          })}
        {launched.length > 0 && (
          <li className={styles.item}>
            <ThreadListDropdown
              threads={launched}
              activeThreadId={null}
              label={`Launched here (${launched.length})`}
              listLabel={`Launched by ${active.label}`}
              childCountOf={childCountOf}
              align="start"
              triggerClassName={styles.dropdownTrigger}
              onSelect={(id) => {
                if (id !== null) onSelect(id);
              }}
            />
          </li>
        )}
      </ol>
      <button
        type="button"
        className={styles.copy}
        aria-label={copied ? 'Copied' : 'Copy link to this subagent'}
        onClick={() => void copy(threadDeepLink(sessionId, active.id)).catch(() => {})}
      >
        {copied ? CheckIcon : LinkIcon}
        {copied && <span className={styles.copiedText}>Copied</span>}
      </button>
    </nav>
  );
}
