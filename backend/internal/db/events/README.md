# events

Session event insertion for recording timestamped events with JSON payloads.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct definition and OpenTelemetry tracer |
| `events.go` | `InsertSessionEvent` -- inserts a row into `session_events` |

## Key API

- **`InsertSessionEvent(ctx, params)`** -- Inserts a session event with `session_id`, `event_type`, `event_timestamp`, and a `json.RawMessage` payload. This is a simple append-only write with no deduplication or upsert logic.

## How to Extend

1. **New event type**: No schema or code changes needed. Pass a new `event_type` string in `SessionEventParams`. The `event_type` column is a plain text field with no enum constraint.
2. **Querying events**: Add a `GetSessionEvents(ctx, sessionID, eventType)` method if event retrieval is needed. Currently events are write-only from this package's perspective.
3. **Batch insertion**: Add a multi-row INSERT variant if high-throughput event ingestion is needed.

## Invariants

- Events are append-only. There is no update or delete API.
- The `session_id` must reference an existing session (foreign key constraint in the database).
- `event_timestamp` is the time the event occurred (provided by the caller), not the database insertion time.

## Design Decisions

- **Minimal package**: This package has a single function. It exists as a separate sub-package to keep the DB layer's domain boundaries clean rather than attaching event insertion to the session package.
- **Raw JSON payload**: Events use `json.RawMessage` for the payload to avoid coupling the DB layer to specific event schemas. Interpretation of the payload is the caller's responsibility.

## Testing

- No dedicated test file; event insertion is tested through API-level integration tests.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for `SessionEventParams` type
- `go.opentelemetry.io/otel` -- Distributed tracing
