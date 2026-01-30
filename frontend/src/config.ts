// Application configuration
// Values are read from environment variables at build time

/**
 * Support email address for help requests and account issues.
 * Set via VITE_SUPPORT_EMAIL environment variable.
 * Defaults to 'support@example.com' for self-hosted instances.
 */
export const SUPPORT_EMAIL = import.meta.env.VITE_SUPPORT_EMAIL || 'support@example.com';
