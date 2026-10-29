import { lazy, Suspense, type ReactNode } from 'react';
import { createBrowserRouter, Navigate, useParams, type RouteObject } from 'react-router-dom';
import App from './App';
import ProtectedRoute from '@/components/ProtectedRoute';
import { useAppConfig } from '@/hooks/useAppConfig';
import { useAuth } from '@/hooks/useAuth';
import { LEGAL_DOCS } from '@/legal/docs';

// Lazy load pages for code splitting
const HomePage = lazy(() => import('@/pages/HomePage'));
const SessionsPage = lazy(() => import('@/pages/SessionsPage'));
const SessionDetailPage = lazy(() => import('@/pages/SessionDetailPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const ShareLinksPage = lazy(() => import('@/pages/ShareLinksPage'));
const TrendsPage = lazy(() => import('@/pages/TrendsPage'));
const OrgPage = lazy(() => import('@/pages/OrgPage'));
const AdminPage = lazy(() => import('@/pages/admin/AdminPage'));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage'));
const PoliciesPage = lazy(() => import('@/pages/PoliciesPage'));
const LoginPage = lazy(() => import('@/pages/LoginPage'));
const LegalDocPage = lazy(() => import('@/pages/LegalDocPage'));

/** Redirect old /sessions/:id/shared/:token URLs to canonical /sessions/:id (CF-132) */
// eslint-disable-next-line react-refresh/only-export-components
function RedirectToCanonicalSession() {
  const { sessionId } = useParams<{ sessionId: string }>();
  // Preserve query params (e.g., ?email=...) for login flow
  const search = window.location.search;
  return <Navigate to={`/sessions/${sessionId}${search}`} replace />;
}

/** Gate SaaS-only routes — renders children when SaaS footer is enabled, NotFound otherwise */
// eslint-disable-next-line react-refresh/only-export-components
function SaasRoute({ children }: { children: ReactNode }) {
  const { saasFooterEnabled } = useAppConfig();
  if (!saasFooterEnabled) return <NotFoundPage />;
  return <>{children}</>;
}

/** Gate org analytics route — shows a message when feature is disabled */
// eslint-disable-next-line react-refresh/only-export-components
function OrgAnalyticsRoute({ children }: { children: ReactNode }) {
  const { orgAnalyticsEnabled } = useAppConfig();
  if (!orgAnalyticsEnabled) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--color-text-secondary)', fontSize: '14px' }}>
        Feature not enabled. Contact your administrator.
      </div>
    );
  }
  return <>{children}</>;
}

/** Gate admin routes — redirect non-admins to home */
// eslint-disable-next-line react-refresh/only-export-components
function AdminRoute({ children }: { children: ReactNode }) {
  const { user, isAuthenticated } = useAuth();
  if (!isAuthenticated) return null;
  if (!user?.is_admin) return <Navigate to="/" replace />;
  return <>{children}</>;
}

/** Wrap a page component with Suspense and optional ProtectedRoute */
function page(component: ReactNode, isProtected = false) {
  const wrapped = isProtected ? <ProtectedRoute>{component}</ProtectedRoute> : component;
  return <Suspense fallback={null}>{wrapped}</Suspense>;
}

/** Wrap a SaaS-only page (404 on self-hosted instances) with Suspense */
function saasPage(component: ReactNode) {
  return <Suspense fallback={null}><SaasRoute>{component}</SaasRoute></Suspense>;
}

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <App />,
    children: [
      { index: true, element: page(<HomePage />) },
      { path: 'sessions', element: page(<SessionsPage />, true) },
      { path: 'trends', element: page(<TrendsPage />, true) },
      { path: 'org', element: page(<OrgAnalyticsRoute><OrgPage /></OrgAnalyticsRoute>, true) },
      { path: 'sessions/:id', element: page(<SessionDetailPage />) },
      { path: 'sessions/:sessionId/shared/:token', element: <RedirectToCanonicalSession /> },
      { path: 'keys', element: page(<APIKeysPage />, true) },
      { path: 'shares', element: page(<ShareLinksPage />, true) },
      // Legal documents (SaaS only): /terms, /privacy, /cookies, … from the registry.
      // /terms and /privacy are linked from OAuth consent screens; keep them stable.
      ...LEGAL_DOCS.map((d) => ({ path: d.slug, element: saasPage(<LegalDocPage slug={d.slug} />) })),
      { path: 'login', element: page(<LoginPage />) },
      { path: 'policies', element: saasPage(<PoliciesPage />) },
      { path: 'admin/*', element: page(<AdminRoute><AdminPage /></AdminRoute>, true) },
      { path: '*', element: page(<NotFoundPage />) },
    ],
  },
];

export const router = createBrowserRouter(routes);
