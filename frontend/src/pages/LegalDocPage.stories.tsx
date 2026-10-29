import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router-dom';
import LegalDocPage from './LegalDocPage';

const meta: Meta<typeof LegalDocPage> = {
  title: 'Pages/LegalDocPage',
  component: LegalDocPage,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story, { args }) => (
      <MemoryRouter initialEntries={[`/${args.slug}`]}>
        <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof LegalDocPage>;

export const TermsOfService: Story = { args: { slug: 'terms' } };

/** Long document with a table of contents and a wide table. */
export const PrivacyNotice: Story = { args: { slug: 'privacy' } };

/** Cookie and storage tables. */
export const CookiePolicy: Story = { args: { slug: 'cookies' } };

export const Disclaimer: Story = { args: { slug: 'disclaimer' } };

export const AcceptableUsePolicy: Story = { args: { slug: 'acceptable-use' } };
