import { useContext } from 'react';
import { ThemeContext, type Theme } from '@/contexts/ThemeContext';

interface UseThemeReturn {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
  /** @deprecated Use theme instead */
  resolvedTheme: Theme;
}

export function useTheme(): UseThemeReturn {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }

  const toggleTheme = () => {
    context.setTheme(context.theme === 'light' ? 'dark' : 'light');
  };

  return {
    ...context,
    toggleTheme,
    // Keep resolvedTheme for backwards compatibility with ThemedImage
    resolvedTheme: context.theme,
  };
}
