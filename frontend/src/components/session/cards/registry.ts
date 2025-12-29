import { TokensCard } from './TokensCard';
import { CostCard } from './CostCard';
import { CompactionCard } from './CompactionCard';
import { SessionCard } from './SessionCard';
import { ToolsCard } from './ToolsCard';
import type { CardDefinition } from './types';

/**
 * Registry of all summary cards.
 * Cards are rendered in order by their `order` field.
 *
 * To add a new card:
 * 1. Add the card data type to AnalyticsCards schema
 * 2. Create a card component in this directory
 * 3. Add it to this registry with appropriate order
 */
export const cardRegistry: CardDefinition[] = [
  {
    key: 'tokens',
    title: 'Tokens',
    component: TokensCard,
    order: 0,
  },
  {
    key: 'cost',
    title: 'Cost',
    component: CostCard,
    order: 1,
  },
  {
    key: 'compaction',
    title: 'Compaction',
    component: CompactionCard,
    order: 2,
  },
  {
    key: 'session',
    title: 'Session',
    component: SessionCard,
    order: 3,
  },
  {
    key: 'tools',
    title: 'Tools',
    component: ToolsCard,
    order: 4,
  },
];

/**
 * Get cards sorted by display order.
 */
export function getOrderedCards() {
  return [...cardRegistry].sort((a, b) => a.order - b.order);
}
