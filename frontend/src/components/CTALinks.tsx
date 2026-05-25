import styles from './CTALinks.module.css';

const LINKS = [
  { label: 'Demo', href: 'https://demo.confabulous.dev', color: styles.demo },
  { label: 'Docs', href: 'https://docs.confabulous.dev/getting-started/introduction/', color: styles.docs },
  { label: 'GitHub', href: 'https://github.com/ConfabulousDev/confab-web', color: styles.github },
] as const;

function CTALinks() {
  return (
    <div className={styles.row}>
      {LINKS.map(({ label, href, color }) => (
        <a
          key={label}
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className={[styles.link, color].filter(Boolean).join(' ')}
        >
          {label}
        </a>
      ))}
    </div>
  );
}

export default CTALinks;
