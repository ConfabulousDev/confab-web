/**
 * Generic sorting utility for arrays
 */

export type SortDirection = 'asc' | 'desc';

export interface SortConfig<T, K extends keyof T> {
  data: T[];
  sortBy: K;
  direction: SortDirection;
  filter?: (item: T) => boolean;
}

/**
 * Sort an array of objects by a specific key
 * Supports strings, numbers, and dates
 */
export function sortData<T, K extends keyof T>({
  data,
  sortBy,
  direction,
  filter,
}: SortConfig<T, K>): T[] {
  // Apply filter if provided
  const filtered = filter ? data.filter(filter) : data;

  // Sort the array
  const sorted = [...filtered].sort((a, b) => {
    const aVal = a[sortBy];
    const bVal = b[sortBy];

    // Handle null/undefined
    if (aVal == null && bVal == null) return 0;
    if (aVal == null) return 1;
    if (bVal == null) return -1;

    // Handle dates (string timestamps)
    if (typeof aVal === 'string' && typeof bVal === 'string') {
      // Check if it looks like a date
      const aDate = new Date(aVal);
      const bDate = new Date(bVal);

      if (!isNaN(aDate.getTime()) && !isNaN(bDate.getTime())) {
        const diff = aDate.getTime() - bDate.getTime();
        return direction === 'asc' ? diff : -diff;
      }
    }

    // Handle numbers and strings
    if (aVal < bVal) return direction === 'asc' ? -1 : 1;
    if (aVal > bVal) return direction === 'asc' ? 1 : -1;
    return 0;
  });

  return sorted;
}
