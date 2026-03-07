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
function getDateRangeLabel(startDate: string, endDate: string): string {
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

/** Get the Monday of the week containing the given date. */
function getStartOfWeek(date: Date): Date {
  const d = new Date(date);
  const day = d.getDay();
  const diff = d.getDate() - day + (day === 0 ? -6 : 1);
  return new Date(d.setDate(diff));
}

/** Get the first day of the month containing the given date. */
function getStartOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

/** Standard date range presets for filter dropdowns. */
export function getDatePresets(): DateRange[] {
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  const startOfThisWeek = getStartOfWeek(today);
  const startOfLastWeek = new Date(startOfThisWeek);
  startOfLastWeek.setDate(startOfLastWeek.getDate() - 7);
  const endOfLastWeek = new Date(startOfThisWeek);
  endOfLastWeek.setDate(endOfLastWeek.getDate() - 1);

  const startOfThisMonth = getStartOfMonth(today);
  const startOfLastMonth = new Date(today.getFullYear(), today.getMonth() - 1, 1);
  const endOfLastMonth = new Date(today.getFullYear(), today.getMonth(), 0);

  const last30Days = new Date(today);
  last30Days.setDate(last30Days.getDate() - 29);

  const last90Days = new Date(today);
  last90Days.setDate(last90Days.getDate() - 89);

  return [
    { startDate: formatLocalDate(startOfThisWeek), endDate: formatLocalDate(today), label: 'This Week' },
    { startDate: formatLocalDate(startOfLastWeek), endDate: formatLocalDate(endOfLastWeek), label: 'Last Week' },
    { startDate: formatLocalDate(startOfThisMonth), endDate: formatLocalDate(today), label: 'This Month' },
    { startDate: formatLocalDate(startOfLastMonth), endDate: formatLocalDate(endOfLastMonth), label: 'Last Month' },
    { startDate: formatLocalDate(last30Days), endDate: formatLocalDate(today), label: 'Last 30 Days' },
    { startDate: formatLocalDate(last90Days), endDate: formatLocalDate(today), label: 'Last 90 Days' },
  ];
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
