import type { Meta, StoryObj } from '@storybook/react';
import HeroCards from '@/components/HeroCards';
import styles from './HomePage.module.css';

// Story-only component that renders the HomePage layout without auth/routing
function HomePageLayout() {
  return (
    <div className={styles.wrapper}>
      <div className={styles.container}>
        <div className={styles.hero}>
          <h1 className={styles.headline}>Understand your Claude Code sessions</h1>
        </div>
        <HeroCards />
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
};

export default meta;
type Story = StoryObj<typeof HomePageLayout>;

export const Default: Story = {};
