/**
 * Centralized session error handling (CF-132)
 *
 * This module provides consistent error types, messages, and display helpers
 * for session access errors across the application.
 */

/** Session error types returned by the API */
export type SessionErrorType = 'not_found' | 'expired' | 'forbidden' | 'auth_required' | 'general' | null;

/** Error configuration for each error type */
interface SessionErrorConfig {
  message: string;
  icon: string;
  description?: string;
}

/** Mapping of error types to their display configuration */
const SESSION_ERROR_CONFIG: Record<NonNullable<SessionErrorType>, SessionErrorConfig> = {
  not_found: {
    message: 'Session not found',
    icon: '🔍',
  },
  expired: {
    message: 'This share has expired',
    icon: '⏰',
    description: 'Please request a new share from the session owner.',
  },
  forbidden: {
    message: 'You are not authorized to view this session',
    icon: '🚫',
    description: 'This session is only accessible to the owner or invited users.',
  },
  auth_required: {
    message: 'Sign in to view this session',
    icon: '🔐',
    description: 'This session may be shared with you. Sign in to check access.',
  },
  general: {
    message: 'Failed to load session',
    icon: '⚠️',
  },
};

/** Get the display message for an error type */
export function getErrorMessage(type: SessionErrorType): string {
  if (!type) return SESSION_ERROR_CONFIG.general.message;
  return SESSION_ERROR_CONFIG[type].message;
}

/** Get the icon for an error type */
export function getErrorIcon(type: SessionErrorType): string {
  if (!type) return SESSION_ERROR_CONFIG.general.icon;
  return SESSION_ERROR_CONFIG[type].icon;
}

/** Get the description for an error type (may be undefined) */
export function getErrorDescription(type: SessionErrorType): string | undefined {
  if (!type) return undefined;
  return SESSION_ERROR_CONFIG[type].description;
}

/** Map HTTP status code to error type */
export function statusToErrorType(status: number): NonNullable<SessionErrorType> {
  switch (status) {
    case 404:
      return 'not_found';
    case 410:
      return 'expired';
    case 401:
      return 'auth_required';
    case 403:
      return 'forbidden';
    default:
      return 'general';
  }
}

/**
 * Endpoints that handle 401 gracefully and should not trigger auth redirect.
 * These endpoints return 401 for informational purposes (e.g., "please log in").
 */
const SKIP_401_REDIRECT_ENDPOINTS = [
  '/me', // useAuth checks login status
] as const;

/** Regex pattern for session detail endpoint: /sessions/{uuid} */
const SESSION_DETAIL_PATTERN = /^\/sessions\/[^/]+$/;

/**
 * Check if an endpoint should skip the automatic 401 redirect.
 * Some endpoints handle 401 gracefully (e.g., showing a login prompt).
 */
export function shouldSkip401Redirect(endpoint: string): boolean {
  // Check exact matches
  if (SKIP_401_REDIRECT_ENDPOINTS.some(e => endpoint.endsWith(e))) {
    return true;
  }
  // Check session detail pattern (returns 401 when auth may help)
  if (SESSION_DETAIL_PATTERN.test(endpoint)) {
    return true;
  }
  return false;
}
