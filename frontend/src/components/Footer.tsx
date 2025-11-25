import styles from './Footer.module.css';

function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={styles.links}>
        <a href="https://github.com/anthropics/confab" target="_blank" rel="noopener noreferrer">GitHub</a>
        <a href="https://discord.gg/confab" target="_blank" rel="noopener noreferrer">Discord</a>
        <a href="/privacy">Privacy</a>
        <a href="/terms">Terms</a>
      </div>
      <div className={styles.copyright}>
        © {new Date().getFullYear()} Confab
      </div>
    </footer>
  );
}

export default Footer;
