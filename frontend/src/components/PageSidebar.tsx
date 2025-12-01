import styles from './PageSidebar.module.css';

interface PageSidebarProps {
  title?: string;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
  children: React.ReactNode;
  collapsible?: boolean;
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
  collapsed = false,
  onToggleCollapse,
  children,
  collapsible = true,
}: PageSidebarProps) {
  const isCollapsed = collapsible && collapsed;
  const showHeader = title || collapsible;

  return (
    <aside className={`${styles.sidebar} ${isCollapsed ? styles.collapsed : ''}`}>
      {showHeader && (
        <div className={styles.header}>
          {!isCollapsed && title && <h2 className={styles.title}>{title}</h2>}
          {collapsible && (
            <button
              className={styles.collapseBtn}
              onClick={onToggleCollapse}
              title={isCollapsed ? `Expand ${title?.toLowerCase() || 'sidebar'}` : `Collapse ${title?.toLowerCase() || 'sidebar'}`}
              aria-label={isCollapsed ? `Expand ${title?.toLowerCase() || 'sidebar'}` : `Collapse ${title?.toLowerCase() || 'sidebar'}`}
            >
              <span className={isCollapsed ? styles.rotated : ''}>
                {CollapseIcon}
              </span>
            </button>
          )}
        </div>
      )}

      <div className={styles.content}>
        {children}
      </div>
    </aside>
  );
}

export type SidebarItemColor = 'default' | 'green' | 'blue' | 'gray' | 'cyan' | 'purple' | 'amber';

export interface SidebarItemProps {
  icon: React.ReactNode;
  label: string;
  count?: number;
  active?: boolean;
  disabled?: boolean;
  onClick?: () => void;
  collapsed?: boolean;
  activeColor?: SidebarItemColor;
}

export function SidebarItem({
  icon,
  label,
  count,
  active = false,
  disabled = false,
  onClick,
  collapsed = false,
  activeColor = 'default',
}: SidebarItemProps) {
  const colorClass = active ? styles[activeColor] : '';

  return (
    <button
      className={`${styles.item} ${active ? styles.active : ''} ${colorClass} ${disabled ? styles.disabled : ''}`}
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
