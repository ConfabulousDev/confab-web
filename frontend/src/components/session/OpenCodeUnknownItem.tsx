// CF-574: forward-compat fallback row for unrecognized OpenCode shapes (unknown
// message role, unrecognized part type, or a malformed line). Built on the
// shared UnknownRawDetails shell with a "Report bug" affordance, so a
// new OpenCode message type surfaces and can be reported instead of being
// silently dropped.

import { useMemo } from 'react';
import { computeKeyFingerprint } from '@/utils/reportUnknown';
import ReportUnknownButton from '@/components/transcript/ReportUnknownButton';
import UnknownRawDetails from '@/components/transcript/UnknownRawDetails';
import type { OpenCodeRenderItem } from './opencodeCategories';

type OpenCodeUnknownItemType = Extract<OpenCodeRenderItem, { kind: 'unknown' }>;

function stringifyRaw(value: unknown): string {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export default function OpenCodeUnknownItem({ item }: { item: OpenCodeUnknownItemType }) {
  const raw = useMemo(() => stringifyRaw(item.rawLine), [item.rawLine]);

  return (
    <UnknownRawDetails
      label="Unrecognized line"
      rawText={raw}
      summaryAside={<span>{item.reason}</span>}
      actions={
        <ReportUnknownButton
          descriptor={{
            provider: 'opencode',
            surface: 'line',
            type: item.unrecognizedType,
            reason: item.reason,
            keyFingerprint: computeKeyFingerprint(item.rawLine),
          }}
        />
      }
    />
  );
}
