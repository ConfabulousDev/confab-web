// 0rcv: collapsible injected-context sections beneath a Cursor user prompt.
// Cursor user envelopes carry context blocks (user_rules, attached_files,
// manually_attached_skills, system_reminder, …) alongside the <user_query>
// prompt. nfbe parses them into `sections`; this component renders each one as a
// collapsed-by-default disclosure so the prompt reads cleanly while the audit
// context stays one click away.

import { describe, it, expect } from 'vitest';
import { render, fireEvent, within } from '@testing-library/react';
import CursorContextSections from './CursorContextSections';
import type { CursorUserSection } from './cursorCategories';

const skills: CursorUserSection = {
  tag: 'manually_attached_skills',
  label: 'Manually attached skills',
  content: 'Skill body: follow the workflow instructions.',
};
const rules: CursorUserSection = {
  tag: 'user_rules',
  label: 'User rules',
  content: 'Always prefer the latest stable versions.',
};

describe('CursorContextSections (0rcv)', () => {
  it('renders nothing when there are no sections', () => {
    const { container } = render(<CursorContextSections sections={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders one disclosure per section, labeled by the section label', () => {
    const { getByText } = render(<CursorContextSections sections={[skills, rules]} />);
    expect(getByText('Manually attached skills')).toBeTruthy();
    expect(getByText('User rules')).toBeTruthy();
  });

  it('collapses every section by default (content not visible until expanded)', () => {
    const { container } = render(<CursorContextSections sections={[skills, rules]} />);
    const details = container.querySelectorAll<HTMLDetailsElement>('details');
    expect(details).toHaveLength(2);
    for (const d of details) {
      expect(d.open).toBe(false);
    }
  });

  it('expands a section on toggle, revealing its preformatted content', () => {
    const { container, getByText } = render(<CursorContextSections sections={[skills]} />);
    const details = container.querySelector<HTMLDetailsElement>('details');
    expect(details?.open).toBe(false);
    const summary = container.querySelector('summary');
    fireEvent.click(summary!);
    expect(details?.open).toBe(true);
    expect(getByText(/Skill body: follow the workflow instructions\./)).toBeTruthy();
  });

  it('renders duplicate section types as separate disclosures in wire order', () => {
    const dupA: CursorUserSection = { tag: 'attached_files', label: 'Attached files', content: 'a.ts' };
    const dupB: CursorUserSection = { tag: 'attached_files', label: 'Attached files', content: 'b.ts' };
    const { container } = render(<CursorContextSections sections={[dupA, dupB]} />);
    const details = container.querySelectorAll<HTMLDetailsElement>('details');
    // One disclosure per parsed section, in order (no grouping in v1).
    expect(details).toHaveLength(2);
    fireEvent.click(details[0]!.querySelector('summary')!);
    fireEvent.click(details[1]!.querySelector('summary')!);
    expect(within(details[0]!).getByText(/a\.ts/)).toBeTruthy();
    expect(within(details[1]!).getByText(/b\.ts/)).toBeTruthy();
  });

  it('never renders a <user_query> tag inside a section body', () => {
    const sneaky: CursorUserSection = {
      tag: 'attached_files',
      label: 'Attached files',
      content: 'context with a stray <user_query>leak</user_query> inside',
    };
    const { container } = render(<CursorContextSections sections={[sneaky]} />);
    // Force-open so the body is in the DOM.
    fireEvent.click(container.querySelector('summary')!);
    // The literal text may appear (it is opaque content), but no real element.
    expect(container.querySelector('user_query')).toBeNull();
  });
});
