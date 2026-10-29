import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, Outlet, RouterProvider } from 'react-router-dom';
import { AppConfigContext } from '@/contexts/AppConfigContext';
import { defaultAppConfig } from '@/contexts/appConfigDefaults';
import { LEGAL_DOCS } from '@/legal/docs';

// The real App shell pulls in auth, theme and data providers; the legal routes
// only need the outlet.
vi.mock('./App', () => ({ default: () => <Outlet /> }));

const { routes } = await import('./router');

function renderAt(path: string, saasFooterEnabled: boolean) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  return render(
    <AppConfigContext.Provider value={{ ...defaultAppConfig, saasFooterEnabled }}>
      <RouterProvider router={router} />
    </AppConfigContext.Provider>
  );
}

describe('legal routes', () => {
  it.each(LEGAL_DOCS.map((d) => [d.slug, d.title] as const))(
    '/%s renders its policy page when the SaaS footer is enabled',
    async (slug, title) => {
      renderAt(`/${slug}`, true);
      expect(await screen.findByRole('heading', { level: 1, name: title })).toBeInTheDocument();
    }
  );

  it.each(LEGAL_DOCS.map((d) => [d.slug] as const))(
    '/%s renders Not Found on a self-hosted instance',
    async (slug) => {
      renderAt(`/${slug}`, false);
      expect(await screen.findByText('Page Not Found')).toBeInTheDocument();
    }
  );

  it('/policies lists the policies when the SaaS footer is enabled', async () => {
    renderAt('/policies', true);
    expect(await screen.findByRole('heading', { level: 1, name: 'Policies' })).toBeInTheDocument();
  });

  it('/policies renders Not Found on a self-hosted instance', async () => {
    renderAt('/policies', false);
    expect(await screen.findByText('Page Not Found')).toBeInTheDocument();
  });
});
