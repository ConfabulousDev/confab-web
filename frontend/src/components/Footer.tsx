import { useAppConfig } from '@/hooks/useAppConfig';
import { DOCS_URL, GITHUB_ISSUES_URL } from '@/utils/externalLinks';
import styles from './Footer.module.css';

declare global {
  interface Window {
    displayPreferenceModal?: () => void;
  }
}

function Footer() {
  const { supportEmail, saasTermlyEnabled } = useAppConfig();
  const handleCookieSettings = (e: React.MouseEvent) => {
    e.preventDefault();
    window.displayPreferenceModal?.();
  };

  return (
    <footer className={styles.footer}>
      <div className={styles.links}>
        <a href={DOCS_URL} target="_blank" rel="noopener noreferrer">Docs</a>
        <a href="https://github.com/ConfabulousDev/confab" target="_blank" rel="noopener noreferrer">GitHub</a>
        <a href="https://discord.gg/p6H7MQnQD8" target="_blank" rel="noopener noreferrer">Discord</a>
        <a href={GITHUB_ISSUES_URL} target="_blank" rel="noopener noreferrer">Report an issue</a>
        <a href={`mailto:${supportEmail}`}>Help</a>
        <a href="/policies">Policies</a>
        {saasTermlyEnabled && (
          <a href="#" onClick={handleCookieSettings} className="termly-display-preferences">Cookie Settings</a>
        )}
      </div>
      <div className={styles.copyright}>
        © {new Date().getFullYear()} Confabulous Software LLC
      </div>
    </footer>
  );
}

export default Footer;
