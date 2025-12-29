import type { AnalyticsCards } from '@/schemas/api';

/**
 * Props passed to each card component.
 * Using a simpler type that accepts the specific card data or null.
 */
export interface CardProps<T> {
  /** Card data from the API, null if not yet loaded */
  data: T | null;
  /** Whether data is currently being fetched */
  loading: boolean;
}

/**
 * Definition for a summary card in the registry.
 * Using `any` for the component type to avoid complex generic constraints.
 * Type safety is maintained at the card component level.
 */
export interface CardDefinition {
  /** Unique key matching the backend cards map key */
  key: keyof AnalyticsCards;
  /** Display title for the card header */
  title: string;
  /** React component to render the card */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: React.ComponentType<CardProps<any>>;
  /** Display order (lower = earlier) */
  order: number;
  /** Number of grid columns to span (default: 1) */
  span?: 1 | 2 | 3;
}

/**
 * Type for the full registry of all cards.
 */
export type CardRegistry = CardDefinition[];
