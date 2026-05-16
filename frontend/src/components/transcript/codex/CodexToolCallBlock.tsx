// Renders a paired Codex tool call + output.
//
// Dispatch by `toolName`:
//   exec_command       command + BashOutput (ANSI-stripped, terminal styling)
//   apply_patch        file-list summary + CodeBlock(language="diff")
//   web_search_call    query chip(s)
//   <anything else>    generic Input / Output via CodeBlock (json / plain)

import type { CodexToolCallItem } from '@/types/codexRenderItem';
import { tryParseAsJson } from '@/utils';
import { renderTextWithHighlight } from '@/utils/renderHighlight';
import { cx } from '@/utils/utils';
import BashOutput from '../BashOutput';
import CodeBlock from '../CodeBlock';
import {
  formatCodexTimestamp,
  leafFileName,
  stringifyForDisplay,
} from './codexFormat';
import CodexRowActions from './CodexRowActions';
import {
  buildToolCallCopyText,
  readPatchChanges,
  readStringField,
  readWebSearchQueries,
} from './codexToolCallHelpers';
import styles from './CodexToolCallBlock.module.css';

export interface CodexToolCallBlockProps {
  item: CodexToolCallItem;
  /**
   * Session ID for the per-row copy-link URL. Optional so the renderer can
   * be used in isolation; timeline always passes it in production.
   */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /**
   * Speaker continuity flag. tool_call items never break user/assistant
   * continuity, so this prop is accepted for shape uniformity but does
   * not affect rendering.
   */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** Skip to next same-kind tool call (CF-360 — split by toolName). */
  onSkipToNext?: () => void;
  /** Skip to previous same-kind tool call (CF-360). */
  onSkipToPrevious?: () => void;
  /** Human-readable kind for aria-label (CF-360). */
  kindLabel?: string;
  /** CF-359: transcript search query — wraps matches in `<mark>` inside the body. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexToolCallBlock({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  onSkipToNext,
  onSkipToPrevious,
  kindLabel,
  searchQuery,
  isCurrentSearchMatch,
}: CodexToolCallBlockProps) {
  const className = cx(
    styles.toolCall,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  return (
    <div className={className} data-kind="tool_call" data-tool={item.toolName}>
      <div className={styles.header}>
        <span className={styles.toolName}>{toolNameLabel(item)}</span>
        {renderStatusBadge(item)}
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
        {sessionId && (
          <CodexRowActions
            sessionId={sessionId}
            lineId={item.lineId}
            copyText={buildToolCallCopyText(item)}
            onSkipToNext={onSkipToNext}
            onSkipToPrevious={onSkipToPrevious}
            kindLabel={kindLabel ?? item.toolName}
          />
        )}
      </div>
      {renderBody(item, { searchQuery, isCurrentSearchMatch })}
    </div>
  );
}

// Display labels for known tool names. Anything not listed gets a generic
// "Tool: <name>" prefix so unfamiliar future tools still stand out.
const TOOL_NAME_LABELS: Record<string, string> = {
  exec_command: 'exec_command',
  apply_patch: 'apply_patch',
  web_search_call: 'web_search',
};

function toolNameLabel(item: CodexToolCallItem): string {
  return TOOL_NAME_LABELS[item.toolName] ?? `Tool: ${item.toolName}`;
}

function renderStatusBadge(item: CodexToolCallItem) {
  if (item.toolName === 'exec_command' && item.execMetadata) {
    const cls = item.execMetadata.exitCode === 0 ? styles.exitOk : styles.exitFail;
    return (
      <span className={cx(styles.badge, cls)}>
        exit {item.execMetadata.exitCode}
        {item.execMetadata.wallTimeMs > 0
          ? ` · ${item.execMetadata.wallTimeMs}ms`
          : null}
      </span>
    );
  }
  // For pending status we let the body's "pending — no output yet" line carry
  // the indicator; an extra badge would be redundant and duplicate text.
  if (item.status === 'failed') {
    return <span className={cx(styles.badge, styles.exitFail)}>failed</span>;
  }
  return null;
}

// Search-highlight props threaded through every body renderer (CF-359).
// Kept as a separate interface so the no-`item` `GenericInputBlock` can share
// the exact same shape.
interface SearchHighlightProps {
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
}

interface BodyProps extends SearchHighlightProps {
  item: CodexToolCallItem;
}

function renderBody(item: CodexToolCallItem, search: SearchHighlightProps) {
  switch (item.toolName) {
    case 'exec_command':
      return <ExecCommandBody item={item} {...search} />;
    case 'apply_patch':
      return <ApplyPatchBody item={item} {...search} />;
    case 'web_search_call':
      return <WebSearchBody item={item} {...search} />;
    default:
      return <GenericToolBody item={item} {...search} />;
  }
}

// ----------------------------------------------------------------------------
// exec_command
// ----------------------------------------------------------------------------

function ExecCommandBody({ item, searchQuery, isCurrentSearchMatch }: BodyProps) {
  const cmd = readStringField(item.rawInput, 'cmd');
  const output = item.rawOutput;
  // Empty stdout falls through to NoOutputIndicator — otherwise BashOutput
  // would render a blank terminal frame with just a copy button (CF-378).
  const hasOutput = typeof output === 'string' && output !== '';
  return (
    <div className={styles.body}>
      {cmd ? (
        <pre className={styles.commandLine}>
          $ {renderTextWithHighlight(cmd, searchQuery, isCurrentSearchMatch)}
        </pre>
      ) : null}
      {hasOutput ? (
        <BashOutput
          output={output}
          exitCode={item.execMetadata?.exitCode ?? null}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
      ) : (
        <NoOutputIndicator status={item.status} />
      )}
    </div>
  );
}

function NoOutputIndicator({ status }: { status: CodexToolCallItem['status'] }) {
  const label = status === 'pending' ? 'pending — no output yet' : 'no output';
  return <div className={styles.noOutput}>{label}</div>;
}

// ----------------------------------------------------------------------------
// apply_patch
// ----------------------------------------------------------------------------

function ApplyPatchBody({ item, searchQuery, isCurrentSearchMatch }: BodyProps) {
  const changes = readPatchChanges(item.structuredOutput);
  const filePaths = Object.keys(changes);

  return (
    <div className={styles.body}>
      {filePaths.length > 0 ? (
        <div className={styles.patchSummary}>
          <span className={styles.patchSummaryLabel}>Files changed:</span>
          <ul className={styles.fileList}>
            {filePaths.map((path) => {
              const change = changes[path];
              return (
                <li key={path}>
                  <span className={styles.fileChangeType}>
                    {patchOpLabel(change?.type ?? 'unknown')}
                  </span>{' '}
                  <span className={styles.filePath}>
                    {renderTextWithHighlight(leafFileName(path), searchQuery, isCurrentSearchMatch)}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}

      {typeof item.rawInput === 'string' ? (
        <CodeBlock
          code={item.rawInput}
          language="diff"
          maxHeight="500px"
          truncateLines={100}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
      ) : null}
      {item.rawOutput ? (
        <CodeBlock
          code={item.rawOutput}
          language="plain"
          maxHeight="300px"
          truncateLines={100}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
      ) : null}
    </div>
  );
}

function patchOpLabel(type: string): string {
  switch (type) {
    case 'add':
      return '+ add';
    case 'update':
      return '~ edit';
    case 'delete':
      return '- delete';
    default:
      return type;
  }
}

// ----------------------------------------------------------------------------
// web_search_call
// ----------------------------------------------------------------------------

function WebSearchBody({ item, searchQuery, isCurrentSearchMatch }: BodyProps) {
  const queries = readWebSearchQueries(item.rawInput);

  return (
    <div className={styles.body}>
      <div className={styles.queryChips}>
        {queries.length === 0 ? (
          <span className={styles.noOutput}>no query</span>
        ) : (
          queries.map((q) => (
            <span key={q} className={styles.queryChip}>
              {renderTextWithHighlight(q, searchQuery, isCurrentSearchMatch)}
            </span>
          ))
        )}
      </div>
    </div>
  );
}

// ----------------------------------------------------------------------------
// generic / unknown tool
// ----------------------------------------------------------------------------

function GenericToolBody({ item, searchQuery, isCurrentSearchMatch }: BodyProps) {
  // Treat `null` as "no input" so unknown tools don't render a tiny `null`
  // code block. `undefined` is also absent; non-null values render.
  const hasInput = item.rawInput !== undefined && item.rawInput !== null;
  const output = item.rawOutput;
  const hasOutput = typeof output === 'string' && output !== '';
  // Show the no-output line when the user would otherwise see literally
  // nothing under the header (no input + no output) or while we're still
  // waiting on the call to complete.
  const showNoOutput = !hasOutput && (!hasInput || item.status === 'pending');

  return (
    <div className={styles.body}>
      {hasInput ? (
        <GenericInputBlock
          input={item.rawInput}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
      ) : null}
      {hasOutput ? (
        <CodeBlock
          code={output}
          language="plain"
          maxHeight="400px"
          truncateLines={100}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
      ) : null}
      {showNoOutput ? <NoOutputIndicator status={item.status} /> : null}
    </div>
  );
}

// Render `rawInput` as a CodeBlock. Object/array inputs are always JSON.
// String inputs pretty-print as JSON when parseable, otherwise stay as-is in
// a plain block.
function GenericInputBlock({
  input,
  searchQuery,
  isCurrentSearchMatch,
}: SearchHighlightProps & { input: unknown }) {
  if (typeof input === 'string') {
    const jsonPretty = tryParseAsJson(input);
    return (
      <CodeBlock
        code={jsonPretty ?? input}
        language={jsonPretty ? 'json' : 'plain'}
        maxHeight="400px"
        truncateLines={100}
        searchQuery={searchQuery}
        isCurrentSearchMatch={isCurrentSearchMatch}
      />
    );
  }
  return (
    <CodeBlock
      code={stringifyForDisplay(input)}
      language="json"
      maxHeight="400px"
      truncateLines={100}
      searchQuery={searchQuery}
      isCurrentSearchMatch={isCurrentSearchMatch}
    />
  );
}

// Shape-readers and copy-text composition live in `codexToolCallHelpers.ts`
// so the component file exports only the component (react-refresh rule).
