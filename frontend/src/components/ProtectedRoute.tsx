import { useLocation, Navigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useServerRecovery } from '@/hooks/useServerRecovery';
import ServerError from '@/components/ServerError';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

/**
 * Wrapper component that protects routes requiring authentication.
 * - Shows nothing while auth is loading (prevents flash of content)
 * - Shows server error UI when backend is unreachable (5xx/network errors)
 * - Redirects to login page if not authenticated, preserving intended destination
 * - Renders children if authenticated
 */
function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { loading, isAuthenticated, serverError, error, refetch } = useAuth();
  const location = useLocation();

  // Invalidate data caches when server recovers
  useServerRecovery(serverError);

  // Show nothing while loading
  if (loading) {
    return null;
  }

  // Server unreachable — show error with auto-retry, don't redirect to login
  if (serverError) {
    return <ServerError message={error} onRetry={refetch} />;
  }

  // Not authenticated (401) — redirect to login
  if (!isAuthenticated) {
    const intendedPath = location.pathname + location.search;
    return <Navigate to={`/login?redirect=${encodeURIComponent(intendedPath)}`} replace />;
  }

  return <>{children}</>;
}

export default ProtectedRoute;
