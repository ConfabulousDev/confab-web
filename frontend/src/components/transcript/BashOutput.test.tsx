import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import BashOutput from './BashOutput';

describe('BashOutput', () => {
  it('renders plain stdout with no metadata badges', () => {
    const { container, queryByText } = render(<BashOutput output="file.txt\nfile2.txt" />);
    expect(container.textContent).toContain('file.txt');
    expect(queryByText(/interrupted/i)).toBeNull();
  });

  it('shows an "interrupted" badge when interrupted is true', () => {
    const { getByText } = render(<BashOutput output="partial" interrupted />);
    expect(getByText(/interrupted/i)).toBeInTheDocument();
  });

  it('does not show the interrupted badge when interrupted is false', () => {
    const { queryByText } = render(<BashOutput output="done" interrupted={false} />);
    expect(queryByText(/interrupted/i)).toBeNull();
  });

  it('renders a persisted-output footer with humanized size and the path', () => {
    const { container } = render(
      <BashOutput
        output="...truncated preview..."
        persistedOutputPath="/Users/x/.claude/projects/p/tool-results/abc.txt"
        persistedOutputSize={44276}
      />
    );
    // 44276 bytes ≈ 43.2 KB — assert the unit and the raw path text appear.
    expect(container.textContent).toMatch(/KB/);
    expect(container.textContent).toContain('/Users/x/.claude/projects/p/tool-results/abc.txt');
    // Path is plain text, never a link.
    expect(container.querySelector('a')).toBeNull();
  });

  it('shows returnCodeInterpretation near the exit code area when present', () => {
    const { getByText } = render(
      <BashOutput output="" exitCode={1} returnCodeInterpretation="No matches found" />
    );
    expect(getByText('No matches found')).toBeInTheDocument();
  });

  it('shows a no-output hint when noOutputExpected is true and stdout is empty', () => {
    const { container } = render(<BashOutput output="" noOutputExpected />);
    expect(container.textContent).toMatch(/no output|background/i);
  });

  it('labels output as an image and suppresses text rendering when isImage is true', () => {
    const { container, getByText } = render(
      <BashOutput output="<binary image bytes>" isImage />
    );
    expect(getByText(/image/i)).toBeInTheDocument();
    // Raw image bytes must not be dumped as text.
    expect(container.textContent).not.toContain('<binary image bytes>');
  });

  it('still renders the exit code footer for a non-zero exit code', () => {
    const { container } = render(<BashOutput output="boom" exitCode={126} />);
    expect(container.textContent).toContain('126');
  });
});
