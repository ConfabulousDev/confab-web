// AI Provider filter constants.
//
// The set of session providers is a closed enum on the backend
// (validation.ProviderClaudeCode / ProviderCodex). The Provider filter
// dropdown shows ALL options regardless of whether the current user has data
// of each type, so the list is hardcoded rather than data-driven.

export const PROVIDER_VALUES = ['claude-code', 'codex'] as const;

const PROVIDER_LABELS: Record<string, string> = {
  'claude-code': 'Claude Code',
  codex: 'Codex',
};

// Display label for any provider string (including unknown future values).
// Unknown values pass through as-is so a backend-first provider rollout still
// renders a readable label.
export function providerLabel(value: string): string {
  return PROVIDER_LABELS[value] ?? value;
}
