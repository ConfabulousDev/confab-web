// kpqz: locks the contract for inter-agent (parent <-> subagent) message
// rows — headed `author → recipient`, body through the shared markdown / JSON
// pipeline, and never styled or labelled as a user prompt.

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import CodexAgentMessage from './CodexAgentMessage';
import type { CodexAgentMessageItem } from '@/types/codexRenderItem';

function agentMessage(overrides: Partial<CodexAgentMessageItem> = {}): CodexAgentMessageItem {
  return {
    kind: 'agent_message',
    lineId: '0',
    timestamp: '2026-11-25T10:00:00Z',
    author: '/root/worker',
    recipient: '/root',
    text: 'subagent reply',
    ...overrides,
  };
}

describe('CodexAgentMessage', () => {
  it('heads the row with author → recipient', () => {
    const { getByText } = render(<CodexAgentMessage item={agentMessage()} />);
    expect(getByText('/root/worker → /root')).toBeInTheDocument();
  });

  it('renders the body through the markdown pipeline', () => {
    const { container } = render(
      <CodexAgentMessage item={agentMessage({ text: 'the **fix** landed' })} />,
    );
    expect(container.querySelector('strong')?.textContent).toBe('fix');
  });

  it('renders JSON-shaped text as syntax-highlighted JSON', () => {
    const { container } = render(
      <CodexAgentMessage item={agentMessage({ text: '{"status":"done"}' })} />,
    );
    expect(container.querySelector('code[class*="language-json"]')).not.toBeNull();
  });

  it('tags the row as agent_message, not user', () => {
    const { container } = render(<CodexAgentMessage item={agentMessage()} />);
    const row = container.firstElementChild;
    expect(row).toHaveAttribute('data-kind', 'agent_message');
    expect(row).toHaveClass(/agentMessage/);
    expect(row).not.toHaveClass(/\buser\b/);
  });

  it('renders row-actions with the agent message kind label when sessionId is provided', () => {
    const { getByLabelText } = render(
      <CodexAgentMessage item={agentMessage()} sessionId="abc" onSkipToNext={() => {}} />,
    );
    expect(getByLabelText(/copy link/i)).toBeInTheDocument();
    expect(getByLabelText(/next agent message/i)).toBeInTheDocument();
  });

  it('omits row-actions when sessionId is absent', () => {
    const { queryByLabelText } = render(<CodexAgentMessage item={agentMessage()} />);
    expect(queryByLabelText(/copy link/i)).toBeNull();
  });

  it('applies selection, deep-link and search-match classes', () => {
    const { container } = render(
      <CodexAgentMessage item={agentMessage()} isSelected isDeepLinkTarget isCurrentSearchMatch />,
    );
    const row = container.firstElementChild;
    expect(row).toHaveClass(/selected/);
    expect(row).toHaveClass(/deepLinkTarget/);
    expect(row).toHaveClass(/searchMatch/);
  });

  it('renders attached images', () => {
    const src = 'data:image/png;base64,AAAA';
    const { container } = render(
      <CodexAgentMessage item={agentMessage({ text: '', images: [src] })} />,
    );
    expect(container.querySelector(`img[src="${src}"]`)).not.toBeNull();
  });

  it('renders no image gallery for a text-only message', () => {
    const { container } = render(<CodexAgentMessage item={agentMessage()} />);
    expect(container.querySelector('img')).toBeNull();
  });
});
