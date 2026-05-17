import { TokensCardForRegistry } from './TokensCard';
import { SessionCard } from './SessionCard';
import { CodeActivityCard } from './CodeActivityCard';
import { ToolsCard } from './ToolsCard';
import { ConversationCardForRegistry } from './ConversationCard';
import { AgentsAndSkillsCard } from './AgentsAndSkillsCard';
import { RedactionsCard } from './RedactionsCard';
import { SmartRecapCard } from './SmartRecapCard';
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
 *
 * ConversationCardForRegistry / TokensCardForRegistry wrap their cards to
 * default the required `provider` prop the registry shape can't model
 * (CF-441, CF-436).
 */
export const cardRegistry: CardDefinition[] = [
  {
    key: 'smart_recap',
    title: 'Smart Recap',
    component: SmartRecapCard,
    order: 0,
    span: 'full',
    // No shouldRender gate — the component handles no-data states internally
    // (quota_exceeded placeholder, unavailable placeholder, or returns null)
  },
  {
    key: 'tokens',
    title: 'Tokens',
    component: TokensCardForRegistry,
    order: 1,
    size: 'standard',
  },
  {
    key: 'session',
    title: 'Session',
    component: SessionCard,
    order: 2,
    size: 'standard',
  },
  {
    key: 'conversation',
    title: 'Conversation',
    component: ConversationCardForRegistry,
    order: 3,
    size: 'compact',
  },
  {
    key: 'code_activity',
    title: 'Code Activity',
    component: CodeActivityCard,
    order: 4,
    size: 'standard',
  },
  {
    key: 'tools',
    title: 'Tools',
    component: ToolsCard,
    order: 5,
    span: 2,
    size: 'tall',
    shouldRender: (data: ToolsCardData | null) => !!data && data.total_calls > 0,
  },
  {
    key: 'agents_and_skills',
    title: 'Agents and Skills',
    component: AgentsAndSkillsCard,
    order: 6,
    span: 2,
    size: 'tall',
    shouldRender: (data: AgentsAndSkillsCardData | null) =>
      !!data && data.agent_invocations + data.skill_invocations > 0,
  },
  {
    key: 'redactions',
    title: 'Redactions',
    component: RedactionsCard,
    order: 7,
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
