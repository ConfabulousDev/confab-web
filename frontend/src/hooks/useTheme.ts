import { useContext } from 'react';
import { ThemeContext, type ThemeMode, type ResolvedTheme } from '@/contexts/ThemeContext';

interface UseThemeReturn {
  mode: ThemeMode;
  resolvedTheme: ResolvedTheme;
  setMode: (mode: ThemeMode) => void;
  toggleMode: () => void;
}

export function useTheme(): UseThemeReturn {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }

  const toggleMode = () => {
    const modes: ThemeMode[] = ['system', 'light', 'dark'];
    const currentIndex = modes.indexOf(context.mode);
    const nextIndex = (currentIndex + 1) % modes.length;
    const nextMode = modes[nextIndex];
    if (nextMode) {
      context.setMode(nextMode);
    }
  };

  return {
    ...context,
    toggleMode,
  };
}
