import styles from './HeroCards.module.css';

interface DocsLink {
  label: string;
  href: string;
}

interface HeroCard {
  icon: string;
  title: string;
  description: string;
  demoUrl?: string;
  docsLinks: DocsLink[];
}

const DEMO = 'https://demo.confabulous.dev';
const DOCS = 'https://docs.confabulous.dev';
const DEMO_SESSION = `${DEMO}/sessions/e8b54496-44f2-40f5-94c1-d786b5443901`;

const cards: HeroCard[] = [
  {
    icon: '🚀',
    title: 'Quickstart',
    description: 'Get up and running in under a minute with our simple CLI installer.',
    docsLinks: [{ label: 'Docs', href: `${DOCS}/getting-started/admin-quickstart/` }],
  },
  {
    icon: '📊',
    title: 'Analytics',
    description: 'Track token usage, costs, and productivity metrics across all your sessions.',
    demoUrl: DEMO_SESSION,
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/analytics/` }],
  },
  {
    icon: '🏢',
    title: 'Org cost metrics',
    description:
      'See per-user spend, session counts, and time breakdowns across your whole team. Sort any column to find the heaviest users or longest sessions.',
    demoUrl: `${DEMO}/org`,
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/organization-analytics/` }],
  },
  {
    icon: '✨',
    title: 'Smart Recap',
    description: 'AI-powered session insights with actionable feedback and suggestions.',
    demoUrl: DEMO_SESSION,
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/smart-recap/` }],
  },
  {
    icon: '📖',
    title: 'Review',
    description:
      'Browse your Claude Code, Codex, and OpenCode sessions with full conversation history and context.',
    demoUrl: `${DEMO_SESSION}?tab=transcript`,
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/sessions/` }],
  },
  {
    icon: '🔌',
    title: 'Multi-provider support',
    description:
      'Claude Code, Codex, and OpenCode — one dashboard for every AI coding session.',
    demoUrl: `${DEMO}/sessions?provider=codex`,
    docsLinks: [
      { label: 'Claude Code', href: `${DOCS}/providers/claude-code/` },
      { label: 'Codex', href: `${DOCS}/providers/codex/` },
      { label: 'OpenCode', href: `${DOCS}/providers/opencode/` },
    ],
  },
  {
    icon: '🔁',
    title: 'Retro',
    description:
      "Use the /retro skill to load any past session into a new one — even a teammate's, even across providers. Reference how a problem was solved, or distill it into a reusable skill.",
    docsLinks: [{ label: 'Docs', href: `${DOCS}/cli/skills/#retro--session-retrospective` }],
  },
  {
    icon: '🔀',
    title: 'PR Linking',
    description: 'Connect sessions to pull requests for full context on code changes.',
    demoUrl: DEMO_SESSION,
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/pr-linking/` }],
  },
  {
    icon: '💡',
    title: 'Today I Learned',
    description:
      'Capture insights and learnings from your sessions. Search, filter, and share with your team.',
    demoUrl: `${DEMO}/tils`,
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/tils/` }],
  },
  {
    icon: '🔗',
    title: 'Share',
    description: 'Generate shareable links to collaborate on sessions with your team.',
    docsLinks: [{ label: 'Docs', href: `${DOCS}/features/sharing/` }],
  },
  {
    icon: '🏠',
    title: 'Self-Hosted',
    description:
      'Deploy on your own infrastructure. Your data never leaves your servers. MIT licensed and open source.',
    docsLinks: [{ label: 'Docs', href: `${DOCS}/self-hosting/deploy/` }],
  },
  {
    icon: '⚙️',
    title: 'How it works',
    description:
      'Learn how Confab syncs and organizes your Claude Code, Codex, and OpenCode sessions on your own server.',
    docsLinks: [{ label: 'Docs', href: `${DOCS}/architecture/overview/` }],
  },
];

function ariaLabelFor(title: string, label: string): string {
  if (label === 'Demo' || label === 'Docs') {
    return `${title}: ${label}`;
  }
  return `${title}: ${label} docs`;
}

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
            <div className={styles.links}>
              {card.demoUrl && (
                <a
                  href={card.demoUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={ariaLabelFor(card.title, 'Demo')}
                  className={styles.link}
                >
                  Demo →
                </a>
              )}
              {card.docsLinks.map(({ label, href }) => (
                <a
                  key={label}
                  href={href}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={ariaLabelFor(card.title, label)}
                  className={styles.link}
                >
                  {label} →
                </a>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default HeroCards;
