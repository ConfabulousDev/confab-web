import type { Meta, StoryObj } from '@storybook/react-vite';

const meta: Meta = {
  title: 'Pages/TILsPage',
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj;

// TILsPage requires routing and API context which is hard to mock in Storybook.
// These stories serve as documentation placeholders.
// Visual testing is done via the dev server with mock data.

export const Documentation: Story = {
  render: () => (
    <div style={{ padding: '2rem', fontFamily: 'sans-serif', color: 'var(--color-text-primary, #333)' }}>
      <h2>TILs Page</h2>
      <p>The TILs page lists all TILs visible to the authenticated user.</p>
      <h3>Features</h3>
      <ul>
        <li>FilterChipsBar with owner, repo, branch, and search filters</li>
        <li>Cursor-based pagination</li>
        <li>Each TIL card shows title, summary, session context chips, and timestamp</li>
        <li>Click navigates to transcript position</li>
        <li>Inline delete confirmation (owner only)</li>
        <li>Empty state with CLI hint</li>
      </ul>
      <h3>Empty State</h3>
      <div style={{ padding: '2rem', textAlign: 'center', color: '#999', border: '1px dashed #ccc', borderRadius: '8px' }}>
        No TILs yet. Use <code>/til</code> in Claude Code to save learnings from your sessions.
      </div>
    </div>
  ),
};
