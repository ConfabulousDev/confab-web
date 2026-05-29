import { createContext, useEffect, useState, type ReactNode } from 'react';
import { fetchConfigWithRetry } from './fetchAppConfig';
import { fetchPricing } from './fetchPricing';
import { defaultAppConfig } from './appConfigDefaults';

export interface VersionInfo {
  current: string;
  latest?: string;
  latestUrl?: string;
  updateAvailable: boolean;
  updateSeverity?: 'available' | 'recommended';
  updateCheckDisabled: boolean;
  updateCheckFailed: boolean;
}

export interface AppConfig {
  sharesEnabled: boolean;
  saasFooterEnabled: boolean;
  saasTermlyEnabled: boolean;
  orgAnalyticsEnabled: boolean;
  passwordAuthEnabled: boolean;
  smartRecapEnabled: boolean;
  supportEmail: string;
  version: VersionInfo;
}

const AppConfigContext = createContext<AppConfig>(defaultAppConfig);

interface AppConfigProviderProps {
  children: ReactNode;
}

export function AppConfigProvider({ children }: AppConfigProviderProps) {
  const [config, setConfig] = useState<AppConfig>(defaultAppConfig);

  useEffect(() => {
    fetchConfigWithRetry().then(setConfig);
    // Install the model price table from our backend, concurrently. Not gated:
    // cost UI renders well after this resolves (CF-515).
    void fetchPricing();
  }, []);

  return (
    <AppConfigContext.Provider value={config}>
      {children}
    </AppConfigContext.Provider>
  );
}

export { AppConfigContext };
