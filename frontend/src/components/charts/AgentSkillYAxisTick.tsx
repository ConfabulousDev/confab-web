import { truncateName } from '@/utils/agentSkillChart';

/** Recharts custom tick for the Y-axis that truncates long names with a hover tooltip */
interface YAxisTickProps {
  x?: number;
  y?: number;
  payload?: { value: string };
}

export function AgentSkillYAxisTick({ x, y, payload }: YAxisTickProps): React.ReactElement | null {
  if (!payload) return null;
  const fullName = payload.value;
  const display = truncateName(fullName);
  return (
    <g transform={`translate(${x},${y})`}>
      <title>{fullName}</title>
      <text
        x={0}
        y={0}
        dy={4}
        textAnchor="end"
        fill="var(--color-text-secondary)"
        fontSize={11}
      >
        {display}
      </text>
    </g>
  );
}
