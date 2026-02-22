import styles from './DeployCTA.module.css';

function DeployCTA() {
  return (
    <div className={styles.container}>
      <p className={styles.tagline}>Open source. Self-hosted. Deploy in minutes.</p>
      <a
        href="https://github.com/ConfabulousDev/confab-web"
        target="_blank"
        rel="noopener noreferrer"
        className={styles.githubLink}
      >
        View on GitHub &rarr;
      </a>
    </div>
  );
}

export default DeployCTA;
