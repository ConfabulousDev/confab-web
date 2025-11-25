import { useState, useCallback } from 'react';

interface UseCopyToClipboardOptions {
  /** Success message to display (if not provided, no message state is managed) */
  successMessage?: string;
  /** Duration in ms to show success message (default: 3000) */
  messageDuration?: number;
}

interface UseCopyToClipboardReturn {
  /** Copy text to clipboard */
  copy: (text: string) => Promise<void>;
  /** Whether copy was recently successful (resets after messageDuration) */
  copied: boolean;
  /** Success message if provided in options */
  message: string | null;
}

/**
 * Hook for copying text to clipboard with optional success feedback
 */
export function useCopyToClipboard(
  options: UseCopyToClipboardOptions = {}
): UseCopyToClipboardReturn {
  const { successMessage, messageDuration = 3000 } = options;
  const [copied, setCopied] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  const copy = useCallback(
    async (text: string) => {
      await navigator.clipboard.writeText(text);
      setCopied(true);

      if (successMessage) {
        setMessage(successMessage);
      }

      setTimeout(() => {
        setCopied(false);
        setMessage(null);
      }, messageDuration);
    },
    [successMessage, messageDuration]
  );

  return { copy, copied, message };
}
