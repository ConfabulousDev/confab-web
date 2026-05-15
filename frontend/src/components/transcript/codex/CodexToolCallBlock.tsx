// Renders a paired Codex tool call + output.
//
// Dispatch by `toolName`:
//   exec_command       command + BashOutput (ANSI-stripped, terminal styling)
//   apply_patch        file-list summary + CodeBlock(language="diff")
//   web_search_call    query chip(s)
//   <anything else>    generic Input / Output via CodeBlock (json / plain)

import type { CodexToolCallItem } from '@/types/codexRenderItem';
import { tryParseAsJson } from '@/utils';
import { cx, isRecord } from '@/utils/utils';
import BashOutput from '../BashOutput';
import CodeBlock from '../CodeBlock';
import {
  formatCodexTimestamp,
  leafFileName,
  stringifyForDisplay,
} from './codexFormat';
import styles from './CodexToolCallBlock.module.css';

export interface CodexToolCallBlockProps {
  item: CodexToolCallItem;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /**
   * Speaker continuity flag. tool_call items never break user/assistant
   * continuity, so this prop is accepted for shape uniformity but does
   * not affect rendering.
   */
  isNewSpeaker?: boolean;
}

export default function CodexToolCallBlock({ item, isSelected }: CodexToolCallBlockProps) {
  const className = cx(styles.toolCall, isSelected && styles.selected);
  return (
    <div className={className} data-kind="tool_call" data-tool={item.toolName}>
      <div className={styles.header}>
        <span className={styles.toolName}>{toolNameLabel(item)}</span>
        {renderStatusBadge(item)}
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
      </div>
      {renderBody(item)}
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

function renderBody(item: CodexToolCallItem) {
  switch (item.toolName) {
    case 'exec_command':
      return <ExecCommandBody item={item} />;
    case 'apply_patch':
      return <ApplyPatchBody item={item} />;
    case 'web_search_call':
      return <WebSearchBody item={item} />;
    default:
      return <GenericToolBody item={item} />;
  }
}

// ----------------------------------------------------------------------------
// exec_command
// ----------------------------------------------------------------------------

function ExecCommandBody({ item }: { item: CodexToolCallItem }) {
  const cmd = readStringField(item.rawInput, 'cmd');
  const output = item.rawOutput;
  // Empty stdout falls through to NoOutputIndicator — otherwise BashOutput
  // would render a blank terminal frame with just a copy button (CF-378).
  const hasOutput = typeof output === 'string' && output !== '';
  return (
    <div className={styles.body}>
      {cmd ? <pre className={styles.commandLine}>$ {cmd}</pre> : null}
      {hasOutput ? (
        <BashOutput
          output={output}
          exitCode={item.execMetadata?.exitCode ?? null}
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

interface PatchChange {
  type: string;
  content?: string;
}

function ApplyPatchBody({ item }: { item: CodexToolCallItem }) {
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
                  <span className={styles.filePath}>{leafFileName(path)}</span>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}

      {typeof item.rawInput === 'string' ? (
        <CodeBlock code={item.rawInput} language="diff" maxHeight="500px" />
      ) : null}
      {item.rawOutput ? (
        <CodeBlock code={item.rawOutput} language="plain" maxHeight="300px" />
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

function WebSearchBody({ item }: { item: CodexToolCallItem }) {
  const queries = readWebSearchQueries(item.rawInput);

  return (
    <div className={styles.body}>
      <div className={styles.queryChips}>
        {queries.length === 0 ? (
          <span className={styles.noOutput}>no query</span>
        ) : (
          queries.map((q) => (
            <span key={q} className={styles.queryChip}>
              {q}
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

function GenericToolBody({ item }: { item: CodexToolCallItem }) {
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
      {hasInput ? <GenericInputBlock input={item.rawInput} /> : null}
      {hasOutput ? (
        <CodeBlock code={output} language="plain" maxHeight="400px" />
      ) : null}
      {showNoOutput ? <NoOutputIndicator status={item.status} /> : null}
    </div>
  );
}

// Render `rawInput` as a CodeBlock. Object/array inputs are always JSON.
// String inputs pretty-print as JSON when parseable, otherwise stay as-is in
// a plain block.
function GenericInputBlock({ input }: { input: unknown }) {
  if (typeof input === 'string') {
    const jsonPretty = tryParseAsJson(input);
    return (
      <CodeBlock
        code={jsonPretty ?? input}
        language={jsonPretty ? 'json' : 'plain'}
        maxHeight="400px"
      />
    );
  }
  return (
    <CodeBlock code={stringifyForDisplay(input)} language="json" maxHeight="400px" />
  );
}

// ----------------------------------------------------------------------------
// Unknown-shape readers
// ----------------------------------------------------------------------------
//
// `rawInput` and `structuredOutput` are typed `unknown` because the underlying
// JSONL can carry whatever fields a given tool emits. These helpers extract
// only the fields each renderer cares about, with runtime guards so unfamiliar
// payloads degrade gracefully instead of crashing.

function readStringField(value: unknown, key: string): string | null {
  if (!isRecord(value)) return null;
  const v = value[key];
  return typeof v === 'string' ? v : null;
}

function readPatchChanges(value: unknown): Record<string, PatchChange> {
  if (!isRecord(value)) return {};
  const changes = value.changes;
  if (!isRecord(changes)) return {};
  const out: Record<string, PatchChange> = {};
  for (const [path, raw] of Object.entries(changes)) {
    if (isRecord(raw)) {
      const type = typeof raw.type === 'string' ? raw.type : 'unknown';
      const content = typeof raw.content === 'string' ? raw.content : undefined;
      out[path] = { type, content };
    }
  }
  return out;
}

function readWebSearchQueries(value: unknown): string[] {
  if (!isRecord(value)) return [];
  const queries = value.queries;
  if (Array.isArray(queries)) {
    return queries.filter((q): q is string => typeof q === 'string');
  }
  const query = value.query;
  return typeof query === 'string' ? [query] : [];
}
