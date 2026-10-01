// jgk8: subagent path row under the thread strip.
//
//   Main › Implement 0e6y › Judge arrays r1 ▾ › Launched here (n) ▾  [link]
//
// The strip lists only Main's direct subagents, so this row is how deeper
// levels are found. Main and the depth-1 agent are links (the strip already
// lists depth-1 siblings); each deeper level is a dropdown of its siblings; a
// trailing "Launched here (n)" lists the open agent's children. Picking an
// ancestor lands on the launch row of the child you came from. A nested agent
// gets a copy-link button (it has no chip, so no chip menu). With unknown
// ancestry (a cold deep link) only "Launched here" shows. Renders nothing when
// there is neither a nested path nor a known child.

import { useCopyToClipboard } from '@/hooks';
import { CheckIcon, LinkIcon } from '@/components/icons';
import type { TranscriptThreadRef } from '@/providers/types';
import { cx } from '@/utils/utils';
import ThreadListDropdown from './ThreadListDropdown';
import { threadDeepLink } from './threadDetails';
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
  if (!nested && launched.length === 0) return null;

  const childCountOf = (threadId: string) => childrenOf(threadId).length;

  function renderAncestry(chain: readonly TranscriptThreadRef[]) {
    const [depth1, ...deeper] = chain;
    if (!depth1) return null;
    const current = chain.length - 1;
    return (
      <>
        <li className={styles.item}>
          <button type="button" className={styles.link} onClick={() => onSelect(null, depth1.launchTargetId)}>
            Main
          </button>
        </li>
        <li className={styles.item} aria-current={current === 0 ? 'location' : undefined}>
          {current === 0 ? (
            <span className={cx(styles.segmentLabel, styles.current)}>{depth1.label}</span>
          ) : (
            <button
              type="button"
              className={styles.link}
              aria-label={depth1.label}
              onClick={() => onSelect(depth1.id, chain[1]?.launchTargetId)}
            >
              <span className={styles.segmentLabel}>{depth1.label}</span>
            </button>
          )}
        </li>
        {deeper.map((segment, i) => {
          const level = i + 1;
          const parent = chain[level - 1]!;
          const known = childrenOf(parent.id);
          const siblings = known.some((t) => t.id === segment.id) ? known : [segment];
          return (
            <li key={segment.id} className={styles.item} aria-current={level === current ? 'location' : undefined}>
              <ThreadListDropdown
                threads={siblings}
                activeThreadId={segment.id}
                label={segment.label}
                listLabel={`Launched by ${parent.label}`}
                buttonContent={
                  <span className={cx(styles.segmentLabel, level === current && styles.current)}>{segment.label}</span>
                }
                childCountOf={childCountOf}
                align="start"
                triggerClassName={styles.dropdownTrigger}
                onSelect={(id) => {
                  if (id === null) return;
                  if (id !== segment.id) onSelect(id);
                  else if (level < current) onSelect(id, chain[level + 1]?.launchTargetId);
                }}
              />
            </li>
          );
        })}
      </>
    );
  }

  return (
    <nav aria-label="Subagent path" className={styles.bar}>
      <ol className={styles.list}>
        {path && renderAncestry(path)}
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
      {nested && (
        <button
          type="button"
          className={styles.copy}
          aria-label={copied ? 'Copied' : 'Copy link to this subagent'}
          onClick={() => void copy(threadDeepLink(sessionId, active.id)).catch(() => {})}
        >
          {copied ? CheckIcon : LinkIcon}
          {copied && <span className={styles.copiedText}>Copied</span>}
        </button>
      )}
    </nav>
  );
}
