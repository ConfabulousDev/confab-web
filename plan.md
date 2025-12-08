# Plan: Remove Todo File Support

## Goal

Remove all code related to loading/displaying todo FILES from confab-web. This mirrors the change made in confab CLI (commit d4cfbc0) which removed todo file sync support.

**Keep**: Message types like `TodoItem` that are used for displaying TodoWrite tool results in transcripts.

**Remove**: All code that reads/displays todo files (the separate `.json` files from `~/.claude/todos/`).

## Rationale (from confab CLI commit)

> TODO files (~/.claude/todos/) are transient state that gets cleared when tasks complete, leaving empty [] files. Not useful for transcript history.

## Current State

### Files to Remove/Modify

| File | Action | Description |
|------|--------|-------------|
| `frontend/src/hooks/useTodos.ts` | DELETE | Hook that loads todo files from session |
| `frontend/src/hooks/index.ts` | MODIFY | Remove `useTodos` export |
| `frontend/src/components/TodoListDisplay.tsx` | DELETE | Component for rendering todo lists (unused) |
| `frontend/src/components/TodoListDisplay.module.css` | DELETE | Styles for TodoListDisplay (unused) |
| `frontend/src/components/SessionCard.tsx` | MODIFY | Remove todo file loading and display |
| `frontend/src/components/SessionCard.module.css` | MODIFY | Remove `.fileType.todo` and `.todosSection` styles |
| `frontend/src/types/index.ts` | KEEP | `TodoItem` type stays (used for transcript messages) |
| `backend/internal/models/models.go` | KEEP | Comment mentions "todo" but no code change needed |
| `backend/internal/api/sync.go` | KEEP | Comment mentions "agent/todo files" but logic is generic |

## Implementation Steps

### 1. Delete Frontend Files

Delete these files entirely:
- `frontend/src/hooks/useTodos.ts`
- `frontend/src/components/TodoListDisplay.tsx`
- `frontend/src/components/TodoListDisplay.module.css`

### 2. Update `frontend/src/hooks/index.ts`

Remove the `useTodos` export:
```diff
- export { useTodos } from './useTodos';
```

### 3. Update `frontend/src/components/SessionCard.tsx`

Remove:
- `useTodos` import
- `useTodos({ session, shareToken })` call
- The entire `{todos.length > 0 && ...}` JSX block (lines 140-158)

### 4. Update `frontend/src/components/SessionCard.module.css`

Remove unused todo-related styles:
- `.fileType.todo` (lines 195-198)
- `.todosSection` and all related styles (lines 216-305)

## What to Keep

1. **`TodoItem` type** in `frontend/src/types/index.ts` - Used for displaying TodoWrite tool results inline in transcripts
2. **Backend file type handling** - The sync API accepts any file_type string; no backend changes needed
3. **`docs/claude-code-data-directory.md`** - Documentation reference (can be updated separately if needed)

## Testing

```bash
cd frontend && npm run build && npm run lint && npm test
```

## Notes

- This is frontend-only cleanup
- The backend sync API is generic and will continue to work if todo files are ever synced (they just won't be displayed specially)
- The `TodoItem` type remains because it's used for rendering todo items from `todowrite` tool messages in transcripts
