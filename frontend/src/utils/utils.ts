// Shared utility functions for Confab frontend

/**
 * Format a date string for display
 */
export function formatDate(dateStr: string): string {
	// Ensure we're parsing the date correctly regardless of timezone format
	// If the string ends with 'Z', it's UTC. Otherwise, treat it as UTC by appending 'Z'
	const normalizedDateStr = dateStr.endsWith('Z') ? dateStr : `${dateStr}Z`;
	const date = new Date(normalizedDateStr);
	return date.toLocaleString();
}

/**
 * Format bytes into human-readable size
 */
export function formatBytes(bytes: number): string {
	if (bytes === 0) return '0 B';
	const k = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

/**
 * Format a date as relative time (e.g., "5m ago" or "in 5m")
 */
export function formatRelativeTime(dateStr: string): string {
	// Ensure we're parsing the date correctly regardless of timezone format
	// If the string ends with 'Z', it's UTC. Otherwise, treat it as UTC by appending 'Z'
	const normalizedDateStr = dateStr.endsWith('Z') ? dateStr : `${dateStr}Z`;
	const date = new Date(normalizedDateStr);
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
