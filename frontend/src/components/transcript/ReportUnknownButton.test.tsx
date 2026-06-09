import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import ReportUnknownButton from './ReportUnknownButton';
import { AppConfigContext, type AppConfig } from '@/contexts/AppConfigContext';
import { defaultAppConfig } from '@/contexts/appConfigDefaults';
import type { UnknownDescriptor } from '@/utils/reportUnknown';

function withConfig(current: string): (props: { children: ReactNode }) => ReactNode {
  const cfg: AppConfig = {
    ...defaultAppConfig,
    version: { ...defaultAppConfig.version, current },
  };
  return function Wrapper({ children }: { children: ReactNode }) {
    return <AppConfigContext.Provider value={cfg}>{children}</AppConfigContext.Provider>;
  };
}

const descriptor: UnknownDescriptor = {
  provider: 'claude',
  surface: 'message',
  type: 'mystery_type',
  keyFingerprint: ['type', 'mystery'],
};

describe('ReportUnknownButton', () => {
  it('renders a new-tab link to GitHub issues/new', () => {
    render(<ReportUnknownButton descriptor={descriptor} />, { wrapper: withConfig('1.0.0') });

    const link = screen.getByRole('link', { name: /report this message/i });
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link.getAttribute('href')).toContain('/issues/new');
  });

  it('encodes the unrecognized type into the prefilled issue', () => {
    render(<ReportUnknownButton descriptor={descriptor} />, { wrapper: withConfig('1.0.0') });

    const href = screen.getByRole('link', { name: /report this message/i }).getAttribute('href') ?? '';
    expect(href).toContain(encodeURIComponent('mystery_type'));
  });

  it('includes the Confab app version from context in the prefilled body', () => {
    render(<ReportUnknownButton descriptor={descriptor} />, { wrapper: withConfig('9.9.9') });

    const href = screen.getByRole('link', { name: /report this message/i }).getAttribute('href') ?? '';
    const body = new URL(href).searchParams.get('body') ?? '';
    expect(body).toContain('9.9.9');
  });
});
