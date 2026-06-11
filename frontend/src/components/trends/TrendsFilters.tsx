import { useMemo } from 'react';
import { useDropdown } from '@/hooks';
import type { DateRange } from '@/utils/dateRange';
import { getDatePresets } from '@/utils/dateRange';
import { CalendarIcon, CheckIcon, UserIcon, TokenIcon } from '@/components/icons';
import { PROVIDER_VALUES } from '@/utils/providers';
import { formatModelKey } from '@/utils/formatting';
import ProviderFilter from '@/components/filters/ProviderFilter';
import RepoFilter from '@/components/filters/RepoFilter';
import styles from '@/styles/filterDropdown.module.css';

export interface TrendsFiltersValue {
  dateRange: DateRange;
  repos: string[];
  includeNoRepo: boolean;
  // CF-424: canonical providers (`claude-code`, `codex`). Empty array =
  // aggregate across all providers (distinct from selecting every provider).
  providers: string[];
  // CF-495: owner emails (lowercased). Empty array = aggregate across all
  // visible owners (distinct from selecting every owner).
  owners: string[];
  // 2hh1: model-family keys (e.g. "opus-4-5", "opus-4-5 · fast"). Empty array =
  // all models. Session-level filter (AND-combined with providers): narrows the
  // whole view to sessions that used a selected model.
  models: string[];
}

interface TrendsFiltersProps {
  repos: string[];
  // CF-495: owner dropdown source. Frontend pins viewer's own email to the
  // top in the rendered dropdown.
  owners: string[];
  // CF-495: viewer's own email (used for self-first ordering in the owner
  // dropdown). Optional — when omitted, owners render in source order.
  selfEmail?: string;
  // 2hh1: model dropdown source (normalized family keys from filter_options).
  models: string[];
  value: TrendsFiltersValue;
  onChange: (value: TrendsFiltersValue) => void;
}

function TrendsFilters({ repos, owners, selfEmail, models, value, onChange }: TrendsFiltersProps) {
  const {
    isOpen: dateIsOpen,
    setIsOpen: setDateIsOpen,
    toggle: toggleDate,
    containerRef: dateContainerRef,
  } = useDropdown<HTMLDivElement>();
  const {
    isOpen: ownerIsOpen,
    toggle: toggleOwner,
    containerRef: ownerContainerRef,
  } = useDropdown<HTMLDivElement>();
  const {
    isOpen: modelIsOpen,
    toggle: toggleModel,
    containerRef: modelContainerRef,
  } = useDropdown<HTMLDivElement>();

  // CF-495: owner dropdown source with viewer's own email pinned to the
  // top. Memoized to keep stable identity for the keyed list below.
  const orderedOwners = useMemo(() => {
    if (!selfEmail) return owners;
    const self = selfEmail.toLowerCase();
    if (!owners.some((o) => o.toLowerCase() === self)) return owners;
    return [self, ...owners.filter((o) => o.toLowerCase() !== self)];
  }, [owners, selfEmail]);

  const datePresets = useMemo(() => getDatePresets(), []);

  const handleDateRangeChange = (preset: DateRange) => {
    onChange({ ...value, dateRange: preset });
    setDateIsOpen(false);
  };

  const handleOwnerToggle = (owner: string) => {
    const next = value.owners.includes(owner)
      ? value.owners.filter((o) => o !== owner)
      : [...value.owners, owner];
    onChange({ ...value, owners: next });
  };

  function getOwnerButtonLabel(): string {
    if (value.owners.length === 0) return 'All Owners';
    if (value.owners.length === 1) return value.owners[0] ?? '';
    return `${value.owners.length} owners`;
  }

  const handleModelToggle = (model: string) => {
    const next = value.models.includes(model)
      ? value.models.filter((m) => m !== model)
      : [...value.models, model];
    onChange({ ...value, models: next });
  };

  function getModelButtonLabel(): string {
    if (value.models.length === 0) return 'All Models';
    if (value.models.length === 1) return formatModelKey(value.models[0] ?? '');
    return `${value.models.length} models`;
  }

  return (
    <div className={styles.container}>
      {/* Provider Filter (CF-424) — leftmost, mirroring FilterChipsBar's coarsest-cut ordering */}
      <ProviderFilter
        availableProviders={[...PROVIDER_VALUES]}
        selectedProviders={value.providers}
        onChange={(providers) => onChange({ ...value, providers })}
      />

      {/* Date Range Filter */}
      <div className={styles.filterWrapper} ref={dateContainerRef}>
        <button
          className={styles.filterBtn}
          onClick={toggleDate}
          title="Date Range"
          aria-label="Date Range"
          aria-expanded={dateIsOpen}
        >
          {CalendarIcon}
          <span className={styles.filterLabel}>{value.dateRange.label}</span>
        </button>

        {dateIsOpen && (
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

      {/* Repo Filter */}
      <RepoFilter
        availableRepos={repos}
        selectedRepos={value.repos}
        includeNoRepo={value.includeNoRepo}
        onChange={(next) => onChange({ ...value, ...next })}
      />

      {/* Owner Filter (CF-495) — hidden when no owners are visible to the
          caller (single-user self-hosted with no shares). Self is pinned to
          the top of the dropdown so the dominant case is one click. */}
      {orderedOwners.length > 0 && (
        <div className={styles.filterWrapper} ref={ownerContainerRef}>
          <button
            className={`${styles.filterBtn} ${value.owners.length > 0 ? styles.active : ''}`}
            onClick={toggleOwner}
            title="Owner Filter"
            aria-label="Owner Filter"
            aria-expanded={ownerIsOpen}
          >
            {UserIcon}
            <span className={styles.filterLabel}>{getOwnerButtonLabel()}</span>
          </button>

          {ownerIsOpen && (
            <div className={styles.dropdown}>
              <div className={styles.dropdownContent}>
                <div className={styles.section}>
                  {orderedOwners.map((owner) => (
                    <label key={owner} className={styles.checkboxItem}>
                      <input
                        type="checkbox"
                        checked={value.owners.includes(owner)}
                        onChange={() => handleOwnerToggle(owner)}
                      />
                      <span className={styles.repoName}>{owner}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Model Filter (2hh1) — family-grain; AND-combined with the provider
          filter. Hidden until the visible session set has any per-model data. */}
      {models.length > 0 && (
        <div className={styles.filterWrapper} ref={modelContainerRef}>
          <button
            className={`${styles.filterBtn} ${value.models.length > 0 ? styles.active : ''}`}
            onClick={toggleModel}
            title="Model Filter"
            aria-label="Model Filter"
            aria-expanded={modelIsOpen}
          >
            {TokenIcon}
            <span className={styles.filterLabel}>{getModelButtonLabel()}</span>
          </button>

          {modelIsOpen && (
            <div className={styles.dropdown}>
              <div className={styles.dropdownContent}>
                <div className={styles.section}>
                  {models.map((model) => (
                    <label key={model} className={styles.checkboxItem}>
                      <input
                        type="checkbox"
                        checked={value.models.includes(model)}
                        onChange={() => handleModelToggle(model)}
                      />
                      <span className={styles.repoName}>{formatModelKey(model)}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default TrendsFilters;
