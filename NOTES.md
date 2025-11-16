# Confab Development Notes

## TODO

- Add log rotation support
- Use SQLite to track upload state (which files/sessions uploaded, timestamps, errors)

## Recent Changes

- **Runs tracking**: Added `runs` table to track each session exit separately. Supports resumed sessions by creating a new run entry each time. Schema: `sessions` (session_id, first_seen), `runs` (id, session_id, transcript_path, cwd, reason, end_timestamp), `files` (id, run_id, file_path, file_type, size_bytes). Enables analytics on session resumption patterns.
