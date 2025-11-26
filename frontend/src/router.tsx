import { lazy, Suspense } from 'react';
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

// Loading fallback - minimal to avoid flash (inline to avoid separate component file)
const PageLoader = null;

export const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      {
        index: true,
        element: (
          <Suspense fallback={PageLoader}>
            <HomePage />
          </Suspense>
        ),
      },
      {
        path: 'sessions',
        element: (
          <Suspense fallback={PageLoader}>
            <ProtectedRoute>
              <SessionsPage />
            </ProtectedRoute>
          </Suspense>
        ),
      },
      {
        path: 'sessions/:id',
        element: (
          <Suspense fallback={PageLoader}>
            <ProtectedRoute>
              <SessionDetailPage />
            </ProtectedRoute>
          </Suspense>
        ),
      },
      {
        path: 'sessions/:sessionId/shared/:token',
        element: (
          <Suspense fallback={PageLoader}>
            <SharedSessionPage />
          </Suspense>
        ),
      },
      {
        path: 'keys',
        element: (
          <Suspense fallback={PageLoader}>
            <ProtectedRoute>
              <APIKeysPage />
            </ProtectedRoute>
          </Suspense>
        ),
      },
      {
        path: 'shares',
        element: (
          <Suspense fallback={PageLoader}>
            <ProtectedRoute>
              <ShareLinksPage />
            </ProtectedRoute>
          </Suspense>
        ),
      },
      {
        path: '*',
        element: (
          <Suspense fallback={PageLoader}>
            <NotFoundPage />
          </Suspense>
        ),
      },
    ],
  },
]);
