import type { AppConfig, VersionInfo } from './AppConfigContext';

export const defaultVersionInfo: VersionInfo = {
  current: '',
  updateAvailable: false,
  updateCheckDisabled: true,
  updateCheckFailed: false,
};

export const defaultAppConfig: AppConfig = {
  sharesEnabled: false,
  saasFooterEnabled: false,
  saasTermlyEnabled: false,
  orgAnalyticsEnabled: false,
  passwordAuthEnabled: false,
  smartRecapEnabled: false,
  supportEmail: '',
  version: defaultVersionInfo,
};
