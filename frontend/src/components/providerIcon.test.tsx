import { describe, it, expect } from 'vitest';
import { getProviderIcon } from './providerIcon';
import { ClaudeCodeIcon, CodexIcon } from './icons';

// getProviderIcon is the single seam SessionsPage / SessionHeader / any
// future session-row component uses to pick the brand icon for a session.
// CF-353 pins that codex sessions render CodexIcon and never ClaudeCodeIcon,
// and that the fallback for unknown/empty values is ClaudeCodeIcon
// (intentional default for legacy and unrecognised rows).
describe('getProviderIcon', () => {
  it('returns CodexIcon for the canonical codex provider', () => {
    expect(getProviderIcon('codex')).toBe(CodexIcon);
  });

  it('returns CodexIcon and never ClaudeCodeIcon for codex sessions', () => {
    const icon = getProviderIcon('codex');
    expect(icon).toBe(CodexIcon);
    expect(icon).not.toBe(ClaudeCodeIcon);
  });

  it('returns ClaudeCodeIcon for the canonical claude-code provider', () => {
    expect(getProviderIcon('claude-code')).toBe(ClaudeCodeIcon);
  });

  it('returns ClaudeCodeIcon for the legacy "Claude Code" display form', () => {
    // Defensive: legacy rows still in production. Backend normalizes at
    // every Scan site, but the frontend can also see this value if a
    // response slips through unnormalised.
    expect(getProviderIcon('Claude Code')).toBe(ClaudeCodeIcon);
  });

  it('falls back to ClaudeCodeIcon for empty provider', () => {
    expect(getProviderIcon('')).toBe(ClaudeCodeIcon);
  });

  it('falls back to ClaudeCodeIcon for unknown future providers', () => {
    expect(getProviderIcon('windsurf')).toBe(ClaudeCodeIcon);
  });
});
