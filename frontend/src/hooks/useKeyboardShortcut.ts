import { useContext, useEffect } from 'react';
import {
  KeyboardShortcutContext,
  type KeyboardShortcutContextValue,
} from '@/contexts/KeyboardShortcutContext';

function useKeyboardShortcutContext(): KeyboardShortcutContextValue {
  const context = useContext(KeyboardShortcutContext);
  if (!context) {
    throw new Error('useKeyboardShortcut must be used within a KeyboardShortcutProvider');
  }
  return context;
}

/**
 * Register a keyboard shortcut that will be active while the component is mounted.
 *
 * @param shortcut - The shortcut string (e.g., 'mod+k', 'mod+shift+e', 'escape')
 *   - Use 'mod' for Cmd on Mac or Ctrl on Windows/Linux
 *   - Modifiers: ctrl, alt, shift, meta, mod
 * @param handler - The callback to invoke when the shortcut is pressed
 *
 * @example
 * useKeyboardShortcut('mod+shift+e', () => {
 *   toggleShowEmptySessions();
 * });
 */
export function useKeyboardShortcut(
  shortcut: string,
  handler: (event: KeyboardEvent) => void
): void {
  const { registerShortcut, unregisterShortcut } = useKeyboardShortcutContext();

  useEffect(() => {
    registerShortcut(shortcut, handler);
    return () => unregisterShortcut(shortcut);
  }, [shortcut, handler, registerShortcut, unregisterShortcut]);
}
