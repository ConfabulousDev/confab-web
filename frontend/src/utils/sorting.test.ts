import { describe, it, expect } from 'vitest';
import { sortData } from './sorting';

interface Row {
  name: string;
  count: number;
  created_at: string | null;
}

const rows: Row[] = [
  { name: 'banana', count: 3, created_at: '2025-01-03T00:00:00Z' },
  { name: 'apple', count: 10, created_at: '2025-01-01T00:00:00Z' },
  { name: 'cherry', count: 1, created_at: '2025-01-02T00:00:00Z' },
];

describe('sortData', () => {
  it('sorts strings ascending', () => {
    const result = sortData({ data: rows, sortBy: 'name', direction: 'asc' });
    expect(result.map((r) => r.name)).toEqual(['apple', 'banana', 'cherry']);
  });

  it('sorts strings descending', () => {
    const result = sortData({ data: rows, sortBy: 'name', direction: 'desc' });
    expect(result.map((r) => r.name)).toEqual(['cherry', 'banana', 'apple']);
  });

  it('sorts numbers ascending', () => {
    const result = sortData({ data: rows, sortBy: 'count', direction: 'asc' });
    expect(result.map((r) => r.count)).toEqual([1, 3, 10]);
  });

  it('sorts numbers descending', () => {
    const result = sortData({ data: rows, sortBy: 'count', direction: 'desc' });
    expect(result.map((r) => r.count)).toEqual([10, 3, 1]);
  });

  it('treats parseable ISO strings as dates and sorts chronologically', () => {
    const result = sortData({ data: rows, sortBy: 'created_at', direction: 'asc' });
    expect(result.map((r) => r.created_at)).toEqual([
      '2025-01-01T00:00:00Z',
      '2025-01-02T00:00:00Z',
      '2025-01-03T00:00:00Z',
    ]);
  });

  it('places nulls at end regardless of direction', () => {
    const data: Row[] = [
      { name: 'a', count: 1, created_at: null },
      { name: 'b', count: 2, created_at: '2025-01-01T00:00:00Z' },
      { name: 'c', count: 3, created_at: null },
    ];
    const asc = sortData({ data, sortBy: 'created_at', direction: 'asc' });
    expect(asc[0]?.name).toBe('b');
    expect(asc.slice(1).map((r) => r.created_at)).toEqual([null, null]);

    const desc = sortData({ data, sortBy: 'created_at', direction: 'desc' });
    expect(desc[0]?.name).toBe('b');
    expect(desc.slice(1).map((r) => r.created_at)).toEqual([null, null]);
  });

  it('returns 0 (stable) when both values are null', () => {
    const data: Row[] = [
      { name: 'a', count: 1, created_at: null },
      { name: 'b', count: 2, created_at: null },
    ];
    const result = sortData({ data, sortBy: 'created_at', direction: 'asc' });
    expect(result.map((r) => r.name)).toEqual(['a', 'b']);
  });

  it('applies filter predicate before sorting', () => {
    const result = sortData({
      data: rows,
      sortBy: 'name',
      direction: 'asc',
      filter: (r) => r.count > 1,
    });
    expect(result.map((r) => r.name)).toEqual(['apple', 'banana']);
  });

  it('does not mutate the input array', () => {
    const data: Row[] = [...rows];
    const before = data.map((r) => r.name);
    sortData({ data, sortBy: 'name', direction: 'desc' });
    expect(data.map((r) => r.name)).toEqual(before);
  });
});
