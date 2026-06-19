import type { Meta, StoryObj } from '@storybook/react';
import { MemoryRouter } from 'react-router-dom';
import CTALinks from '@/components/CTALinks';
import HeroCards from '@/components/HeroCards';
import Quickstart from '@/components/Quickstart';
import { PROVIDER_METADATA, PROVIDER_VALUES } from '@/utils/providers';
import styles from './HomePage.module.css';

// Story-only component that renders the HomePage layout without auth/routing
function HomePageLayout() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.container}>
        <div className={styles.hero}>
          <h1 className={styles.headline}>Understand your AI coding sessions</h1>
          <ul className={styles.bullets}>
            <li>Open source and self-hostable. Maintain data sovereignty.</li>
          </ul>
          <div className={styles.worksWith}>
            <span className={styles.worksWithLabel}>Works with</span>
            {PROVIDER_VALUES.map((id) => (
              <span key={id} className={styles.worksWithItem}>
                <span aria-hidden="true">{PROVIDER_METADATA[id].icon}</span>
                {PROVIDER_METADATA[id].label}
              </span>
            ))}
          </div>
        </div>
        <Quickstart variant="landing" />
        <CTALinks />
        <HeroCards />
        <CTALinks />
      </div>
    </div>
  );
}

const meta: Meta<typeof HomePageLayout> = {
  title: 'Pages/HomePage',
  component: HomePageLayout,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof HomePageLayout>;

export const Default: Story = {};
