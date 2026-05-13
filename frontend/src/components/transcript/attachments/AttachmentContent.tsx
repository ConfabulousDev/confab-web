import type { AttachmentMessage } from '@/types';
import {
  isHookSuccessAttachment,
  isHookBlockingErrorAttachment,
  isEditedTextFileAttachment,
  isQueuedCommandAttachment,
  isDeferredToolsDeltaAttachment,
  isMcpInstructionsDeltaAttachment,
} from '@/types';
import { HookSuccessOutput, HookBlockingError } from './HookOutput';
import EditedFileSnippet from './EditedFileSnippet';
import QueuedCommand from './QueuedCommand';
import ToolDelta from './ToolDelta';

interface AttachmentContentProps {
  message: AttachmentMessage;
}

/**
 * Dispatch component: routes an attachment message to the appropriate renderer
 * based on `attachment.type`. Each discriminator narrows the inner attachment
 * to the matching branch type. Returns null for noisy or unknown subtypes —
 * the categorizer should already prevent those from reaching here (no filter
 * chip routes to them), but this is defense in depth.
 */
export default function AttachmentContent({ message }: AttachmentContentProps) {
  if (isHookSuccessAttachment(message)) {
    return <HookSuccessOutput attachment={message.attachment} />;
  }
  if (isHookBlockingErrorAttachment(message)) {
    return <HookBlockingError attachment={message.attachment} />;
  }
  if (isEditedTextFileAttachment(message)) {
    return <EditedFileSnippet attachment={message.attachment} />;
  }
  if (isQueuedCommandAttachment(message)) {
    return <QueuedCommand attachment={message.attachment} />;
  }
  if (isDeferredToolsDeltaAttachment(message) || isMcpInstructionsDeltaAttachment(message)) {
    return <ToolDelta attachment={message.attachment} />;
  }
  return null;
}
