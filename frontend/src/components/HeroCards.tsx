import { useState } from 'react';
import AnalysisModal from './AnalysisModal';
import HowItWorksModal from './HowItWorksModal';
import MultiProviderModal from './MultiProviderModal';
import OrgCostMetricsModal from './OrgCostMetricsModal';
import PRLinkingModal from './PRLinkingModal';
import QuickstartModal from './QuickstartModal';
import RetroModal from './RetroModal';
import ReviewModal from './ReviewModal';
import SelfHostedModal from './SelfHostedModal';
import ShareModal from './ShareModal';
import SmartRecapModal from './SmartRecapModal';
import TILModal from './TILModal';
import styles from './HeroCards.module.css';

interface HeroCard {
  id: string;
  icon: string;
  title: string;
  description: string;
}

const cards: HeroCard[] = [
  {
    id: 'quickstart',
    icon: '🚀',
    title: 'Quickstart',
    description: 'Get up and running in under a minute with our simple CLI installer.',
  },
  {
    id: 'analysis',
    icon: '📊',
    title: 'Analysis',
    description: 'Track token usage, costs, and productivity metrics across all your sessions.',
  },
  {
    id: 'org-cost-metrics',
    icon: '🏢',
    title: 'Org cost metrics',
    description: 'See per-user spend, session counts, and time breakdowns across your whole team. Sort any column to find the heaviest users or longest sessions.',
  },
  {
    id: 'smart-recap',
    icon: '✨',
    title: 'Smart Recap',
    description: 'AI-powered session insights with actionable feedback and suggestions.',
  },
  {
    id: 'review',
    icon: '📖',
    title: 'Review',
    description: 'Browse your Claude Code and Codex sessions with full conversation history and context.',
  },
  {
    id: 'multi-provider',
    icon: '🔌',
    title: 'Multi-provider support',
    description: 'Started with Claude Code, added Codex, more on the way (OpenCode next). One dashboard for every AI coding session.',
  },
  {
    id: 'retro',
    icon: '🔁',
    title: 'Retro',
    description: 'Use the /retro skill to load any past session into a new one — even a teammate\'s, even across providers. Reference how a problem was solved, or distill it into a reusable skill.',
  },
  {
    id: 'pr-linking',
    icon: '🔀',
    title: 'PR Linking',
    description: 'Connect sessions to pull requests for full context on code changes.',
  },
  {
    id: 'til',
    icon: '💡',
    title: 'Today I Learned',
    description: 'Capture insights and learnings from your sessions. Search, filter, and share with your team.',
  },
  {
    id: 'share',
    icon: '🔗',
    title: 'Share',
    description: 'Generate shareable links to collaborate on sessions with your team.',
  },
  {
    id: 'self-hosted',
    icon: '🏠',
    title: 'Self-Hosted',
    description: 'Deploy on your own infrastructure. Your data never leaves your servers. MIT licensed and open source.',
  },
  {
    id: 'how-it-works',
    icon: '⚙️',
    title: 'How it works',
    description: 'Learn how Confab syncs and organizes your Claude Code and Codex sessions on your own server.',
  },
];

type ModalProps = { isOpen: boolean; onClose: () => void };

const MODAL_COMPONENTS: Record<string, React.ComponentType<ModalProps>> = {
  quickstart: QuickstartModal,
  analysis: AnalysisModal,
  'org-cost-metrics': OrgCostMetricsModal,
  'smart-recap': SmartRecapModal,
  review: ReviewModal,
  'multi-provider': MultiProviderModal,
  retro: RetroModal,
  'pr-linking': PRLinkingModal,
  til: TILModal,
  share: ShareModal,
  'self-hosted': SelfHostedModal,
  'how-it-works': HowItWorksModal,
};

function HeroCards() {
  const [openModalId, setOpenModalId] = useState<string | null>(null);
  const ActiveModal = openModalId ? MODAL_COMPONENTS[openModalId] : null;

  return (
    <div className={styles.container}>
      <div className={styles.grid}>
        {cards.map((card) => (
          <div
            key={card.id}
            className={`${styles.card} ${styles.clickable}`}
            onClick={() => setOpenModalId(card.id)}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                setOpenModalId(card.id);
              }
            }}
          >
            <div className={styles.header}>
              <span className={styles.icon}>{card.icon}</span>
              <h3 className={styles.title}>{card.title}</h3>
            </div>
            <p className={styles.description}>{card.description}</p>
          </div>
        ))}
      </div>

      {ActiveModal && (
        <ActiveModal isOpen={true} onClose={() => setOpenModalId(null)} />
      )}
    </div>
  );
}

export default HeroCards;
