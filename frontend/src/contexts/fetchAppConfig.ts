import type { AppConfig } from './AppConfigContext';

const defaultAppConfig: AppConfig = {
  sharesEnabled: false,
  saasFooterEnabled: false,
  saasTermlyEnabled: false,
  orgAnalyticsEnabled: false,
  supportEmail: '',
};

export async function fetchConfigWithRetry(): Promise<AppConfig> {
  const maxRetries = 3;
  const baseDelay = 1000; // 1s, 2s, 4s

  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      const res = await fetch('/api/v1/auth/config');
      if (!res.ok) throw new Error('Failed to fetch config');
      const data = await res.json();
      return {
        sharesEnabled: data.features?.shares_enabled ?? false,
        saasFooterEnabled: data.features?.saas_footer_enabled ?? false,
        saasTermlyEnabled: data.features?.saas_termly_enabled ?? false,
        orgAnalyticsEnabled: data.features?.org_analytics_enabled ?? false,
        supportEmail: data.features?.support_email ?? '',
      };
    } catch {
      if (attempt < maxRetries - 1) {
        await new Promise((resolve) => setTimeout(resolve, baseDelay * Math.pow(2, attempt)));
      }
    }
  }

  // All retries exhausted — default to SaaS features off
  return defaultAppConfig;
}
