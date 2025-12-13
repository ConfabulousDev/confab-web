import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import SortableHeader from './SortableHeader';

// Interactive wrapper for demonstrating sorting behavior
type Column = 'name' | 'date' | 'size';

function SortableHeaderDemo() {
  const [sortColumn, setSortColumn] = useState<Column>('name');
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc');

  const handleSort = (column: Column) => {
    if (column === sortColumn) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortDirection('asc');
    }
  };

  return (
    <table style={{ borderCollapse: 'collapse', width: '100%' }}>
      <thead>
        <tr>
          <SortableHeader
            column="name"
            label="Name"
            currentColumn={sortColumn}
            direction={sortDirection}
            onSort={handleSort}
          />
          <SortableHeader
            column="date"
            label="Date"
            currentColumn={sortColumn}
            direction={sortDirection}
            onSort={handleSort}
          />
          <SortableHeader
            column="size"
            label="Size"
            currentColumn={sortColumn}
            direction={sortDirection}
            onSort={handleSort}
          />
        </tr>
      </thead>
      <tbody>
        <tr>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>document.pdf</td>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>Jan 15, 2025</td>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>2.4 MB</td>
        </tr>
        <tr>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>image.png</td>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>Jan 10, 2025</td>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>1.1 MB</td>
        </tr>
        <tr>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>notes.txt</td>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>Jan 5, 2025</td>
          <td style={{ padding: '12px', borderBottom: '1px solid #eee' }}>12 KB</td>
        </tr>
      </tbody>
    </table>
  );
}

const meta: Meta<typeof SortableHeader> = {
  title: 'Components/SortableHeader',
  component: SortableHeader,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof SortableHeader>;

export const ActiveAscending: Story = {
  decorators: [
    (Story) => (
      <table>
        <thead>
          <tr>
            <Story />
          </tr>
        </thead>
      </table>
    ),
  ],
  args: {
    column: 'name',
    label: 'Name',
    currentColumn: 'name',
    direction: 'asc',
    onSort: () => {},
  },
};

export const ActiveDescending: Story = {
  decorators: [
    (Story) => (
      <table>
        <thead>
          <tr>
            <Story />
          </tr>
        </thead>
      </table>
    ),
  ],
  args: {
    column: 'name',
    label: 'Name',
    currentColumn: 'name',
    direction: 'desc',
    onSort: () => {},
  },
};

export const Inactive: Story = {
  decorators: [
    (Story) => (
      <table>
        <thead>
          <tr>
            <Story />
          </tr>
        </thead>
      </table>
    ),
  ],
  args: {
    column: 'name',
    label: 'Name',
    currentColumn: 'date',
    direction: 'asc',
    onSort: () => {},
  },
};

export const InteractiveTable: Story = {
  render: () => <SortableHeaderDemo />,
};
