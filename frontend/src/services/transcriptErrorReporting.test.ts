import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { TranscriptValidationError } from '@/schemas/claudeTranscript';
import { reportTranscriptErrors } from './transcriptErrorReporting';

describe('reportTranscriptErrors', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"status":"ok"}'));
  });

  afterEach(() => {
    fetchSpy.mockRestore();
  });

  const parseFetchBody = (spy: ReturnType<typeof vi.spyOn>, callIndex = 0) =>
    JSON.parse(String(spy.mock.calls[callIndex]![1]?.body ?? ''));

  const makeError = (line: number, messageType?: string): TranscriptValidationError => ({
    line,
    rawJson: `{"type":"${messageType ?? 'unknown'}","bad":"data"}`,
    messageType,
    errors: [
      { path: 'content.0.type', message: 'Invalid type', expected: 'text', received: 'new_type' },
    ],
  });

  it('threads the provided category into the payload', () => {
    reportTranscriptErrors('session-codex', [makeError(1)], 'codex_transcript_validation');
    expect(parseFetchBody(fetchSpy).category).toBe('codex_transcript_validation');

    reportTranscriptErrors('session-claude', [makeError(1)], 'transcript_validation');
    expect(parseFetchBody(fetchSpy, 1).category).toBe('transcript_validation');
  });

  it('POSTs the full payload structure once to /api/v1/client-errors', () => {
    reportTranscriptErrors('session-abc', [makeError(42, 'assistant')], 'transcript_validation');

    expect(fetchSpy).toHaveBeenCalledOnce();
    const [url, options] = fetchSpy.mock.calls[0]!;
    expect(url).toBe('/api/v1/client-errors');
    expect(options?.method).toBe('POST');
    expect(options?.credentials).toBe('include');

    const body = parseFetchBody(fetchSpy);
    expect(body.session_id).toBe('session-abc');
    expect(body.errors).toHaveLength(1);
    expect(body.errors[0].line).toBe(42);
    expect(body.errors[0].message_type).toBe('assistant');
    expect(body.errors[0].details[0].path).toBe('content.0.type');
    expect(body.errors[0].details[0].expected).toBe('text');
    expect(body.errors[0].details[0].received).toBe('new_type');
  });

  it('truncates raw_json_preview to 500 chars and limits to 50 errors', () => {
    const errors = Array.from({ length: 100 }, (_, i) => ({
      line: i + 1,
      rawJson: 'x'.repeat(1000),
      errors: [{ path: 'root', message: 'bad' }],
    }));
    reportTranscriptErrors('session-many', errors, 'transcript_validation');

    const body = parseFetchBody(fetchSpy);
    expect(body.errors).toHaveLength(50);
    expect(body.errors[0].raw_json_preview).toHaveLength(500);
  });

  it('silently ignores fetch failures (fire-and-forget)', () => {
    fetchSpy.mockRejectedValue(new Error('Network error'));
    expect(() =>
      reportTranscriptErrors('session-fail', [makeError(1)], 'transcript_validation'),
    ).not.toThrow();
  });
});
