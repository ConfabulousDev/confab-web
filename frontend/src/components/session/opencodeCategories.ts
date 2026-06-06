export type OpenCodeCategory = 'user' | 'assistant' | 'tool';

export type OpenCodeRenderItem =
  | { kind: 'user'; id: string; text: string; timeCreated: number }
  | { kind: 'assistant'; id: string; text: string; reasoning?: string; timeCreated: number }
  | { kind: 'tool'; id: string; toolName: string; status: string; timeCreated: number };

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
