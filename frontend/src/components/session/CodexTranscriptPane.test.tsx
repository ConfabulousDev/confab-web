// CF-386 — CodexTranscriptPane is presentational after the lift. Fetch + poll
// live in SessionViewer; this component just renders the props it's given.
// These tests pin the new prop contract.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import CodexTranscriptPane from './CodexTranscriptPane';
import { parseCodexJSONL } from '@/services/codexTranscriptService';
import type { RawCodexLine } from '@/schemas/codexTranscript';

// CodexMessageTimeline is heavy (virtualization, search bar, etc.) — stub it
// so these tests are pure prop-rendering assertions.
vi.mock('@/components/transcript/codex/CodexMessageTimeline', () => ({
  default: ({ items }: { items: unknown[] }) => (
    <div data-testid="codex-message-timeline" data-items-count={items.length} />
  ),
}));

// Schema-validate test rawLines via the real parser so we don't hand-roll
// `as unknown as RawCodexLine` casts (matches the service test's `rawLine`).
// Throws on parse failure so a malformed test fixture surfaces immediately.
function rawLine(jsonl: string): RawCodexLine {
  const line = parseCodexJSONL(jsonl).rawLines[0];
  if (!line) throw new Error(`rawLine helper: failed to parse ${jsonl}`);
  return line;
}

const minimalRollout: RawCodexLine[] = [
  rawLine(
    '{"timestamp":"2026-05-13T01:00:00Z","type":"session_meta","payload":{"id":"x","model":"gpt-5-codex"}}',
  ),
  rawLine(
    '{"timestamp":"2026-05-13T01:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}',
  ),
];

describe('CodexTranscriptPane (presentational)', () => {
  it('renders a loading indicator when loading is true', () => {
    render(
      <CodexTranscriptPane
        sessionId="s1"
        rawLines={[]}
        loading={true}
        error={null}
      />
    );
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
    expect(screen.queryByTestId('codex-message-timeline')).not.toBeInTheDocument();
  });

  it('renders the error message when error is set', () => {
    render(
      <CodexTranscriptPane
        sessionId="s1"
        rawLines={[]}
        loading={false}
        error="boom"
      />
    );
    expect(screen.getByText(/boom/i)).toBeInTheDocument();
    expect(screen.queryByTestId('codex-message-timeline')).not.toBeInTheDocument();
  });

  it('renders the timeline when rawLines are present and not loading', () => {
    render(
      <CodexTranscriptPane
        sessionId="s1"
        rawLines={minimalRollout}
        loading={false}
        error={null}
      />
    );
    expect(screen.getByTestId('codex-message-timeline')).toBeInTheDocument();
  });

  it('renders the timeline with zero items when rawLines is empty (not loading, no error)', () => {
    render(
      <CodexTranscriptPane
        sessionId="s1"
        rawLines={[]}
        loading={false}
        error={null}
      />
    );
    const timeline = screen.getByTestId('codex-message-timeline');
    expect(timeline).toHaveAttribute('data-items-count', '0');
  });
});
