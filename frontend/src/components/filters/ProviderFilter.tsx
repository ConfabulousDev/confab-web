import { useDropdown } from '@/hooks';
import { RobotIcon } from '@/components/icons';
import { getProviderIcon } from '@/components/providerIcon';
import { providerLabel } from '@/utils/providers';
import styles from '@/styles/filterDropdown.module.css';

export interface ProviderFilterProps {
  /** Canonical providers to offer (e.g. PROVIDER_VALUES, or a narrowed list). */
  availableProviders: string[];
  /** Currently selected providers. Empty = all providers. */
  selectedProviders: string[];
  /** Emits the next provider selection. */
  onChange: (providers: string[]) => void;
}

/**
 * Shared Provider dropdown for OrgFilters and TrendsFilters (CF-508). The
 * option source is supplied via `availableProviders` (Org narrows to
 * `providers_present`; Trends passes the full `PROVIDER_VALUES`). Preserves
 * the CF-424 contract: empty `providers[]` = "All Providers" and there is no
 * Select-all affordance.
 */
function ProviderFilter({ availableProviders, selectedProviders, onChange }: ProviderFilterProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  function getLabel(): string {
    if (selectedProviders.length === 0) return 'All Providers';
    if (selectedProviders.length === 1) return providerLabel(selectedProviders[0] ?? '');
    return `${selectedProviders.length} providers`;
  }

  const handleToggle = (provider: string) => {
    const next = selectedProviders.includes(provider)
      ? selectedProviders.filter((p) => p !== provider)
      : [...selectedProviders, provider];
    onChange(next);
  };

  return (
    <div className={styles.filterWrapper} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${selectedProviders.length > 0 ? styles.active : ''}`}
        onClick={toggle}
        title="Provider Filter"
        aria-label="Provider Filter"
        aria-expanded={isOpen}
      >
        {RobotIcon}
        <span className={styles.filterLabel}>{getLabel()}</span>
      </button>

      {isOpen && (
        <div className={styles.dropdown}>
          <div className={styles.dropdownContent}>
            <div className={styles.section}>
              {availableProviders.map((p) => (
                <label key={p} className={styles.checkboxItem}>
                  <input
                    type="checkbox"
                    checked={selectedProviders.includes(p)}
                    onChange={() => handleToggle(p)}
                  />
                  <span className={styles.providerIcon}>{getProviderIcon(p)}</span>
                  <span>{providerLabel(p)}</span>
                </label>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default ProviderFilter;
