import { setPricingTable, type PricingTable } from '@/utils/tokenStats';

interface PricingResponse {
  pricing?: PricingTable;
}

// Fetch the effective model price table from this app's own backend
// (GET /api/v1/pricing) and install it for all cost arithmetic. The backend
// always returns a valid table (its embedded floor, or a freshest remote pull
// from confabulous.dev), so there is no client-side fallback or version logic.
//
// Best-effort and single-shot: on failure the table stays empty and cost bills
// $0 until the next load. Cost UI renders only after auth + session-data load,
// well after this bootstrap fetch resolves, so the empty window isn't observed.
export async function fetchPricing(): Promise<void> {
  try {
    const res = await fetch('/api/v1/pricing');
    if (!res.ok) return;
    const data: PricingResponse = await res.json();
    if (data.pricing) {
      setPricingTable(data.pricing);
    }
  } catch {
    // Swallow: same-origin call to our own backend; a transient failure just
    // leaves the table as-is.
  }
}
