// Pure helpers for the `Artifact` tool renderers in ArtifactTool.tsx (kept
// separate so that file only exports components — react-refresh).

import type { ArtifactToolResult } from '@/schemas/claudeTranscript';

export const ARTIFACT_TOOL_NAME = 'Artifact';

// Only claude.ai artifact links become hrefs — the input is model-authored, so
// anything else (incl. javascript: URLs) renders as plain text.
const SAFE_ARTIFACT_URL = /^https:\/\/claude\.ai\//;

export function isSafeArtifactUrl(value: unknown): value is string {
  return typeof value === 'string' && SAFE_ARTIFACT_URL.test(value);
}

export function str(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

export function count(value: unknown): number {
  if (Array.isArray(value)) return value.length;
  if (value && typeof value === 'object') return Object.keys(value).length;
  return 0;
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`;
}

const ACTION_LABELS: Record<string, string> = {
  publish: 'Publish',
  read: 'Read',
  list: 'List',
  delete: 'Delete',
  open: 'Open',
  pin: 'Pin',
  unpin: 'Unpin',
  quickstart: 'Quickstart',
};

/** Human label for the call, e.g. "Publish", "Upload asset", "Read". */
export function artifactActionLabel(input: Record<string, unknown>): string {
  const action = str(input.action) ?? 'publish';
  if (action === 'publish') {
    if (input.asset === true) return 'Upload asset';
    if (input.type_url && !input.file_path) return 'Create from type';
  }
  return ACTION_LABELS[action] ?? action;
}

export interface ArtifactResultLink {
  url: string;
  title?: string;
  version?: number;
}

/** The artifact a successful result points at, or null when it names none. */
export function getArtifactResultLink(result: ArtifactToolResult | undefined): ArtifactResultLink | null {
  if (!result) return null;
  const url = str(result.url) ?? str(result.read?.url) ?? str(result.file_read?.url);
  if (!url) return null;
  const title = str(result.title) ?? str(result.read?.title) ?? str(result.file_read?.title);
  return { url, title, version: result.seq };
}
