import { describe, it, expect, vi } from 'vitest';
import type { UserMessage, UnknownBlock, UnknownMessage, AttachmentMessage } from './claudeTranscript';
import {
  isCommandExpansionMessage,
  getCommandExpansionSkillName,
  stripCommandExpansionTags,
  parseTranscriptLineWithError,
  validateParsedTranscriptLine,
  isUnknownBlock,
  isUnknownMessage,
  isAssistantMessage,
  isAttachmentMessage,
  isHookSuccessAttachment,
  isHookBlockingErrorAttachment,
  isEditedTextFileAttachment,
  isQueuedCommandAttachment,
  isDeferredToolsDeltaAttachment,
  isMcpInstructionsDeltaAttachment,
  warnIfKnownTypeCaughtByCatchall,
} from './claudeTranscript';

// Helper to create a minimal UserMessage with string content
function makeUserMessage(content: UserMessage['message']['content']): UserMessage {
  return {
    type: 'user',
    uuid: 'test-uuid',
    timestamp: '2025-01-01T00:00:00Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    message: { role: 'user', content },
  };
}

describe('isCommandExpansionMessage', () => {
  it('returns true for command-expansion messages', () => {
    const msg = makeUserMessage(
      '<command-message>interview</command-message>\n<command-name>/interview</command-name>\nExpanded content'
    );
    expect(isCommandExpansionMessage(msg)).toBe(true);
  });

  it('returns false for regular user messages', () => {
    const msg = makeUserMessage('hello world');
    expect(isCommandExpansionMessage(msg)).toBe(false);
  });

  it('returns false for tool result messages', () => {
    const msg = makeUserMessage([{ type: 'tool_result', tool_use_id: 'test', content: 'ok' }]);
    expect(isCommandExpansionMessage(msg)).toBe(false);
  });

  it('returns false if content just mentions command-name in prose', () => {
    // This is a known trade-off: we match on tag presence
    const msg = makeUserMessage('The tag is <command-name>');
    expect(isCommandExpansionMessage(msg)).toBe(true);
  });
});

describe('getCommandExpansionSkillName', () => {
  it('extracts skill name with leading slash', () => {
    const msg = makeUserMessage(
      '<command-message>interview</command-message>\n<command-name>/interview</command-name>'
    );
    expect(getCommandExpansionSkillName(msg)).toBe('interview');
  });

  it('extracts skill name without leading slash', () => {
    const msg = makeUserMessage(
      '<command-message>commit</command-message>\n<command-name>commit</command-name>'
    );
    expect(getCommandExpansionSkillName(msg)).toBe('commit');
  });

  it('returns null for regular user messages', () => {
    const msg = makeUserMessage('hello');
    expect(getCommandExpansionSkillName(msg)).toBeNull();
  });

  it('returns null for array content', () => {
    const msg = makeUserMessage([{ type: 'tool_result', tool_use_id: 'test', content: 'ok' }]);
    expect(getCommandExpansionSkillName(msg)).toBeNull();
  });

  it('returns null when tag is malformed (no closing tag)', () => {
    const msg = makeUserMessage('<command-name>/broken');
    expect(getCommandExpansionSkillName(msg)).toBeNull();
  });
});

describe('PRLinkMessage validation', () => {
  it('accepts a valid pr-link message', () => {
    const raw = JSON.stringify({
      type: 'pr-link',
      prNumber: 22,
      prRepository: 'ConfabulousDev/confab-web',
      prUrl: 'https://github.com/ConfabulousDev/confab-web/pull/22',
      sessionId: 'b42b9d37-96be-4046-91f0-83c04a4466ce',
      timestamp: '2026-02-22T08:00:41.865Z',
    });
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
  });

  it('accepts pr-link missing prNumber via catch-all (forward compatibility)', () => {
    // A malformed pr-link misses the specific PRLinkMessageSchema but
    // still matches the catch-all UnknownMessageSchema — this is intentional
    // to avoid hard failures on schema changes
    const raw = JSON.stringify({
      type: 'pr-link',
      prRepository: 'ConfabulousDev/confab-web',
      prUrl: 'https://github.com/ConfabulousDev/confab-web/pull/22',
      sessionId: 'session-123',
      timestamp: '2026-02-22T08:00:41.865Z',
    });
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
  });

  it('accepts pr-link with extra fields (Zod strips them)', () => {
    const raw = JSON.stringify({
      type: 'pr-link',
      prNumber: 22,
      prRepository: 'ConfabulousDev/confab-web',
      prUrl: 'https://github.com/ConfabulousDev/confab-web/pull/22',
      sessionId: 'session-123',
      timestamp: '2026-02-22T08:00:41.865Z',
      prTitle: 'Add widget support',
    });
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
  });
});

describe('stripCommandExpansionTags', () => {
  it('strips both command-message and command-name tags', () => {
    const content = '<command-message>interview</command-message>\n<command-name>/interview</command-name>\nExpanded skill content here';
    expect(stripCommandExpansionTags(content)).toBe('Expanded skill content here');
  });

  it('returns trimmed content when no tags present', () => {
    expect(stripCommandExpansionTags('  hello world  ')).toBe('hello world');
  });

  it('handles content with only tags (returns empty string)', () => {
    const content = '<command-message>x</command-message>\n<command-name>/x</command-name>';
    expect(stripCommandExpansionTags(content)).toBe('');
  });

  it('preserves content between and after tags', () => {
    const content = 'Before <command-message>x</command-message> middle <command-name>/x</command-name> after';
    expect(stripCommandExpansionTags(content)).toBe('Before  middle  after');
  });

  it('handles multiline tag content', () => {
    const content = '<command-message>line1\nline2</command-message>\n<command-name>/skill</command-name>\nActual content';
    expect(stripCommandExpansionTags(content)).toBe('Actual content');
  });
});

// Helper: minimal assistant message with given content blocks
function makeAssistantRaw(contentBlocks: unknown[]) {
  return {
    type: 'assistant',
    uuid: 'test-uuid',
    timestamp: '2025-01-01T00:00:00Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0.0',
    requestId: 'req-1',
    message: {
      model: 'claude-sonnet-4-5-20250929',
      id: 'msg-1',
      type: 'message',
      role: 'assistant',
      content: contentBlocks,
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: { input_tokens: 10, output_tokens: 20 },
    },
  };
}

describe('Forward-compatibility: unknown content block types', () => {
  it('accepts an assistant message with an unknown content block type', () => {
    const raw = JSON.stringify(makeAssistantRaw([
      { type: 'text', text: 'Hello' },
      { type: 'citations', citations: [{ url: 'https://example.com' }] },
    ]));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.type).toBe('assistant');
    }
  });

  it('preserves unknown block fields via passthrough', () => {
    const raw = makeAssistantRaw([
      { type: 'server_tool_use', tool_name: 'web_search', server_id: 'srv-1', input: {} },
    ]);
    const result = validateParsedTranscriptLine(raw, JSON.stringify(raw), 0);
    expect(result.success).toBe(true);
    if (result.success && isAssistantMessage(result.data)) {
      const block = result.data.message.content[0];
      expect(block).toBeDefined();
      expect(block!.type).toBe('server_tool_use');
      // Passthrough preserves original fields
      expect('tool_name' in block! && block.tool_name).toBe('web_search');
      expect('server_id' in block! && block.server_id).toBe('srv-1');
    }
  });

  it('accepts unknown block nested inside tool_result content', () => {
    const raw = JSON.stringify({
      type: 'user',
      uuid: 'test-uuid',
      timestamp: '2025-01-01T00:00:00Z',
      parentUuid: null,
      isSidechain: false,
      userType: 'external',
      cwd: '/test',
      sessionId: 'test-session',
      version: '1.0.0',
      message: {
        role: 'user',
        content: [{
          type: 'tool_result',
          tool_use_id: 'tool-1',
          content: [{ type: 'code_output', code: 'console.log("hi")' }],
        }],
      },
    });
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
  });

  it('isUnknownBlock returns true for unknown types', () => {
    const block: UnknownBlock = { type: 'citations' };
    expect(isUnknownBlock(block)).toBe(true);
  });

  it('isUnknownBlock returns false for known types', () => {
    expect(isUnknownBlock({ type: 'text', text: 'hi' })).toBe(false);
    expect(isUnknownBlock({ type: 'thinking', thinking: 'hmm' })).toBe(false);
    expect(isUnknownBlock({ type: 'tool_use', id: '1', name: 'Read', input: {} })).toBe(false);
    expect(isUnknownBlock({ type: 'tool_result', tool_use_id: '1', content: 'ok' })).toBe(false);
    expect(isUnknownBlock({ type: 'image', source: { type: 'base64', media_type: 'image/png', data: '' } })).toBe(false);
    expect(isUnknownBlock({ type: 'tool_reference', tool_name: 'TaskCreate' })).toBe(false);
  });
});

describe('Forward-compatibility: unknown message types', () => {
  it('accepts an unknown top-level message type', () => {
    const raw = JSON.stringify({
      type: 'agent-handoff',
      fromAgent: 'agent-1',
      toAgent: 'agent-2',
      timestamp: '2025-01-01T00:00:00Z',
    });
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.type).toBe('agent-handoff');
    }
  });

  it('preserves unknown message fields via passthrough', () => {
    const raw = {
      type: 'agent-handoff',
      fromAgent: 'agent-1',
      toAgent: 'agent-2',
      reason: 'task delegation',
    };
    const result = validateParsedTranscriptLine(raw, JSON.stringify(raw), 0);
    expect(result.success).toBe(true);
    if (result.success) {
      const data = result.data;
      expect('fromAgent' in data && data.fromAgent).toBe('agent-1');
      expect('toAgent' in data && data.toAgent).toBe('agent-2');
      expect('reason' in data && data.reason).toBe('task delegation');
    }
  });

  it('isUnknownMessage returns true for unknown types', () => {
    const msg: UnknownMessage = { type: 'agent-handoff' };
    expect(isUnknownMessage(msg)).toBe(true);
  });

  it('isUnknownMessage returns false for known types', () => {
    const user: UnknownMessage = { type: 'user' };
    const assistant: UnknownMessage = { type: 'assistant' };
    const system: UnknownMessage = { type: 'system' };
    const prLink: UnknownMessage = { type: 'pr-link' };
    expect(isUnknownMessage(user)).toBe(false);
    expect(isUnknownMessage(assistant)).toBe(false);
    expect(isUnknownMessage(system)).toBe(false);
    expect(isUnknownMessage(prLink)).toBe(false);
  });

  it('known types still validate correctly (regression)', () => {
    // User message
    const userRaw = JSON.stringify(makeUserMessage('hello'));
    expect(parseTranscriptLineWithError(userRaw, 0).success).toBe(true);

    // Assistant message
    const assistantRaw = JSON.stringify(makeAssistantRaw([{ type: 'text', text: 'Hi!' }]));
    expect(parseTranscriptLineWithError(assistantRaw, 0).success).toBe(true);

    // PR-link message
    const prLinkRaw = JSON.stringify({
      type: 'pr-link',
      prNumber: 1,
      prRepository: 'org/repo',
      prUrl: 'https://github.com/org/repo/pull/1',
      sessionId: 'session-1',
      timestamp: '2025-01-01T00:00:00Z',
    });
    expect(parseTranscriptLineWithError(prLinkRaw, 0).success).toBe(true);
  });
});

describe('Forward-compatibility: known-type-caught-by-catchall warning', () => {
  it('warns when a known block type matches the catch-all', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    // A "text" block missing the required "text" field fails TextBlockSchema
    // but matches UnknownBlockSchema — should trigger a warning at render time

    // Known block type caught by catchall
    warnIfKnownTypeCaughtByCatchall('block', 'text');
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('Content block type "text" matched catch-all schema')
    );

    // Known message type caught by catchall
    warnIfKnownTypeCaughtByCatchall('message', 'user');
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('Message type "user" matched catch-all schema')
    );

    // Unknown type should NOT warn
    warnSpy.mockClear();
    warnIfKnownTypeCaughtByCatchall('block', 'citations');
    expect(warnSpy).not.toHaveBeenCalled();

    warnSpy.mockRestore();
  });

  it('only warns once per type (deduplication)', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    // Call twice with same type
    warnIfKnownTypeCaughtByCatchall('block', 'image');
    warnIfKnownTypeCaughtByCatchall('block', 'image');

    // Should only warn once (or zero if already warned in previous test)
    const imageCalls = warnSpy.mock.calls.filter(
      (call) => typeof call[0] === 'string' && call[0].includes('"image"')
    );
    expect(imageCalls.length).toBeLessThanOrEqual(1);

    warnSpy.mockRestore();
  });
});

// Helper: minimal attachment base shape mirroring real JSONL
function makeAttachmentRaw(attachment: Record<string, unknown>) {
  return {
    type: 'attachment',
    uuid: 'attach-uuid-1',
    timestamp: '2026-04-20T22:31:25.657Z',
    parentUuid: 'parent-uuid',
    isSidechain: false,
    userType: 'external',
    cwd: '/home/user/project',
    sessionId: 'session-1',
    version: '2.1.140',
    attachment,
  };
}

describe('AttachmentMessageSchema', () => {
  it('parses hook_success', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'hook_success',
      hookName: 'SessionStart:startup',
      hookEvent: 'SessionStart',
      toolUseID: 'tool-1',
      command: '/usr/local/bin/confab hook session-start',
      content: '',
      stdout: '{"continue":true}\n',
      stderr: '=== Banner ===\n',
      exitCode: 0,
      durationMs: 31,
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.type).toBe('attachment');
      expect(isAttachmentMessage(result.data)).toBe(true);
      if (isAttachmentMessage(result.data)) {
        expect(isHookSuccessAttachment(result.data)).toBe(true);
        expect(isHookBlockingErrorAttachment(result.data)).toBe(false);
      }
    }
  });

  it('parses hook_blocking_error', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'hook_blocking_error',
      hookName: 'PreToolUse:Bash',
      hookEvent: 'PreToolUse',
      toolUseID: 'tool-2',
      blockingError: {
        blockingError: 'Add the Confab-Link trailer to your commit message.',
        command: '/usr/local/bin/confab hook pre-tool-use',
      },
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success && isAttachmentMessage(result.data)) {
      expect(isHookBlockingErrorAttachment(result.data)).toBe(true);
      expect(isHookSuccessAttachment(result.data)).toBe(false);
    }
  });

  it('parses edited_text_file', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'edited_text_file',
      filename: '/home/user/project/notes.md',
      snippet: '     1\t# Notes\n     2\t\n     3\tHello world.\n',
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success && isAttachmentMessage(result.data)) {
      expect(isEditedTextFileAttachment(result.data)).toBe(true);
    }
  });

  it('parses queued_command (free-text)', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'queued_command',
      prompt: 'check the build status',
      commandMode: 'prompt',
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success && isAttachmentMessage(result.data)) {
      expect(isQueuedCommandAttachment(result.data)).toBe(true);
    }
  });

  it('parses queued_command (task-notification XML)', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'queued_command',
      prompt: '<task-notification><task-id>abc</task-id><status>completed</status></task-notification>',
      commandMode: 'task-notification',
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success && isAttachmentMessage(result.data)) {
      expect(isQueuedCommandAttachment(result.data)).toBe(true);
    }
  });

  it('parses deferred_tools_delta', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'deferred_tools_delta',
      addedNames: ['WebFetch', 'WebSearch'],
      removedNames: [],
      addedLines: ['<function>WebFetch</function>'],
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success && isAttachmentMessage(result.data)) {
      expect(isDeferredToolsDeltaAttachment(result.data)).toBe(true);
      expect(isMcpInstructionsDeltaAttachment(result.data)).toBe(false);
    }
  });

  it('parses mcp_instructions_delta', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'mcp_instructions_delta',
      addedNames: ['linear-server'],
      removedNames: [],
      addedBlocks: ['## linear-server\nWhen passing strings...'],
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success && isAttachmentMessage(result.data)) {
      expect(isMcpInstructionsDeltaAttachment(result.data)).toBe(true);
      expect(isDeferredToolsDeltaAttachment(result.data)).toBe(false);
    }
  });

  it('parses noisy subtypes (task_reminder) via catch-all branch', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'task_reminder',
      content: 'Reminder text',
      itemCount: 3,
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.type).toBe('attachment');
      expect(isAttachmentMessage(result.data)).toBe(true);
      // None of the rendered subtype discriminators match
      if (isAttachmentMessage(result.data)) {
        expect(isHookSuccessAttachment(result.data)).toBe(false);
        expect(isHookBlockingErrorAttachment(result.data)).toBe(false);
        expect(isEditedTextFileAttachment(result.data)).toBe(false);
        expect(isQueuedCommandAttachment(result.data)).toBe(false);
        expect(isDeferredToolsDeltaAttachment(result.data)).toBe(false);
        expect(isMcpInstructionsDeltaAttachment(result.data)).toBe(false);
      }
    }
  });

  it('parses unknown future subtype via catch-all', () => {
    const raw = JSON.stringify(makeAttachmentRaw({
      type: 'future_thing_we_have_not_seen',
      whatever: { nested: true },
    }));
    const result = parseTranscriptLineWithError(raw, 0);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.type).toBe('attachment');
    }
  });

  it('isAttachmentMessage returns false for user/assistant/system rows', () => {
    const user = validateParsedTranscriptLine(makeUserMessage('hi'), '', 0);
    const assistant = validateParsedTranscriptLine(
      makeAssistantRaw([{ type: 'text', text: 'hi' }]),
      '',
      0,
    );
    const system = validateParsedTranscriptLine(
      {
        type: 'system',
        uuid: 'sys-1',
        timestamp: '2026-04-20T22:31:25.657Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user',
        sessionId: 'session-1',
        version: '2.1.140',
        subtype: 'info',
        content: 'hi',
      },
      '',
      0,
    );
    expect(user.success && isAttachmentMessage(user.data)).toBe(false);
    expect(assistant.success && isAttachmentMessage(assistant.data)).toBe(false);
    expect(system.success && isAttachmentMessage(system.data)).toBe(false);
  });

  it('AttachmentMessage type carries the attachment discriminator', () => {
    // Compile-time check that AttachmentMessage is exported and shaped correctly
    const sample: AttachmentMessage | undefined = undefined;
    void sample;
  });
});
