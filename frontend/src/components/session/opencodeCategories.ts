import type { TokenUsage } from '@/utils/tokenStats';

export type OpenCodeCategory = 'user' | 'assistant' | 'tool';

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
    };

export type OpenCodeFilterState = {
  user: boolean;
  assistant: boolean;
  tool: boolean;
};

export type OpenCodeHierarchicalCounts = {
  user: number;
  assistant: number;
  tool: number;
};

export const DEFAULT_OPENCODE_FILTER_STATE: OpenCodeFilterState = {
  user: true,
  assistant: true,
  tool: true,
};

export function countOpenCodeCategories(items: OpenCodeRenderItem[]): OpenCodeHierarchicalCounts {
  const counts: OpenCodeHierarchicalCounts = { user: 0, assistant: 0, tool: 0 };
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
