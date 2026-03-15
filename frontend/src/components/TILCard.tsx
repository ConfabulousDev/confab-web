import { useState, useRef, useEffect, useCallback, type MouseEvent } from 'react';
import type { TILWithSession } from '@/schemas/api';
import { RelativeTime } from '@/components/RelativeTime';
import Chip from '@/components/Chip';
import { useDropdown } from '@/hooks';
import { RepoIcon, BranchIcon, PersonIcon, MoreVerticalIcon } from '@/components/icons';
import styles from './TILCard.module.css';

interface TILCardProps {
  til: TILWithSession;
  onNavigate: () => void;
  onDelete: () => void;
}

export default function TILCard({ til, onNavigate, onDelete }: TILCardProps) {
  const { isOpen: menuOpen, setIsOpen: setMenuOpen, toggle: toggleMenu, containerRef: menuRef } = useDropdown();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [touched, setTouched] = useState(false);
  const confirmTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const touchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Clean up timers
  useEffect(() => {
    return () => {
      if (confirmTimer.current) clearTimeout(confirmTimer.current);
      if (touchTimer.current) clearTimeout(touchTimer.current);
    };
  }, []);

  const handleCardClick = useCallback(() => {
    // On touch devices, first tap reveals the menu icon
    if ('ontouchstart' in window && til.is_owner && !touched) {
      setTouched(true);
      if (touchTimer.current) clearTimeout(touchTimer.current);
      touchTimer.current = setTimeout(() => setTouched(false), 3000);
    }
  }, [touched, til.is_owner]);

  const handleSessionClick = useCallback((e: MouseEvent) => {
    e.stopPropagation();
    onNavigate();
  }, [onNavigate]);

  const handleMenuToggle = useCallback((e: MouseEvent) => {
    e.stopPropagation();
    toggleMenu();
    setConfirmDelete(false);
  }, [toggleMenu]);

  const handleViewTranscript = useCallback((e: MouseEvent) => {
    e.stopPropagation();
    setMenuOpen(false);
    onNavigate();
  }, [onNavigate, setMenuOpen]);

  const handleDelete = useCallback((e: MouseEvent) => {
    e.stopPropagation();
    if (confirmDelete) {
      onDelete();
      setMenuOpen(false);
      if (confirmTimer.current) clearTimeout(confirmTimer.current);
    } else {
      setConfirmDelete(true);
      confirmTimer.current = setTimeout(() => setConfirmDelete(false), 3000);
    }
  }, [confirmDelete, onDelete, setMenuOpen]);

  const cardClass = [
    styles.card,
    til.is_owner ? styles.hasMenu : '',
    menuOpen ? styles.menuOpen : '',
    touched ? styles.touched : '',
  ].filter(Boolean).join(' ');

  return (
    <div className={cardClass} onClick={handleCardClick}>
      <div className={styles.header}>
        <div className={styles.title}>{til.title}</div>
        <div className={styles.corner} ref={menuRef}>
          <span className={styles.timestamp}>
            <RelativeTime date={til.created_at} />
          </span>
          {til.is_owner && (
            <>
              <button
                className={styles.menuBtn}
                onClick={handleMenuToggle}
                title="Actions"
                aria-label="TIL actions"
                aria-expanded={menuOpen}
              >
                {MoreVerticalIcon}
              </button>
              {menuOpen && (
                <div className={styles.dropdown} role="menu">
                  <button className={styles.dropdownItem} onClick={handleViewTranscript} role="menuitem">
                    View in transcript
                  </button>
                  <button className={styles.dropdownItemDanger} onClick={handleDelete} role="menuitem">
                    {confirmDelete ? 'Confirm delete?' : 'Delete'}
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      </div>

      <div className={styles.summary}>{til.summary}</div>

      <div className={styles.chipRow}>
        {til.session_title && (
          <span className={styles.sessionLink} onClick={handleSessionClick}>
            <Chip icon={null} variant="purple">{til.session_title}</Chip>
          </span>
        )}
        <Chip icon={PersonIcon} variant="neutral" copyValue={til.owner_email}>
          {til.owner_email}
        </Chip>
        {til.git_repo && (
          <Chip icon={RepoIcon} variant="neutral">{til.git_repo}</Chip>
        )}
        {til.git_branch && (
          <Chip icon={BranchIcon} variant="blue">{til.git_branch}</Chip>
        )}
      </div>
    </div>
  );
}
