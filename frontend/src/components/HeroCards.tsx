import { useState } from 'react';
import HowItWorksModal from './HowItWorksModal';
import QuickstartModal from './QuickstartModal';
import styles from './HeroCards.module.css';

interface HeroCard {
  id: string;
  icon: string;
  title: string;
  description: string;
  href?: string;
}

const cards: HeroCard[] = [
  {
    id: 'review',
    icon: '📖',
    title: 'Review',
    description: 'Browse your Claude Code sessions with full conversation history and context.',
  },
  {
    id: 'share',
    icon: '🔗',
    title: 'Share',
    description: 'Generate shareable links to collaborate on sessions with your team.',
  },
  {
    id: 'pr-linking',
    icon: '🔀',
    title: 'PR Linking',
    description: 'Connect sessions to pull requests for full context on code changes.',
  },
  {
    id: 'analysis',
    icon: '📊',
    title: 'Analysis',
    description: 'Track token usage, costs, and productivity metrics across all your sessions.',
  },
  {
    id: 'how-it-works',
    icon: '⚙️',
    title: 'How it works',
    description: 'Learn how Confabulous syncs and organizes your Claude Code sessions.',
  },
  {
    id: 'quickstart',
    icon: '🚀',
    title: 'Quickstart',
    description: 'Get up and running in under a minute with our simple CLI installer.',
  },
];

const CLICKABLE_CARDS = ['quickstart', 'how-it-works'];

function HeroCards() {
  const [quickstartOpen, setQuickstartOpen] = useState(false);
  const [howItWorksOpen, setHowItWorksOpen] = useState(false);

  const handleCardClick = (cardId: string) => {
    if (cardId === 'quickstart') {
      setQuickstartOpen(true);
    } else if (cardId === 'how-it-works') {
      setHowItWorksOpen(true);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.grid}>
        {cards.map((card) => {
          const isClickable = CLICKABLE_CARDS.includes(card.id);
          return (
            <div
              key={card.id}
              className={`${styles.card} ${isClickable ? styles.clickable : ''}`}
              onClick={isClickable ? () => handleCardClick(card.id) : undefined}
              role={isClickable ? 'button' : undefined}
              tabIndex={isClickable ? 0 : undefined}
              onKeyDown={isClickable ? (e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  handleCardClick(card.id);
                }
              } : undefined}
            >
              <div className={styles.header}>
                <span className={styles.icon}>{card.icon}</span>
                <h3 className={styles.title}>{card.title}</h3>
              </div>
              <p className={styles.description}>{card.description}</p>
            </div>
          );
        })}
      </div>

      <QuickstartModal
        isOpen={quickstartOpen}
        onClose={() => setQuickstartOpen(false)}
      />
      <HowItWorksModal
        isOpen={howItWorksOpen}
        onClose={() => setHowItWorksOpen(false)}
      />
    </div>
  );
}

export default HeroCards;
