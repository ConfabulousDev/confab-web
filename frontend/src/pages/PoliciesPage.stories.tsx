import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router-dom';
import PoliciesPage from './PoliciesPage';

const meta: Meta<typeof PoliciesPage> = {
  title: 'Pages/PoliciesPage',
  component: PoliciesPage,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <MemoryRouter initialEntries={['/policies']}>
        <Story />
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof PoliciesPage>;

export const Default: Story = {};
