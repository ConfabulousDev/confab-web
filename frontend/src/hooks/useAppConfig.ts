import { useContext } from 'react';
import { AppConfigContext, type AppConfig } from '@/contexts/AppConfigContext';

export function useAppConfig(): AppConfig {
  return useContext(AppConfigContext);
}
