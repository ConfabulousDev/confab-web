import type { Preview } from '@storybook/react-vite'

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
  },
};

export default preview;