// et0r: subtab strip under the Transcript tab (Main + one tab per subagent).

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TranscriptThreadRef } from '@/providers/types';
import TranscriptThreadTabs from './TranscriptThreadTabs';

const threads: TranscriptThreadRef[] = [
  { id: 'a1', fileName: 'agent-a1.jsonl', label: 'Explore the codebase', parentThreadId: null, launchTargetId: 'r1' },
  { id: 'a2', fileName: 'agent-a2.jsonl', label: 'Write tests', parentThreadId: null, launchTargetId: 'r2' },
];

describe('TranscriptThreadTabs', () => {
  it('renders a tablist with Main pinned first, then threads in order', () => {
    render(<TranscriptThreadTabs threads={threads} activeThreadId={null} onSelect={() => {}} />);
    expect(screen.getByRole('tablist')).toBeInTheDocument();
    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((t) => t.textContent)).toEqual(['Main', 'Explore the codebase', 'Write tests']);
  });

  it('marks the active tab with aria-selected', () => {
    render(<TranscriptThreadTabs threads={threads} activeThreadId="a2" onSelect={() => {}} />);
    expect(screen.getByRole('tab', { name: 'Write tests' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Main' })).toHaveAttribute('aria-selected', 'false');
  });

  it('marks Main active when no thread is selected', () => {
    render(<TranscriptThreadTabs threads={threads} activeThreadId={null} onSelect={() => {}} />);
    expect(screen.getByRole('tab', { name: 'Main' })).toHaveAttribute('aria-selected', 'true');
  });

  it('puts the full label in the title attribute (labels truncate visually)', () => {
    render(<TranscriptThreadTabs threads={threads} activeThreadId={null} onSelect={() => {}} />);
    expect(screen.getByRole('tab', { name: 'Explore the codebase' })).toHaveAttribute('title', 'Explore the codebase');
  });

  it('calls onSelect with the thread id, and null for Main', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TranscriptThreadTabs threads={threads} activeThreadId="a1" onSelect={onSelect} />);
    await user.click(screen.getByRole('tab', { name: 'Write tests' }));
    await user.click(screen.getByRole('tab', { name: 'Main' }));
    expect(onSelect).toHaveBeenNthCalledWith(1, 'a2');
    expect(onSelect).toHaveBeenNthCalledWith(2, null);
  });

  it('renders the back link and calls its handler', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <TranscriptThreadTabs
        threads={threads}
        activeThreadId="a1"
        onSelect={() => {}}
        backLink={{ label: 'Main', onClick }}
      />,
    );
    await user.click(screen.getByRole('button', { name: '← Launched from Main' }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('omits the back link when not provided', () => {
    render(<TranscriptThreadTabs threads={threads} activeThreadId="a1" onSelect={() => {}} />);
    expect(screen.queryByRole('button', { name: /Launched from/ })).not.toBeInTheDocument();
  });
});
