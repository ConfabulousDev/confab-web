import styles from './HeroCards.module.css';

interface HeroCard {
  icon: string;
  title: string;
  description: string;
  href?: string;
  onClick?: () => void;
}

const cards: HeroCard[] = [
  {
    icon: '📖',
    title: 'Review',
    description: 'Browse your Claude Code sessions with full conversation history and context.',
  },
  {
    icon: '🔗',
    title: 'Share',
    description: 'Generate shareable links to collaborate on sessions with your team.',
  },
  {
    icon: '🔀',
    title: 'PR Linking',
    description: 'Connect sessions to pull requests for full context on code changes.',
  },
  {
    icon: '📊',
    title: 'Analysis',
    description: 'Track token usage, costs, and productivity metrics across all your sessions.',
  },
  {
    icon: '▶️',
    title: 'Demo',
    description: 'See Confabulous in action with a sample session walkthrough.',
  },
  {
    icon: '🚀',
    title: 'Quickstart',
    description: 'Get up and running in under a minute with our simple CLI installer.',
  },
];

function HeroCards() {
  return (
    <div className={styles.container}>
      <div className={styles.grid}>
        {cards.map((card) => (
          <div key={card.title} className={styles.card}>
            <div className={styles.header}>
              <span className={styles.icon}>{card.icon}</span>
              <h3 className={styles.title}>{card.title}</h3>
            </div>
            <p className={styles.description}>{card.description}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

export default HeroCards;
