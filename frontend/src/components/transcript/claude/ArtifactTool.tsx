// Readable renderers for Claude Code's claude.ai artifact tools: one summary
// card for Artifact / ArtifactComments / ArtifactData tool_use inputs, and a
// link line for a successful Artifact tool_result. Both keep the raw payload in
// a collapsed `<details>` that auto-opens when the row becomes the active
// search match.

import type { ReactNode } from 'react';
import { useOpenOnRisingEdge } from '@/hooks/useOpenOnRisingEdge';
import { describeArtifactToolInput, isSafeHttpsUrl, type ArtifactResultLink } from './artifactToolHelpers';
import styles from './ArtifactTool.module.css';

function ArtifactLink({ url, children }: { url: string; children?: ReactNode }) {
  const label = children ?? url;
  if (!isSafeHttpsUrl(url)) return <span className={styles.mono}>{label}</span>;
  return (
    <a className={styles.link} href={url} target="_blank" rel="noopener noreferrer">
      {label}
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
  /** `Artifact`, `ArtifactComments` or `ArtifactData`. */
  toolName: string;
  input: Record<string, unknown>;
  isCurrentSearchMatch?: boolean;
  /** Raw input JSON (a CodeBlock), collapsed by default. */
  children: ReactNode;
}

export function ArtifactToolUse({ toolName, input, isCurrentSearchMatch, children }: ArtifactToolUseProps) {
  const { toolLabel, actionLabel, paths, url, rows, chips } = describeArtifactToolInput(toolName, input);

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <span className={styles.toolName}>{toolLabel}</span>
        {actionLabel && <span className={styles.action}>{actionLabel}</span>}
      </div>
      {(paths.length > 0 || url) && (
        <div className={styles.target}>
          {paths.map((p, i) => (
            <span key={`${i}:${p}`} className={styles.mono}>{p}</span>
          ))}
          {url && <ArtifactLink url={url} />}
        </div>
      )}
      {rows.length > 0 && (
        <dl className={styles.details}>
          {rows.map(({ key, label, value, isUrl }) => (
            <div key={key} className={styles.detailRow}>
              <dt>{label}</dt>
              <dd>{isUrl ? <ArtifactLink url={value} /> : value}</dd>
            </div>
          ))}
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
