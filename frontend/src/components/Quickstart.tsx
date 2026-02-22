import { useCopyToClipboard } from '@/hooks';
import styles from './Quickstart.module.css';

function CopyableCode({ label, code }: { label: string; code: string }) {
  const { copy, copied } = useCopyToClipboard();

  return (
    <div className={styles.codeBlock}>
      <div className={styles.codeHeader}>
        <p className={styles.stepLabel}>{label}</p>
        <button
          className={styles.copyBtn}
          onClick={() => copy(code)}
          title="Copy to clipboard"
          aria-label="Copy to clipboard"
        >
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <code className={styles.code}>{code}</code>
    </div>
  );
}

function Quickstart() {
  const origin = window.location.origin;

  return (
    <div className={styles.container}>
      <div className={styles.icon}>🚀</div>
      <h2 className={styles.headline}>Quickstart</h2>
      <p className={styles.description}>
        Install the CLI to automatically sync your <em>Claude Code</em> sessions.
      </p>
      <p className={styles.platformNote}>macOS, Linux & WSL</p>

      <div className={styles.steps}>
        <div className={styles.step}>
          <span className={styles.stepNumber}>1</span>
          <div className={styles.stepContent}>
            <CopyableCode label="Install the CLI" code="curl -fsSL https://raw.githubusercontent.com/ConfabulousDev/confab/main/install.sh | bash" />
          </div>
        </div>

        <div className={styles.step}>
          <span className={styles.stepNumber}>2</span>
          <div className={styles.stepContent}>
            <CopyableCode label="Run setup" code={`confab setup --backend-url ${origin}`} />
          </div>
        </div>

        <div className={styles.step}>
          <span className={styles.stepNumber}>3</span>
          <div className={styles.stepContent}>
            <p className={styles.stepLabel}><em>Use Claude Code as usual</em></p>
            <p className={styles.stepDescription}>
              Your sessions will automatically sync here.
            </p>
          </div>
        </div>
      </div>

      <a
        href="https://github.com/ConfabulousDev/confab?tab=readme-ov-file#installation"
        target="_blank"
        rel="noopener noreferrer"
        className={styles.docsLink}
      >
        View installation docs →
      </a>
    </div>
  );
}

export default Quickstart;
