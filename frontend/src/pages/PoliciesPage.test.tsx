import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import PoliciesPage from './PoliciesPage';

describe('PoliciesPage', () => {
  it('links to each policy as an internal page', () => {
    render(
      <MemoryRouter>
        <PoliciesPage />
      </MemoryRouter>
    );
    const links = screen.getAllByRole('link');
    expect(links.map((a) => a.getAttribute('href'))).toEqual([
      '/terms',
      '/privacy',
      '/cookies',
      '/disclaimer',
      '/acceptable-use',
    ]);
    for (const link of links) {
      expect(link).not.toHaveAttribute('target');
    }
    expect(screen.getByRole('link', { name: /Cookie Policy/ })).toHaveAttribute('href', '/cookies');
  });
});
