import { lazy, Suspense } from 'react';
import { createBrowserRouter } from 'react-router-dom';
import App from './App';
import LoadingSkeleton from '@/components/LoadingSkeleton';

// Lazy load pages for code splitting
const HomePage = lazy(() => import('@/pages/HomePage'));
const SessionsPage = lazy(() => import('@/pages/SessionsPage'));
const SessionDetailPage = lazy(() => import('@/pages/SessionDetailPage'));
const SharedSessionPage = lazy(() => import('@/pages/SharedSessionPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));

// Loading fallback component
function PageLoader() {
  return (
    <div style={{ padding: '2rem' }}>
      <LoadingSkeleton variant="list" count={5} />
    </div>
  );
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      {
        index: true,
        element: (
          <Suspense fallback={<PageLoader />}>
            <HomePage />
          </Suspense>
        ),
      },
      {
        path: 'sessions',
        element: (
          <Suspense fallback={<PageLoader />}>
            <SessionsPage />
          </Suspense>
        ),
      },
      {
        path: 'sessions/:id',
        element: (
          <Suspense fallback={<PageLoader />}>
            <SessionDetailPage />
          </Suspense>
        ),
      },
      {
        path: 'sessions/:sessionId/shared/:token',
        element: (
          <Suspense fallback={<PageLoader />}>
            <SharedSessionPage />
          </Suspense>
        ),
      },
      {
        path: 'keys',
        element: (
          <Suspense fallback={<PageLoader />}>
            <APIKeysPage />
          </Suspense>
        ),
      },
    ],
  },
]);
