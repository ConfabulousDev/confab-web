import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import TranscriptPaneStatus from './TranscriptPaneStatus';

describe('TranscriptPaneStatus', () => {
  it('shows the loading placeholder', () => {
    render(<TranscriptPaneStatus loading error={null} />);
    expect(screen.getByText('Loading transcript...')).toBeInTheDocument();
  });

  it('shows the error placeholder', () => {
    render(<TranscriptPaneStatus loading={false} error="Boom" />);
    expect(screen.getByText('Boom')).toBeInTheDocument();
  });

  // et0r D7: a subagent file that isn't uploaded yet is a retrying state, not an error.
  it('shows a retrying not-synced state instead of the error when notSynced is set', () => {
    render(<TranscriptPaneStatus loading={false} error="File not found" notSynced />);
    expect(screen.getByText('Subagent transcript not synced yet — retrying')).toBeInTheDocument();
    expect(screen.queryByText('File not found')).not.toBeInTheDocument();
  });

  it('renders nothing when idle', () => {
    const { container } = render(<TranscriptPaneStatus loading={false} error={null} />);
    expect(container).toBeEmptyDOMElement();
  });
});
