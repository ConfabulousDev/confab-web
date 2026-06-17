import { useCallback, useMemo } from 'react';
import { useAuth, useDocumentTitle, useTrends, useURLFilters } from '@/hooks';
import type { URLFiltersConfig } from '@/hooks';
import type { TrendsParams } from '@/services/api';
import { getDefaultDateRange } from '@/utils';
import { computeTokenSpeed } from '@/utils/tokenStats';
import PageHeader from '@/components/PageHeader';
import TrendsFilters, { type TrendsFiltersValue } from '@/components/trends/TrendsFilters';
import {
  TrendsOverviewCard,
  TrendsTokensCard,
  TrendsActivityCard,
  TrendsToolsCard,
  TrendsUtilizationCard,
  TrendsAgentsAndSkillsCard,
  TrendsTopSessionsCard,
  TrendsCostByModelCard,
  TrendsCostDistributionCard,
} from '@/components/trends/cards';
import Alert from '@/components/Alert';
import CardGrid from '@/components/CardGrid';
import styles from './TrendsPage.module.css';

function TrendsPage() {
  useDocumentTitle('Trends');
  const { user } = useAuth();

  // Config inside component so getDefaultDateRange() is fresh each render
  const config: URLFiltersConfig = {
    dateRange: { type: 'dateRange', default: getDefaultDateRange(), paramName: { start: 'start', end: 'end' } },
    repos: { type: 'string[]', default: [], paramName: 'repo' },
    includeNoRepo: { type: 'boolean', default: true, paramName: 'includeNoRepo' },
    // CF-424: singular `provider` wire key matches the session-list endpoint.
    // Empty default = "all providers"; we deliberately do not auto-select-all
    // like repos so the URL stays clean for the common case.
    providers: { type: 'string[]', default: [], paramName: 'provider' },
    // CF-495: owner narrows within visible set. Empty = "all owners"; same
    // semantics as providers. URL uses singular `owner` key matching Sessions.
    owners: { type: 'string[]', default: [], paramName: 'owner' },
    // 2hh1: model-family filter (?model=). Empty = "all models"; same semantics
    // as providers/owners. Session-level, AND-combined with the provider filter.
    models: { type: 'string[]', default: [], paramName: 'model' },
    // h7xe: Costliest Sessions limit (?topN=). Lives in page state, not the
    // filter-bar value — its control is on the card — but rides the same URL
    // hook so it survives reload/share. Stored as a string; parsed to a number
    // for the API.
    topN: { type: 'string', default: '10', paramName: 'topN' },
  };

  // Page state = filter-bar fields + topN (whose control lives on a card, not
  // the filter bar). TrendsFilters still receives only the TrendsFiltersValue
  // subset, keeping the filter-bar contract clean.
  type TrendsPageState = TrendsFiltersValue & { topN: string };

  const { filters, setAll } = useURLFilters<TrendsPageState>(config);

  const buildParams = useCallback(
    (f: TrendsPageState): TrendsParams => ({
      startDate: f.dateRange.startDate,
      endDate: f.dateRange.endDate,
      repos: f.repos,
      includeNoRepo: f.includeNoRepo,
      providers: f.providers,
      owners: f.owners,
      models: f.models,
      topN: Number(f.topN),
    }),
    [],
  );

  // Fetch trends data. CF-495: filter_options.repos + .owners come from the
  // response itself — no side-call to /api/sessions needed.
  const { data, loading, error, refetch } = useTrends(buildParams(filters));

  const availableRepos = useMemo(() => data?.filter_options.repos ?? [], [data]);
  const availableOwners = useMemo(() => data?.filter_options.owners ?? [], [data]);
  const availableModels = useMemo(() => data?.filter_options.models ?? [], [data]);
  const modelFilterActive = filters.models.length > 0;

  // Commit a new page state: persist to the URL and refetch in one step. Both
  // the filter bar and the card's topN selector route through here.
  const commit = useCallback((next: TrendsPageState) => {
    setAll(next);
    refetch(buildParams(next));
  }, [setAll, refetch, buildParams]);

  const handleFilterChange = useCallback((newFilters: TrendsFiltersValue) => {
    commit({ ...newFilters, topN: filters.topN });
  }, [commit, filters.topN]);

  const handleTopNChange = useCallback((n: number) => {
    commit({ ...filters, topN: String(n) });
  }, [commit, filters]);

  // CF-495: owner-narrowed empty state — when a filter is set but yields
  // zero sessions, hint at the cause and offer a one-click clear.
  const clearOwnerFilter = useCallback(() => {
    handleFilterChange({ ...filters, owners: [] });
  }, [filters, handleFilterChange]);

  const showEmptyState = !loading && data && data.session_count === 0;
  const ownerNarrowedEmpty = showEmptyState && filters.owners.length > 0;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Trends</h1>}
          actions={
            <TrendsFilters
              repos={availableRepos}
              owners={availableOwners}
              selfEmail={user?.email}
              models={availableModels}
              value={filters}
              onChange={handleFilterChange}
            />
          }
        />

        <div className={styles.container}>
          {error && <Alert variant="error">{error.message}</Alert>}

          {loading && !data && (
            <div className={styles.loading}>Loading trends...</div>
          )}

          {showEmptyState && ownerNarrowedEmpty && (
            <div className={styles.emptyState}>
              <div className={styles.emptyStateTitle}>No sessions match the owner filter</div>
              <div className={styles.emptyStateText}>
                Try a different owner, or clear the filter to aggregate across all visible sessions.
              </div>
              <button className={styles.clearFilterBtn} onClick={clearOwnerFilter}>
                Clear owner filter
              </button>
            </div>
          )}

          {showEmptyState && !ownerNarrowedEmpty && (
            <div className={styles.emptyState}>
              <div className={styles.emptyStateTitle}>No sessions found</div>
              <div className={styles.emptyStateText}>
                No sessions match the selected filters. Try adjusting the date range, repo filter, or provider filter.
              </div>
            </div>
          )}

          {data && data.session_count > 0 && (
            <CardGrid>
              <TrendsOverviewCard
                data={data.cards.overview}
                // CF-525: aggregate token speed over the range. Numerator and
                // denominator live in different card slices, so compute it here
                // — the only place holding both — and pass the precomputed value.
                tokenSpeed={
                  data.cards.overview && data.cards.tokens
                    ? computeTokenSpeed(
                        data.cards.tokens.total_output_tokens,
                        data.cards.overview.total_assistant_duration_ms,
                      )
                    : null
                }
              />
              <TrendsTokensCard
                data={data.cards.tokens}
                modelFilterActive={modelFilterActive}
                providersPresent={data.providers_present}
              />
              <TrendsCostByModelCard data={data.cards.cost_by_model} />
              <TrendsCostDistributionCard
                data={data.cards.cost_distribution}
                modelFilterActive={modelFilterActive}
              />
              <TrendsTopSessionsCard
                data={data.cards.top_sessions}
                topN={Number(filters.topN)}
                onTopNChange={handleTopNChange}
                loading={loading}
                modelFilterActive={modelFilterActive}
              />
              <TrendsActivityCard data={data.cards.activity} providersPresent={data.providers_present} />
              <TrendsToolsCard data={data.cards.tools} />
              <TrendsUtilizationCard data={data.cards.utilization} />
              <TrendsAgentsAndSkillsCard data={data.cards.agents_and_skills} />
            </CardGrid>
          )}
        </div>
      </div>
    </div>
  );
}

export default TrendsPage;
