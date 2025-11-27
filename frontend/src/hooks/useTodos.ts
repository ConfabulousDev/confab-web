import { useState, useEffect } from 'react';
import type { TodoItem, SessionDetail } from '@/types';
import { syncFilesAPI } from '@/services/api';

interface TodoGroup {
  agent_id: string;
  items: TodoItem[];
}

interface UseTodosOptions {
  session: SessionDetail;
  shareToken?: string;
}

interface UseTodosReturn {
  todos: TodoGroup[];
  loading: boolean;
}

/**
 * Extract agent ID from todo file name
 * Format: {sessionID}-agent-{agentID}.json -> agentID
 */
function extractAgentID(fileName: string): string {
  const match = fileName.match(/-agent-([^.]+)\.json$/);
  return match?.[1] ?? 'unknown';
}

/**
 * Hook to load todo lists from session files
 */
export function useTodos({ session, shareToken }: UseTodosOptions): UseTodosReturn {
  const [todos, setTodos] = useState<TodoGroup[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const todoFiles = session.files.filter((f) => f.file_type === 'todo');
    if (todoFiles.length === 0) return;

    let cancelled = false;

    async function loadTodos() {
      setLoading(true);
      const loadedTodos: TodoGroup[] = [];

      for (const file of todoFiles) {
        if (cancelled) break;

        try {
          // Fetch todo file content from backend using sync API
          const content = shareToken
            ? await syncFilesAPI.getSharedContent(session.id, shareToken, file.file_name)
            : await syncFilesAPI.getContent(session.id, file.file_name);

          const items: TodoItem[] = JSON.parse(content);

          // Only add if there are actual todos
          if (items.length > 0) {
            loadedTodos.push({
              agent_id: extractAgentID(file.file_name),
              items,
            });
          }
        } catch (err) {
          console.error('Failed to load todo file:', file.file_name, err);
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
  }, [session.id]);

  return { todos, loading };
}
