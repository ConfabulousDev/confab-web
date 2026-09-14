// we3k D3/D4: status glyph for a transcript thread. The shape differs per status
// so it never relies on color alone (callers also put the status word in text).
// The running ring's rotating arc is the strip's only ambient motion and stops
// under `prefers-reduced-motion`.

import type { ReactElement } from 'react';
import type { TranscriptThreadStatus } from '@/providers/types';
import { cx } from '@/utils/utils';
import styles from './ThreadStatusGlyph.module.css';

const GLYPHS: Record<TranscriptThreadStatus, ReactElement> = {
  running: (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" className={styles.spinner}>
      <circle cx="5" cy="5" r="3.75" stroke="currentColor" strokeWidth="1.5" opacity="0.3" />
      <path d="M5 1.25a3.75 3.75 0 0 1 3.75 3.75" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  ),
  completed: (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
      <polyline
        points="1.75 5.25 4 7.5 8.25 2.5"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  ),
  failed: (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
      <path d="M2.5 2.5l5 5M7.5 2.5l-5 5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  ),
  stopped: (
    <svg width="10" height="10" viewBox="0 0 10 10">
      <rect x="2.5" y="2.5" width="5" height="5" rx="1" fill="currentColor" />
    </svg>
  ),
  unknown: (
    <svg width="10" height="10" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="2.25" fill="currentColor" />
    </svg>
  ),
};

export default function ThreadStatusGlyph({ status }: { status: TranscriptThreadStatus }) {
  return (
    <span className={cx(styles.glyph, styles[status])} data-status={status} aria-hidden="true">
      {GLYPHS[status]}
    </span>
  );
}
