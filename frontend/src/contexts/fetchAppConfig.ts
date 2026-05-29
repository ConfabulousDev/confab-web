import type { AppConfig } from './AppConfigContext';
import { defaultAppConfig, defaultVersionInfo } from './appConfigDefaults';

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
        passwordAuthEnabled: data.features?.password_auth_enabled ?? false,
        smartRecapEnabled: data.features?.smart_recap_enabled ?? false,
        supportEmail: data.features?.support_email ?? '',
        version: data.version
          ? {
              current: data.version.current ?? '',
              latest: data.version.latest,
              latestUrl: data.version.latest_url,
              updateAvailable: data.version.update_available ?? false,
              updateSeverity: data.version.update_severity,
              updateCheckDisabled: data.version.update_check_disabled ?? false,
              updateCheckFailed: data.version.update_check_failed ?? false,
            }
          : defaultVersionInfo,
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
