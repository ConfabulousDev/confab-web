import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo, useRef } from 'react';
import type { DateRange } from '@/utils/dateRange';
import { getDateRangeLabel } from '@/utils/dateRange';

// --- Field configuration types ---

interface StringFieldConfig {
  type: 'string';
  default: string;
  paramName: string;
}

interface StringArrayFieldConfig {
  type: 'string[]';
  default: string[];
  paramName: string;
}

interface BooleanFieldConfig {
  type: 'boolean';
  default: boolean;
  paramName: string;
}

interface DateRangeFieldConfig {
  type: 'dateRange';
  default: DateRange;
  paramName: { start: string; end: string };
}

export type URLFilterFieldConfig =
  | StringFieldConfig
  | StringArrayFieldConfig
  | BooleanFieldConfig
  | DateRangeFieldConfig;

export type URLFiltersConfig = Record<string, URLFilterFieldConfig>;

interface SetFilterOptions {
  replace?: boolean;
}

function shouldReplace(opts?: SetFilterOptions): boolean {
  return opts?.replace ?? false;
}

export interface URLFiltersResult<T> {
  filters: T;
  setFilter: <K extends keyof T & string>(key: K, value: T[K], opts?: SetFilterOptions) => void;
  setAll: (updates: Partial<T>, opts?: SetFilterOptions) => void;
  toggleArrayValue: (key: string, value: string, opts?: SetFilterOptions) => void;
  clearAll: () => void;
  commitHistory: () => void;
}

// --- Helpers ---

function parseCommaSeparated(value: string | null): string[] {
  if (!value) return [];
  return value.split(',').filter(Boolean);
}

const DATE_REGEX = /^\d{4}-\d{2}-\d{2}$/;

function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((v, i) => v === sortedB[i]);
}

function readFieldFromURL(
  searchParams: URLSearchParams,
  config: URLFilterFieldConfig,
): unknown {
  switch (config.type) {
    case 'string':
      return searchParams.get(config.paramName) ?? config.default;
    case 'string[]': {
      const value = searchParams.get(config.paramName);
      return value !== null ? parseCommaSeparated(value) : [...config.default];
    }
    case 'boolean': {
      const value = searchParams.get(config.paramName);
      return value !== null ? value === 'true' : config.default;
    }
    case 'dateRange': {
      const start = searchParams.get(config.paramName.start);
      const end = searchParams.get(config.paramName.end);
      if (!start || !end || !DATE_REGEX.test(start) || !DATE_REGEX.test(end)) {
        return config.default;
      }
      return {
        startDate: start,
        endDate: end,
        label: getDateRangeLabel(start, end),
      } satisfies DateRange;
    }
  }
}

// Type assertions below are safe: config.type discriminates the value type
/* eslint-disable @typescript-eslint/consistent-type-assertions */
function isDefaultValue(value: unknown, config: URLFilterFieldConfig): boolean {
  switch (config.type) {
    case 'string':
    case 'boolean':
      return value === config.default;
    case 'string[]':
      return arraysEqual(value as string[], config.default);
    case 'dateRange': {
      const dr = value as DateRange;
      return dr.startDate === config.default.startDate && dr.endDate === config.default.endDate;
    }
  }
}

function writeFieldToURL(
  params: URLSearchParams,
  config: URLFilterFieldConfig,
  value: unknown,
): void {
  if (isDefaultValue(value, config)) {
    if (config.type === 'dateRange') {
      params.delete(config.paramName.start);
      params.delete(config.paramName.end);
    } else {
      params.delete(config.paramName);
    }
    return;
  }

  switch (config.type) {
    case 'string': {
      const str = value as string;
      if (str) {
        params.set(config.paramName, str);
      } else {
        params.delete(config.paramName);
      }
      break;
    }
    case 'string[]': {
      const arr = value as string[];
      if (arr.length > 0) {
        params.set(config.paramName, arr.join(','));
      } else {
        params.delete(config.paramName);
      }
      break;
    }
    case 'boolean':
      params.set(config.paramName, String(value));
      break;
    case 'dateRange': {
      const dr = value as DateRange;
      params.set(config.paramName.start, dr.startDate);
      params.set(config.paramName.end, dr.endDate);
      break;
    }
  }
}
/* eslint-enable @typescript-eslint/consistent-type-assertions */

// --- Hook ---

export function useURLFilters<T extends object>(
  config: URLFiltersConfig,
): URLFiltersResult<T> {
  const [searchParams, setSearchParams] = useSearchParams();

  // Store config in a ref so callbacks remain stable
  const configRef = useRef(config);
  configRef.current = config;

  const filters = useMemo(() => {
    const result: Record<string, unknown> = {};
    for (const [key, fieldConfig] of Object.entries(config)) {
      result[key] = readFieldFromURL(searchParams, fieldConfig);
    }
    // Generic hook must assert — config structure guarantees T shape
    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
    return result as T;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  const setFilter = useCallback(
    <K extends keyof T & string>(key: K, value: T[K], opts?: SetFilterOptions) => {
      const fieldConfig = configRef.current[key];
      if (!fieldConfig) return;

      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          writeFieldToURL(next, fieldConfig, value);
          return next;
        },
        { replace: shouldReplace(opts) },
      );
    },
    [setSearchParams],
  );

  const setAll = useCallback(
    (updates: Partial<T>, opts?: SetFilterOptions) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [key, value] of Object.entries(updates)) {
            const fieldConfig = configRef.current[key];
            if (fieldConfig) {
              writeFieldToURL(next, fieldConfig, value);
            }
          }
          return next;
        },
        { replace: shouldReplace(opts) },
      );
    },
    [setSearchParams],
  );

  const toggleArrayValue = useCallback(
    (key: string, value: string, opts?: SetFilterOptions) => {
      const fieldConfig = configRef.current[key];
      if (!fieldConfig || fieldConfig.type !== 'string[]') return;

      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          const current = parseCommaSeparated(prev.get(fieldConfig.paramName));
          const updated = current.includes(value)
            ? current.filter((v) => v !== value)
            : [...current, value];
          writeFieldToURL(next, fieldConfig, updated);
          return next;
        },
        { replace: shouldReplace(opts) },
      );
    },
    [setSearchParams],
  );

  const clearAll = useCallback(() => {
    setSearchParams({}, { replace: true });
  }, [setSearchParams]);

  const commitHistory = useCallback(() => {
    setSearchParams(
      (prev) => new URLSearchParams(prev),
      { replace: false },
    );
  }, [setSearchParams]);

  return { filters, setFilter, setAll, toggleArrayValue, clearAll, commitHistory };
}
