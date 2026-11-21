import type { SessionScope } from '@/hooks/useSessionFilters';
import styles from './SessionScopeToggle.module.css';

interface SessionScopeToggleProps {
  scope: SessionScope;
  onScopeChange: (scope: SessionScope) => void;
  disabled?: boolean;
}

const OPTIONS: { value: SessionScope; label: string }[] = [
  { value: '', label: 'Mine & shared' },
  { value: 'all', label: 'All sessions' },
];

/**
 * xtrf: admin-only segmented control on the Sessions page. "All sessions"
 * lists every session on the instance (GET /sessions?scope=all); the
 * default lists the viewer's own and shared sessions. The caller decides
 * whether to render it (only for admins).
 */
function SessionScopeToggle({ scope, onScopeChange, disabled = false }: SessionScopeToggleProps) {
  return (
    <span className={styles.group} role="group" aria-label="Session scope">
      {OPTIONS.map((opt) => (
        <button
          key={opt.value || 'default'}
          type="button"
          className={styles.option}
          aria-pressed={opt.value === scope}
          disabled={disabled}
          onClick={() => onScopeChange(opt.value)}
        >
          {opt.label}
        </button>
      ))}
    </span>
  );
}

export default SessionScopeToggle;
