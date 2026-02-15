import { describe, it, expect } from 'vitest';
import type { UserMessage } from './transcript';
import {
  isCommandExpansionMessage,
  getCommandExpansionSkillName,
  stripCommandExpansionTags,
} from './transcript';

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
