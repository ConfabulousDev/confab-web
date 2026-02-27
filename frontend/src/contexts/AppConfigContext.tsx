import { createContext, useEffect, useState, type ReactNode } from 'react';
import { fetchConfigWithRetry } from './fetchAppConfig';

export interface AppConfig {
  sharesEnabled: boolean;
  saasFooterEnabled: boolean;
  saasTermlyEnabled: boolean;
  orgAnalyticsEnabled: boolean;
  supportEmail: string;
}

const defaultAppConfig: AppConfig = {
  sharesEnabled: false,
  saasFooterEnabled: false,
  saasTermlyEnabled: false,
  orgAnalyticsEnabled: false,
  supportEmail: '',
};

const AppConfigContext = createContext<AppConfig>(defaultAppConfig);

interface AppConfigProviderProps {
  children: ReactNode;
}

export function AppConfigProvider({ children }: AppConfigProviderProps) {
  const [config, setConfig] = useState<AppConfig>(defaultAppConfig);

  useEffect(() => {
    fetchConfigWithRetry().then(setConfig);
  }, []);

  return (
    <AppConfigContext.Provider value={config}>
      {children}
    </AppConfigContext.Provider>
  );
}

export { AppConfigContext };
