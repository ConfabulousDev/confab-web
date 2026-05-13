// Renders a paired Codex tool call + output.
//
// Dispatch by `toolName`:
//   exec_command       command + output + exit-code badge
//   apply_patch        file-list summary + raw + click-to-expand
//   web_search_call    query chip(s)
//   <anything else>    generic "Tool: <name>" with rawInput / rawOutput

import { useState } from 'react';
import type { CodexToolCallItem } from '@/types/codexRenderItem';
import { isRecord } from '@/utils/utils';
import {
  formatCodexTimestamp,
  leafFileName,
  stringifyForDisplay,
} from './codexFormat';
import styles from './CodexToolCallBlock.module.css';

export interface CodexToolCallBlockProps {
  item: CodexToolCallItem;
}

const EXEC_OUTPUT_SOFT_CAP = 100;

export default function CodexToolCallBlock({ item }: CodexToolCallBlockProps) {
  return (
    <div className={styles.toolCall} data-kind="tool_call" data-tool={item.toolName}>
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
    const cls =
      item.execMetadata.exitCode === 0 ? styles.exitOk : styles.exitFail;
    return (
      <span className={`${styles.badge} ${cls}`}>
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
    return <span className={`${styles.badge} ${styles.exitFail}`}>failed</span>;
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

  return (
    <div className={styles.body}>
      {cmd ? <pre className={styles.command}>$ {cmd}</pre> : null}
      {item.rawOutput !== undefined ? (
        <ExecOutput output={item.rawOutput} />
      ) : (
        <NoOutputIndicator status={item.status} />
      )}
    </div>
  );
}

function ExecOutput({ output }: { output: string }) {
  const [expanded, setExpanded] = useState(false);
  const lines = output.split('\n');
  const isOverCap = lines.length > EXEC_OUTPUT_SOFT_CAP;
  const visible = expanded || !isOverCap ? lines : lines.slice(0, EXEC_OUTPUT_SOFT_CAP);

  return (
    <>
      <pre className={styles.output}>{visible.join('\n')}</pre>
      {isOverCap ? (
        <button
          type="button"
          className={styles.showAllButton}
          onClick={() => setExpanded((prev) => !prev)}
        >
          {expanded
            ? `Show fewer (${EXEC_OUTPUT_SOFT_CAP} of ${lines.length})`
            : `Show all (${lines.length} lines)`}
        </button>
      ) : null}
    </>
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
  const [expanded, setExpanded] = useState(false);
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

      <button
        type="button"
        className={styles.expandButton}
        onClick={() => setExpanded((prev) => !prev)}
      >
        {expanded ? 'Hide raw patch' : 'Show raw patch'}
      </button>

      {expanded ? (
        <>
          {typeof item.rawInput === 'string' ? (
            <pre className={styles.output}>{item.rawInput}</pre>
          ) : null}
          {item.rawOutput ? (
            <pre className={styles.output}>{item.rawOutput}</pre>
          ) : null}
        </>
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
  return (
    <div className={styles.body}>
      {item.rawInput !== undefined ? (
        <details className={styles.detailsBlock}>
          <summary>Input</summary>
          <pre className={styles.output}>{stringifyForDisplay(item.rawInput)}</pre>
        </details>
      ) : null}
      {item.rawOutput ? (
        <details className={styles.detailsBlock}>
          <summary>Output</summary>
          <pre className={styles.output}>{item.rawOutput}</pre>
        </details>
      ) : (
        <NoOutputIndicator status={item.status} />
      )}
    </div>
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
