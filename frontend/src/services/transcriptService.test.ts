import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { parseJSONL, fetchNewTranscriptMessages, fetchParsedTranscript, reportTranscriptErrors, _resetReportedSessions } from './transcriptService';
import type { TranscriptValidationError } from '@/schemas/transcript';
import * as api from './api';

// Mock the api module
vi.mock('./api', () => ({
  syncFilesAPI: {
    getContent: vi.fn(),
  },
}));

// Valid system message that matches the schema
const createSystemMessage = (id: number) => JSON.stringify({
  uuid: `uuid-${id}`,
  timestamp: '2024-01-01T00:00:00Z',
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/test',
  sessionId: 'session-123',
  version: '1.0.0',
  type: 'system',
  subtype: 'info',
  content: `Message ${id}`,
  isMeta: false,
  level: 'info',
});

describe('parseJSONL', () => {
  it('parses valid JSONL content', () => {
    const content = `${createSystemMessage(1)}
${createSystemMessage(2)}`;

    const result = parseJSONL(content);

    expect(result.successCount).toBe(2);
    expect(result.errorCount).toBe(0);
    expect(result.messages).toHaveLength(2);
    expect(result.totalLines).toBe(2);
  });

  it('handles empty lines', () => {
    const content = `${createSystemMessage(1)}

${createSystemMessage(2)}
`;

    const result = parseJSONL(content);

    expect(result.successCount).toBe(2);
    expect(result.errorCount).toBe(0);
    expect(result.totalLines).toBe(2); // Empty lines filtered
  });

  it('handles empty content', () => {
    const result = parseJSONL('');

    expect(result.successCount).toBe(0);
    expect(result.errorCount).toBe(0);
    expect(result.messages).toHaveLength(0);
    expect(result.totalLines).toBe(0);
  });

  it('reports parse errors for invalid JSON', () => {
    const content = `${createSystemMessage(1)}
invalid json line
${createSystemMessage(2)}`;

    const result = parseJSONL(content);

    expect(result.successCount).toBe(2);
    expect(result.errorCount).toBe(1);
    expect(result.totalLines).toBe(3);
    expect(result.errors).toHaveLength(1);
    expect(result.errors[0]?.rawJson).toBe('invalid json line');
  });

  it('accepts unknown message types via catch-all schema (forward compatibility)', () => {
    const content = `${createSystemMessage(1)}
{"type":"unknown","invalid":"data"}
${createSystemMessage(2)}`;

    const result = parseJSONL(content);

    // All 3 lines pass — the unknown type matches the catch-all schema
    expect(result.successCount).toBe(3);
    expect(result.errorCount).toBe(0);
    expect(result.totalLines).toBe(3);
  });

  it('skips progress messages silently', () => {
    // Progress messages are streaming updates that shouldn't be included in transcript view
    const progressMessage = JSON.stringify({
      type: 'progress',
      content: { type: 'bash_output', output: 'some streaming output' },
    });

    const content = `${createSystemMessage(1)}
${progressMessage}
${createSystemMessage(2)}`;

    const result = parseJSONL(content);

    // Progress message should be skipped without adding to errors
    expect(result.successCount).toBe(2);
    expect(result.errorCount).toBe(0);
    expect(result.messages).toHaveLength(2);
    expect(result.totalLines).toBe(3); // All 3 lines counted in totalLines
  });

  it('parses user message with tool_result content block', () => {
    // Real message that was failing to parse
    const userMessageWithToolResult = JSON.stringify({
      "parentUuid": "65cfc905-c9f8-4eb2-b37a-3da75eeeab8d",
      "isSidechain": false,
      "userType": "external",
      "cwd": "/Users/jackie/dev/Nooks.in",
      "sessionId": "75dfb958-2558-46ff-8840-1a4588c13905",
      "version": "2.0.76",
      "gitBranch": "dev",
      "type": "user",
      "message": {
        "role": "user",
        "content": [
          {
            "tool_use_id": "toolu_016wg3ieC28itGiopnDjTN6H",
            "type": "tool_result",
            "content": [
              {
                "type": "text",
                "text": "{\"id\":\"test-id\",\"title\":\"test title\"}"
              }
            ]
          }
        ]
      },
      "uuid": "cd731ee8-edba-4689-a3cd-66e02a1cd0e6",
      "timestamp": "2026-01-05T21:56:57.242Z",
      "toolUseResult": [
        {
          "type": "text",
          "text": "{\"id\":\"test-id\",\"title\":\"test title\"}"
        }
      ]
    });

    const result = parseJSONL(userMessageWithToolResult);

    expect(result.successCount).toBe(1);
    expect(result.errorCount).toBe(0);
    expect(result.messages).toHaveLength(1);
    expect(result.messages[0]?.type).toBe('user');
  });
});

describe('fetchNewTranscriptMessages', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches new messages with line offset', async () => {
    const newContent = `${createSystemMessage(1)}
${createSystemMessage(2)}`;

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(newContent);

    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 5);

    expect(api.syncFilesAPI.getContent).toHaveBeenCalledWith('session-123', 'transcript.jsonl', 5);
    expect(result.newMessages).toHaveLength(2);
    expect(result.newTotalLineCount).toBe(7); // 5 existing + 2 new
  });

  it('returns empty when no new content', async () => {
    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue('');

    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 10);

    expect(result.newMessages).toHaveLength(0);
    expect(result.newTotalLineCount).toBe(10); // unchanged
  });

  it('returns empty for whitespace-only content', async () => {
    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue('   \n  \n  ');

    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 10);

    expect(result.newMessages).toHaveLength(0);
    expect(result.newTotalLineCount).toBe(10);
  });

  it('handles parse errors gracefully - counts all lines for offset tracking', async () => {
    const content = `${createSystemMessage(1)}
invalid line
${createSystemMessage(2)}`;

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(content);

    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 0);

    // Messages array only contains successfully parsed lines
    expect(result.newMessages).toHaveLength(2);
    // But totalLineCount includes ALL lines (including parse errors)
    // This ensures line_offset stays in sync with actual file line numbers
    // and prevents re-fetching lines that failed to parse
    expect(result.newTotalLineCount).toBe(3); // All 3 lines counted
  });

  it('starts from line 0 for initial fetch', async () => {
    const content = createSystemMessage(1);

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(content);

    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 0);

    expect(api.syncFilesAPI.getContent).toHaveBeenCalledWith('session-123', 'transcript.jsonl', 0);
    expect(result.newMessages).toHaveLength(1);
    expect(result.newTotalLineCount).toBe(1);
  });

  it('correctly calculates new total when appending', async () => {
    const newContent = `${createSystemMessage(1)}
${createSystemMessage(2)}
${createSystemMessage(3)}`;

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(newContent);

    // Simulate starting with 100 existing messages
    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 100);

    expect(result.newMessages).toHaveLength(3);
    expect(result.newTotalLineCount).toBe(103); // 100 + 3
  });

  it('prevents duplicate fetching after parse errors (CF-222 fix)', async () => {
    // Scenario: Initial load has 10 lines, 1 fails to parse
    // Bug: If we track messages.length (9), next poll uses line_offset=9
    //      and re-fetches lines 10+, including the error line again
    // Fix: Track totalLines (10), so next poll uses line_offset=10

    // First fetch: 10 lines, 1 parse error
    const initialContent = Array.from({ length: 9 }, (_, i) => createSystemMessage(i + 1)).join('\n') +
      '\ninvalid line that fails to parse';

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(initialContent);

    const firstResult = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 0);

    expect(firstResult.newMessages).toHaveLength(9); // 9 valid messages
    expect(firstResult.newTotalLineCount).toBe(10); // But 10 total lines

    // Second fetch: no new content (empty response when line_offset >= total lines)
    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue('');

    // Using the correct totalLineCount from first result
    const secondResult = await fetchNewTranscriptMessages(
      'session-123',
      'transcript.jsonl',
      firstResult.newTotalLineCount // 10, not 9
    );

    // Should get no duplicates
    expect(secondResult.newMessages).toHaveLength(0);
    expect(secondResult.newTotalLineCount).toBe(10);

    // Verify the API was called with correct offset
    expect(api.syncFilesAPI.getContent).toHaveBeenLastCalledWith('session-123', 'transcript.jsonl', 10);
  });
});

describe('reportTranscriptErrors', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    _resetReportedSessions();
    fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"status":"ok"}'));
  });

  afterEach(() => {
    fetchSpy.mockRestore();
  });

  /** Extract the parsed JSON body from a fetch spy call */
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

  it('sends errors to the backend with correct payload structure', () => {
    const errors = [makeError(42, 'assistant')];
    reportTranscriptErrors('session-abc', errors);

    expect(fetchSpy).toHaveBeenCalledOnce();
    const [url, options] = fetchSpy.mock.calls[0]!;
    expect(url).toBe('/api/v1/client-errors');
    expect(options?.method).toBe('POST');
    expect(options?.credentials).toBe('include');

    const body = parseFetchBody(fetchSpy);
    expect(body.category).toBe('transcript_validation');
    expect(body.session_id).toBe('session-abc');
    expect(body.errors).toHaveLength(1);
    expect(body.errors[0].line).toBe(42);
    expect(body.errors[0].message_type).toBe('assistant');
    expect(body.errors[0].details).toHaveLength(1);
    expect(body.errors[0].details[0].path).toBe('content.0.type');
    expect(body.errors[0].details[0].expected).toBe('text');
    expect(body.errors[0].details[0].received).toBe('new_type');
  });

  it('truncates raw_json_preview to 500 chars', () => {
    const longJson = 'x'.repeat(1000);
    const errors: TranscriptValidationError[] = [{
      line: 1,
      rawJson: longJson,
      errors: [{ path: 'root', message: 'bad' }],
    }];

    reportTranscriptErrors('session-long', errors);

    const body = parseFetchBody(fetchSpy);
    expect(body.errors[0].raw_json_preview).toHaveLength(500);
  });

  it('limits to 50 errors per report', () => {
    const errors = Array.from({ length: 100 }, (_, i) => makeError(i + 1));
    reportTranscriptErrors('session-many', errors);

    const body = parseFetchBody(fetchSpy);
    expect(body.errors).toHaveLength(50);
  });

  it('silently ignores fetch failures', () => {
    fetchSpy.mockRejectedValue(new Error('Network error'));

    // Should not throw
    expect(() => reportTranscriptErrors('session-fail', [makeError(1)])).not.toThrow();
  });
});

describe('error reporting dedup in fetchParsedTranscript', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    _resetReportedSessions();
    vi.clearAllMocks();
    fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"status":"ok"}'));
  });

  afterEach(() => {
    fetchSpy.mockRestore();
    vi.restoreAllMocks();
  });

  it('reports errors on first parse but not on subsequent parses of same session', async () => {
    // Content with a line that will fail validation (not valid JSON, not a valid message)
    const contentWithError = `${createSystemMessage(1)}
not-valid-json
${createSystemMessage(2)}`;

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(contentWithError);

    // First call: should report errors
    await fetchParsedTranscript('session-dedup', 'transcript.jsonl', true);
    expect(fetchSpy).toHaveBeenCalledOnce();

    // Second call (same session, skipCache): should NOT report again
    await fetchParsedTranscript('session-dedup', 'transcript.jsonl', true);
    expect(fetchSpy).toHaveBeenCalledOnce(); // still 1

    // Different session: should report
    await fetchParsedTranscript('session-dedup-2', 'transcript.jsonl', true);
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it('does not report when there are no errors', async () => {
    const validContent = `${createSystemMessage(1)}
${createSystemMessage(2)}`;

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(validContent);

    await fetchParsedTranscript('session-no-errors', 'transcript.jsonl', true);
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
