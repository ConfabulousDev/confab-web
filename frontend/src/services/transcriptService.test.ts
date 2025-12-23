import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { parseJSONL, fetchNewTranscriptMessages } from './transcriptService';
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

  it('reports validation errors for invalid schema', () => {
    const content = `${createSystemMessage(1)}
{"type":"unknown","invalid":"data"}
${createSystemMessage(2)}`;

    const result = parseJSONL(content);

    expect(result.successCount).toBe(2);
    expect(result.errorCount).toBe(1);
    expect(result.totalLines).toBe(3);
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

  it('handles parse errors gracefully - only counts successful parses', async () => {
    const content = `${createSystemMessage(1)}
invalid line
${createSystemMessage(2)}`;

    vi.mocked(api.syncFilesAPI.getContent).mockResolvedValue(content);

    const result = await fetchNewTranscriptMessages('session-123', 'transcript.jsonl', 0);

    // Only successfully parsed messages count toward the total
    expect(result.newMessages).toHaveLength(2);
    expect(result.newTotalLineCount).toBe(2); // Only successful parses
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
});
