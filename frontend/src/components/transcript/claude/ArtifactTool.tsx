// Readable renderers for Claude Code's claude.ai `Artifact` tool: a summary card
// for the tool_use input, and a link line for a successful tool_result. Both keep
// the raw payload in a collapsed `<details>` that auto-opens when the row becomes
// the active search match.

import type { ReactNode } from 'react';
import { useOpenOnRisingEdge } from '@/hooks/useOpenOnRisingEdge';
import { artifactActionLabel, count, isSafeArtifactUrl, plural, str, type ArtifactResultLink } from './artifactToolHelpers';
import styles from './ArtifactTool.module.css';

// Secondary string fields shown as label/value rows, in display order.
const DETAIL_FIELDS: Array<[key: string, label: string]> = [
  ['title', 'Title'],
  ['description', 'Description'],
  ['path', 'Path'],
  ['intent', 'Intent'],
  ['scope', 'Scope'],
  ['type', 'Type'],
  ['prompt', 'Prompt'],
  ['label', 'Label'],
  ['icon', 'Icon'],
];

function ArtifactLink({ url, children }: { url: string; children?: ReactNode }) {
  if (!isSafeArtifactUrl(url)) return <span className={styles.mono}>{url}</span>;
  return (
    <a className={styles.link} href={url} target="_blank" rel="noopener noreferrer">
      {children ?? url}
    </a>
  );
}

function RawDetails({ label, isCurrentSearchMatch, children }: {
  label: string;
  isCurrentSearchMatch?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useOpenOnRisingEdge(!!isCurrentSearchMatch);
  return (
    <details className={styles.raw} open={open} onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>{label}</summary>
      {children}
    </details>
  );
}

interface ArtifactToolUseProps {
  input: Record<string, unknown>;
  isCurrentSearchMatch?: boolean;
  /** Raw input JSON (a CodeBlock), collapsed by default. */
  children: ReactNode;
}

export function ArtifactToolUse({ input, isCurrentSearchMatch, children }: ArtifactToolUseProps) {
  const filePath = str(input.file_path);
  const filePaths = Array.isArray(input.file_paths) ? input.file_paths.filter((p): p is string => typeof p === 'string') : [];
  const url = str(input.url);
  const typeUrl = str(input.type_url);
  const fromUrl = str(input.from_url);

  const chips: string[] = [];
  const edits = count(input.edits);
  if (edits > 0) chips.push(plural(edits, 'edit'));
  const files = count(input.files);
  if (files > 0) chips.push(plural(files, 'file'));
  const assetIds = count(input.asset_ids);
  if (assetIds > 0) chips.push(plural(assetIds, 'asset'));
  const capabilities = input.capabilities && typeof input.capabilities === 'object' && !Array.isArray(input.capabilities)
    ? Object.keys(input.capabilities)
    : [];
  if (capabilities.length > 0) chips.push(`capabilities: ${capabilities.join(', ')}`);
  if (input.force === true) chips.push('force');
  if (input.pin === true) chips.push('pin');

  const details = DETAIL_FIELDS.flatMap(([key, label]) => {
    const value = str(input[key]);
    return value ? [{ key, label, value }] : [];
  });

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <span className={styles.toolName}>Artifact</span>
        <span className={styles.action}>{artifactActionLabel(input)}</span>
      </div>
      {(filePath || filePaths.length > 0 || url) && (
        <div className={styles.target}>
          {filePath && <span className={styles.mono}>{filePath}</span>}
          {filePaths.map((p) => (
            <span key={p} className={styles.mono}>{p}</span>
          ))}
          {url && <ArtifactLink url={url} />}
        </div>
      )}
      {(details.length > 0 || typeUrl || fromUrl) && (
        <dl className={styles.details}>
          {details.map(({ key, label, value }) => (
            <div key={key} className={styles.detailRow}>
              <dt>{label}</dt>
              <dd>{value}</dd>
            </div>
          ))}
          {typeUrl && (
            <div className={styles.detailRow}>
              <dt>From type</dt>
              <dd><ArtifactLink url={typeUrl} /></dd>
            </div>
          )}
          {fromUrl && (
            <div className={styles.detailRow}>
              <dt>Copy from</dt>
              <dd><ArtifactLink url={fromUrl} /></dd>
            </div>
          )}
        </dl>
      )}
      {chips.length > 0 && (
        <div className={styles.chips}>
          {chips.map((chip) => (
            <span key={chip} className={styles.chip}>{chip}</span>
          ))}
        </div>
      )}
      <RawDetails label="Raw input" isCurrentSearchMatch={isCurrentSearchMatch}>
        {children}
      </RawDetails>
    </div>
  );
}

interface ArtifactResultProps {
  link: ArtifactResultLink;
  isCurrentSearchMatch?: boolean;
  /** Full tool_result content, collapsed by default (it is mostly model guidance). */
  children: ReactNode;
}

export function ArtifactResult({ link, isCurrentSearchMatch, children }: ArtifactResultProps) {
  return (
    <div className={styles.result}>
      <div className={styles.resultLine}>
        <ArtifactLink url={link.url}>{link.title ?? link.url}</ArtifactLink>
        {link.version !== undefined && <span className={styles.chip}>Version {link.version}</span>}
      </div>
      <RawDetails label="Full result" isCurrentSearchMatch={isCurrentSearchMatch}>
        {children}
      </RawDetails>
    </div>
  );
}
