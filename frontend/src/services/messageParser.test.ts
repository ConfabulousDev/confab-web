import { describe, it, expect } from 'vitest';
import {
  parseMessage,
  extractTextContent,
  getRoleLabel,
} from './messageParser';
import type { UserMessage, AssistantMessage, PRLinkMessage, ContentBlock } from '@/types';

describe('messageParser', () => {
  describe('parseMessage', () => {
    it('should parse user message with string content', () => {
      const message: UserMessage = {
        type: 'user',
        uuid: 'test-uuid',
        timestamp: '2024-01-15T10:30:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-1',
        version: '2.0.42',
        message: {
          role: 'user',
          content: 'Hello world',
        },
      };

      const result = parseMessage(message);

      expect(result.role).toBe('user');
      expect(result.timestamp).toBe('2024-01-15T10:30:00Z');
      expect(result.content).toHaveLength(1);
      expect(result.content[0]).toEqual({ type: 'text', text: 'Hello world' });
      expect(result.isToolResult).toBe(false);
    });

    it('should parse assistant message', () => {
      const message: AssistantMessage = {
        type: 'assistant',
        uuid: 'test-uuid',
        timestamp: '2024-01-15T10:30:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-1',
        version: '2.0.42',
        requestId: 'req-1',
        message: {
          model: 'claude-sonnet-4-5-20250929',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello!' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: {
            input_tokens: 10,
            output_tokens: 20,
          },
        },
      };

      const result = parseMessage(message);

      expect(result.role).toBe('assistant');
      expect(result.messageModel).toBe('claude-sonnet-4-5-20250929');
      expect(result.hasThinkingContent).toBe(false);
      expect(result.hasToolUse).toBe(false);
    });

    it('should parse pr-link message as system with markdown link', () => {
      const message: PRLinkMessage = {
        type: 'pr-link',
        prNumber: 22,
        prRepository: 'ConfabulousDev/confab-web',
        prUrl: 'https://github.com/ConfabulousDev/confab-web/pull/22',
        sessionId: 'session-1',
        timestamp: '2024-01-15T10:30:00Z',
      };

      const result = parseMessage(message);

      expect(result.role).toBe('system');
      expect(result.timestamp).toBe('2024-01-15T10:30:00Z');
      expect(result.content).toHaveLength(1);
      expect(result.content[0]).toEqual({
        type: 'text',
        text: '🔗 PR #22 — [ConfabulousDev/confab-web](https://github.com/ConfabulousDev/confab-web/pull/22)',
      });
    });
  });

  describe('extractTextContent', () => {
    it('should extract text from multiple blocks', () => {
      const content: ContentBlock[] = [
        { type: 'text', text: 'First line' },
        { type: 'text', text: 'Second line' },
      ];

      const result = extractTextContent(content);
      expect(result).toBe('First line\n\nSecond line');
    });

    it('should handle thinking blocks', () => {
      const content: ContentBlock[] = [
        { type: 'thinking', thinking: 'Analyzing...', signature: 'sig-1' },
      ];

      const result = extractTextContent(content);
      expect(result).toContain('[Thinking]');
      expect(result).toContain('Analyzing...');
    });
  });

  describe('getRoleLabel', () => {
    it('should return capitalized role', () => {
      expect(getRoleLabel('user', false)).toBe('User');
      expect(getRoleLabel('assistant', false)).toBe('Assistant');
    });

    it('should return "Tool Result" for user tool results', () => {
      expect(getRoleLabel('user', true)).toBe('Tool Result');
    });

    it('should return "Unknown" for unknown role', () => {
      expect(getRoleLabel('unknown', false)).toBe('Unknown');
    });
  });

  describe('parseMessage with unknown message type', () => {
    it('should return role "unknown" for unrecognized message types', () => {
      // Simulate an unknown message type that passed Zod validation via catch-all
      const unknownMessage: import('@/types').UnknownMessage = {
        type: 'agent-handoff',
        fromAgent: 'agent-1',
        toAgent: 'agent-2',
        timestamp: '2025-01-01T00:00:00Z',
      };

      const result = parseMessage(unknownMessage);
      expect(result.role).toBe('unknown');
      expect(result.content[0]).toEqual({
        type: 'text',
        text: 'Unknown message type: agent-handoff',
      });
    });

    it('should extract timestamp from unknown message if available', () => {
      const unknownMessage: import('@/types').UnknownMessage = {
        type: 'future-type',
        timestamp: '2025-06-15T12:00:00Z',
      };

      const result = parseMessage(unknownMessage);
      expect(result.role).toBe('unknown');
      expect(result.timestamp).toBe('2025-06-15T12:00:00Z');
    });
  });
});
