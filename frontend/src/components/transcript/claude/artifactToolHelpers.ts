// Pure helpers for the claude.ai Artifact tool renderers in ArtifactTool.tsx
// (kept separate so that file only exports components — react-refresh).

import type { ArtifactToolResult } from '@/schemas/claudeTranscript';
import { plural } from '@/utils/formatting';

export const ARTIFACT_TOOL_NAME = 'Artifact';
const ARTIFACT_COMMENTS_TOOL_NAME = 'ArtifactComments';
const ARTIFACT_DATA_TOOL_NAME = 'ArtifactData';

// Card header label per tool. Also the set of tool_use names that render as a card.
const TOOL_LABELS: Record<string, string> = {
  [ARTIFACT_TOOL_NAME]: 'Artifact',
  [ARTIFACT_COMMENTS_TOOL_NAME]: 'Artifact comments',
  [ARTIFACT_DATA_TOOL_NAME]: 'Artifact data',
};

export function isArtifactToolName(name: string): boolean {
  return Object.hasOwn(TOOL_LABELS, name);
}

// The input is model-authored, so only https URLs become hrefs; anything else
// (javascript:, data:, http:, garbage) renders as plain text.
export function isSafeHttpsUrl(value: string): boolean {
  try {
    return new URL(value).protocol === 'https:';
  } catch {
    return false;
  }
}

function str(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

function count(value: unknown): number {
  if (Array.isArray(value)) return value.length;
  if (value && typeof value === 'object') return Object.keys(value).length;
  return 0;
}

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

const ARTIFACT_ACTION_LABELS: Record<string, string> = {
  publish: 'Publish',
  quickstart: 'Quickstart',
};

/** Human label for an `Artifact` call, e.g. "Publish", "Upload asset", "Read". */
function artifactActionLabel(input: Record<string, unknown>): string {
  const action = str(input.action) ?? 'publish';
  if (action === 'publish') {
    if (input.asset === true) return 'Upload asset';
    if (input.type_url && !input.file_path) return 'Create from type';
  }
  return ARTIFACT_ACTION_LABELS[action] ?? capitalize(action);
}

// Secondary `Artifact` string fields shown as labeled rows, in display order.
const ARTIFACT_DETAIL_FIELDS: Array<[key: string, label: string]> = [
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

// Every `Artifact` input key given a hand-labeled rendering (or folded into
// the action label); the rest fall through to generic rows and chips.
const ARTIFACT_HANDLED_KEYS = new Set([
  'action', 'asset', 'file_path', 'file_paths', 'url', 'type_url', 'from_url',
  'edits', 'files', 'asset_ids', 'capabilities', 'force', 'pin',
  ...ARTIFACT_DETAIL_FIELDS.map(([key]) => key),
]);

const SIBLING_HANDLED_KEYS = new Set(['action', 'url']);

interface CardRow {
  key: string;
  label: string;
  value: string;
  /** Render the value through the https link rule. */
  isUrl?: boolean;
}

interface ArtifactToolCard {
  toolLabel: string;
  actionLabel?: string;
  /** Local file path(s) the call targets, shown monospace. */
  paths: string[];
  /** The artifact URL the call targets. */
  url?: string;
  rows: CardRow[];
  chips: string[];
}

/**
 * Top-level fields not in `handled`: strings / numbers / booleans become rows
 * keyed by the field name, non-empty objects and arrays become count chips.
 * Empty strings, null and empty containers are skipped.
 */
function genericFields(input: Record<string, unknown>, handled: Set<string>): Pick<ArtifactToolCard, 'rows' | 'chips'> {
  const rows: CardRow[] = [];
  const chips: string[] = [];
  for (const [key, value] of Object.entries(input)) {
    if (handled.has(key)) continue;
    if (typeof value === 'string') {
      if (value !== '') rows.push({ key, label: key, value });
    } else if (typeof value === 'number' || typeof value === 'boolean') {
      rows.push({ key, label: key, value: String(value) });
    } else if (Array.isArray(value)) {
      if (value.length > 0) chips.push(`${key}: ${plural(value.length, 'item')}`);
    } else if (value && typeof value === 'object') {
      const n = count(value);
      if (n > 0) chips.push(`${key}: ${plural(n, 'key')}`);
    }
  }
  return { rows, chips };
}

function describeArtifactInput(input: Record<string, unknown>): ArtifactToolCard {
  const filePath = str(input.file_path);
  const filePaths = Array.isArray(input.file_paths) ? input.file_paths.filter((p): p is string => typeof p === 'string') : [];

  const rows: CardRow[] = ARTIFACT_DETAIL_FIELDS.flatMap(([key, label]) => {
    const value = str(input[key]);
    return value ? [{ key, label, value }] : [];
  });
  const typeUrl = str(input.type_url);
  if (typeUrl) rows.push({ key: 'type_url', label: 'From type', value: typeUrl, isUrl: true });
  const fromUrl = str(input.from_url);
  if (fromUrl) rows.push({ key: 'from_url', label: 'Copy from', value: fromUrl, isUrl: true });

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

  const generic = genericFields(input, ARTIFACT_HANDLED_KEYS);
  return {
    toolLabel: TOOL_LABELS[ARTIFACT_TOOL_NAME] ?? ARTIFACT_TOOL_NAME,
    actionLabel: artifactActionLabel(input),
    paths: filePath ? [filePath, ...filePaths] : filePaths,
    url: str(input.url),
    rows: [...rows, ...generic.rows],
    chips: [...chips, ...generic.chips],
  };
}

/**
 * Card model for an Artifact / ArtifactComments / ArtifactData tool_use.
 * Callers gate on `isArtifactToolName` first.
 */
export function describeArtifactToolInput(toolName: string, input: Record<string, unknown>): ArtifactToolCard {
  if (toolName === ARTIFACT_TOOL_NAME) return describeArtifactInput(input);
  const action = str(input.action);
  return {
    toolLabel: TOOL_LABELS[toolName] ?? toolName,
    actionLabel: action && capitalize(action),
    paths: [],
    url: str(input.url),
    ...genericFields(input, SIBLING_HANDLED_KEYS),
  };
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
