import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useDocumentTitle } from '@/hooks';
import { useAppConfig } from '@/hooks/useAppConfig';
import PageHeader from '@/components/PageHeader';
import PageSidebar, { SidebarItem } from '@/components/PageSidebar';
import styles from './AdminPage.module.css';

const AdminUsersPage = lazy(() => import('./AdminUsersPage'));
const AdminSystemSharesPage = lazy(() => import('./AdminSystemSharesPage'));
const AdminSettingsPage = lazy(() => import('./AdminSettingsPage'));
const AdminCardInvalidationsPage = lazy(() => import('./AdminCardInvalidationsPage'));
const AdminUnpricedModelsPage = lazy(() => import('./AdminUnpricedModelsPage'));

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

const SmartRecapIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M12 2a7 7 0 0 1 7 7c0 2.38-1.19 4.47-3 5.74V17a2 2 0 0 1-2 2h-4a2 2 0 0 1-2-2v-2.26C6.19 13.47 5 11.38 5 9a7 7 0 0 1 7-7z" />
    <line x1="10" y1="22" x2="14" y2="22" />
  </svg>
);

const CardsIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="3" y="4" width="18" height="16" rx="2" />
    <line x1="3" y1="10" x2="21" y2="10" />
    <line x1="9" y1="4" x2="9" y2="20" />
  </svg>
);

const UnpricedIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="12" y1="1" x2="12" y2="23" />
    <path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
  </svg>
);

function AdminPage() {
  useDocumentTitle('Admin');
  const location = useLocation();
  const navigate = useNavigate();
  const { sharesEnabled, smartRecapEnabled } = useAppConfig();

  let currentTab = 'users';
  if (location.pathname.includes('/admin/system-shares')) {
    currentTab = 'system-shares';
  } else if (location.pathname.includes('/admin/smart-recap')) {
    currentTab = 'smart-recap';
  } else if (location.pathname.includes('/admin/cards')) {
    currentTab = 'cards';
  } else if (location.pathname.includes('/admin/unpriced-models')) {
    currentTab = 'unpriced-models';
  }

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
        {smartRecapEnabled && (
          <SidebarItem
            icon={SmartRecapIcon}
            label="Smart Recap"
            active={currentTab === 'smart-recap'}
            onClick={() => navigate('/admin/smart-recap')}
            collapsed={false}
          />
        )}
        <SidebarItem
          icon={CardsIcon}
          label="Card Invalidations"
          active={currentTab === 'cards'}
          onClick={() => navigate('/admin/cards')}
          collapsed={false}
        />
        <SidebarItem
          icon={UnpricedIcon}
          label="Unpriced Models"
          active={currentTab === 'unpriced-models'}
          onClick={() => navigate('/admin/unpriced-models')}
          collapsed={false}
        />
      </PageSidebar>

      <div className={styles.mainContent}>
        <PageHeader title="Admin" />

        <div className={styles.container}>
          <Suspense fallback={null}>
            <Routes>
              <Route index element={<Navigate to="users" replace />} />
              <Route path="users" element={<AdminUsersPage />} />
              <Route path="system-shares" element={<AdminSystemSharesPage />} />
              <Route path="smart-recap" element={<AdminSettingsPage />} />
              <Route path="cards" element={<AdminCardInvalidationsPage />} />
              <Route path="unpriced-models" element={<AdminUnpricedModelsPage />} />
            </Routes>
          </Suspense>
        </div>
      </div>
    </div>
  );
}

export default AdminPage;
