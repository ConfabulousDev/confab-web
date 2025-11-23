import { createBrowserRouter } from 'react-router-dom';
import App from './App';
import HomePage from '@/pages/HomePage';
import SessionsPage from '@/pages/SessionsPage';
import SessionDetailPage from '@/pages/SessionDetailPage';
import SharedSessionPage from '@/pages/SharedSessionPage';
import APIKeysPage from '@/pages/APIKeysPage';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      {
        index: true,
        element: <HomePage />,
      },
      {
        path: 'sessions',
        element: <SessionsPage />,
      },
      {
        path: 'sessions/:id',
        element: <SessionDetailPage />,
      },
      {
        path: 'sessions/:sessionId/shared/:token',
        element: <SharedSessionPage />,
      },
      {
        path: 'keys',
        element: <APIKeysPage />,
      },
    ],
  },
]);
