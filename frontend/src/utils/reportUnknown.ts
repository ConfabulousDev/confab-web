// CF-574: privacy-conscious "Report this message" plumbing for UNKNOWN transcript
// rows. The descriptor is a small NORMALIZED fingerprint of an unrecognized
// message/line — it deliberately carries key *names* and a type string, never
// payload values, text, tool I/O, or file paths. All redaction lives here so the
// four render surfaces (Claude message, Claude content block, Codex line,
// OpenCode line) share one privacy contract.
import { buildGitHubIssueUrl } from './externalLinks';

export interface UnknownDescriptor {
  provider: 'claude' | 'codex' | 'opencode';
  /** Where it rendered: 'message' | 'content block' | 'line'. */
  surface: string;
  /** The unrecognized type/role string. '(none)' when absent. */
  type: string;
  /** Human-readable classification path (Codex/OpenCode); omitted otherwise. */
  reason?: string;
  /** Structural fingerprint: key NAMES only (one nesting level), never values. */
  keyFingerprint: string[];
}

/** Plain object (not an array) — arrays are excluded so we don't fingerprint
 *  their numeric indices. */
function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Structural fingerprint of an unknown object: own top-level key names plus, for
 * each top-level value that is a plain object, its child key names as
 * `parent.child` (one level only). Names only — never values. Arrays and
 * non-object values contribute only their own key name (no index spam).
 */
export function computeKeyFingerprint(obj: unknown): string[] {
  if (!isPlainObject(obj)) return [];
  const keys: string[] = [];
  for (const [key, value] of Object.entries(obj)) {
    keys.push(key);
    if (isPlainObject(value)) {
      for (const childKey of Object.keys(value)) {
        keys.push(`${key}.${childKey}`);
      }
    }
  }
  return keys;
}

/**
 * Build the prefilled issue title + Markdown body for an unknown-row report.
 * The Confab app version line is included only when `appVersion` is non-empty.
 * No payload, no label, no raw-paste invitation (CF-574 decisions).
 */
export function buildUnknownReportIssue(
  descriptor: UnknownDescriptor,
  appVersion?: string,
): { title: string; body: string } {
  const { provider, surface, type, reason, keyFingerprint } = descriptor;

  const title = `[parser-gap] ${provider}: unrecognized ${surface} "${type}"`;

  const lines: string[] = [
    'A transcript row was not recognized by the Confab parser. The details below are',
    'structural metadata only — **no message content is included**.',
    '',
    `- **Provider:** ${provider}`,
    `- **Surface:** ${surface}`,
    `- **Unrecognized type:** \`${type}\``,
  ];
  if (reason) lines.push(`- **Classification:** ${reason}`);
  lines.push(
    `- **Top-level keys:** ${
      keyFingerprint.length > 0 ? keyFingerprint.map((k) => `\`${k}\``).join(', ') : '_(none)_'
    }`,
  );
  if (appVersion) lines.push(`- **Confab version:** ${appVersion}`);
  lines.push('', '**What were you doing when you saw this?**', '', '<!-- describe here -->');

  return { title, body: lines.join('\n') };
}

/** Convenience: descriptor → full prefilled GitHub `/issues/new` URL. */
export function buildUnknownReportUrl(descriptor: UnknownDescriptor, appVersion?: string): string {
  const { title, body } = buildUnknownReportIssue(descriptor, appVersion);
  return buildGitHubIssueUrl(title, body);
}
