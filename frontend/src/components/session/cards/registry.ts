import { TokensCard } from './TokensCard';
import { SessionCard } from './SessionCard';
import { CodeActivityCard } from './CodeActivityCard';
import { ToolsCard } from './ToolsCard';
import { ConversationCard } from './ConversationCard';
import { AgentsAndSkillsCard } from './AgentsAndSkillsCard';
import { RedactionsCard } from './RedactionsCard';
import type { CardDefinition } from './types';
import type {
  ToolsCardData,
  AgentsAndSkillsCardData,
  RedactionsCardData,
} from '@/schemas/api';

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
    size: 'standard',
  },
  {
    key: 'session',
    title: 'Session',
    component: SessionCard,
    order: 1,
    size: 'standard',
  },
  {
    key: 'conversation',
    title: 'Conversation',
    component: ConversationCard,
    order: 2,
    size: 'compact',
  },
  {
    key: 'code_activity',
    title: 'Code Activity',
    component: CodeActivityCard,
    order: 3,
    size: 'standard',
  },
  {
    key: 'tools',
    title: 'Tools',
    component: ToolsCard,
    order: 4,
    span: 2,
    size: 'tall',
    shouldRender: (data: ToolsCardData | null) => !!data && data.total_calls > 0,
  },
  {
    key: 'agents_and_skills',
    title: 'Agents and Skills',
    component: AgentsAndSkillsCard,
    order: 5,
    span: 2,
    size: 'tall',
    shouldRender: (data: AgentsAndSkillsCardData | null) =>
      !!data && data.agent_invocations + data.skill_invocations > 0,
  },
  {
    key: 'redactions',
    title: 'Redactions',
    component: RedactionsCard,
    order: 6,
    size: 'compact',
    shouldRender: (data: RedactionsCardData | null) =>
      !!data && data.total_redactions > 0,
  },
];

/**
 * Get cards sorted by display order.
 */
export function getOrderedCards() {
  return [...cardRegistry].sort((a, b) => a.order - b.order);
}
