import { afterEach, describe, expect, it, vi } from 'vitest';
import { getDemoIdentity, notifyReadOnlyDemo, READ_ONLY_EVENT } from './demoIdentity';

// CF-483: getDemoIdentity must defensively read window.__DEMO_IDENTITY__
// and only return a non-empty string. Anything else returns null so
// downstream components stay hidden.

declare global {
  interface Window {
    __DEMO_IDENTITY__?: unknown;
  }
}

afterEach(() => {
  delete window.__DEMO_IDENTITY__;
});

describe('getDemoIdentity', () => {
  it('returns null when global is undefined', () => {
    expect(getDemoIdentity()).toBeNull();
  });

  it('returns null when global is empty string', () => {
    window.__DEMO_IDENTITY__ = '';
    expect(getDemoIdentity()).toBeNull();
  });

  it('returns null when global is non-string', () => {
    window.__DEMO_IDENTITY__ = 123;
    expect(getDemoIdentity()).toBeNull();
    window.__DEMO_IDENTITY__ = { email: 'demo@example.com' };
    expect(getDemoIdentity()).toBeNull();
    window.__DEMO_IDENTITY__ = null;
    expect(getDemoIdentity()).toBeNull();
  });

  it('returns the email when global is a non-empty string', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    expect(getDemoIdentity()).toBe('demo@confabulous.dev');
  });
});

describe('notifyReadOnlyDemo', () => {
  it('dispatches the documented CustomEvent name on window', () => {
    const listener = vi.fn();
    window.addEventListener(READ_ONLY_EVENT, listener);
    try {
      notifyReadOnlyDemo();
      expect(listener).toHaveBeenCalledTimes(1);
      const firstCall = listener.mock.calls[0];
      expect(firstCall).toBeDefined();
      const ev = firstCall?.[0];
      expect(ev).toBeInstanceOf(Event);
      expect(ev?.type).toBe(READ_ONLY_EVENT);
    } finally {
      window.removeEventListener(READ_ONLY_EVENT, listener);
    }
  });
});
