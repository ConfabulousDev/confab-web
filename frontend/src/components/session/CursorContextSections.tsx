// 0rcv: collapsible injected-context sections for a Cursor user row.
//
// A Cursor user `text` block is an envelope: the human prompt lives in
// `<user_query>`, but the model also receives injected context — user rules,
// attached files, manually attached skills, system reminders, and so on. nfbe
// parses everything that is NOT the prompt into `sections` (tag + humanized
// label + opaque content). Showing those blocks inline (as the raw envelope did)
// is unreadable; hiding them loses audit context. So each section renders as a
// native `<details>` disclosure, collapsed by default: the prompt reads cleanly
// above, and the context is one click away.
//
// Spirit mirrors Claude's skill/command-expansion handling
// (ClaudeTimelineMessage.tsx) — the prompt is primary, the scaffolding folds
// away. v1 renders section bodies as plain preformatted text (no syntax
// highlight or image preview — that is an explicit follow-up).

import type { ReactElement } from 'react';
import type { CursorUserSection } from './cursorCategories';
import styles from './CursorContextSections.module.css';

interface CursorContextSectionsProps {
  /** Injected-context sections parsed off the user envelope (nfbe). */
  sections: CursorUserSection[];
}

/** Render each injected-context section as a collapsed-by-default disclosure.
 *  Returns null when there are no sections so the user row shows only its
 *  prompt (the common case — most user turns carry no injected context). */
export default function CursorContextSections({
  sections,
}: CursorContextSectionsProps): ReactElement | null {
  if (sections.length === 0) return null;

  return (
    <div className={styles.sections}>
      {sections.map((section, i) => (
        <details key={i} className={styles.section}>
          <summary className={styles.summary}>{section.label}</summary>
          {/* Content is opaque text — any tags inside (even a stray
              `<user_query>`) render as literal characters via text node, never
              as real elements. */}
          <pre className={styles.content}>{section.content}</pre>
        </details>
      ))}
    </div>
  );
}
