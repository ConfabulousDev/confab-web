import { useState, type MouseEvent } from 'react';
import { useDropdown } from '@/hooks';
import { CopyIcon, CheckIcon } from '@/components/icons';
import styles from './CopyIdDropdown.module.css';

interface CopyIdDropdownProps {
  /** Backend session UUID (Confab ID) */
  confabId: string;
  /** Claude Code local session ID */
  claudeCodeId: string;
  /** Show truncated UUID chip as trigger (detail page). If false, shows just a copy icon (list page). */
  showChip?: boolean;
}

function CopyIdDropdown({ confabId, claudeCodeId, showChip = false }: CopyIdDropdownProps) {
  const { isOpen, setIsOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();
  const [copiedLabel, setCopiedLabel] = useState<string | null>(null);

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
            onClick={(e) => handleCopy(claudeCodeId, 'claude', e)}
          >
            <span className={styles.menuLabel}>
              Copy Claude Code ID
              <span className={styles.menuHint}>for /resume</span>
            </span>
            {copiedLabel === 'claude' && <span className={styles.menuCheck}>{CheckIcon}</span>}
          </button>
        </div>
      )}
    </div>
  );
}

export default CopyIdDropdown;
