import { useMemo } from 'react';
import { useDropdown } from '@/hooks';
import { formatLocalDate } from '@/utils';
import type { DateRange } from '@/utils/dateRange';
import { CalendarIcon, CheckIcon } from '@/components/icons';
import styles from './OrgFilters.module.css';

export interface OrgFiltersValue {
  dateRange: DateRange;
}

interface OrgFiltersProps {
  value: OrgFiltersValue;
  onChange: (value: OrgFiltersValue) => void;
}

function getStartOfWeek(date: Date): Date {
  const d = new Date(date);
  const day = d.getDay();
  const diff = d.getDate() - day + (day === 0 ? -6 : 1);
  return new Date(d.setDate(diff));
}

function getStartOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function getDatePresets(): DateRange[] {
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

function OrgFilters({ value, onChange }: OrgFiltersProps) {
  const {
    isOpen,
    setIsOpen,
    toggle,
    containerRef,
  } = useDropdown<HTMLDivElement>();

  const datePresets = useMemo(() => getDatePresets(), []);

  const handleDateRangeChange = (preset: DateRange) => {
    onChange({ ...value, dateRange: preset });
    setIsOpen(false);
  };

  return (
    <div className={styles.container}>
      <div className={styles.filterWrapper} ref={containerRef}>
        <button
          className={styles.filterBtn}
          onClick={toggle}
          title="Date Range"
          aria-label="Date Range"
          aria-expanded={isOpen}
        >
          {CalendarIcon}
          <span className={styles.filterLabel}>{value.dateRange.label}</span>
        </button>

        {isOpen && (
          <div className={styles.dropdown}>
            <div className={styles.dropdownContent}>
              <div className={styles.section}>
                {datePresets.map((preset) => (
                  <button
                    key={preset.label}
                    className={`${styles.filterItem} ${value.dateRange.label === preset.label ? styles.selected : ''}`}
                    onClick={() => handleDateRangeChange(preset)}
                  >
                    <span className={styles.itemLabel}>{preset.label}</span>
                    {value.dateRange.label === preset.label && (
                      <span className={styles.checkIcon}>{CheckIcon}</span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default OrgFilters;
