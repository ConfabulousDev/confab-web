import { lazy, Suspense, type ReactNode } from 'react';
import { createBrowserRouter } from 'react-router-dom';
import App from './App';
import ProtectedRoute from '@/components/ProtectedRoute';

// Lazy load pages for code splitting
const HomePage = lazy(() => import('@/pages/HomePage'));
const SessionsPage = lazy(() => import('@/pages/SessionsPage'));
const SessionDetailPage = lazy(() => import('@/pages/SessionDetailPage'));
const SharedSessionPage = lazy(() => import('@/pages/SharedSessionPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const ShareLinksPage = lazy(() => import('@/pages/ShareLinksPage'));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage'));

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
      { path: 'sessions/:id', element: page(<SessionDetailPage />, true) },
      { path: 'sessions/:sessionId/shared/:token', element: page(<SharedSessionPage />) },
      { path: 'keys', element: page(<APIKeysPage />, true) },
      { path: 'shares', element: page(<ShareLinksPage />, true) },
      { path: '*', element: page(<NotFoundPage />) },
    ],
  },
]);
