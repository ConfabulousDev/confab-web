import { truncateName } from '@/utils/chartLabels';

interface TruncatedYAxisTickProps {
  x?: number;
  y?: number;
  payload?: { value: string };
}

/**
 * Recharts custom tick that truncates long Y-axis labels and exposes the
 * full value via an SVG `<title>` for hover. Used by Agents & Skills and
 * Tools charts in both session and trends scopes.
 */
export function TruncatedYAxisTick({ x, y, payload }: TruncatedYAxisTickProps): React.ReactElement | null {
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
