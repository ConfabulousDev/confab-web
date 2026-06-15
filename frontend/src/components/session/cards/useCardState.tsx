import type { ReactElement, ReactNode } from 'react';
import { CardError, CardWrapper, CardLoading } from './Card';

/**
 * Collapses the repeated loading/error guard at the top of every session card
 * (x8j0). Returns the element to short-circuit with — `CardError` on error, a
 * wrapped `CardLoading` while loading, both only before data has arrived — or
 * `null` when the card should proceed to render.
 *
 * Callers keep their own trailing `if (!data) return null` (and any
 * domain-specific empty check), since "no data, not loading, no error" renders
 * nothing rather than an element:
 *
 *   const guard = useCardState(data, loading, error, { title: 'Tokens', icon: TokenIcon });
 *   if (guard) return guard;
 *   if (!data) return null;
 *
 * Not a stateful hook (it calls no hooks); the `use` prefix marks it as a
 * card-render helper consumed exactly like the inline guard it replaces.
 */
export function useCardState<T>(
  data: T | null,
  loading: boolean,
  error: string | undefined,
  opts: { title: string; icon?: ReactNode },
): ReactElement | null {
  if (error && !data) {
    return <CardError title={opts.title} error={error} icon={opts.icon} />;
  }
  if (loading && !data) {
    return (
      <CardWrapper title={opts.title} icon={opts.icon}>
        <CardLoading />
      </CardWrapper>
    );
  }
  return null;
}
