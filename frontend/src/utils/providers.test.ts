import { describe, it, expect } from 'vitest';
import {
  PROVIDER_VALUES,
  PROVIDER_METADATA,
  getProviderMetadata,
  getProviderMetadataOrFallback,
  providerLabel,
} from './providers';
import { ClaudeCodeIcon, CodexIcon, OpenCodeIcon } from '@/components/icons';

// CF-416 (Phase 1 of frontend provider abstraction).
//
// These tests pin the registry contract:
//   - every canonical PROVIDER_VALUES entry has metadata (drift guard)
//   - getProviderMetadata is the strict canonical lookup
//   - getProviderMetadataOrFallback is the SINGLE place encoding the
//     unknown-provider policy: 'claude' → claude-code metadata,
//     'neutral' → null
//   - legacy 'Claude Code' display form normalizes to the claude-code entry
//   - providerLabel preserves passthrough-for-unknown (so backend-first
//     provider rollouts still render readable chips)
describe('PROVIDER_METADATA', () => {
  it('has an entry for every PROVIDER_VALUES id (drift guard)', () => {
    for (const id of PROVIDER_VALUES) {
      expect(PROVIDER_METADATA[id]).toBeDefined();
      expect(PROVIDER_METADATA[id].id).toBe(id);
    }
  });

  it('has non-empty cosmetic fields on every entry', () => {
    for (const id of PROVIDER_VALUES) {
      const meta = PROVIDER_METADATA[id];
      expect(meta.label).not.toBe('');
      expect(meta.brandDisplayName).not.toBe('');
      expect(meta.icon).toBeDefined();
      expect(meta.brandColor).toMatch(/^#[0-9a-fA-F]{6}$/);
      expect(meta.resumeCommand.idLabel).not.toBe('');
      expect(meta.resumeCommand.commandHint).not.toBe('');
    }
  });

  it('pins the claude-code entry strings (UI contract)', () => {
    const meta = PROVIDER_METADATA['claude-code'];
    expect(meta.label).toBe('Claude Code');
    expect(meta.brandDisplayName).toBe('Claude Code');
    expect(meta.icon).toBe(ClaudeCodeIcon);
    expect(meta.brandColor).toBe('#d97757');
    expect(meta.resumeCommand.idLabel).toBe('Copy Claude Code ID');
    expect(meta.resumeCommand.commandHint).toBe('for /resume');
  });

  it('pins the codex entry strings (UI contract)', () => {
    const meta = PROVIDER_METADATA.codex;
    expect(meta.label).toBe('Codex');
    expect(meta.brandDisplayName).toBe('Codex');
    expect(meta.icon).toBe(CodexIcon);
    expect(meta.brandColor).toBe('#10a37f');
    expect(meta.resumeCommand.idLabel).toBe('Copy Codex ID');
    expect(meta.resumeCommand.commandHint).toBe('for codex resume');
  });

  // CF-544: pins the official OpenCode branding (icon mark + grayscale
  // brandColor) against future drift. brandColor is the official medium-dark
  // gray #656363 (the monochrome brand) — NOT the placeholder #6366f1 it
  // replaced.
  it('pins the opencode entry strings (UI contract)', () => {
    const meta = PROVIDER_METADATA.opencode;
    expect(meta.label).toBe('OpenCode');
    expect(meta.brandDisplayName).toBe('OpenCode');
    expect(meta.icon).toBe(OpenCodeIcon);
    expect(meta.brandColor).toBe('#656363');
    expect(meta.resumeCommand.idLabel).toBe('Copy OpenCode ID');
    expect(meta.resumeCommand.commandHint).toBe('for opencode resume');
  });
});

describe('getProviderMetadata', () => {
  it('returns the claude-code entry', () => {
    expect(getProviderMetadata('claude-code')).toBe(PROVIDER_METADATA['claude-code']);
  });

  it('returns the codex entry', () => {
    expect(getProviderMetadata('codex')).toBe(PROVIDER_METADATA.codex);
  });
});

describe('getProviderMetadataOrFallback', () => {
  describe("fallback: 'claude'", () => {
    it('returns claude-code for the canonical id', () => {
      expect(getProviderMetadataOrFallback('claude-code', 'claude')).toBe(
        PROVIDER_METADATA['claude-code'],
      );
    });

    it('returns codex for the canonical id', () => {
      expect(getProviderMetadataOrFallback('codex', 'claude')).toBe(PROVIDER_METADATA.codex);
    });

    it('falls back to claude-code for an unknown provider', () => {
      expect(getProviderMetadataOrFallback('windsurf', 'claude')).toBe(
        PROVIDER_METADATA['claude-code'],
      );
    });

    it('falls back to claude-code for an empty provider', () => {
      expect(getProviderMetadataOrFallback('', 'claude')).toBe(PROVIDER_METADATA['claude-code']);
    });

    it("normalizes the legacy 'Claude Code' display form to claude-code", () => {
      expect(getProviderMetadataOrFallback('Claude Code', 'claude')).toBe(
        PROVIDER_METADATA['claude-code'],
      );
    });
  });

  describe("fallback: 'neutral'", () => {
    it('returns claude-code for the canonical id', () => {
      expect(getProviderMetadataOrFallback('claude-code', 'neutral')).toBe(
        PROVIDER_METADATA['claude-code'],
      );
    });

    it('returns codex for the canonical id', () => {
      expect(getProviderMetadataOrFallback('codex', 'neutral')).toBe(PROVIDER_METADATA.codex);
    });

    it('returns null for an unknown provider', () => {
      expect(getProviderMetadataOrFallback('windsurf', 'neutral')).toBeNull();
    });

    it('returns null for an empty provider', () => {
      expect(getProviderMetadataOrFallback('', 'neutral')).toBeNull();
    });

    it("normalizes the legacy 'Claude Code' display form to claude-code", () => {
      expect(getProviderMetadataOrFallback('Claude Code', 'neutral')).toBe(
        PROVIDER_METADATA['claude-code'],
      );
    });
  });
});

describe('providerLabel', () => {
  it('returns the canonical label for claude-code', () => {
    expect(providerLabel('claude-code')).toBe('Claude Code');
  });

  it('returns the canonical label for codex', () => {
    expect(providerLabel('codex')).toBe('Codex');
  });

  it('passes through unknown values (backend-first rollout safety)', () => {
    expect(providerLabel('windsurf')).toBe('windsurf');
  });

  it('passes through empty string', () => {
    expect(providerLabel('')).toBe('');
  });
});
