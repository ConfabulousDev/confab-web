// Shared chart utilities for the Agents & Skills bar charts.
// Used by both the session-level and trends-level AgentsAndSkillsCard.

/** Color schemes for agent vs skill bars */
export const AGENT_SKILL_COLORS = {
  agent: {
    success: '#3B82F6', // Blue
    error: '#EF4444', // Red
  },
  skill: {
    success: '#8B5CF6', // Purple
    error: '#EF4444', // Red
  },
} as const;

/** Data shape for a single bar in the agent/skill chart */
export interface ChartDataItem {
  name: string;
  success: number;
  errors: number;
  total: number;
  type: 'agent' | 'skill';
}

const TRUNCATE_PREFIX = 6;
const TRUNCATE_SUFFIX = 6;
const TRUNCATE_MAX = TRUNCATE_PREFIX + TRUNCATE_SUFFIX + 3; // "prefix...suffix"

/** Truncate long agent/skill names for Y-axis labels (e.g. "execut...ctron") */
export function truncateName(name: string): string {
  if (name.length <= TRUNCATE_MAX) return name;
  return `${name.slice(0, TRUNCATE_PREFIX)}...${name.slice(-TRUNCATE_SUFFIX)}`;
}
