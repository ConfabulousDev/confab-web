import { useCallback, useMemo } from 'react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';

/**
 * Per-provider configuration for {@link useProviderTranscriptFilters}.
 *
 * `pathsFromState`/`stateFromPaths` are the provider's canonical bridge between
 * its typed filter state and the flat `?hide=` dot-path list (a path appears iff
 * its state boolean is `false`). `hierarchicalKeys` maps each hierarchical
 * category to its subcategory keys; flat categories are omitted.
 */
export interface ProviderTranscriptFiltersConfig<TState> {
  defaultState: TState;
  pathsFromState: (state: TState) => string[];
  stateFromPaths: (paths: string[]) => TState;
  hierarchicalKeys: Record<string, readonly string[]>;
}

interface ProviderTranscriptFiltersResult<TState> {
  filterState: TState;
  setFilterState: (state: TState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: string) => void;
  toggleSubcategory: (category: string, subcategory: string) => void;
}

interface HideFilters {
  hide: string[];
}

/**
 * Shared machinery for the per-provider transcript category filters (x5w2).
 *
 * Owns the `?hide=` URL sync (`useURLFilters`), derives the typed `filterState`
 * via the provider's `stateFromPaths`, and provides generic category /
 * subcategory toggles plus tri-state behavior for hierarchical categories.
 *
 * Toggles operate on the canonical hidden-path SET — `pathsFromState ∘
 * stateFromPaths` strips foreign-provider tokens and re-imposes the provider's
 * canonical order — so the generic never has to index the opaque `TState`. This
 * matches the hand-written hooks exactly, including dropping cross-provider
 * tokens on any write.
 */
export function useProviderTranscriptFilters<TState>(
  config: ProviderTranscriptFiltersConfig<TState>,
): ProviderTranscriptFiltersResult<TState> {
  const { defaultState, pathsFromState, stateFromPaths, hierarchicalKeys } = config;

  const urlConfig = useMemo<URLFiltersConfig>(
    () => ({ hide: { type: 'string[]', default: pathsFromState(defaultState), paramName: 'hide' } }),
    [pathsFromState, defaultState],
  );

  const { filters, setFilter } = useURLFilters<HideFilters>(urlConfig);

  const filterState = useMemo(() => stateFromPaths(filters.hide), [stateFromPaths, filters.hide]);

  // Re-canonicalize a path list to this provider's vocabulary + order (foreign
  // tokens dropped), matching the hand-written hooks' `pathsFromState(next)`.
  const canonicalize = useCallback(
    (paths: string[]) => pathsFromState(stateFromPaths(paths)),
    [pathsFromState, stateFromPaths],
  );

  const setFilterState = useCallback(
    (state: TState, opts?: { replace?: boolean }) => setFilter('hide', pathsFromState(state), opts),
    [setFilter, pathsFromState],
  );

  const toggleCategory = useCallback(
    (category: string) => {
      const hidden = new Set(canonicalize(filters.hide));
      const subs = hierarchicalKeys[category];
      if (subs) {
        // Tri-state: if every subcategory is visible, hide them all; else reveal all.
        const subPaths = subs.map((s) => `${category}.${s}`);
        const allVisible = subPaths.every((p) => !hidden.has(p));
        for (const p of subPaths) {
          if (allVisible) hidden.add(p);
          else hidden.delete(p);
        }
      } else if (hidden.has(category)) {
        hidden.delete(category);
      } else {
        hidden.add(category);
      }
      setFilter('hide', canonicalize([...hidden]));
    },
    [canonicalize, filters.hide, hierarchicalKeys, setFilter],
  );

  const toggleSubcategory = useCallback(
    (category: string, subcategory: string) => {
      const hidden = new Set(canonicalize(filters.hide));
      const path = `${category}.${subcategory}`;
      if (hidden.has(path)) hidden.delete(path);
      else hidden.add(path);
      setFilter('hide', canonicalize([...hidden]));
    },
    [canonicalize, filters.hide, setFilter],
  );

  return { filterState, setFilterState, toggleCategory, toggleSubcategory };
}
