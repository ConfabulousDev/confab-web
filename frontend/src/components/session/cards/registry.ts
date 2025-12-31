import { TokensCard } from './TokensCard';
import { SessionCard } from './SessionCard';
import { CodeActivityCard } from './CodeActivityCard';
import { ToolsCard } from './ToolsCard';
import { ConversationCard } from './ConversationCard';
import { AgentsCard } from './AgentsCard';
import { SkillsCard } from './SkillsCard';
import type { CardDefinition } from './types';

/**
 * Registry of all summary cards.
 * Cards are rendered in order by their `order` field.
 *
 * To add a new card:
 * 1. Add the card data type to AnalyticsCards schema
 * 2. Create a card component in this directory
 * 3. Add it to this registry with appropriate order
 *
 * Note: Cost is now included in the Tokens card, and
 * Compaction stats are now included in the Session card.
 */
export const cardRegistry: CardDefinition[] = [
  {
    key: 'tokens',
    title: 'Tokens',
    component: TokensCard,
    order: 0,
  },
  {
    key: 'session',
    title: 'Session',
    component: SessionCard,
    order: 1,
  },
  {
    key: 'conversation',
    title: 'Conversation',
    component: ConversationCard,
    order: 2,
  },
  {
    key: 'code_activity',
    title: 'Code Activity',
    component: CodeActivityCard,
    order: 3,
  },
  {
    key: 'tools',
    title: 'Tools',
    component: ToolsCard,
    order: 4,
    span: 2,
  },
  {
    key: 'agents',
    title: 'Agents',
    component: AgentsCard,
    order: 5,
    span: 2,
  },
  {
    key: 'skills',
    title: 'Skills',
    component: SkillsCard,
    order: 6,
    span: 2,
  },
];

/**
 * Get cards sorted by display order.
 */
export function getOrderedCards() {
  return [...cardRegistry].sort((a, b) => a.order - b.order);
}
