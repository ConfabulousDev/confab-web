import { describe, it, expect } from 'vitest';
import { getProviderIcon } from './providerIcon';
import { ClaudeCodeIcon, CodexIcon, OpenCodeIcon, RobotIcon } from './icons';

// getProviderIcon is the single seam SessionsPage / SessionHeader / any
// future session-row component uses to pick the brand icon for a session.
// CF-353 pins that codex sessions render CodexIcon. CF-366 split the
// fallback policy: canonical and legacy values still resolve to their
// brand icon (claude-code and the legacy "Claude Code" form both render
// the Claude logo), but truly unknown values now render the neutral
// RobotIcon — so a future third-party provider mid-rollout never silently
// impersonates Claude on the UI.
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

  // CF-544: pins the OpenCode mark to opencode sessions.
  it('returns OpenCodeIcon for the canonical opencode provider', () => {
    expect(getProviderIcon('opencode')).toBe(OpenCodeIcon);
  });

  it('returns ClaudeCodeIcon for the legacy "Claude Code" display form', () => {
    // Backend normalizes at every Scan site, but a response may slip
    // through unnormalised — the icon helper must still recover.
    expect(getProviderIcon('Claude Code')).toBe(ClaudeCodeIcon);
  });

  it('returns ClaudeCodeIcon for uppercase canonical form', () => {
    expect(getProviderIcon('CLAUDE-CODE')).toBe(ClaudeCodeIcon);
  });

  it('returns RobotIcon for empty provider', () => {
    expect(getProviderIcon('')).toBe(RobotIcon);
  });

  it('returns RobotIcon for unknown future providers', () => {
    expect(getProviderIcon('windsurf')).toBe(RobotIcon);
  });
});
