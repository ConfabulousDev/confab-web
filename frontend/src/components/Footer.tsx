import styles from './Footer.module.css';

const SUPPORT_EMAIL = 'help@confabulous.dev';

function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={styles.links}>
        <a href="https://github.com/ConfabulousDev/confab-cli" target="_blank" rel="noopener noreferrer">GitHub</a>
        <a href="https://discord.gg/p6H7MQnQD8" target="_blank" rel="noopener noreferrer">Discord</a>
        <a href={`mailto:${SUPPORT_EMAIL}`}>Help</a>
        <a href="/privacy">Privacy</a>
        <a href="/terms">Terms</a>
        <a href="/legal">Legal</a>
      </div>
      <div className={styles.copyright}>
        © {new Date().getFullYear()} Confabulous
      </div>
    </footer>
  );
}

export default Footer;
