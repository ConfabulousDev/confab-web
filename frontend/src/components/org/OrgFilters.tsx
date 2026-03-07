import { useMemo } from 'react';
import { useDropdown } from '@/hooks';
import type { DateRange } from '@/utils/dateRange';
import { getDatePresets } from '@/utils/dateRange';
import { CalendarIcon, CheckIcon } from '@/components/icons';
import styles from './OrgFilters.module.css';

export interface OrgFiltersValue {
  dateRange: DateRange;
}

interface OrgFiltersProps {
  value: OrgFiltersValue;
  onChange: (value: OrgFiltersValue) => void;
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
