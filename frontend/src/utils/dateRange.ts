import { formatLocalDate } from './formatting';

/**
 * A date range with start/end dates and a human-readable label.
 * Used by filter components across Trends and Organization pages.
 */
export interface DateRange {
  startDate: string; // YYYY-MM-DD
  endDate: string;   // YYYY-MM-DD
  label: string;
}

/** Returns a "Last 7 Days" date range ending today. */
export function getDefaultDateRange(): DateRange {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const last7Days = new Date(today);
  last7Days.setDate(last7Days.getDate() - 6);
  return {
    startDate: formatLocalDate(last7Days),
    endDate: formatLocalDate(today),
    label: 'Last 7 Days',
  };
}

/** Infer a human-readable label for a date range, falling back to "start - end". */
export function getDateRangeLabel(startDate: string, endDate: string): string {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayStr = formatLocalDate(today);

  const daysDiff = Math.round(
    (new Date(endDate).getTime() - new Date(startDate).getTime()) / (1000 * 60 * 60 * 24)
  );

  if (endDate === todayStr) {
    if (daysDiff === 6) return 'Last 7 Days';
    if (daysDiff === 29) return 'Last 30 Days';
    if (daysDiff === 89) return 'Last 90 Days';
  }

  return `${startDate} - ${endDate}`;
}

const DATE_REGEX = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Parse start/end date params from a URLSearchParams, returning a DateRange
 * if both are present and valid YYYY-MM-DD strings.
 */
export function parseDateRangeFromURL(searchParams: URLSearchParams): DateRange | null {
  const start = searchParams.get('start');
  const end = searchParams.get('end');

  if (!start || !end) return null;
  if (!DATE_REGEX.test(start) || !DATE_REGEX.test(end)) return null;

  return {
    startDate: start,
    endDate: end,
    label: getDateRangeLabel(start, end),
  };
}
