import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import LegalDocPage from './LegalDocPage';
import { LEGAL_DOCS, type LegalSlug } from '@/legal/docs';

function renderPage(slug: LegalSlug, entry = `/${slug}`) {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <LegalDocPage slug={slug} />
    </MemoryRouter>
  );
}

describe('LegalDocPage', () => {
  it.each(LEGAL_DOCS.map((d) => [d.slug, d.title] as const))(
    'renders the %s title as the page heading',
    (slug, title) => {
      renderPage(slug);
      expect(screen.getByRole('heading', { level: 1, name: title })).toBeInTheDocument();
    }
  );

  it('shows the Last updated date from the registry', () => {
    renderPage('terms');
    expect(screen.getByText(/Last updated/)).toHaveTextContent('Last updated January 9, 2026');
  });

  it('sets the document title', () => {
    renderPage('privacy');
    expect(document.title).toBe('Privacy Notice | Confabulous');
  });

  it('renders the policy body with heading anchors', () => {
    const { container } = renderPage('privacy');
    expect(container.querySelector('h2[id]')).not.toBeNull();
  });

  describe('deep links', () => {
    const scrollIntoView = vi.fn();
    beforeEach(() => {
      Element.prototype.scrollIntoView = scrollIntoView;
    });
    afterEach(() => {
      scrollIntoView.mockReset();
    });

    it('scrolls to the heading named in the URL hash', () => {
      const { container } = renderPage('privacy', '/privacy#table-of-contents');
      const target = container.querySelector('#table-of-contents');
      expect(target).not.toBeNull();
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
      expect(scrollIntoView.mock.contexts[0]).toBe(target);
    });

    it('does not scroll when the hash matches nothing', () => {
      renderPage('privacy', '/privacy#no-such-section');
      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('ignores a malformed percent-escape in the hash', () => {
      renderPage('privacy', '/privacy#%E0%A4%A');
      expect(screen.getByRole('heading', { level: 1, name: 'Privacy Notice' })).toBeInTheDocument();
      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('does not scroll without a hash', () => {
      renderPage('privacy');
      expect(scrollIntoView).not.toHaveBeenCalled();
    });
  });

  describe('links to other policies', () => {
    function renderWithRoutes() {
      return render(
        <MemoryRouter initialEntries={['/privacy']}>
          <Routes>
            <Route path="/privacy" element={<LegalDocPage slug="privacy" />} />
            <Route path="/cookies" element={<p>cookies route</p>} />
          </Routes>
        </MemoryRouter>
      );
    }

    it('navigates client-side on a plain click', () => {
      const { container } = renderWithRoutes();
      const link = container.querySelector('a[href="/cookies"]')!;
      expect(link).not.toBeNull();
      fireEvent.click(link);
      expect(screen.getByText('cookies route')).toBeInTheDocument();
    });

    it('leaves modified clicks (open in new tab) to the browser', () => {
      const { container } = renderWithRoutes();
      fireEvent.click(container.querySelector('a[href="/cookies"]')!, { metaKey: true });
      expect(screen.queryByText('cookies route')).toBeNull();
    });
  });
});
