import { createContext, useEffect, useState, type ReactNode } from 'react';

export interface AppConfig {
  sharesEnabled: boolean;
  footerEnabled: boolean;
}

const defaultAppConfig: AppConfig = {
  sharesEnabled: true,
  footerEnabled: true,
};

const AppConfigContext = createContext<AppConfig>(defaultAppConfig);

interface AppConfigProviderProps {
  children: ReactNode;
}

export function AppConfigProvider({ children }: AppConfigProviderProps) {
  const [config, setConfig] = useState<AppConfig>(defaultAppConfig);

  useEffect(() => {
    fetch('/api/v1/auth/config')
      .then((res) => {
        if (!res.ok) throw new Error('Failed to fetch config');
        return res.json();
      })
      .then((data) => {
        setConfig({
          sharesEnabled: data.features?.shares_enabled ?? true,
          footerEnabled: data.features?.footer_enabled ?? true,
        });
      })
      .catch(() => {
        // On error, keep defaults (shares enabled) for safe fallback
      });
  }, []);

  return (
    <AppConfigContext.Provider value={config}>
      {children}
    </AppConfigContext.Provider>
  );
}

export { AppConfigContext };
