import { Outlet } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AppConfigProvider } from '@/contexts/AppConfigContext';
import { useAppConfig } from '@/hooks/useAppConfig';
import { KeyboardShortcutProvider } from '@/contexts/KeyboardShortcutContext';
import { ThemeProvider } from '@/contexts/ThemeContext';
import ErrorBoundary from '@/components/ErrorBoundary';
import DemoBanner from '@/components/DemoBanner';
import ReadOnlyToast from '@/components/ReadOnlyToast';
import Header from '@/components/Header';
import Footer from '@/components/Footer';
import './index.css';

// Create a client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes
    },
  },
});

function AppLayout() {
  const { saasFooterEnabled } = useAppConfig();

  return (
    <div className="app-container">
      {/* CF-483: above the nav, not dismissible, only rendered when
          window.__DEMO_IDENTITY__ is set. */}
      <DemoBanner />
      <Header />
      <main>
        <Outlet />
      </main>
      {saasFooterEnabled && <Footer />}
      <ReadOnlyToast />
    </div>
  );
}

function App() {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <AppConfigProvider>
          <QueryClientProvider client={queryClient}>
            <KeyboardShortcutProvider>
              <AppLayout />
            </KeyboardShortcutProvider>
          </QueryClientProvider>
        </AppConfigProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}

export default App;
