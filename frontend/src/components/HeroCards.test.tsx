import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import HeroCards from './HeroCards';

const ALL_TITLES = [
  'Quickstart',
  'Analytics',
  'Org cost metrics',
  'Smart Recap',
  'Review',
  'Multi-provider support',
  'Retro',
  'PR Linking',
  'Share',
  'Self-Hosted',
  'How it works',
] as const;

const NO_DEMO_TITLES = ['Quickstart', 'Retro', 'Share', 'Self-Hosted', 'How it works'] as const;

function linkPosition(ariaLabel: string): number {
  const all = screen.getAllByRole('link');
  const i = all.findIndex((el) => el.getAttribute('aria-label') === ariaLabel);
  if (i < 0) throw new Error(`No link with aria-label "${ariaLabel}"`);
  return i;
}

describe('HeroCards', () => {
  it('renders all 11 cards as h3 headings', () => {
    render(<HeroCards />);
    for (const title of ALL_TITLES) {
      expect(screen.getByRole('heading', { name: title, level: 3 })).toBeInTheDocument();
    }
  });

  it('opens every link in a new tab with safe rel attrs', () => {
    render(<HeroCards />);
    const links = screen.getAllByRole('link');
    expect(links.length).toBeGreaterThan(0);
    for (const link of links) {
      expect(link).toHaveAttribute('target', '_blank');
      const rel = link.getAttribute('rel') ?? '';
      expect(rel).toContain('noopener');
      expect(rel).toContain('noreferrer');
    }
  });

  it('every card except Multi-provider has a Docs link with {title}: Docs aria-label', () => {
    render(<HeroCards />);
    for (const title of ALL_TITLES) {
      if (title === 'Multi-provider support') continue;
      expect(screen.getByRole('link', { name: `${title}: Docs` })).toBeInTheDocument();
    }
  });

  it('cards without a demo target render no Demo link', () => {
    render(<HeroCards />);
    for (const title of NO_DEMO_TITLES) {
      expect(screen.queryByRole('link', { name: `${title}: Demo` })).not.toBeInTheDocument();
    }
  });

  it('cards with a demo target render the Demo link before the Docs link', () => {
    render(<HeroCards />);
    const noDemoSet: ReadonlySet<string> = new Set(NO_DEMO_TITLES);
    const titlesWithDemo = ALL_TITLES.filter(
      (t) => !noDemoSet.has(t) && t !== 'Multi-provider support'
    );
    for (const title of titlesWithDemo) {
      expect(screen.getByRole('link', { name: `${title}: Demo` })).toBeInTheDocument();
      expect(linkPosition(`${title}: Demo`)).toBeLessThan(linkPosition(`${title}: Docs`));
    }
  });

  it('Multi-provider card renders Demo, Claude Code, Codex, OpenCode, Cursor docs in that order', () => {
    render(<HeroCards />);
    const demo = linkPosition('Multi-provider support: Demo');
    const claudeCode = linkPosition('Multi-provider support: Claude Code docs');
    const codex = linkPosition('Multi-provider support: Codex docs');
    const openCode = linkPosition('Multi-provider support: OpenCode docs');
    const cursor = linkPosition('Multi-provider support: Cursor docs');
    expect(demo).toBeLessThan(claudeCode);
    expect(claudeCode).toBeLessThan(codex);
    expect(codex).toBeLessThan(openCode);
    expect(openCode).toBeLessThan(cursor);
  });

  it('cards are not themselves buttons (no card-level role=button)', () => {
    render(<HeroCards />);
    expect(screen.queryAllByRole('button')).toHaveLength(0);
  });
});
