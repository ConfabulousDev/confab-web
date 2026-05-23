// CF-483: Demo identity helpers.
//
// The backend injects `<script>window.__DEMO_IDENTITY__ = "<email>"</script>`
// into index.html when DEMO_IDENTITY_EMAIL is set. The frontend reads that
// global to decide whether to render the read-only demo banner and to fire
// the read-only toast on blocked mutations.

declare global {
  interface Window {
    __DEMO_IDENTITY__?: unknown;
  }
}

/**
 * Returns the configured demo identity email if the backend injected the
 * global, or null when demo mode is off or the global is malformed.
 *
 * Defensive: never throws, never trusts unknown shapes, never returns
 * empty string (treated as "not configured"). Consumers can safely
 * `if (getDemoIdentity()) { ... }` to gate UI.
 */
export function getDemoIdentity(): string | null {
  const raw = window.__DEMO_IDENTITY__;
  if (typeof raw !== 'string' || raw === '') return null;
  return raw;
}

/**
 * True when demo mode is on AND the supplied email matches the configured
 * demo identity. Used by Header, HomePage, and LoginPage to branch UI for
 * the read-only demo viewer (skip ?owner= pre-filter, show "Log in as
 * yourself", skip post-login redirect, etc.).
 */
export function isDemoViewer(email: string | undefined | null): boolean {
  const demoEmail = getDemoIdentity();
  return demoEmail !== null && demoEmail === email;
}

/**
 * Name of the CustomEvent dispatched on window when an API call returns
 * the documented `read_only_user` structured error. ReadOnlyToast listens
 * for this event; api.ts dispatches it.
 *
 * Decoupled via CustomEvent so api.ts has no React import and no provider
 * dependency.
 */
export const READ_ONLY_EVENT = 'confab:read-only';

export function notifyReadOnlyDemo(): void {
  window.dispatchEvent(new CustomEvent(READ_ONLY_EVENT));
}
