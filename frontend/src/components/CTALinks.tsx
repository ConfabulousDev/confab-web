import styles from './CTALinks.module.css';

const LINKS: ReadonlyArray<{ label: string; href: string }> = [
  { label: 'Demo', href: 'https://demo.confabulous.dev' },
  { label: 'Docs', href: 'https://docs.confabulous.dev' },
  { label: 'GitHub', href: 'https://github.com/ConfabulousDev/confab-web' },
];

function CTALinks() {
  return (
    <div className={styles.row}>
      {LINKS.map(({ label, href }) => (
        <a
          key={label}
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className={styles.pill}
        >
          {label} &rarr;
        </a>
      ))}
    </div>
  );
}

export default CTALinks;
