import styles from './SortableHeader.module.css';

interface SortableHeaderProps<T extends string> {
  column: T;
  label: string;
  currentColumn: T;
  direction: 'asc' | 'desc';
  onSort: (column: T) => void;
}

function SortableHeader<T extends string>({
  column,
  label,
  currentColumn,
  direction,
  onSort,
}: SortableHeaderProps<T>) {
  const isActive = currentColumn === column;

  return (
    <th className={styles.sortable} onClick={() => onSort(column)}>
      {label}
      {isActive && (
        <span className={styles.indicator}>
          {direction === 'asc' ? '↑' : '↓'}
        </span>
      )}
    </th>
  );
}

export default SortableHeader;
