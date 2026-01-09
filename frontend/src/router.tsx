import { lazy, Suspense, type ReactNode } from 'react';
import { createBrowserRouter, Navigate, useParams } from 'react-router-dom';
import App from './App';
import ProtectedRoute from '@/components/ProtectedRoute';

// Lazy load pages for code splitting
const HomePage = lazy(() => import('@/pages/HomePage'));
const SessionsPage = lazy(() => import('@/pages/SessionsPage'));
const SessionDetailPage = lazy(() => import('@/pages/SessionDetailPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const ShareLinksPage = lazy(() => import('@/pages/ShareLinksPage'));
const TrendsPage = lazy(() => import('@/pages/TrendsPage'));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage'));
const PoliciesPage = lazy(() => import('@/pages/PoliciesPage'));

/** Redirect old /sessions/:id/shared/:token URLs to canonical /sessions/:id (CF-132) */
// eslint-disable-next-line react-refresh/only-export-components
function RedirectToCanonicalSession() {
  const { sessionId } = useParams<{ sessionId: string }>();
  // Preserve query params (e.g., ?email=...) for login flow
  const search = window.location.search;
  return <Navigate to={`/sessions/${sessionId}${search}`} replace />;
}

/** Redirect to external Termly-hosted Terms of Service */
// eslint-disable-next-line react-refresh/only-export-components
function RedirectToTerms() {
  window.location.href =
    'https://app.termly.io/policy-viewer/policy.html?policyUUID=69001385-5934-4a9f-9ade-ca93873b3e6c';
  return null;
}

/** Redirect to external Termly-hosted Privacy Notice */
// eslint-disable-next-line react-refresh/only-export-components
function RedirectToPrivacy() {
  window.location.href =
    'https://app.termly.io/policy-viewer/policy.html?policyUUID=7366762a-c58a-4a7a-9cf0-f39620707a60';
  return null;
}

/** Wrap a page component with Suspense and optional ProtectedRoute */
function page(component: ReactNode, isProtected = false) {
  const wrapped = isProtected ? <ProtectedRoute>{component}</ProtectedRoute> : component;
  return <Suspense fallback={null}>{wrapped}</Suspense>;
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      { index: true, element: page(<HomePage />) },
      { path: 'sessions', element: page(<SessionsPage />, true) },
      { path: 'trends', element: page(<TrendsPage />, true) },
      { path: 'sessions/:id', element: page(<SessionDetailPage />) },
      { path: 'sessions/:sessionId/shared/:token', element: <RedirectToCanonicalSession /> },
      { path: 'keys', element: page(<APIKeysPage />, true) },
      { path: 'shares', element: page(<ShareLinksPage />, true) },
      { path: 'terms', element: <RedirectToTerms /> },
      { path: 'privacy', element: <RedirectToPrivacy /> },
      { path: 'policies', element: page(<PoliciesPage />) },
      { path: '*', element: page(<NotFoundPage />) },
    ],
  },
]);
