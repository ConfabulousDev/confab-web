import { describe, it, expect } from 'vitest';
import { renderLegalMarkdown, renderMarkdownToHtml } from './markdown';

describe('renderLegalMarkdown', () => {
  it('gives every heading a slug id derived from its text', () => {
    const html = renderLegalMarkdown('## 1. WHAT INFORMATION DO WE COLLECT?\n\n### Personal information you disclose to us');
    expect(html).toContain('<h2 id="1-what-information-do-we-collect">');
    expect(html).toContain('<h3 id="personal-information-you-disclose-to-us">');
  });

  it('suffixes duplicate heading ids with -2, -3', () => {
    const html = renderLegalMarkdown('## In Short\n\n## In Short\n\n## In Short');
    expect(html).toContain('id="in-short"');
    expect(html).toContain('id="in-short-2"');
    expect(html).toContain('id="in-short-3"');
  });

  it('restarts duplicate numbering on each call', () => {
    renderLegalMarkdown('## Same');
    expect(renderLegalMarkdown('## Same')).toContain('id="same"');
  });

  it('builds the slug from plain text, not inline markup', () => {
    const html = renderLegalMarkdown('## Rights &amp; **choices** [here](/x)');
    expect(html).toContain('id="rights-choices-here"');
  });

  it('renders GFM tables', () => {
    const html = renderLegalMarkdown('| Name | Kind |\n|---|---|\n| confab_session | cookie |');
    expect(html).toContain('<table>');
    expect(html).toContain('<th>Name</th>');
    expect(html).toContain('<td>confab_session</td>');
  });

  it('does not turn a single newline into <br>', () => {
    const html = renderLegalMarkdown('line one\nline two');
    expect(html).not.toContain('<br');
  });

  it('strips <script> tags', () => {
    const html = renderLegalMarkdown('hello\n\n<script>alert(1)</script>');
    expect(html).not.toContain('<script');
    expect(html).toContain('hello');
  });

  it('returns an empty string for empty input', () => {
    expect(renderLegalMarkdown('')).toBe('');
  });
});

describe('renderMarkdownToHtml (transcript renderer) is unaffected', () => {
  it('still turns a single newline into <br> after renderLegalMarkdown runs', () => {
    renderLegalMarkdown('## Heading\n\nline one\nline two');
    expect(renderMarkdownToHtml('line one\nline two')).toContain('<br');
  });

  it('still emits headings without ids', () => {
    renderLegalMarkdown('## Heading');
    expect(renderMarkdownToHtml('## Heading')).toContain('<h2>Heading</h2>');
  });
});
