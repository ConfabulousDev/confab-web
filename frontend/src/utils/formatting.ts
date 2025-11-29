// Formatting utilities for display

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

/**
 * Format duration in a human-readable way
 */
export function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return `${hours}h ${remainingMinutes}m`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
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
 * Extract a short model name from the full model string
 * Examples:
 *   "claude-3-5-sonnet-20241022" -> "claude-sonnet-3.5"
 *   "claude-opus-4-20250514" -> "claude-opus-4"
 *   "claude-sonnet-4-5-20250514" -> "claude-sonnet-4.5"
 *   "opus" -> "opus"
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
  // Pattern 1: claude-X-Y-variant (e.g., claude-3-5-sonnet)
  const preVariantMatch = model.match(/claude-(\d+(?:-\d+)?)-\w+/i);
  // Pattern 2: claude-variant-X (e.g., claude-opus-4)
  const postVariantMatch = model.match(new RegExp(`${foundVariant}-(\\d+(?:-\\d+)?)`, 'i'));

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
