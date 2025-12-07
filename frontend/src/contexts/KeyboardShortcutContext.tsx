import { createContext, useCallback, useEffect, useRef, type ReactNode } from 'react';

type ShortcutHandler = (event: KeyboardEvent) => void;

interface KeyboardShortcutContextValue {
  registerShortcut: (shortcut: string, handler: ShortcutHandler) => void;
  unregisterShortcut: (shortcut: string) => void;
}

const KeyboardShortcutContext = createContext<KeyboardShortcutContextValue | null>(null);

/**
 * Normalize a keyboard shortcut string to a canonical format.
 *
 * Supported modifiers:
 * - `mod` = Cmd on Mac, Ctrl on Windows/Linux
 * - `ctrl`, `alt`, `shift`, `meta`
 *
 * Examples: 'mod+k', 'mod+shift+e', 'escape', 'ctrl+alt+delete'
 */
function normalizeShortcut(shortcut: string): string {
  return shortcut
    .toLowerCase()
    .split('+')
    .map((part) => part.trim())
    .sort()
    .join('+');
}

/**
 * Check if a keyboard event matches a shortcut string.
 */
function eventMatchesShortcut(event: KeyboardEvent, shortcut: string): boolean {
  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const parts = shortcut.toLowerCase().split('+').map((p) => p.trim());

  // Extract modifiers and key from the shortcut
  const modifiers = {
    ctrl: false,
    alt: false,
    shift: false,
    meta: false,
    mod: false,
  };
  let key = '';

  for (const part of parts) {
    if (part === 'ctrl') modifiers.ctrl = true;
    else if (part === 'alt') modifiers.alt = true;
    else if (part === 'shift') modifiers.shift = true;
    else if (part === 'meta') modifiers.meta = true;
    else if (part === 'mod') modifiers.mod = true;
    else key = part;
  }

  // Check if the event key matches
  const eventKey = event.key.toLowerCase();
  if (eventKey !== key) return false;

  // Handle 'mod' modifier (Cmd on Mac, Ctrl elsewhere)
  if (modifiers.mod) {
    if (isMac) {
      if (!event.metaKey) return false;
    } else {
      if (!event.ctrlKey) return false;
    }
  }

  // Check explicit modifiers
  if (modifiers.ctrl && !event.ctrlKey) return false;
  if (modifiers.alt && !event.altKey) return false;
  if (modifiers.shift && !event.shiftKey) return false;
  if (modifiers.meta && !event.metaKey) return false;

  // Ensure no extra modifiers are pressed (except for 'mod' which we already handled)
  if (!modifiers.mod && !modifiers.ctrl && event.ctrlKey) return false;
  if (!modifiers.alt && event.altKey) return false;
  if (!modifiers.shift && event.shiftKey) return false;
  if (!modifiers.mod && !modifiers.meta && event.metaKey) return false;

  return true;
}

interface KeyboardShortcutProviderProps {
  children: ReactNode;
}

export function KeyboardShortcutProvider({ children }: KeyboardShortcutProviderProps) {
  const shortcutsRef = useRef<Map<string, ShortcutHandler>>(new Map());

  const registerShortcut = useCallback((shortcut: string, handler: ShortcutHandler) => {
    const normalized = normalizeShortcut(shortcut);
    shortcutsRef.current.set(normalized, handler);
  }, []);

  const unregisterShortcut = useCallback((shortcut: string) => {
    const normalized = normalizeShortcut(shortcut);
    shortcutsRef.current.delete(normalized);
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Don't trigger shortcuts when typing in inputs
      const target = event.target;
      if (!(target instanceof HTMLElement)) {
        return;
      }
      if (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable
      ) {
        return;
      }

      for (const [shortcut, handler] of shortcutsRef.current) {
        if (eventMatchesShortcut(event, shortcut)) {
          event.preventDefault();
          handler(event);
          return;
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <KeyboardShortcutContext.Provider value={{ registerShortcut, unregisterShortcut }}>
      {children}
    </KeyboardShortcutContext.Provider>
  );
}

// Exported for use by useKeyboardShortcut hook
export { KeyboardShortcutContext };
export type { KeyboardShortcutContextValue };
