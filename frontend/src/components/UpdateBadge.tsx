import { useAppConfig } from '@/hooks/useAppConfig';
import UpdateBadgeView from './UpdateBadgeView';

function UpdateBadge() {
  const { version } = useAppConfig();
  const show =
    version.updateAvailable &&
    !version.updateCheckDisabled &&
    !version.updateCheckFailed &&
    Boolean(version.latestUrl);

  return (
    <UpdateBadgeView
      show={show}
      current={version.current}
      latest={version.latest}
      latestUrl={version.latestUrl}
    />
  );
}

export default UpdateBadge;
