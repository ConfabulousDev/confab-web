import type { Meta, StoryObj } from '@storybook/react-vite';
import type { TILWithSession } from '@/schemas/api';
import TILCard from '@/components/TILCard';
import { useColumnCount, distributeToColumns } from '@/hooks';
import { useMemo } from 'react';
import styles from './TILsPage.module.css';

const makeTIL = (id: number, overrides: Partial<TILWithSession> = {}): TILWithSession => ({
  id,
  title: `TIL #${id}: Sample learning`,
  summary: 'This is a sample summary for demonstration purposes.',
  session_id: `session-${id}`,
  message_uuid: `msg-${id}`,
  created_at: new Date(Date.now() - id * 60 * 60 * 1000).toISOString(),
  session_title: 'Dev session',
  git_repo: 'confab-web',
  git_branch: 'main',
  owner_email: 'jackie@confab.dev',
  is_owner: true,
  access_type: 'owner',
  ...overrides,
});

const sampleTILs: TILWithSession[] = [
  makeTIL(1, {
    title: 'PostgreSQL EXISTS subqueries need qualified column names',
    summary: 'When writing EXISTS (SELECT 1 FROM table WHERE table.session_id = id), PostgreSQL resolves bare "id" to the FROM table\'s own column. Always qualify.',
  }),
  makeTIL(2, {
    title: 'Use -short for quick tests',
    summary: 'Skips integration tests requiring Docker.',
    session_title: null,
    git_repo: null,
    git_branch: null,
  }),
  makeTIL(3, {
    title: 'Integration tests require Docker containers',
    summary: 'Use testutil.SetupTestEnvironment(t) which spins up containerized Postgres and MinIO. Sessions need total_lines > 0 AND (summary IS NOT NULL OR first_user_message IS NOT NULL) to be visible in the paginated list endpoint.',
    git_branch: 'feature/ci-containers',
  }),
  makeTIL(4, {
    title: 'CSS column-count vs flexbox masonry',
    summary: 'CSS column-count fills top-to-bottom per column. For left-to-right reading order, use JS distribution into flex columns.',
    is_owner: false,
    owner_email: 'teammate@confab.dev',
  }),
  makeTIL(5, {
    title: 'Go deadcode -test for whole-program analysis',
    summary: 'Catches dead call chains from main() and test entry points.',
    session_title: 'Backend maintenance',
  }),
  makeTIL(6, {
    title: 'Zod schemas for API validation',
    summary: 'Frontend uses Zod to parse and validate API responses at runtime. Schemas live in schemas/api.ts.',
    git_branch: 'feature/zod-schemas',
  }),
  makeTIL(7, {
    title: 'Always run full tests before presenting work',
    summary: 'The -short flag is only for quick iteration. Full integration tests with Docker are required for final verification.',
  }),
  makeTIL(8, {
    title: 'Storybook stories are required for all new components',
    summary: 'Stories live alongside components. Run npm run build-storybook to verify.',
    session_title: null,
  }),
  makeTIL(9, {
    title: 'Theme-aware CSS variables',
    summary: 'Use --color-bg-primary, --color-text-primary, etc. Avoid hardcoded colors. Test in both light and dark themes.',
    git_branch: 'feature/dark-mode',
  }),
  makeTIL(10, {
    title: 'Cursor-based pagination for TILs',
    summary: 'The TIL API uses cursor-based pagination with has_more and next_cursor fields. The useTILsFetch hook manages a cursor stack for prev/next navigation.',
  }),
];

/** Mock masonry grid for Storybook — renders TILCards in responsive columns. */
function MockMasonryGrid({ tils }: { tils: TILWithSession[] }) {
  const columnCount = useColumnCount();
  const columns = useMemo(
    () => distributeToColumns(tils, columnCount),
    [tils, columnCount],
  );

  return (
    <div style={{ display: 'flex', gap: 12, padding: 12 }}>
      {columns.map((colTils, colIndex) => (
        <div key={colIndex} style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 12, minWidth: 0 }}>
          {colTils.map((til) => (
            <TILCard
              key={til.id}
              til={til}
              onNavigate={() => console.log('navigate', til.id)}
              onDelete={() => console.log('delete', til.id)}
            />
          ))}
        </div>
      ))}
    </div>
  );
}

const meta: Meta = {
  title: 'Pages/TILsPage',
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj;

export const ManyCards: Story = {
  render: () => <MockMasonryGrid tils={sampleTILs} />,
};

export const FewCards: Story = {
  render: () => <MockMasonryGrid tils={sampleTILs.slice(0, 3)} />,
};

export const SingleCard: Story = {
  render: () => <MockMasonryGrid tils={sampleTILs.slice(0, 1)} />,
};

export const MixedOwnership: Story = {
  render: () => (
    <MockMasonryGrid
      tils={sampleTILs.map((til, i) =>
        i % 2 === 0
          ? { ...til, is_owner: false, owner_email: 'teammate@confab.dev' }
          : til,
      )}
    />
  ),
};

export const EmptyState: Story = {
  render: () => (
    <div className={styles.emptyState}>
      No TILs yet. Use <code>/til</code> in Claude Code to save learnings from your sessions.
    </div>
  ),
};
