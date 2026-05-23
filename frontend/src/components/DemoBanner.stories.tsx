import type { Meta, StoryObj } from '@storybook/react-vite';
import { useEffect } from 'react';
import DemoBanner from './DemoBanner';

// CF-483: stories cover the two interesting states: shown (global set)
// and hidden (global unset). The "shown" story stamps the window global
// inside an effect because the banner reads it on render. The augmented
// Window type for __DEMO_IDENTITY__ is declared by utils/demoIdentity.ts.

const meta: Meta<typeof DemoBanner> = {
  title: 'Components/DemoBanner',
  component: DemoBanner,
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj<typeof DemoBanner>;

function WithDemoIdentity({ email }: { email: string }) {
  useEffect(() => {
    window.__DEMO_IDENTITY__ = email;
    return () => {
      delete window.__DEMO_IDENTITY__;
    };
  }, [email]);
  return <DemoBanner />;
}

export const Shown: Story = {
  render: () => <WithDemoIdentity email="demo@confabulous.dev" />,
};

export const Hidden: Story = {
  render: () => <DemoBanner />,
};
