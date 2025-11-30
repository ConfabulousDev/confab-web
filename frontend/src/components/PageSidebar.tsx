import styles from './PageSidebar.module.css';

interface PageSidebarProps {
  title: string;
  collapsed: boolean;
  onToggleCollapse: () => void;
  children: React.ReactNode;
}

const CollapseIcon = (
  <svg
    width="16"
    height="16"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
  >
    <polyline points="15 18 9 12 15 6" />
  </svg>
);

function PageSidebar({
  title,
  collapsed,
  onToggleCollapse,
  children,
}: PageSidebarProps) {
  return (
    <aside className={`${styles.sidebar} ${collapsed ? styles.collapsed : ''}`}>
      <div className={styles.header}>
        {!collapsed && <h2 className={styles.title}>{title}</h2>}
        <button
          className={styles.collapseBtn}
          onClick={onToggleCollapse}
          title={collapsed ? `Expand ${title.toLowerCase()}` : `Collapse ${title.toLowerCase()}`}
          aria-label={collapsed ? `Expand ${title.toLowerCase()}` : `Collapse ${title.toLowerCase()}`}
        >
          <span className={collapsed ? styles.rotated : ''}>
            {CollapseIcon}
          </span>
        </button>
      </div>

      <div className={styles.content}>
        {children}
      </div>
    </aside>
  );
}

export interface SidebarItemProps {
  icon: React.ReactNode;
  label: string;
  count?: number;
  active?: boolean;
  disabled?: boolean;
  onClick?: () => void;
  collapsed?: boolean;
}

export function SidebarItem({
  icon,
  label,
  count,
  active = false,
  disabled = false,
  onClick,
  collapsed = false,
}: SidebarItemProps) {
  return (
    <button
      className={`${styles.item} ${active ? styles.active : ''} ${disabled ? styles.disabled : ''}`}
      onClick={() => !disabled && onClick?.()}
      disabled={disabled}
      title={collapsed ? `${label}${count !== undefined ? ` (${count})` : ''}` : undefined}
    >
      <span className={styles.itemIcon}>{icon}</span>
      {!collapsed && (
        <>
          <span className={styles.itemLabel}>{label}</span>
          {count !== undefined && (
            <span className={styles.itemCount}>{count}</span>
          )}
        </>
      )}
    </button>
  );
}

export default PageSidebar;
