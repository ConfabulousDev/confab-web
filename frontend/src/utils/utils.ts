// Shared utility functions for Confab frontend

/**
 * Strip ANSI escape codes from text.
 * Handles color codes, cursor movement, clearing, and other terminal sequences.
 */
export function stripAnsi(text: string): string {
	// Matches all ANSI escape sequences:
	// - \x1b (or \u001b) followed by [ and any params ending in a letter
	// - Also handles OSC sequences (\x1b]) and other escape types
	// eslint-disable-next-line no-control-regex
	return text.replace(/\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[()][AB012]|\x1b[PX^_][^\x1b]*\x1b\\|\x1b.?/g, '');
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
