// Shared chart label helpers. Used wherever a Recharts axis or label needs
// long identifiers (agent names, skill names, tool names) capped to a
// readable width while keeping the full value available on hover.

const TRUNCATE_PREFIX = 6;
const TRUNCATE_SUFFIX = 6;
const TRUNCATE_MAX = TRUNCATE_PREFIX + TRUNCATE_SUFFIX + 3; // "prefix...suffix"

// YAxis width sizing constants. The font is 11px; ~7px per char is a safe
// approximation across the Latin-character set we see in agent/skill/tool
// names. The +8 leaves a small gutter between text and the bars.
const CHAR_WIDTH_PX = 7;
const AXIS_PADDING_PX = 8;
const MIN_AXIS_WIDTH_PX = 40;

/** Truncate a long label to "prefix...suffix" form (e.g. "execut...ctron"). */
export function truncateName(name: string): string {
  if (name.length <= TRUNCATE_MAX) return name;
  return `${name.slice(0, TRUNCATE_PREFIX)}...${name.slice(-TRUNCATE_SUFFIX)}`;
}

/**
 * Compute the YAxis width (px) needed to fit the *truncated* form of every
 * label. Sizing off the truncated form prevents a single long MCP tool name
 * from pushing the bars off-screen; the full name is preserved on hover by
 * `TruncatedYAxisTick`. `minChars` is the floor on label length to use when
 * every label is short (keeps the axis from collapsing to nothing).
 */
export function truncatedYAxisWidth(labels: readonly string[], minChars: number): number {
  const maxLabelLength = Math.max(minChars, ...labels.map((label) => truncateName(label).length));
  return Math.max(MIN_AXIS_WIDTH_PX, maxLabelLength * CHAR_WIDTH_PX + AXIS_PADDING_PX);
}
