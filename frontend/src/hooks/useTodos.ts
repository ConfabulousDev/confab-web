import { useState, useEffect } from 'react';
import type { TodoItem, RunDetail } from '@/types';

interface TodoGroup {
  agent_id: string;
  items: TodoItem[];
}

interface UseTodosOptions {
  run: RunDetail;
  shareToken?: string;
  sessionId?: string;
}

interface UseTodosReturn {
  todos: TodoGroup[];
  loading: boolean;
}

/**
 * Extract agent ID from todo file path
 * Format: {sessionID}-agent-{agentID}.json
 */
function extractAgentID(filePath: string): string {
  const fileName = filePath.split('/').pop() ?? '';
  const match = fileName.match(/-agent-([^.]+)\.json$/);
  return match?.[1] ?? 'unknown';
}

/**
 * Hook to load todo lists from run files
 */
export function useTodos({ run, shareToken, sessionId }: UseTodosOptions): UseTodosReturn {
  const [todos, setTodos] = useState<TodoGroup[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const todoFiles = run.files.filter((f) => f.file_type === 'todo');
    if (todoFiles.length === 0) return;

    let cancelled = false;

    async function loadTodos() {
      setLoading(true);
      const loadedTodos: TodoGroup[] = [];

      for (const file of todoFiles) {
        if (cancelled) break;

        try {
          // Fetch todo file content from backend
          // Use shared endpoint if shareToken is provided
          const url =
            shareToken && sessionId
              ? `/api/v1/sessions/${sessionId}/shared/${shareToken}/files/${file.id}/content`
              : `/api/v1/runs/${run.id}/files/${file.id}/content`;
          const response = await fetch(url, {
            credentials: 'include',
          });

          if (!response.ok) continue;

          const content = await response.text();
          const items: TodoItem[] = JSON.parse(content);

          // Only add if there are actual todos
          if (items.length > 0) {
            loadedTodos.push({
              agent_id: extractAgentID(file.file_path),
              items,
            });
          }
        } catch (err) {
          console.error('Failed to load todo file:', file.file_path, err);
        }
      }

      if (!cancelled) {
        setTodos(loadedTodos);
        setLoading(false);
      }
    }

    loadTodos();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [run.id]);

  return { todos, loading };
}
