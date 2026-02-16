// Formatting utilities for display

/**
 * Format a Date as YYYY-MM-DD using local date components (not UTC).
 * Used by TrendsPage and TrendsFilters for date range parameters.
 */
export function formatLocalDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/** Parse date string, normalizing to UTC if no timezone specified */
function parseDate(dateStr: string): Date {
  const normalized = dateStr.endsWith('Z') ? dateStr : `${dateStr}Z`;
  return new Date(normalized);
}

/**
 * Format a date string for display (locale string)
 */
export function formatDateString(dateStr: string): string {
  return parseDate(dateStr).toLocaleString();
}

/**
 * Format a date string as relative time (e.g., "5m ago" or "in 5m")
 */
export function formatRelativeTime(dateStr: string): string {
  const date = parseDate(dateStr);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  const absDiff = Math.abs(diff);
  const seconds = Math.floor(absDiff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  const isFuture = diff < 0;
  const suffix = isFuture ? '' : ' ago';
  const prefix = isFuture ? 'in ' : '';

  if (days > 0) return `${prefix}${days}d${suffix}`;
  if (hours > 0) return `${prefix}${hours}h${suffix}`;
  if (minutes > 0) return `${prefix}${minutes}m${suffix}`;
  if (seconds > 0) return `${prefix}${seconds}s${suffix}`;
  return 'just now';
}

interface FormatDurationOptions {
  /** Show decimal seconds for sub-minute durations (e.g., "4.2s" vs "4s") */
  decimalSeconds?: boolean;
}

/**
 * Format duration in a human-readable way.
 * Examples: "1d 2h", "5h 30m", "15m", "45s", "4.2s" (with decimalSeconds)
 *
 * NOTE: Context-specific variants exist for different UI needs:
 * - SessionCard: Simplified ("5m" not "5m 30s") for session overview
 * - ConversationCard/TimelineBar: Precise ("5m 30s", "500ms") for timing display
 */
export function formatDuration(ms: number, options: FormatDurationOptions = {}): string {
  const { decimalSeconds = false } = options;

  // Round to nearest second first to avoid "1m 60s" edge cases
  const totalSeconds = Math.round(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    const remainingHours = hours % 24;
    return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
  }
  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    const remainingSeconds = totalSeconds % 60;
    return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
  }

  // Sub-minute: optionally show decimal
  if (decimalSeconds) {
    const seconds = ms / 1000;
    return `${seconds.toFixed(1)}s`;
  }
  return `${totalSeconds}s`;
}

/**
 * Format a Date object for display with date and time
 */
export function formatDateTime(date: Date): string {
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Extract a short model name from the full model string (technical format).
 * Examples:
 *   "claude-3-5-sonnet-20241022" -> "claude-sonnet-3.5"
 *   "claude-opus-4-20250514" -> "claude-opus-4"
 *   "claude-sonnet-4-5-20250514" -> "claude-sonnet-4.5"
 *   "opus" -> "claude-opus"
 *
 * NOTE: SessionCard has a different variant that returns user-friendly format
 * ("Sonnet 4" instead of "claude-sonnet-4") for the card UI.
 */
export function formatModelName(model: string): string {
  const lowerModel = model.toLowerCase();

  // Look for variant names
  const variants = ['sonnet', 'opus', 'haiku'];
  const foundVariant = variants.find(v => lowerModel.includes(v));

  if (!foundVariant) {
    // No known variant, return as-is or truncated
    const parts = model.split('-');
    if (parts.length > 3) {
      return parts.slice(0, 3).join('-');
    }
    return model;
  }

  // Try to extract version number - handles both "claude-3-5-sonnet" and "claude-opus-4"
  // Version numbers are 1-2 digits, optionally followed by -X (e.g., 3, 4, 3-5, 4-5)
  // We explicitly avoid matching date suffixes like -20250514
  // Pattern 1: claude-X-Y-variant (e.g., claude-3-5-sonnet)
  const preVariantMatch = model.match(/claude-(\d{1,2}(?:-\d{1,2})?)-\w+/i);
  // Pattern 2: claude-variant-X[-Y] (e.g., claude-opus-4, claude-opus-4-5)
  // Use word boundary or dash+long-number to stop before date suffixes
  const postVariantMatch = model.match(new RegExp(`${foundVariant}-(\\d{1,2}(?:-\\d{1,2})?)(?:-\\d{6,}|$)`, 'i'));

  let version = '';
  if (preVariantMatch?.[1]) {
    version = preVariantMatch[1].replace('-', '.');
  } else if (postVariantMatch?.[1]) {
    version = postVariantMatch[1].replace('-', '.');
  }

  // Build the display name
  if (version) {
    return `claude-${foundVariant}-${version}`;
  }
  return `claude-${foundVariant}`;
}

/**
 * Extract repo name from URL
 * e.g., "https://github.com/user/repo.git" -> "user/repo"
 */
export function formatRepoName(repoUrl: string): string {
  // Handle GitHub/GitLab URLs
  const match = repoUrl.match(/(?:github\.com|gitlab\.com)[/:]([^/]+\/[^/]+?)(?:\.git)?$/i);
  if (match?.[1]) {
    return match[1];
  }
  // Fallback: just show last two path segments
  const parts = repoUrl.replace(/\.git$/, '').split('/');
  if (parts.length >= 2) {
    return `${parts[parts.length - 2]}/${parts[parts.length - 1]}`;
  }
  return repoUrl;
}
