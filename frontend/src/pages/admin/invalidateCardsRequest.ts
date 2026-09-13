import { PROVIDER_VALUES } from '@/utils/providers';
import type { InvalidateCardsRequest } from '@/schemas/api';

// nbrd: the non-card invalidation target served by GET /admin/cards/types
// (analytics.SessionTitleInvalidationTarget).
export const SESSION_TITLE_TARGET = 'session_title';

// toIsoUtc converts a `datetime-local` input value (e.g. "2026-04-20T12:34")
// to an ISO-8601 timestamp with explicit UTC timezone. The input is treated as
// wall-clock UTC per the "UTC" label next to the field.
function toIsoUtc(datetimeLocal: string): string {
  if (!datetimeLocal) return '';
  const withSeconds = datetimeLocal.length === 16 ? `${datetimeLocal}:00` : datetimeLocal;
  return `${withSeconds}Z`;
}

interface InvalidateCardsFormState {
  startDate: string;
  endDate: string;
  selectedCards: Set<string>;
  selectedProviders: Set<string>;
  reason: string;
  confirmInput: string;
  dryRun: boolean;
}

// buildInvalidateCardsRequest maps the form state to the POST body. `providers`
// is sent only when at least one is selected (none = all providers), and the
// kyrr `confirm` echo only on execute (the server re-counts and rejects on
// mismatch; it is irrelevant on dry-run).
export function buildInvalidateCardsRequest(form: InvalidateCardsFormState): InvalidateCardsRequest {
  const providers = PROVIDER_VALUES.filter((p) => form.selectedProviders.has(p));
  return {
    start_date: toIsoUtc(form.startDate),
    end_date: form.endDate ? toIsoUtc(form.endDate) : undefined,
    card_types: Array.from(form.selectedCards),
    ...(providers.length > 0 ? { providers } : {}),
    reason: form.reason.trim(),
    dry_run: form.dryRun,
    ...(form.dryRun ? {} : { confirm: form.confirmInput.trim() }),
  };
}
