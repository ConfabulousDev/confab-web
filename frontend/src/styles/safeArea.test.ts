// Guards the mobile Safari safe-area / dynamic-viewport handling (46zc).
// jsdom can't evaluate env() or dvh, so these tests check the CSS source.
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Vitest stubs CSS imports (even `?raw`), so read the sources with fs, resolved
// from the Vitest root (`frontend/`), as tokenStats.test.ts does.
const read = (rel: string) => readFileSync(resolve(process.cwd(), rel), 'utf-8');

const indexHtml = read('index.html');
const variablesCss = read('src/styles/variables.css');
const indexCss = read('src/index.css');
const headerCss = read('src/components/Header.module.css');
const scrollNavCss = read('src/components/ScrollNavButtons.module.css');
const searchBarCss = read('src/components/session/TranscriptSearchBar.module.css');
const toastCss = read('src/components/ReadOnlyToast.module.css');
const modalCss = read('src/components/Modal.module.css');
const shareDialogCss = read('src/components/ShareDialog.module.css');
const sessionDetailCss = read('src/pages/SessionDetailPage.module.css');
const errorBoundaryCss = read('src/components/ErrorBoundary.module.css');
const serverErrorCss = read('src/components/ServerError.module.css');

/** Body of the first top-level rule whose selector list is exactly `selector`. */
function ruleBody(css: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = new RegExp(`(?:^|\\n)${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  if (!match) throw new Error(`rule "${selector}" not found`);
  return match[1]!;
}

/** Values of every `prop:` declaration in a rule body, in source order. */
function declValues(body: string, prop: string): string[] {
  const re = new RegExp(`(?:^|[;\\s])${prop}\\s*:\\s*([^;]+);`, 'g');
  return [...body.matchAll(re)].map((m) => m[1]!.trim());
}

describe('safe-area handling', () => {
  it('opts the viewport into viewport-fit=cover so env(safe-area-inset-*) is non-zero', () => {
    const meta = /<meta name="viewport" content="([^"]*)"/.exec(indexHtml);
    expect(meta, 'viewport meta tag must exist').not.toBeNull();
    expect(meta![1]).toContain('viewport-fit=cover');
    expect(meta![1]).toContain('width=device-width');
  });

  it('defines one --safe-* token per edge, each falling back to 0px', () => {
    const root = ruleBody(variablesCss, ':root');
    for (const edge of ['top', 'right', 'bottom', 'left']) {
      expect(declValues(root, `--safe-${edge}`)).toEqual([`env(safe-area-inset-${edge}, 0px)`]);
    }
  });

  it.each([
    ['ScrollNavButtons', scrollNavCss, '.navButtons', 'var(--spacing-xl)'],
    ['TranscriptSearchBar', searchBarCss, '.searchBar', 'var(--spacing-xl)'],
    ['ReadOnlyToast', toastCss, '.toast', 'var(--spacing-lg)'],
  ])('lifts the fixed-bottom %s above the bottom safe area', (_name, css, selector, spacing) => {
    expect(declValues(ruleBody(css, selector), 'bottom')).toEqual([
      `calc(${spacing} + var(--safe-bottom))`,
    ]);
  });

  it.each([
    ['ScrollNavButtons', scrollNavCss, '.navButtons'],
    ['TranscriptSearchBar', searchBarCss, '.searchBar'],
  ])('keeps the right-anchored %s clear of the landscape notch', (_name, css, selector) => {
    expect(declValues(ruleBody(css, selector), 'right')).toEqual([
      'calc(var(--spacing-xl) + var(--safe-right))',
    ]);
  });

  it('keeps the full-width narrow-screen TranscriptSearchBar clear of both side insets', () => {
    const narrow = /@media \(max-width: 768px\)\s*\{\s*\.searchBar\s*\{([^}]*)\}/.exec(searchBarCss);
    expect(narrow, 'narrow-screen .searchBar override must exist').not.toBeNull();
    expect(declValues(narrow![1]!, 'left')).toEqual(['calc(var(--spacing-md) + var(--safe-left))']);
    expect(declValues(narrow![1]!, 'right')).toEqual(['calc(var(--spacing-md) + var(--safe-right))']);
  });

  it('insets the app shell from the side and bottom safe areas', () => {
    expect(declValues(ruleBody(indexCss, '.app-container'), 'padding')).toEqual([
      '0 var(--safe-right) var(--safe-bottom) var(--safe-left)',
    ]);
  });

  it('pads the sticky header below the top safe area, including the narrow-screen override', () => {
    const paddings = [...headerCss.matchAll(/\.header\s*\{([^}]*)\}/g)].flatMap((m) =>
      declValues(m[1]!, 'padding'),
    );
    expect(paddings.length).toBeGreaterThanOrEqual(2);
    for (const p of paddings) expect(p.startsWith('var(--safe-top)')).toBe(true);
    expect(declValues(ruleBody(headerCss, '.header'), 'height')).toEqual([
      'calc(48px + var(--safe-top))',
    ]);
  });
});

describe('dynamic viewport units', () => {
  it('sizes html/body to the dynamic viewport with a 100% fallback first', () => {
    expect(declValues(ruleBody(indexCss, 'html, body'), 'height')).toEqual(['100%', '100dvh']);
  });

  it.each([
    ['Modal', modalCss],
    ['ShareDialog', shareDialogCss],
    ['SessionDetailPage', sessionDetailCss],
  ])('caps the %s modal at 90dvh after a 90vh fallback', (_name, css) => {
    expect(declValues(ruleBody(css, '.modal'), 'max-height')).toEqual(['90vh', '90dvh']);
  });

  it.each([
    ['ErrorBoundary', errorBoundaryCss],
    ['ServerError', serverErrorCss],
  ])('gives the %s page a 100dvh min-height after a 100vh fallback', (_name, css) => {
    const firstRule = /^[^{]*\{([^}]*)\}/m.exec(css.replace(/\/\*[\s\S]*?\*\//g, ''));
    expect(declValues(firstRule![1]!, 'min-height')).toEqual(['100vh', '100dvh']);
  });

  it('ruleBody rejects an unknown selector rather than matching nothing silently', () => {
    expect(() => ruleBody(indexCss, '.does-not-exist')).toThrow(/not found/);
  });
});
