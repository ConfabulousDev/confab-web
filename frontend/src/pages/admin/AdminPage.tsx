import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useDocumentTitle } from '@/hooks';
import { useAppConfig } from '@/hooks/useAppConfig';
import PageHeader from '@/components/PageHeader';
import PageSidebar, { SidebarItem } from '@/components/PageSidebar';
import styles from './AdminPage.module.css';

const AdminUsersPage = lazy(() => import('./AdminUsersPage'));
const AdminSystemSharesPage = lazy(() => import('./AdminSystemSharesPage'));

const UsersIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
    <circle cx="9" cy="7" r="4" />
    <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
    <path d="M16 3.13a4 4 0 0 1 0 7.75" />
  </svg>
);

const SharesIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="18" cy="5" r="3" />
    <circle cx="6" cy="12" r="3" />
    <circle cx="18" cy="19" r="3" />
    <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
    <line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
  </svg>
);

function AdminPage() {
  useDocumentTitle('Admin');
  const location = useLocation();
  const navigate = useNavigate();
  const { sharesEnabled } = useAppConfig();

  const currentTab = location.pathname.includes('/admin/system-shares') ? 'system-shares' : 'users';

  return (
    <div className={styles.pageWrapper}>
      <PageSidebar title="Admin" collapsible={false}>
        <SidebarItem
          icon={UsersIcon}
          label="Users"
          active={currentTab === 'users'}
          onClick={() => navigate('/admin/users')}
          collapsed={false}
        />
        {sharesEnabled && (
          <SidebarItem
            icon={SharesIcon}
            label="System Shares"
            active={currentTab === 'system-shares'}
            onClick={() => navigate('/admin/system-shares')}
            collapsed={false}
          />
        )}
      </PageSidebar>

      <div className={styles.mainContent}>
        <PageHeader title="Admin" />

        <div className={styles.container}>
          <Suspense fallback={null}>
            <Routes>
              <Route index element={<Navigate to="users" replace />} />
              <Route path="users" element={<AdminUsersPage />} />
              <Route path="system-shares" element={<AdminSystemSharesPage />} />
            </Routes>
          </Suspense>
        </div>
      </div>
    </div>
  );
}

export default AdminPage;
