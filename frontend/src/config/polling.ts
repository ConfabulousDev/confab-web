/**
 * Smart polling configuration for real-time updates.
 * These values can be tuned based on observed behavior.
 */
export const POLLING_CONFIG = {
  /** Poll interval when user is actively interacting with the page */
  ACTIVE_INTERVAL_MS: 30_000,

  /** Poll interval when tab is visible but user is idle */
  PASSIVE_INTERVAL_MS: 60_000,

  /** Time without activity before switching to passive polling */
  IDLE_THRESHOLD_MS: 60_000,

  /** Events that count as user activity */
  ACTIVITY_EVENTS: [
    'click',
    'keydown',
    'mousedown',
    'touchstart',
    'scroll',
  ] as const,
} as const;

export type PollingState = 'suspended' | 'passive' | 'active';
