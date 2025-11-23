import type { TodoItem } from '@/types';
import styles from './TodoListDisplay.module.css';

interface TodoGroup {
  agent_id: string;
  items: TodoItem[];
}

interface TodoListDisplayProps {
  todos: TodoGroup[];
}

function TodoListDisplay({ todos }: TodoListDisplayProps) {
  if (todos.length === 0) return null;

  return (
    <div className={styles.todosSection}>
      <h4>Todo Lists ({todos.length})</h4>
      {todos.map((todoGroup, i) => (
        <div key={i} className={styles.todoGroup}>
          <h5>Agent: {todoGroup.agent_id}</h5>
          <div className={styles.todoList}>
            {todoGroup.items.map((item, j) => (
              <div key={j} className={`${styles.todoItem} ${styles[`status-${item.status}`]}`}>
                <span className={styles.todoStatusIcon}>
                  {item.status === 'completed' ? '✓' : item.status === 'in_progress' ? '⟳' : '○'}
                </span>
                <span className={styles.todoContent}>{item.content}</span>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

export default TodoListDisplay;
