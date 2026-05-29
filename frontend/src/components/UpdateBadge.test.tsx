import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import UpdateBadgeView from './UpdateBadgeView';
import UpdateBadge from './UpdateBadge';
import {
  AppConfigContext,
  type AppConfig,
  type VersionInfo,
} from '@/contexts/AppConfigContext';
import { defaultVersionInfo } from '@/contexts/appConfigDefaults';

function withConfig(version: Partial<VersionInfo>): (props: { children: ReactNode }) => ReactNode {
  const cfg: AppConfig = {
    sharesEnabled: false,
    saasFooterEnabled: false,
    saasTermlyEnabled: false,
    orgAnalyticsEnabled: false,
    passwordAuthEnabled: false,
    smartRecapEnabled: false,
    supportEmail: '',
    version: { ...defaultVersionInfo, ...version },
  };
  return function Wrapper({ children }: { children: ReactNode }) {
    return <AppConfigContext.Provider value={cfg}>{children}</AppConfigContext.Provider>;
  };
}

describe('UpdateBadgeView', () => {
  it('renders nothing when show=false', () => {
    const { container } = render(
      <UpdateBadgeView show={false} current="v0.4.1" latest="v0.5.0" latestUrl="https://x" />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when latestUrl is missing even if show=true', () => {
    const { container } = render(
      <UpdateBadgeView show={true} current="v0.4.1" latest="v0.5.0" />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders a link with the latestUrl when shown', () => {
    render(
      <UpdateBadgeView
        show={true}
        current="v0.4.1"
        latest="v0.5.0"
        latestUrl="https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0"
      />,
    );

    const link = screen.getByRole('link', { name: /update available/i });
    expect(link).toHaveAttribute(
      'href',
      'https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0',
    );
  });

  it('opens the link in a new tab safely', () => {
    render(
      <UpdateBadgeView show={true} current="v0.4.1" latest="v0.5.0" latestUrl="https://x" />,
    );
    const link = screen.getByRole('link', { name: /update available/i });
    expect(link).toHaveAttribute('target', '_blank');
    expect(link.getAttribute('rel') ?? '').toMatch(/noopener/);
    expect(link.getAttribute('rel') ?? '').toMatch(/noreferrer/);
  });

  it('has tooltip "current → latest" when current is known', () => {
    render(
      <UpdateBadgeView show={true} current="v0.4.1" latest="v0.5.0" latestUrl="https://x" />,
    );
    const link = screen.getByRole('link', { name: /update available/i });
    expect(link).toHaveAttribute('title', 'v0.4.1 → v0.5.0');
  });

  it('has tooltip "(dev) → latest" when current is empty', () => {
    render(
      <UpdateBadgeView show={true} current="" latest="v0.5.0" latestUrl="https://x" />,
    );
    const link = screen.getByRole('link', { name: /update available/i });
    expect(link).toHaveAttribute('title', '(dev) → v0.5.0');
  });

  it('renders "Update recommended" with the red variant class when severity is recommended', () => {
    render(
      <UpdateBadgeView
        show={true}
        current="v0.4.1"
        latest="v0.5.0"
        latestUrl="https://x"
        severity="recommended"
      />,
    );
    const link = screen.getByRole('link', { name: /update recommended/i });
    expect(link.className).toMatch(/recommended/);
    expect(screen.queryByText(/update available/i)).not.toBeInTheDocument();
  });

  it('renders "Update available" without the red variant class when severity is available', () => {
    render(
      <UpdateBadgeView
        show={true}
        current="v0.4.1"
        latest="v0.4.3"
        latestUrl="https://x"
        severity="available"
      />,
    );
    const link = screen.getByRole('link', { name: /update available/i });
    expect(link.className).not.toMatch(/recommended/);
  });

  it('defaults to "Update available" (regular) when severity is undefined (older backend)', () => {
    render(
      <UpdateBadgeView show={true} current="v0.4.1" latest="v0.5.0" latestUrl="https://x" />,
    );
    const link = screen.getByRole('link', { name: /update available/i });
    expect(link.className).not.toMatch(/recommended/);
  });

  it('keeps the "current → latest" tooltip identical for the recommended tier', () => {
    render(
      <UpdateBadgeView
        show={true}
        current="v0.4.1"
        latest="v0.5.0"
        latestUrl="https://x"
        severity="recommended"
      />,
    );
    const link = screen.getByRole('link', { name: /update recommended/i });
    expect(link).toHaveAttribute('title', 'v0.4.1 → v0.5.0');
  });
});

describe('UpdateBadge container', () => {
  it('shows the pill when updateAvailable=true and no failure/disabled', () => {
    const Wrapper = withConfig({
      current: 'v0.4.1',
      latest: 'v0.5.0',
      latestUrl: 'https://x',
      updateAvailable: true,
      updateCheckDisabled: false,
      updateCheckFailed: false,
    });
    render(
      <Wrapper>
        <UpdateBadge />
      </Wrapper>,
    );
    expect(screen.getByRole('link', { name: /update available/i })).toBeInTheDocument();
  });

  it('hides when updateAvailable=false', () => {
    const Wrapper = withConfig({
      current: 'v0.5.0',
      latest: 'v0.5.0',
      latestUrl: 'https://x',
      updateAvailable: false,
    });
    const { container } = render(
      <Wrapper>
        <UpdateBadge />
      </Wrapper>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('hides when updateCheckDisabled=true even if updateAvailable=true', () => {
    const Wrapper = withConfig({
      current: 'v0.4.1',
      latest: 'v0.5.0',
      latestUrl: 'https://x',
      updateAvailable: true,
      updateCheckDisabled: true,
    });
    const { container } = render(
      <Wrapper>
        <UpdateBadge />
      </Wrapper>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('hides when updateCheckFailed=true even if updateAvailable=true', () => {
    const Wrapper = withConfig({
      current: 'v0.4.1',
      latest: 'v0.5.0',
      latestUrl: 'https://x',
      updateAvailable: true,
      updateCheckFailed: true,
    });
    const { container } = render(
      <Wrapper>
        <UpdateBadge />
      </Wrapper>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders "Update recommended" when version.updateSeverity is recommended', () => {
    const Wrapper = withConfig({
      current: 'v0.4.1',
      latest: 'v0.5.0',
      latestUrl: 'https://x',
      updateAvailable: true,
      updateSeverity: 'recommended',
      updateCheckDisabled: false,
      updateCheckFailed: false,
    });
    render(
      <Wrapper>
        <UpdateBadge />
      </Wrapper>,
    );
    const link = screen.getByRole('link', { name: /update recommended/i });
    expect(link.className).toMatch(/recommended/);
  });

  it('hides when latestUrl is missing (defensive)', () => {
    const Wrapper = withConfig({
      current: 'v0.4.1',
      latest: 'v0.5.0',
      updateAvailable: true,
    });
    const { container } = render(
      <Wrapper>
        <UpdateBadge />
      </Wrapper>,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
