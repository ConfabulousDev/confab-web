import { useState } from 'react';
import AnalysisModal from './AnalysisModal';
import HowItWorksModal from './HowItWorksModal';
import PRLinkingModal from './PRLinkingModal';
import SelfHostedModal from './SelfHostedModal';
import QuickstartModal from './QuickstartModal';
import ReviewModal from './ReviewModal';
import ShareModal from './ShareModal';
import SmartRecapModal from './SmartRecapModal';
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
    id: 'smart-recap',
    icon: '✨',
    title: 'Smart Recap',
    description: 'AI-powered session insights with actionable feedback and suggestions.',
  },
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
    id: 'self-hosted',
    icon: '🏠',
    title: 'Self-Hosted',
    description: 'Deploy on your own infrastructure. Your data never leaves your servers. MIT licensed and open source.',
  },
  {
    id: 'how-it-works',
    icon: '⚙️',
    title: 'How it works',
    description: 'Learn how Confab syncs and organizes your Claude Code sessions on your own server.',
  },
  {
    id: 'quickstart',
    icon: '🚀',
    title: 'Quickstart',
    description: 'Get up and running in under a minute with our simple CLI installer.',
  },
];

const CLICKABLE_CARDS = ['smart-recap', 'quickstart', 'how-it-works', 'analysis', 'pr-linking', 'review', 'share', 'self-hosted'];

function HeroCards() {
  const [smartRecapOpen, setSmartRecapOpen] = useState(false);
  const [quickstartOpen, setQuickstartOpen] = useState(false);
  const [howItWorksOpen, setHowItWorksOpen] = useState(false);
  const [analysisOpen, setAnalysisOpen] = useState(false);
  const [prLinkingOpen, setPrLinkingOpen] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [selfHostedOpen, setSelfHostedOpen] = useState(false);

  const handleCardClick = (cardId: string) => {
    if (cardId === 'smart-recap') {
      setSmartRecapOpen(true);
    } else if (cardId === 'quickstart') {
      setQuickstartOpen(true);
    } else if (cardId === 'how-it-works') {
      setHowItWorksOpen(true);
    } else if (cardId === 'analysis') {
      setAnalysisOpen(true);
    } else if (cardId === 'pr-linking') {
      setPrLinkingOpen(true);
    } else if (cardId === 'review') {
      setReviewOpen(true);
    } else if (cardId === 'share') {
      setShareOpen(true);
    } else if (cardId === 'self-hosted') {
      setSelfHostedOpen(true);
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

      <SmartRecapModal
        isOpen={smartRecapOpen}
        onClose={() => setSmartRecapOpen(false)}
      />
      <QuickstartModal
        isOpen={quickstartOpen}
        onClose={() => setQuickstartOpen(false)}
      />
      <HowItWorksModal
        isOpen={howItWorksOpen}
        onClose={() => setHowItWorksOpen(false)}
      />
      <AnalysisModal
        isOpen={analysisOpen}
        onClose={() => setAnalysisOpen(false)}
      />
      <PRLinkingModal
        isOpen={prLinkingOpen}
        onClose={() => setPrLinkingOpen(false)}
      />
      <ReviewModal
        isOpen={reviewOpen}
        onClose={() => setReviewOpen(false)}
      />
      <ShareModal
        isOpen={shareOpen}
        onClose={() => setShareOpen(false)}
      />
      <SelfHostedModal
        isOpen={selfHostedOpen}
        onClose={() => setSelfHostedOpen(false)}
      />
    </div>
  );
}

export default HeroCards;
