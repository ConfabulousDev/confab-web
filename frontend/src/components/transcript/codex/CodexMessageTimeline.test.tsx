// Spec tests for CodexMessageTimeline.
//
// The virtualizer doesn't render rows in jsdom (no layout), so the virtual-
// items contract is tested through the exported `buildVirtualItems` pure
// function. The "renders the empty state" path is still exercised at the
// component level since it short-circuits before the virtualizer.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import CodexMessageTimeline from './CodexMessageTimeline';
import { buildVirtualItems } from './codexVirtualItems';
import type { CodexRenderItem } from '@/types/codexRenderItem';

function user(timestamp: string, text = 'hi'): CodexRenderItem {
  return { kind: 'user', lineId: '0', timestamp, text };
}

function assistant(timestamp: string, text = 'hello'): CodexRenderItem {
  return { kind: 'assistant', lineId: '0', timestamp, text, phase: 'final', model: 'gpt-5' };
}

function toolCall(timestamp: string, callId = 'c1'): CodexRenderItem {
  return {
    kind: 'tool_call',
    lineId: '0',
    timestamp,
    toolName: 'exec_command',
    callId,
    rawInput: { cmd: 'pwd' },
    rawOutput: '/tmp',
    status: 'completed',
    execMetadata: { exitCode: 0, wallTimeMs: 100 },
  };
}

describe('CodexMessageTimeline', () => {
  it('renders the empty-state when items is empty', () => {
    render(<CodexMessageTimeline items={[]} filteredItems={[]} sessionId="test-session" />);
    expect(screen.getByText(/no conversation content/i)).toBeInTheDocument();
  });

  // CF-360 deep-link target highlight is tested at the renderer level
  // (CodexUserMessage.test.tsx etc.) because the @tanstack virtualizer
  // doesn't lay out rows under jsdom — see top-of-file note. The contract
  // tested there: when `isDeepLinkTarget` is true the row gets the
  // `.deepLinkTarget` class. The timeline's job is only to pass that flag
  // through, which the integration test (Storybook + manual click) covers.

  // ---------------------------------------------------------------------------
  // CF-362 — cost mode wiring.
  //
  // The virtualizer doesn't lay out rows under jsdom, so we can't assert per-
  // row badge text here. What IS visible: the CostBar side-rail rendered
  // outside the virtualized region. The test marks coverage of the cost-mode
  // wiring (toggle reaches the bar, bar renders only when there is data).
  // ---------------------------------------------------------------------------

  describe('cost mode', () => {
    const assistantWithUsage: CodexRenderItem = {
      kind: 'assistant',
      lineId: '1',
      timestamp: '2026-05-13T18:00:01Z',
      text: 'hi',
      phase: 'final',
      model: 'gpt-5',
      usage: { input_tokens: 1_000_000, output_tokens: 100_000 },
    };

    it('renders the CostBar side rail when isCostMode is on and items have usage', () => {
      render(
        <CodexMessageTimeline
          items={[assistantWithUsage]}
          filteredItems={[assistantWithUsage]}
          sessionId="test-session"
          isCostMode
        />,
      );
      // The CostBar root carries a recognisable title attribute.
      const bar = document.querySelector('[title*="Color intensity"]');
      expect(bar).not.toBeNull();
    });

    it('does NOT render the CostBar when isCostMode is off', () => {
      render(
        <CodexMessageTimeline
          items={[assistantWithUsage]}
          filteredItems={[assistantWithUsage]}
          sessionId="test-session"
          isCostMode={false}
        />,
      );
      expect(document.querySelector('[title*="Color intensity"]')).toBeNull();
    });

    it('does NOT render the CostBar when isCostMode is on but no item has usage', () => {
      const itemNoUsage: CodexRenderItem = {
        kind: 'assistant',
        lineId: '1',
        timestamp: '2026-05-13T18:00:01Z',
        text: 'hi',
        phase: 'final',
        model: 'gpt-5',
      };
      render(
        <CodexMessageTimeline
          items={[itemNoUsage]}
          filteredItems={[itemNoUsage]}
          sessionId="test-session"
          isCostMode
        />,
      );
      // CostBar returns null when totalCost === 0.
      expect(document.querySelector('[title*="Color intensity"]')).toBeNull();
    });
  });
});

describe('buildVirtualItems', () => {
  describe('time-gap separator', () => {
    it('injects a separator entry between items >5min apart', () => {
      const items: CodexRenderItem[] = [
        user('2026-05-13T18:00:00Z', 'first'),
        user('2026-05-13T18:06:00Z', 'second'), // 6 minute gap
      ];
      const result = buildVirtualItems(items);
      // Layout: item, separator, item
      expect(result).toHaveLength(3);
      expect(result[0]?.type).toBe('item');
      expect(result[1]?.type).toBe('separator');
      expect(result[2]?.type).toBe('item');
    });

    it('does not inject a separator for items <=5min apart', () => {
      const items: CodexRenderItem[] = [
        user('2026-05-13T18:00:00Z', 'first'),
        user('2026-05-13T18:04:59Z', 'second'),
      ];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(2);
      expect(result.every((v) => v.type === 'item')).toBe(true);
    });

    it('does not inject a separator before the first item', () => {
      const result = buildVirtualItems([user('2026-05-13T18:00:00Z', 'first')]);
      expect(result).toHaveLength(1);
      expect(result[0]?.type).toBe('item');
    });
  });

  describe('isNewSpeaker computation', () => {
    function newSpeakerFlags(items: CodexRenderItem[]): boolean[] {
      return buildVirtualItems(items)
        .filter((v): v is Extract<typeof v, { type: 'item' }> => v.type === 'item')
        .map((v) => v.isNewSpeaker);
    }

    it('first user item is never newSpeaker (no previous speaker)', () => {
      expect(newSpeakerFlags([user('2026-05-13T18:00:00Z')])).toEqual([false]);
    });

    it('user → assistant marks the assistant as newSpeaker', () => {
      const flags = newSpeakerFlags([
        user('2026-05-13T18:00:00Z'),
        assistant('2026-05-13T18:00:01Z'),
      ]);
      expect(flags).toEqual([false, true]);
    });

    it('user → tool_call → user does NOT mark the second user as newSpeaker (tool_call carveout)', () => {
      const flags = newSpeakerFlags([
        user('2026-05-13T18:00:00Z', 'first'),
        toolCall('2026-05-13T18:00:01Z'),
        user('2026-05-13T18:00:02Z', 'second'),
      ]);
      // Flags are [user1, tool_call, user2]
      expect(flags).toEqual([false, false, false]);
    });

    it('user → tool_call → assistant marks the assistant as newSpeaker', () => {
      const flags = newSpeakerFlags([
        user('2026-05-13T18:00:00Z'),
        toolCall('2026-05-13T18:00:01Z'),
        assistant('2026-05-13T18:00:02Z'),
      ]);
      expect(flags).toEqual([false, false, true]);
    });

    it('assistant → assistant (back-to-back) is NOT newSpeaker', () => {
      const flags = newSpeakerFlags([
        assistant('2026-05-13T18:00:00Z', 'a'),
        assistant('2026-05-13T18:00:01Z', 'b'),
      ]);
      expect(flags).toEqual([false, false]);
    });

    it('reasoning_hidden between user and user does not break continuity', () => {
      const items: CodexRenderItem[] = [
        user('2026-05-13T18:00:00Z'),
        { kind: 'reasoning_hidden', lineId: '0', timestamp: '2026-05-13T18:00:01Z' },
        user('2026-05-13T18:00:02Z'),
      ];
      const flags = newSpeakerFlags(items);
      expect(flags).toEqual([false, false, false]);
    });
  });

  it('item indices in VirtualItem.index reference the original items array', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:01Z'),
      user('2026-05-13T18:06:01Z'), // triggers separator before this
    ];
    const result = buildVirtualItems(items);
    const itemEntries = result.filter(
      (v): v is Extract<typeof v, { type: 'item' }> => v.type === 'item',
    );
    expect(itemEntries.map((v) => v.index)).toEqual([0, 1, 2]);
  });
});
