// CF-574: shared "Report bug" affordance for UNKNOWN transcript rows.
// Opens GitHub's prefilled New Issue form in a new tab with privacy-conscious
// structural metadata only (provider, type, key names, Confab version) — never
// message content. All redaction lives in `reportUnknown.ts`.
import { useAppConfig } from '@/hooks';
import { buildUnknownReportUrl, type UnknownDescriptor } from '@/utils/reportUnknown';
import styles from './ReportUnknownButton.module.css';

interface ReportUnknownButtonProps {
  descriptor: UnknownDescriptor;
}

export default function ReportUnknownButton({ descriptor }: ReportUnknownButtonProps) {
  const { version } = useAppConfig();
  const href = buildUnknownReportUrl(descriptor, version.current);

  return (
    <a
      className={styles.report}
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      title="Report this unrecognized row as a bug on GitHub (no message content is shared)"
    >
      <svg
        width="13"
        height="13"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M3.5 14.5V2.5" />
        <path d="M3.5 3c2.5-1.5 5 1.5 7.5 0v6c-2.5 1.5-5-1.5-7.5 0" />
      </svg>
      <span>Report bug</span>
    </a>
  );
}
