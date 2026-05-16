import { useState, type MouseEvent } from 'react';
import { useDropdown } from '@/hooks';
import { CopyIcon, CheckIcon } from '@/components/icons';
import { getProviderMetadataOrFallback } from '@/utils/providers';
import styles from './CopyIdDropdown.module.css';

interface CopyIdDropdownProps {
  /** Backend session UUID (Confab ID) */
  confabId: string;
  /** Agent-native session ID (Claude Code UUID or Codex rollout UUID) */
  externalId: string;
  /**
   * Canonical agent identifier from the session record (`'claude-code'`,
   * `'codex'`, …). Drives the second menu item's label and resume-command
   * hint. Defaults to `'claude-code'` for backward compatibility.
   */
  provider?: string;
  /** Show truncated UUID chip as trigger (detail page). If false, shows just a copy icon (list page). */
  showChip?: boolean;
}

function CopyIdDropdown({ confabId, externalId, provider = 'claude-code', showChip = false }: CopyIdDropdownProps) {
  const { isOpen, setIsOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();
  const [copiedLabel, setCopiedLabel] = useState<string | null>(null);
  const { resumeCommand } = getProviderMetadataOrFallback(provider, 'claude');

  function handleToggle(e: MouseEvent) {
    e.stopPropagation();
    e.preventDefault();
    toggle();
  }

  function handleCopy(value: string, label: string, e: MouseEvent) {
    e.stopPropagation();
    e.preventDefault();
    navigator.clipboard.writeText(value);
    setCopiedLabel(label);
    setTimeout(() => {
      setIsOpen(false);
      setCopiedLabel(null);
    }, 600);
  }

  return (
    <div className={styles.container} ref={containerRef}>
      {showChip ? (
        <button
          className={styles.chip}
          onClick={handleToggle}
          title="Copy session ID"
          aria-label="Copy session ID"
        >
          <span className={styles.chipText}>{confabId.substring(0, 8)}</span>
          <span className={styles.chipIcon}>{isOpen ? CheckIcon : CopyIcon}</span>
        </button>
      ) : (
        <button
          className={styles.iconBtn}
          onClick={handleToggle}
          title="Copy session ID"
          aria-label="Copy session ID"
        >
          {CopyIcon}
        </button>
      )}
      {isOpen && (
        <div className={styles.menu}>
          <button
            className={styles.menuItem}
            onClick={(e) => handleCopy(confabId, 'confab', e)}
          >
            <span className={styles.menuLabel}>
              Copy Confab ID
              <span className={styles.menuHint}>for /retro</span>
            </span>
            {copiedLabel === 'confab' && <span className={styles.menuCheck}>{CheckIcon}</span>}
          </button>
          <button
            className={styles.menuItem}
            onClick={(e) => handleCopy(externalId, 'external', e)}
          >
            <span className={styles.menuLabel}>
              {resumeCommand.idLabel}
              <span className={styles.menuHint}>{resumeCommand.commandHint}</span>
            </span>
            {copiedLabel === 'external' && <span className={styles.menuCheck}>{CheckIcon}</span>}
          </button>
        </div>
      )}
    </div>
  );
}

export default CopyIdDropdown;
