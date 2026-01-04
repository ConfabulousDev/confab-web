import type { Preview } from '@storybook/react-vite'
import { withThemeByDataAttribute } from '@storybook/addon-themes'
import { ThemeProvider } from '../src/contexts/ThemeContext'

// Import global styles so components render correctly
import '../src/styles/variables.css'
import '../src/index.css'

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
       color: /(background|color)$/i,
       date: /Date$/i,
      },
    },
    backgrounds: { disable: true },
  },
  decorators: [
    withThemeByDataAttribute({
      themes: {
        light: 'light',
        dark: 'dark',
      },
      defaultTheme: 'light',
      attributeName: 'data-theme',
    }),
    // Wrapper to apply themed background and provide ThemeContext
    (Story) => (
      <ThemeProvider>
        <div style={{
          backgroundColor: 'var(--color-bg)',
          minHeight: '100vh',
          padding: '1rem'
        }}>
          <Story />
        </div>
      </ThemeProvider>
    ),
  ],
};

export default preview;