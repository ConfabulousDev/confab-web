import type { TokenUsage } from '@/utils/tokenStats';

export type OpenCodeCategory = 'user' | 'assistant' | 'tool' | 'unknown';

// Render items the OpenCode transcript pane consumes. `id` is the deep-link
// anchor: the message ULID for user/assistant rows (matches the smart-recap
// idMap), the part id for tool rows. `timeCreated` is epoch ms (info.time.created).
export type OpenCodeRenderItem =
  | { kind: 'user'; id: string; text: string; timeCreated: number }
  | {
      kind: 'assistant';
      id: string;
      text: string;
      reasoning?: string;
      model?: string;
      cost?: number;
      usage?: TokenUsage;
      timeCreated: number;
    }
  | {
      kind: 'tool';
      id: string;
      toolName: string;
      status: string;
      input?: string;
      output?: string;
      timeCreated: number;
    }
  // CF-574: forward-compat fallback for genuinely unrecognized shapes — an
  // unknown message role, an unrecognized part type, or a malformed line — so
  // a new OpenCode message type surfaces (and can be reported) instead of being
  // silently dropped. `reason` is a human-readable classification; `rawLine` is
  // the offending object (or raw string) shown behind a click-to-expand.
  | {
      kind: 'unknown';
      id: string;
      reason: string;
      unrecognizedType: string;
      rawLine: unknown;
      timeCreated: number;
    };

export type OpenCodeFilterState = {
  user: boolean;
  assistant: boolean;
  tool: boolean;
  unknown: boolean;
};

export type OpenCodeHierarchicalCounts = {
  user: number;
  assistant: number;
  tool: number;
  unknown: number;
};

export const DEFAULT_OPENCODE_FILTER_STATE: OpenCodeFilterState = {
  user: true,
  assistant: true,
  tool: true,
  unknown: true,
};

export function countOpenCodeCategories(items: OpenCodeRenderItem[]): OpenCodeHierarchicalCounts {
  const counts: OpenCodeHierarchicalCounts = { user: 0, assistant: 0, tool: 0, unknown: 0 };
  for (const item of items) {
    counts[item.kind]++;
  }
  return counts;
}

export function opencodeItemMatchesFilter(
  item: OpenCodeRenderItem,
  state: OpenCodeFilterState,
): boolean {
  return state[item.kind];
}
