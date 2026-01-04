import type { ReactNode } from 'react';
import { useTheme } from '@/hooks/useTheme';
import type { ThemeMode } from '@/contexts/ThemeContext';
import styles from './ThemeToggle.module.css';

const icons: Record<ThemeMode, { icon: ReactNode; label: string }> = {
  system: {
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="2" y="3" width="20" height="14" rx="2" />
        <path d="M8 21h8M12 17v4" />
      </svg>
    ),
    label: 'Theme: System (click for light)',
  },
  light: {
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="5" />
        <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
      </svg>
    ),
    label: 'Theme: Light (click for dark)',
  },
  dark: {
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
      </svg>
    ),
    label: 'Theme: Dark (click for system)',
  },
};

function ThemeToggle() {
  const { mode, toggleMode } = useTheme();
  const { icon, label } = icons[mode];

  return (
    <button
      className={styles.toggle}
      onClick={toggleMode}
      aria-label={label}
      title={label}
      type="button"
    >
      {icon}
    </button>
  );
}

export default ThemeToggle;
