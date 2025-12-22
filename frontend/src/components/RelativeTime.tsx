import { useRelativeTime } from '@/hooks/useRelativeTime';

interface RelativeTimeProps {
  date: string;
}

/**
 * Component that displays a relative time string (e.g., "5s ago") that
 * automatically updates as time passes.
 */
export function RelativeTime({ date }: RelativeTimeProps) {
  const formatted = useRelativeTime(date);
  return <>{formatted}</>;
}
